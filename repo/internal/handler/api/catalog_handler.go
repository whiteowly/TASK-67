package api

import (
	"github.com/campusrec/campusrec/internal/repo"
	"github.com/campusrec/campusrec/internal/response"
	"github.com/campusrec/campusrec/internal/service"
	"github.com/campusrec/campusrec/internal/util"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type CatalogHandler struct {
	catalogSvc *service.CatalogService
}

func NewCatalogHandler(catalogSvc *service.CatalogService) *CatalogHandler {
	return &CatalogHandler{catalogSvc: catalogSvc}
}

func (h *CatalogHandler) ListSessions(c *gin.Context) {
	pg := util.ParsePagination(c)
	filter := repo.SessionFilter{
		Query:    c.Query("q"),
		Status:   c.DefaultQuery("status", "published"),
		Category: c.Query("category"),
		Limit:    pg.PerPage,
		Offset:   pg.Offset,
	}

	sessions, total, err := h.catalogSvc.ListSessions(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c)
		return
	}
	if sessions == nil {
		response.Paginated(c, []struct{}{}, pg.Page, pg.PerPage, total)
		return
	}
	response.Paginated(c, sessions, pg.Page, pg.PerPage, total)
}

func (h *CatalogHandler) GetSession(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid session ID")
		return
	}

	session, err := h.catalogSvc.GetSession(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c)
		return
	}
	if session == nil {
		response.NotFound(c, "Session not found")
		return
	}
	response.OK(c, session)
}

func (h *CatalogHandler) ListProducts(c *gin.Context) {
	pg := util.ParsePagination(c)
	filter := repo.ProductFilter{
		Query:    c.Query("q"),
		Status:   c.DefaultQuery("status", "published"),
		Category: c.Query("category"),
		Limit:    pg.PerPage,
		Offset:   pg.Offset,
	}

	products, total, err := h.catalogSvc.ListProducts(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c)
		return
	}
	if products == nil {
		response.Paginated(c, []struct{}{}, pg.Page, pg.PerPage, total)
		return
	}
	response.Paginated(c, products, pg.Page, pg.PerPage, total)
}

func (h *CatalogHandler) GetProduct(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.NotFound(c, "Invalid product ID")
		return
	}

	product, err := h.catalogSvc.GetProduct(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c)
		return
	}
	if product == nil {
		response.NotFound(c, "Product not found")
		return
	}
	response.OK(c, product)
}
