package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourorg/deployment-pipeline/api/src/api/controllers"
	"github.com/yourorg/deployment-pipeline/api/src/api/repo"
)

type LogsHandler struct {
	ctrl *controllers.LogsController
	repo *repo.DeployRepo
}

func NewLogsHandler(r *repo.DeployRepo) *LogsHandler {
	return &LogsHandler{
		ctrl: controllers.NewLogsController(r),
		repo: r,
	}
}

// GET /api/deploys/:id/logs  — SSE endpoint
// On connect: replays last N lines from ring buffer, then streams live.
func (h *LogsHandler) Stream(c *gin.Context) {
	id := c.Param("id")
	_, ok := h.repo.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "deploy not found"})
		return
	}

	// Subscribe before writing headers so we don't miss lines
	replay, liveCh, unsub := h.ctrl.Subscribe(id)
	defer unsub()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	w := c.Writer
	flusher, ok := w.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming unsupported"})
		return
	}

	// Helper writing to an SSE event
	send := func(line string) {
		fmt.Fprintf(w, "data: %s\n\n", line)
		flusher.Flush()
	}
	for _, line := range replay {
		send(line)
	}

	// Stream live lines until client disconnects or deploy is done
	clientGone := c.Request.Context().Done()
	for {
		select {
		case <-clientGone:
			return
		case line, open := <-liveCh:
			if !open {
				send("event: done\ndata: stream closed\n")
				flusher.Flush()
				return
			}
			send(line)
		}
	}
}
