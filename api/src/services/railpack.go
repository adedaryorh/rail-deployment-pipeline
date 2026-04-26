package services

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

type RailpackService struct {
	cacheDir string
}

func NewRailpackService() *RailpackService {
	cacheDir := os.Getenv("RAILPACK_CACHE_DIR")
	if cacheDir == "" {
		cacheDir = "/tmp/railpack-cache"
	}
	return &RailpackService{cacheDir: cacheDir}
}

// Build runs `railpack build` on the given source directory and returns a
// combined stdout/stderr reader plus a done channel that closes when the
// process exits (err may be non-nil on that channel).
func (s *RailpackService) Build(ctx context.Context, srcDir, imageTag string) (io.Reader, <-chan error, error) {
	if err := os.MkdirAll(s.cacheDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("railpack: mkdir cache: %w", err)
	}

	cmd := exec.CommandContext(ctx,
		"railpack", "build",
		"--name", imageTag,
		srcDir,
	)
	cmd.Env = append(os.Environ(),
		"RAILPACK_CACHE_DIR="+s.cacheDir,
	)

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		pw.Close()
		return nil, nil, fmt.Errorf("railpack: start: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		pw.CloseWithError(err) // signals EOF or error to reader
		done <- err
	}()

	return pr, done, nil
}
