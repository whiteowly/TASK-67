package api

import (
	"github.com/campusrec/campusrec/internal/response"
	"github.com/campusrec/campusrec/internal/service"
	"github.com/gin-gonic/gin"
)

type DashboardHandler struct {
	svc *service.DashboardService
}

func NewDashboardHandler(svc *service.DashboardService) *DashboardHandler {
	return &DashboardHandler{svc: svc}
}

func (h *DashboardHandler) GetKPIs(c *gin.Context) {
	kpis, err := h.svc.GetKPIs(c.Request.Context())
	if err != nil {
		response.InternalError(c)
		return
	}

	response.OK(c, kpis)
}

func (h *DashboardHandler) GetJobStatus(c *gin.Context) {
	jobs, err := h.svc.GetJobStatus(c.Request.Context())
	if err != nil {
		response.InternalError(c)
		return
	}

	response.OK(c, jobs)
}
