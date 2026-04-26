package controllers

import (
	"bufio"
	"context"
	"fmt"
	"time"

	"github.com/yourorg/deployment-pipeline/api/src/api/repo"
	"github.com/yourorg/deployment-pipeline/api/src/services"
)

const (
	appSrcDir       = "/workspace/sample-app"
	appHostPort     = 3000
	appInternalPort = 3000
	healthURL       = "http://host.docker.internal:3000/health"
	healthTimeout   = 60 * time.Second
)

type DeployController struct {
	repo        *repo.DeployRepo
	railpackSvc *services.RailpackService
	dockerSvc   *services.DockerService
	caddySvc    *services.CaddyService
}

func NewDeployController(r *repo.DeployRepo, rp *services.RailpackService, d *services.DockerService, c *services.CaddyService,
) *DeployController {
	return &DeployController{repo: r, railpackSvc: rp, dockerSvc: d, caddySvc: c}
}

// starts a deploy asynchronously N returns the new deploy record.
func (dc *DeployController) Run(ctx context.Context) *repo.Deploy {
	d := dc.repo.Create()
	go dc.execute(d.ID)
	return d
}

func (dc *DeployController) execute(deployID string) {
	ctx := context.Background()

	imageTag := fmt.Sprintf("sample-app:%s", deployID[:8])
	containerName := fmt.Sprintf("sample-app-%s", deployID[:8])

	dc.repo.UpdateMeta(deployID, imageTag, containerName, appHostPort)
	dc.log(deployID, fmt.Sprintf("==> Starting deploy %s", deployID))

	// transition: pending → building
	if err := dc.repo.Transition(deployID, repo.StateBuilding); err != nil {
		dc.fail(deployID, err.Error())
		return
	}
	dc.log(deployID, "==> Building image with Railpack…")

	reader, done, err := dc.railpackSvc.Build(ctx, appSrcDir, imageTag)
	if err != nil {
		dc.fail(deployID, fmt.Sprintf("build start: %v", err))
		return
	}

	// stream build output → log buffer
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		dc.log(deployID, scanner.Text())
	}

	if buildErr := <-done; buildErr != nil {
		dc.fail(deployID, fmt.Sprintf("build failed: %v", buildErr))
		return
	}
	dc.log(deployID, "==> Build complete.")

	// Stop previous container if any
	prev := dc.repo.Previous()
	if prev != nil && prev.ContainerName != "" {
		dc.log(deployID, fmt.Sprintf("==> Stopping previous container %s…", prev.ContainerName))
		if err := dc.dockerSvc.Stop(ctx, prev.ContainerName); err != nil {
			dc.log(deployID, fmt.Sprintf("warn: stop previous: %v", err))
		}
	}

	dc.log(deployID, fmt.Sprintf("==> Starting container %s on port %d…", containerName, appHostPort))
	cid, err := dc.dockerSvc.Run(ctx, imageTag, containerName, appHostPort, appInternalPort)
	if err != nil {
		dc.fail(deployID, fmt.Sprintf("docker run: %v", err))
		return
	}
	dc.log(deployID, fmt.Sprintf("==> Container started: %s", cid))

	dc.log(deployID, "==> Waiting for health check…")
	if err := dc.dockerSvc.HealthCheck(ctx, healthURL, healthTimeout); err != nil {
		dc.fail(deployID, fmt.Sprintf("health check: %v", err))
		return
	}
	dc.log(deployID, "==> Health check passed.")

	// Zero-downtime Caddy reload
	dc.log(deployID, "==> Reloading Caddy…")
	if err := dc.caddySvc.SwapAppPort(ctx, appHostPort); err != nil {
		dc.log(deployID, fmt.Sprintf("warn: caddy reload: %v", err))
	} else {
		dc.log(deployID, "==> Caddy reloaded.")
	}

	// transition: building → running
	if err := dc.repo.Transition(deployID, repo.StateRunning); err != nil {
		dc.fail(deployID, err.Error())
		return
	}
	dc.log(deployID, "==> Deploy complete ✓")
}

// stops the current running container and restarts the previous image.
func (dc *DeployController) Rollback(ctx context.Context, deployID string) error {
	current, ok := dc.repo.Get(deployID)
	if !ok {
		return fmt.Errorf("deploy %s not found", deployID)
	}
	if current.State != repo.StateRunning {
		return fmt.Errorf("can only rollback a running deploy (state=%s)", current.State)
	}

	prev := dc.repo.Previous()
	if prev == nil || prev.ImageTag == "" {
		return fmt.Errorf("no previous deploy to rollback to")
	}

	dc.log(deployID, fmt.Sprintf("==> Rolling back to %s…", prev.ImageTag))

	if err := dc.dockerSvc.Stop(ctx, current.ContainerName); err != nil {
		dc.log(deployID, fmt.Sprintf("warn: stop current: %v", err))
	}

	// Restart previous
	cid, err := dc.dockerSvc.Run(ctx, prev.ImageTag, current.ContainerName+"-rollback", appHostPort, appInternalPort)
	if err != nil {
		dc.fail(deployID, fmt.Sprintf("rollback run: %v", err))
		return err
	}
	dc.log(deployID, fmt.Sprintf("==> Rollback container started: %s", cid))

	if err := dc.repo.Transition(deployID, repo.StateRolledBack); err != nil {
		return err
	}
	dc.log(deployID, "==> Rollback complete ✓")
	return nil
}

func (dc *DeployController) log(id, line string) {
	dc.repo.AppendLog(id, line)
}

func (dc *DeployController) fail(id, msg string) {
	dc.log(id, "==> ERROR: "+msg)
	dc.repo.SetError(id, msg)
}
