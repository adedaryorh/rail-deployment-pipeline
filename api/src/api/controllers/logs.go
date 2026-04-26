package controllers

import (
	"github.com/yourorg/deployment-pipeline/api/src/api/repo"
)

type LogsController struct {
	repo *repo.DeployRepo
}

func NewLogsController(r *repo.DeployRepo) *LogsController {
	return &LogsController{repo: r}
}

func (lc *LogsController) Subscribe(deployID string) (replay []string, ch <-chan string, unsub func()) {
	return lc.repo.Subscribe(deployID)
}
