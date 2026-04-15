package api

import (
	"net/http"

	"github.com/campusrec/campusrec/internal/middleware"
	"github.com/campusrec/campusrec/internal/repo"
	"github.com/campusrec/campusrec/internal/response"
	"github.com/campusrec/campusrec/internal/service"
	"github.com/campusrec/campusrec/internal/util"
	"github.com/gin-gonic/gin"
)

type ConfigHandler struct {
	configSvc *service.ConfigService
	auditSvc  *service.AuditService
}

func NewConfigHandler(configSvc *service.ConfigService, auditSvc *service.AuditService) *ConfigHandler {
	return &ConfigHandler{configSvc: configSvc, auditSvc: auditSvc}
}

func (h *ConfigHandler) ListConfig(c *gin.Context) {
	configs, err := h.configSvc.ListAll(c.Request.Context())
	if err != nil {
		response.InternalError(c)
		return
	}
	response.OK(c, configs)
}

type updateConfigRequest struct {
	Value   string `json:"value" binding:"required"`
	Version int    `json:"version" binding:"required"`
}

func (h *ConfigHandler) UpdateConfig(c *gin.Context) {
	key := c.Param("key")
	var req updateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	userID := middleware.GetAuthUserID(c)
	if err := h.configSvc.Update(c.Request.Context(), key, req.Value, userID, req.Version); err != nil {
		handleServiceError(c, err, http.StatusConflict, "CONFIG_UPDATE_FAILED")
		return
	}

	updated, _ := h.configSvc.GetByKey(c.Request.Context(), key)
	response.OK(c, updated)
}

func (h *ConfigHandler) ListAuditLogs(c *gin.Context) {
	pg := util.ParsePagination(c)
	filter := repo.AuditFilter{
		Resource:   c.Query("resource"),
		Action:     c.Query("action"),
		ResourceID: c.Query("resource_id"),
		Limit:      pg.PerPage,
		Offset:     pg.Offset,
	}

	actorID := c.Query("actor_id")
	if actorID != "" {
		filter.ActorID = &actorID
	}

	logs, total, err := h.auditSvc.List(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c)
		return
	}
	response.Paginated(c, logs, pg.Page, pg.PerPage, total)
}
