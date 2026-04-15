package web

import (
	"fmt"
	"net/http"

	"github.com/campusrec/campusrec/config"
	"github.com/campusrec/campusrec/internal/middleware"
	"github.com/campusrec/campusrec/internal/repo"
	"github.com/campusrec/campusrec/internal/service"
	"github.com/campusrec/campusrec/internal/util"
	"github.com/campusrec/campusrec/web/templates/components"
	"github.com/campusrec/campusrec/web/templates/pages"
	"github.com/campusrec/campusrec/web/templates/pages/admin"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	svc *service.Services
	cfg *config.Config
}

func NewHandler(svc *service.Services, cfg *config.Config) *Handler {
	return &Handler{svc: svc, cfg: cfg}
}

func (h *Handler) pageCtx(c *gin.Context) components.PageContext {
	user := middleware.GetAuthUser(c)
	roles := middleware.GetAuthRoles(c)
	ctx := components.PageContext{
		FacilityName: h.cfg.Facility.Name,
	}
	if user != nil {
		pub := user.ToPublic(roles)
		ctx.User = &pub
	}
	return ctx
}

// --- Public pages ---

func (h *Handler) Home(c *gin.Context) {
	ctx := h.pageCtx(c)
	pages.Home(ctx).Render(c.Request.Context(), c.Writer)
}

func (h *Handler) LoginPage(c *gin.Context) {
	ctx := h.pageCtx(c)
	pages.Login(ctx, "", "").Render(c.Request.Context(), c.Writer)
}

func (h *Handler) LoginSubmit(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	result, err := h.svc.Auth.Login(c.Request.Context(), service.LoginInput{
		Username:  username,
		Password:  password,
		IPAddr:    c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	})
	if err != nil {
		ctx := h.pageCtx(c)
		pages.Login(ctx, err.Error(), username).Render(c.Request.Context(), c.Writer)
		return
	}

	maxAge := 8 * 3600
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(middleware.SessionCookieName, result.Token, maxAge, "/", "", h.cfg.Session.CookieSecure, true)
	c.Redirect(http.StatusSeeOther, "/")
}

func (h *Handler) RegisterPage(c *gin.Context) {
	ctx := h.pageCtx(c)
	pages.Register(ctx, "", "").Render(c.Request.Context(), c.Writer)
}

func (h *Handler) RegisterSubmit(c *gin.Context) {
	username := c.PostForm("username")
	displayName := c.PostForm("display_name")
	email := c.PostForm("email")
	password := c.PostForm("password")
	confirmPassword := c.PostForm("confirm_password")

	if password != confirmPassword {
		ctx := h.pageCtx(c)
		pages.Register(ctx, "Passwords do not match", username).Render(c.Request.Context(), c.Writer)
		return
	}

	_, err := h.svc.Auth.Register(c.Request.Context(), service.RegisterInput{
		Username:    username,
		DisplayName: displayName,
		Email:       email,
		Password:    password,
	})
	if err != nil {
		ctx := h.pageCtx(c)
		pages.Register(ctx, err.Error(), username).Render(c.Request.Context(), c.Writer)
		return
	}

	c.Redirect(http.StatusSeeOther, "/login")
}

func (h *Handler) LogoutSubmit(c *gin.Context) {
	sess := middleware.GetAuthSession(c)
	user := middleware.GetAuthUser(c)
	if sess != nil && user != nil {
		h.svc.Auth.Logout(c.Request.Context(), sess.ID, user.ID)
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(middleware.SessionCookieName, "", -1, "/", "", h.cfg.Session.CookieSecure, true)
	c.Redirect(http.StatusSeeOther, "/login")
}

// --- Catalog pages ---

func (h *Handler) CatalogList(c *gin.Context) {
	ctx := h.pageCtx(c)
	pg := util.ParsePagination(c)
	tab := c.DefaultQuery("tab", "sessions")

	sessFilter := repo.SessionFilter{
		Query:    c.Query("q"),
		Status:   "published",
		Category: c.Query("category"),
		Limit:    pg.PerPage,
		Offset:   pg.Offset,
	}

	prodFilter := repo.ProductFilter{
		Query:    c.Query("q"),
		Status:   "published",
		Category: c.Query("category"),
		Limit:    pg.PerPage,
		Offset:   pg.Offset,
	}

	sessions, sessTotal, _ := h.svc.Catalog.ListSessions(c.Request.Context(), sessFilter)
	products, prodTotal, _ := h.svc.Catalog.ListProducts(c.Request.Context(), prodFilter)
	sessCategories, _ := h.svc.Catalog.GetSessionCategories(c.Request.Context())
	prodCategories, _ := h.svc.Catalog.GetProductCategories(c.Request.Context())

	pages.CatalogList(ctx, pages.CatalogData{
		Tab:              tab,
		Sessions:         sessions,
		Products:         products,
		SessionTotal:     sessTotal,
		ProductTotal:     prodTotal,
		SessionCategories: sessCategories,
		ProductCategories: prodCategories,
		Query:            c.Query("q"),
		Category:         c.Query("category"),
		Page:             pg.Page,
		PerPage:          pg.PerPage,
	}).Render(c.Request.Context(), c.Writer)
}

func (h *Handler) SessionDetail(c *gin.Context) {
	ctx := h.pageCtx(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.String(http.StatusNotFound, "Session not found")
		return
	}

	session, err := h.svc.Catalog.GetSession(c.Request.Context(), id)
	if err != nil || session == nil {
		c.String(http.StatusNotFound, "Session not found")
		return
	}

	pages.SessionDetail(ctx, session).Render(c.Request.Context(), c.Writer)
}

func (h *Handler) ProductDetail(c *gin.Context) {
	ctx := h.pageCtx(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.String(http.StatusNotFound, "Product not found")
		return
	}

	product, err := h.svc.Catalog.GetProduct(c.Request.Context(), id)
	if err != nil || product == nil {
		c.String(http.StatusNotFound, "Product not found")
		return
	}

	pages.ProductDetail(ctx, product).Render(c.Request.Context(), c.Writer)
}

// --- Order pages ---

func (h *Handler) OrderList(c *gin.Context) {
	ctx := h.pageCtx(c)
	userID := middleware.GetAuthUserID(c)
	pg := util.ParsePagination(c)
	orders, _, _ := h.svc.Order.ListOrders(c.Request.Context(), userID, pg.PerPage, pg.Offset)
	pages.OrderList(ctx, orders).Render(c.Request.Context(), c.Writer)
}

// --- Order detail page ---

func (h *Handler) OrderDetail(c *gin.Context) {
	ctx := h.pageCtx(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/my/orders")
		return
	}

	userID := middleware.GetAuthUserID(c)
	order, err := h.svc.Order.GetOrder(c.Request.Context(), id, userID)
	if err != nil || order == nil {
		c.Redirect(http.StatusSeeOther, "/my/orders")
		return
	}

	items, _ := h.svc.Order.GetOrderItems(c.Request.Context(), id)
	payReq, _ := h.svc.Order.GetActivePaymentRequest(c.Request.Context(), id)

	pages.OrderDetail(ctx, order, items, payReq).Render(c.Request.Context(), c.Writer)
}

// --- Registration pages ---

func (h *Handler) RegistrationCancel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/my/registrations")
		return
	}

	userID := middleware.GetAuthUserID(c)
	reason := c.DefaultPostForm("reason", "user_canceled")
	_, cancelErr := h.svc.Registration.Cancel(c.Request.Context(), id, userID, reason)
	if cancelErr != nil {
		c.Redirect(http.StatusSeeOther, "/my/registrations")
		return
	}
	c.Redirect(http.StatusSeeOther, "/my/registrations")
}

func (h *Handler) RegistrationList(c *gin.Context) {
	ctx := h.pageCtx(c)
	userID := middleware.GetAuthUserID(c)
	pg := util.ParsePagination(c)
	regs, _, _ := h.svc.Registration.ListByUser(c.Request.Context(), userID, pg.PerPage, pg.Offset)

	// Enrich registrations with session titles for display
	titleMap := make(map[string]string)
	for _, r := range regs {
		sid := r.SessionID.String()
		if _, ok := titleMap[sid]; !ok {
			sess, err := h.svc.Catalog.GetSession(c.Request.Context(), r.SessionID)
			if err == nil && sess != nil {
				titleMap[sid] = sess.Title
			}
		}
	}

	pages.RegistrationList(ctx, regs, titleMap).Render(c.Request.Context(), c.Writer)
}

// --- Address pages ---

func (h *Handler) AddressList(c *gin.Context) {
	ctx := h.pageCtx(c)
	userID := middleware.GetAuthUserID(c)
	addrs, _ := h.svc.Address.List(c.Request.Context(), userID)
	pages.AddressList(ctx, addrs).Render(c.Request.Context(), c.Writer)
}

func (h *Handler) AddressNew(c *gin.Context) {
	ctx := h.pageCtx(c)
	pages.AddressForm(ctx, nil, "").Render(c.Request.Context(), c.Writer)
}

func (h *Handler) AddressCreate(c *gin.Context) {
	ctx := h.pageCtx(c)
	userID := middleware.GetAuthUserID(c)

	input := service.AddressInput{
		Label:         c.PostForm("label"),
		RecipientName: c.PostForm("recipient_name"),
		Phone:         c.PostForm("phone"),
		Line1:         c.PostForm("line1"),
		Line2:         c.PostForm("line2"),
		City:          c.PostForm("city"),
		State:         c.PostForm("state"),
		PostalCode:    c.PostForm("postal_code"),
		CountryCode:   c.PostForm("country_code"),
		IsDefault:     c.PostForm("is_default") == "on",
	}

	_, err := h.svc.Address.Create(c.Request.Context(), userID, input)
	if err != nil {
		pages.AddressForm(ctx, nil, err.Error()).Render(c.Request.Context(), c.Writer)
		return
	}
	c.Redirect(http.StatusSeeOther, "/my/addresses")
}

func (h *Handler) AddressEdit(c *gin.Context) {
	ctx := h.pageCtx(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/my/addresses")
		return
	}

	userID := middleware.GetAuthUserID(c)
	addr, err := h.svc.Address.Get(c.Request.Context(), id, userID)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/my/addresses")
		return
	}

	pages.AddressForm(ctx, addr, "").Render(c.Request.Context(), c.Writer)
}

func (h *Handler) AddressUpdate(c *gin.Context) {
	ctx := h.pageCtx(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/my/addresses")
		return
	}

	userID := middleware.GetAuthUserID(c)
	input := service.AddressInput{
		Label:         c.PostForm("label"),
		RecipientName: c.PostForm("recipient_name"),
		Phone:         c.PostForm("phone"),
		Line1:         c.PostForm("line1"),
		Line2:         c.PostForm("line2"),
		City:          c.PostForm("city"),
		State:         c.PostForm("state"),
		PostalCode:    c.PostForm("postal_code"),
		CountryCode:   c.PostForm("country_code"),
		IsDefault:     c.PostForm("is_default") == "on",
	}

	addr, getErr := h.svc.Address.Get(c.Request.Context(), id, userID)
	if getErr != nil {
		c.Redirect(http.StatusSeeOther, "/my/addresses")
		return
	}

	_, updateErr := h.svc.Address.Update(c.Request.Context(), id, userID, input)
	if updateErr != nil {
		pages.AddressForm(ctx, addr, updateErr.Error()).Render(c.Request.Context(), c.Writer)
		return
	}
	c.Redirect(http.StatusSeeOther, "/my/addresses")
}

func (h *Handler) AddressDelete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/my/addresses")
		return
	}

	userID := middleware.GetAuthUserID(c)
	h.svc.Address.Delete(c.Request.Context(), id, userID)
	c.Redirect(http.StatusSeeOther, "/my/addresses")
}

// --- Cart & Checkout pages ---

// CartPage handles GET /my/cart — displays the user's current cart.
func (h *Handler) CartPage(c *gin.Context) {
	ctx := h.pageCtx(c)
	userID := middleware.GetAuthUserID(c)
	cart, items, _ := h.svc.Order.GetCart(c.Request.Context(), userID)
	pages.CartPage(ctx, cart, items).Render(c.Request.Context(), c.Writer)
}

// CheckoutPage handles GET /my/checkout — displays checkout with address selection.
func (h *Handler) CheckoutPage(c *gin.Context) {
	ctx := h.pageCtx(c)
	userID := middleware.GetAuthUserID(c)
	_, items, _ := h.svc.Order.GetCart(c.Request.Context(), userID)
	addrs, _ := h.svc.Address.List(c.Request.Context(), userID)
	pages.CheckoutPage(ctx, items, addrs, "").Render(c.Request.Context(), c.Writer)
}

// RemoveFromCart handles POST /my/cart/remove/:id — removes an item from the cart.
func (h *Handler) RemoveFromCart(c *gin.Context) {
	itemID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/my/cart")
		return
	}
	userID := middleware.GetAuthUserID(c)
	h.svc.Order.RemoveFromCart(c.Request.Context(), userID, itemID)
	c.Redirect(http.StatusSeeOther, "/my/cart")
}

// --- Commerce actions ---

// AddToCart handles POST /my/cart/add — adds an item to the user's cart.
func (h *Handler) AddToCart(c *gin.Context) {
	userID := middleware.GetAuthUserID(c)
	itemType := c.PostForm("item_type")
	itemID, err := uuid.Parse(c.PostForm("item_id"))
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/catalog")
		return
	}
	qty := 1

	_, addErr := h.svc.Order.AddToCart(c.Request.Context(), userID, itemType, itemID, qty)
	if addErr != nil {
		ref := c.PostForm("redirect")
		if ref == "" {
			ref = "/catalog"
		}
		c.Redirect(http.StatusSeeOther, ref)
		return
	}
	c.Redirect(http.StatusSeeOther, "/my/cart")
}

// BuyNow handles POST /my/buy-now — creates an order for a single item immediately.
func (h *Handler) BuyNow(c *gin.Context) {
	userID := middleware.GetAuthUserID(c)
	itemType := c.PostForm("item_type")
	itemID, err := uuid.Parse(c.PostForm("item_id"))
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/catalog")
		return
	}
	qty := 1

	var addressID *uuid.UUID
	if addrStr := c.PostForm("address_id"); addrStr != "" {
		if parsed, parseErr := uuid.Parse(addrStr); parseErr == nil {
			addressID = &parsed
		}
	}

	order, buyErr := h.svc.Order.BuyNow(c.Request.Context(), userID, itemType, itemID, qty, addressID)
	if buyErr != nil {
		ref := c.PostForm("redirect")
		if ref == "" {
			ref = "/catalog"
		}
		c.Redirect(http.StatusSeeOther, ref)
		return
	}
	c.Redirect(http.StatusSeeOther, fmt.Sprintf("/my/orders/%s", order.ID))
}

// CheckoutSubmit handles POST /my/checkout — checks out the user's cart.
func (h *Handler) CheckoutSubmit(c *gin.Context) {
	userID := middleware.GetAuthUserID(c)

	var addressID *uuid.UUID
	if addrStr := c.PostForm("address_id"); addrStr != "" {
		if parsed, parseErr := uuid.Parse(addrStr); parseErr == nil {
			addressID = &parsed
		}
	}

	idempotencyKey := uuid.New().String()
	order, err := h.svc.Order.Checkout(c.Request.Context(), userID, addressID, idempotencyKey)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/my/orders")
		return
	}
	c.Redirect(http.StatusSeeOther, fmt.Sprintf("/my/orders/%s", order.ID))
}

// CreatePaymentRequest handles POST /my/orders/:id/pay — creates a payment request and shows order.
func (h *Handler) CreatePaymentRequest(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/my/orders")
		return
	}

	userID := middleware.GetAuthUserID(c)
	_, payErr := h.svc.Order.CreatePaymentRequest(c.Request.Context(), id, userID)
	if payErr != nil {
		c.Redirect(http.StatusSeeOther, fmt.Sprintf("/my/orders/%s", id))
		return
	}
	c.Redirect(http.StatusSeeOther, fmt.Sprintf("/my/orders/%s", id))
}

// --- Admin pages ---

func (h *Handler) AdminDashboard(c *gin.Context) {
	ctx := h.pageCtx(c)
	admin.Dashboard(ctx).Render(c.Request.Context(), c.Writer)
}

func (h *Handler) AdminConfig(c *gin.Context) {
	ctx := h.pageCtx(c)
	configs, _ := h.svc.Config.ListAll(c.Request.Context())
	admin.ConfigPage(ctx, configs).Render(c.Request.Context(), c.Writer)
}

func (h *Handler) AdminFeatureFlags(c *gin.Context) {
	ctx := h.pageCtx(c)
	flags, _ := h.svc.FeatureFlag.ListAll(c.Request.Context())
	admin.FeatureFlagsPage(ctx, flags).Render(c.Request.Context(), c.Writer)
}

func (h *Handler) AdminAuditLogs(c *gin.Context) {
	ctx := h.pageCtx(c)
	pg := util.ParsePagination(c)
	filter := repo.AuditFilter{
		Resource:   c.Query("resource"),
		Action:     c.Query("action"),
		ResourceID: c.Query("resource_id"),
		Limit:      pg.PerPage,
		Offset:     pg.Offset,
	}

	logs, total, _ := h.svc.Audit.List(c.Request.Context(), filter)
	admin.AuditLogsPage(ctx, logs, total, pg.Page, pg.PerPage).Render(c.Request.Context(), c.Writer)
}

// suppress unused import warnings
var _ = fmt.Sprintf
