package api

import (
	"net/http"

	"github.com/campusrec/campusrec/internal/middleware"
	"github.com/campusrec/campusrec/internal/response"
	"github.com/campusrec/campusrec/internal/service"
	"github.com/gin-gonic/gin"
)

type FeatureFlagHandler struct {
	flagSvc *service.FeatureFlagService
}

func NewFeatureFlagHandler(flagSvc *service.FeatureFlagService) *FeatureFlagHandler {
	return &FeatureFlagHandler{flagSvc: flagSvc}
}

func (h *FeatureFlagHandler) List(c *gin.Context) {
	flags, err := h.flagSvc.ListAll(c.Request.Context())
	if err != nil {
		response.InternalError(c)
		return
	}
	response.OK(c, flags)
}

type updateFlagRequest struct {
	Enabled       bool `json:"enabled"`
	CohortPercent int  `json:"cohort_percent"`
	Version       int  `json:"version" binding:"required"`
}

func (h *FeatureFlagHandler) Update(c *gin.Context) {
	key := c.Param("key")
	var req updateFlagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	userID := middleware.GetAuthUserID(c)
	if err := h.flagSvc.Update(c.Request.Context(), key, req.Enabled, req.CohortPercent, userID, req.Version); err != nil {
		handleServiceError(c, err, http.StatusConflict, "FLAG_UPDATE_FAILED")
		return
	}

	updated, _ := h.flagSvc.GetByKey(c.Request.Context(), key)
	response.OK(c, updated)
}
