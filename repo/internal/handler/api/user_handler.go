package api

import (
	"net/http"

	"github.com/campusrec/campusrec/internal/middleware"
	"github.com/campusrec/campusrec/internal/response"
	"github.com/campusrec/campusrec/internal/service"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userSvc *service.UserService
}

func NewUserHandler(userSvc *service.UserService) *UserHandler {
	return &UserHandler{userSvc: userSvc}
}

func (h *UserHandler) GetMe(c *gin.Context) {
	userID := middleware.GetAuthUserID(c)
	profile, err := h.userSvc.GetProfile(c.Request.Context(), userID)
	if err != nil {
		response.NotFound(c, "User not found")
		return
	}
	response.OK(c, profile)
}

type updateMeRequest struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
}

func (h *UserHandler) UpdateMe(c *gin.Context) {
	var req updateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	userID := middleware.GetAuthUserID(c)
	profile, err := h.userSvc.UpdateProfile(c.Request.Context(), userID, service.UpdateProfileInput{
		DisplayName: req.DisplayName,
		Email:       req.Email,
		Phone:       req.Phone,
	})
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "UPDATE_FAILED")
		return
	}
	response.OK(c, profile)
}
