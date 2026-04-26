package services

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

type DockerService struct{}

func NewDockerService() *DockerService {
	return &DockerService{}
}

// Run starts a new container from imageTag, mapping containerPort to hostPort.
// Returns the container ID.
func (d *DockerService) Run(ctx context.Context, imageTag, containerName string, hostPort, containerPort int) (string, error) {
	portMapping := fmt.Sprintf("%d:%d", hostPort, containerPort)
	out, err := d.exec(ctx,
		"docker", "run", "-d",
		"--name", containerName,
		"-p", portMapping,
		"--restart", "unless-stopped",
		imageTag,
	)
	if err != nil {
		return "", fmt.Errorf("docker run: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// Stop stops and removes a container by name (idempotent — ignores not-found).
func (d *DockerService) Stop(ctx context.Context, containerName string) error {
	// stop (ignore error — container may already be stopped)
	_, _ = d.exec(ctx, "docker", "stop", containerName)
	_, err := d.exec(ctx, "docker", "rm", "-f", containerName)
	if err != nil && !strings.Contains(err.Error(), "No such container") {
		return fmt.Errorf("docker rm: %w", err)
	}
	return nil
}

func (d *DockerService) HealthCheck(ctx context.Context, url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return fmt.Errorf("health check timeout after %s", timeout)
}

func (d *DockerService) ImageExists(ctx context.Context, imageTag string) bool {
	out, err := d.exec(ctx, "docker", "images", "-q", imageTag)
	return err == nil && strings.TrimSpace(out) != ""
}

func (d *DockerService) exec(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
