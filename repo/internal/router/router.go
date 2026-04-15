package router

import (
	"net/http"

	"github.com/campusrec/campusrec/config"
	"github.com/google/uuid"
	"github.com/campusrec/campusrec/internal/handler/api"
	"github.com/campusrec/campusrec/internal/handler/web"
	"github.com/campusrec/campusrec/internal/middleware"
	"github.com/campusrec/campusrec/internal/model"
	"github.com/campusrec/campusrec/internal/response"
	"github.com/campusrec/campusrec/internal/service"
	"github.com/gin-gonic/gin"
)

func Setup(svc *service.Services, cfg *config.Config) *gin.Engine {
	r := gin.New()

	// Global middleware
	r.Use(middleware.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.AuthSession(svc.Auth))
	r.Use(middleware.AuditLog(svc.Audit))

	// Static files
	r.Static("/static", "./web/static")

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Initialize handlers
	authHandler := api.NewAuthHandler(svc.Auth, cfg.Session.CookieSecure)
	userHandler := api.NewUserHandler(svc.User)
	catalogHandler := api.NewCatalogHandler(svc.Catalog)
	addressHandler := api.NewAddressHandler(svc.Address)
	configHandler := api.NewConfigHandler(svc.Config, svc.Audit)
	flagHandler := api.NewFeatureFlagHandler(svc.FeatureFlag)
	registrationHandler := api.NewRegistrationHandler(svc.Registration)
	attendanceHandler := api.NewAttendanceHandler(svc.Attendance)
	orderHandler := api.NewOrderHandler(svc.Order)
	paymentHandler := api.NewPaymentHandler(svc.Payment)
	shipmentHandler := api.NewShipmentHandler(svc.Shipment)
	moderationHandler := api.NewModerationHandler(svc.Moderation)
	ticketHandler := api.NewTicketHandler(svc.Ticket)
	importHandler := api.NewImportHandler(svc.Import)
	backupHandler := api.NewBackupHandler(svc.Backup)
	dashboardHandler := api.NewDashboardHandler(svc.Dashboard)
	webHandler := web.NewHandler(svc, cfg)

	// --- JSON API routes ---
	v1 := r.Group("/api/v1")
	{
		// Auth (public)
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/logout", middleware.RequireAuth(), authHandler.Logout)
		}

		// User profile (authenticated)
		users := v1.Group("/users", middleware.RequireAuth())
		{
			users.GET("/me", userHandler.GetMe)
			users.PATCH("/me", userHandler.UpdateMe)
		}

		// Catalog (public)
		catalog := v1.Group("/catalog")
		{
			catalog.GET("/sessions", catalogHandler.ListSessions)
			catalog.GET("/sessions/:id", catalogHandler.GetSession)
			catalog.GET("/products", catalogHandler.ListProducts)
			catalog.GET("/products/:id", catalogHandler.GetProduct)
		}

		// Addresses (authenticated member+)
		addresses := v1.Group("/addresses", middleware.RequireAuth())
		{
			addresses.GET("", addressHandler.List)
			addresses.POST("", addressHandler.Create)
			addresses.GET("/:id", addressHandler.Get)
			addresses.PATCH("/:id", addressHandler.Update)
			addresses.DELETE("/:id", addressHandler.Delete)
		}

		// Admin (administrator only)
		admin := v1.Group("/admin", middleware.RequireRole(model.RoleAdministrator))
		{
			admin.GET("/config", configHandler.ListConfig)
			admin.PATCH("/config/:key", configHandler.UpdateConfig)
			admin.GET("/feature-flags", flagHandler.List)
			admin.PATCH("/feature-flags/:key", flagHandler.Update)
			admin.GET("/audit-logs", configHandler.ListAuditLogs)

			// Backup / Restore / Archive
			admin.POST("/backups", backupHandler.RunBackup)
			admin.GET("/backups", backupHandler.ListBackups)
			admin.POST("/restore", backupHandler.InitiateRestore)
			admin.GET("/archives", backupHandler.ListArchives)
			admin.POST("/archives", backupHandler.RunArchive)

			// Refund reconciliation
			admin.POST("/refunds/:id/reconcile", func(c *gin.Context) {
				id, err := uuid.Parse(c.Param("id"))
				if err != nil {
					response.NotFound(c, "Invalid refund ID")
					return
				}
				var req struct {
					Status string `json:"status" binding:"required"`
				}
				if err := c.ShouldBindJSON(&req); err != nil {
					response.ValidationError(c, err.Error())
					return
				}
				if err := svc.Payment.ReconcileRefund(c.Request.Context(), id, req.Status); err != nil {
					response.Error(c, http.StatusBadRequest, "RECONCILE_FAILED", "operation failed")
					return
				}
				response.OK(c, gin.H{"status": "reconciled"})
			})

			// Dashboard KPIs & Jobs
			admin.GET("/kpis", dashboardHandler.GetKPIs)
			admin.GET("/jobs", dashboardHandler.GetJobStatus)

			// Admin override registration (bypasses close-hours policy)
			admin.POST("/registrations/override", registrationHandler.AdminOverrideRegister)
		}

		// Registrations (authenticated)
		registrations := v1.Group("/registrations", middleware.RequireAuth())
		{
			registrations.POST("", registrationHandler.Register)
			registrations.GET("", registrationHandler.ListMine)
			registrations.GET("/:id", registrationHandler.Get)
			registrations.POST("/:id/cancel", registrationHandler.Cancel)
			registrations.POST("/:id/approve", middleware.RequireRole(model.RoleStaff, model.RoleAdministrator), registrationHandler.Approve)
			registrations.POST("/:id/reject", middleware.RequireRole(model.RoleStaff, model.RoleAdministrator), registrationHandler.Reject)
		}

		// Attendance
		attendance := v1.Group("/attendance", middleware.RequireAuth())
		{
			attendance.POST("/checkin", middleware.RequireRole(model.RoleStaff, model.RoleAdministrator), attendanceHandler.CheckIn)
			attendance.POST("/leave", attendanceHandler.StartLeave)
			attendance.POST("/leave/:id/return", attendanceHandler.EndLeave)
			attendance.GET("/exceptions", middleware.RequireRole(model.RoleStaff, model.RoleAdministrator), attendanceHandler.ListExceptions)
		}

		// Cart (authenticated)
		cart := v1.Group("/cart", middleware.RequireAuth())
		{
			cart.GET("", orderHandler.GetCart)
			cart.POST("/items", orderHandler.AddToCart)
			cart.DELETE("/items/:id", orderHandler.RemoveFromCart)
		}

		// Checkout & Buy Now (authenticated)
		v1.POST("/checkout", middleware.RequireAuth(), orderHandler.Checkout)
		v1.POST("/buy-now", middleware.RequireAuth(), orderHandler.BuyNow)

		// Orders (authenticated)
		orders := v1.Group("/orders", middleware.RequireAuth())
		{
			orders.GET("", orderHandler.ListOrders)
			orders.GET("/:id", orderHandler.GetOrder)
			orders.POST("/:id/pay", orderHandler.CreatePaymentRequest)
		}

		// Payments callback (public — from local payment bridge)
		payments := v1.Group("/payments")
		{
			payments.POST("/callback", paymentHandler.Callback)
		}

		// Shipments (staff/admin)
		shipments := v1.Group("/shipments", middleware.RequireRole(model.RoleStaff, model.RoleAdministrator))
		{
			shipments.POST("", shipmentHandler.Create)
			shipments.GET("", shipmentHandler.List)
			shipments.PATCH("/:id/status", shipmentHandler.UpdateStatus)
			shipments.POST("/:id/pod", shipmentHandler.RecordPOD)
			shipments.POST("/:id/exception", shipmentHandler.ReportException)
		}

		// Posts (public for list/get, authenticated for create/report)
		posts := v1.Group("/posts")
		{
			posts.GET("", moderationHandler.ListPosts)
			posts.GET("/:id", moderationHandler.GetPost)
			posts.POST("", middleware.RequireAuth(), moderationHandler.CreatePost)
			posts.POST("/:id/report", middleware.RequireAuth(), moderationHandler.ReportPost)
		}

		// Moderation (moderator/admin)
		moderation := v1.Group("/moderation", middleware.RequireRole(model.RoleModerator, model.RoleAdministrator))
		{
			moderation.GET("/reports", moderationHandler.ListReports)
			moderation.GET("/cases", moderationHandler.ListCases)
			moderation.GET("/cases/:id", moderationHandler.GetCase)
			moderation.POST("/cases/:id/action", moderationHandler.ActionCase)
			moderation.POST("/bans", moderationHandler.ApplyBan)
			moderation.POST("/bans/:id/revoke", moderationHandler.RevokeBan)
		}

		// Tickets (authenticated)
		tickets := v1.Group("/tickets", middleware.RequireAuth())
		{
			tickets.POST("", ticketHandler.Create)
			tickets.GET("", ticketHandler.List)
			tickets.GET("/:id", ticketHandler.Get)
			tickets.PATCH("/:id/status", ticketHandler.UpdateStatus)
			tickets.POST("/:id/assign", middleware.RequireRole(model.RoleStaff, model.RoleAdministrator), ticketHandler.Assign)
			tickets.POST("/:id/comments", ticketHandler.AddComment)
			tickets.POST("/:id/resolve", ticketHandler.Resolve)
			tickets.POST("/:id/close", ticketHandler.Close)
		}

		// Imports (admin)
		imports := v1.Group("/imports", middleware.RequireRole(model.RoleAdministrator))
		{
			imports.POST("", importHandler.Upload)
			imports.GET("", importHandler.ListImports)
			imports.GET("/:id", importHandler.GetImportDetail)
			imports.POST("/:id/validate", importHandler.ValidateImport)
			imports.POST("/:id/apply", importHandler.ApplyImport)
		}

		// Exports (admin)
		exports := v1.Group("/exports", middleware.RequireRole(model.RoleAdministrator))
		{
			exports.POST("", importHandler.CreateExport)
			exports.GET("", importHandler.ListExports)
			exports.GET("/:id/download", importHandler.DownloadExport)
		}
	}

	// --- Web (Templ) routes ---
	r.GET("/", webHandler.Home)
	r.GET("/login", webHandler.LoginPage)
	r.POST("/login", webHandler.LoginSubmit)
	r.GET("/register", webHandler.RegisterPage)
	r.POST("/register", webHandler.RegisterSubmit)
	r.POST("/logout", webHandler.LogoutSubmit)

	// Catalog pages (public)
	r.GET("/catalog", webHandler.CatalogList)
	r.GET("/catalog/sessions/:id", webHandler.SessionDetail)
	r.GET("/catalog/products/:id", webHandler.ProductDetail)

	// Member pages (authenticated)
	member := r.Group("/my", middleware.RequireAuth())
	{
		member.GET("/orders", webHandler.OrderList)
		member.GET("/orders/:id", webHandler.OrderDetail)
		member.POST("/orders/:id/pay", webHandler.CreatePaymentRequest)
		member.GET("/cart", webHandler.CartPage)
		member.POST("/cart/add", webHandler.AddToCart)
		member.POST("/cart/remove/:id", webHandler.RemoveFromCart)
		member.GET("/checkout", webHandler.CheckoutPage)
		member.POST("/checkout", webHandler.CheckoutSubmit)
		member.POST("/buy-now", webHandler.BuyNow)
		member.GET("/registrations", webHandler.RegistrationList)
		member.POST("/registrations/:id/cancel", webHandler.RegistrationCancel)
		member.GET("/addresses", webHandler.AddressList)
		member.GET("/addresses/new", webHandler.AddressNew)
		member.POST("/addresses", webHandler.AddressCreate)
		member.GET("/addresses/:id/edit", webHandler.AddressEdit)
		member.POST("/addresses/:id", webHandler.AddressUpdate)
		member.POST("/addresses/:id/delete", webHandler.AddressDelete)
	}

	// Admin pages (administrator only)
	adminWeb := r.Group("/admin", middleware.RequireRole(model.RoleAdministrator))
	{
		adminWeb.GET("", webHandler.AdminDashboard)
		adminWeb.GET("/config", webHandler.AdminConfig)
		adminWeb.GET("/feature-flags", webHandler.AdminFeatureFlags)
		adminWeb.GET("/audit-logs", webHandler.AdminAuditLogs)
	}

	// 404 handler
	r.NoRoute(func(c *gin.Context) {
		response.NotFound(c, "Page not found")
	})

	return r
}
