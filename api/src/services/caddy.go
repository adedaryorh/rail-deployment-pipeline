package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/template"
)

const caddyfileTemplate = `
:80 {
    handle /api/* {
        reverse_proxy api:8080
    }
    handle {
        reverse_proxy frontend:5173
    }
}
`

// caddyUpstreamTemplate is used when we need to point traffic to a specific
// app container port after a zero-downtime swap.
const appUpstreamTemplate = `
:80 {
    handle /api/* {
        reverse_proxy api:8080
    }
    handle_path /app/* {
        reverse_proxy host.docker.internal:{{.Port}}
    }
    handle /static/* {
        reverse_proxy host.docker.internal:{{.Port}}
    }
    handle {
        reverse_proxy frontend:5173
    }
}
`

type CaddyService struct {
	adminURL string
	client   *http.Client
}

func NewCaddyService(adminURL string) *CaddyService {
	if adminURL == "" {
		adminURL = "http://localhost:2019"
	}
	return &CaddyService{
		adminURL: adminURL,
		client:   &http.Client{},
	}
}

// posting new  Caddyfile config to the Caddy admin API.
func (c *CaddyService) Reload(ctx context.Context, caddyfileContent string) error {
	url := c.adminURL + "/load"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url,
		bytes.NewBufferString(caddyfileContent))
	if err != nil {
		return fmt.Errorf("caddy reload: build request: %w", err)
	}
	req.Header.Set("Content-Type", "text/caddyfile")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("caddy reload: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("caddy reload: status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// reloads Caddy pointing /app/* to the new container port.
func (c *CaddyService) SwapAppPort(ctx context.Context, port int) error {
	tmpl, err := template.New("caddy").Parse(appUpstreamTemplate)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct{ Port int }{Port: port}); err != nil {
		return err
	}
	return c.Reload(ctx, buf.String())
}

func (c *CaddyService) ReloadFromFile(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("caddy: read file: %w", err)
	}
	return c.Reload(ctx, string(data))
}
