package api

import (
	"net/http"

	"github.com/campusrec/campusrec/internal/middleware"
	"github.com/campusrec/campusrec/internal/response"
	"github.com/campusrec/campusrec/internal/service"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authSvc *service.AuthService
	secure  bool
}

func NewAuthHandler(authSvc *service.AuthService, secureCookie bool) *AuthHandler {
	return &AuthHandler{authSvc: authSvc, secure: secureCookie}
}

type registerRequest struct {
	Username    string `json:"username" binding:"required"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Password    string `json:"password" binding:"required"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	user, err := h.authSvc.Register(c.Request.Context(), service.RegisterInput{
		Username:    req.Username,
		DisplayName: req.DisplayName,
		Email:       req.Email,
		Phone:       req.Phone,
		Password:    req.Password,
	})
	if err != nil {
		handleServiceError(c, err, http.StatusBadRequest, "REGISTRATION_FAILED")
		return
	}

	response.Created(c, user)
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	result, err := h.authSvc.Login(c.Request.Context(), service.LoginInput{
		Username:  req.Username,
		Password:  req.Password,
		IPAddr:    c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	})
	if err != nil {
		handleServiceError(c, err, http.StatusUnauthorized, "LOGIN_FAILED")
		return
	}

	// Set session cookie
	maxAge := 8 * 3600 // 8 hours in seconds
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		middleware.SessionCookieName,
		result.Token,
		maxAge,
		"/",
		"",
		h.secure,
		true, // httpOnly
	)

	response.OK(c, result.User)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	sess := middleware.GetAuthSession(c)
	user := middleware.GetAuthUser(c)
	if sess == nil || user == nil {
		response.Unauthorized(c, "Not authenticated")
		return
	}

	if err := h.authSvc.Logout(c.Request.Context(), sess.ID, user.ID); err != nil {
		response.InternalError(c)
		return
	}

	// Clear cookie
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(middleware.SessionCookieName, "", -1, "/", "", h.secure, true)
	response.OK(c, gin.H{"message": "Logged out successfully"})
}
