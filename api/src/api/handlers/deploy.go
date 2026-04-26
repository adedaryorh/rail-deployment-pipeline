package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourorg/deployment-pipeline/api/src/api/controllers"
	"github.com/yourorg/deployment-pipeline/api/src/api/repo"
	"github.com/yourorg/deployment-pipeline/api/src/services"
)

type DeployHandler struct {
	ctrl *controllers.DeployController
	repo *repo.DeployRepo
}

func NewDeployHandler(r *repo.DeployRepo, rp *services.RailpackService, d *services.DockerService, c *services.CaddyService) *DeployHandler {
	return &DeployHandler{
		ctrl: controllers.NewDeployController(r, rp, d, c),
		repo: r,
	}
}

// POST /api/deploy
func (h *DeployHandler) Trigger(c *gin.Context) {
	deploy := h.ctrl.Run(c.Request.Context())
	c.JSON(http.StatusAccepted, deploy)
}

// GET /api/deploys
func (h *DeployHandler) List(c *gin.Context) {
	c.JSON(http.StatusOK, h.repo.List())
}

// POST /api/deploys/:id/rollback
func (h *DeployHandler) Rollback(c *gin.Context) {
	id := c.Param("id")
	if err := h.ctrl.Rollback(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	deploy, _ := h.repo.Get(id)
	c.JSON(http.StatusOK, deploy)
}
