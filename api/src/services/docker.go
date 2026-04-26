package services

import (
	"context"
	"fmt"
	"net/http"
	"os"
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
	args := []string{
		"run", "-d",
		"--name", containerName,
		"-p", portMapping,
		"--network", "deployment-pipeline_default",
		"--restart", "unless-stopped",
	}

	// Pass DB environment variables if they exist
	dbEnvVars := []string{"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME"}
	for _, env := range dbEnvVars {
		if val := os.Getenv(env); val != "" {
			args = append(args, "-e", fmt.Sprintf("%s=%s", env, val))
		}
	}

	// Fix Frontend API URL to point to the Caddy-proxied path
	// We use the absolute path /app/api which Caddy handles
	args = append(args, "-e", "REACT_APP_API_URL=/app/api")
	args = append(args, "-e", "API_URL=/app/api")

	args = append(args, imageTag)

	out, err := d.exec(ctx, "docker", args...)
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

// CleanupPort finds and removes any container using the specified host port.
func (d *DockerService) CleanupPort(ctx context.Context, port int) error {
	// Find container ID using this port
	format := `{{.ID}}`
	filter := fmt.Sprintf("publish=%d", port)
	out, err := d.exec(ctx, "docker", "ps", "-aq", "--filter", filter, "--format", format)
	if err != nil || strings.TrimSpace(out) == "" {
		return nil
	}

	ids := strings.Fields(out)
	for _, id := range ids {
		_, _ = d.exec(ctx, "docker", "rm", "-f", id)
	}
	return nil
}

func (d *DockerService) exec(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
