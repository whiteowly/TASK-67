package service

import (
	"github.com/campusrec/campusrec/config"
	"github.com/campusrec/campusrec/internal/repo"
)

// Services holds all business logic services.
type Services struct {
	Auth         *AuthService
	User         *UserService
	Catalog      *CatalogService
	Address      *AddressService
	Config       *ConfigService
	FeatureFlag  *FeatureFlagService
	Audit        *AuditService
	Registration *RegistrationService
	Attendance   *AttendanceService
	Order        *OrderService
	Payment      *PaymentService
	Shipment     *ShipmentService
	Moderation   *ModerationService
	Ticket       *TicketService
	Import       *ImportService
	Backup       *BackupService
	Job          *JobService
	Dashboard    *DashboardService
}

func NewServices(repos *repo.Repositories, cfg *config.Config) *Services {
	auditSvc := NewAuditService(repos.Audit)
	flagSvc := NewFeatureFlagService(repos.FeatureFlag, auditSvc)
	return &Services{
		Auth:         NewAuthService(repos, cfg, auditSvc),
		User:         NewUserService(repos, auditSvc),
		Catalog:      NewCatalogService(repos.Catalog),
		Address:      NewAddressService(repos.Address),
		Config:       NewConfigService(repos.Config, auditSvc),
		FeatureFlag:  flagSvc,
		Audit:        auditSvc,
		Registration: NewRegistrationService(repos, auditSvc),
		Attendance:   NewAttendanceService(repos, auditSvc),
		Order:        NewOrderService(repos, auditSvc),
		Payment:      NewPaymentService(repos, auditSvc, cfg.Payment.MerchantKey),
		Shipment:     NewShipmentService(repos, auditSvc),
		Moderation:   NewModerationService(repos, auditSvc),
		Ticket:       NewTicketService(repos, auditSvc, flagSvc),
		Import:       NewImportService(repos, auditSvc),
		Backup:       NewBackupService(repos, auditSvc, &cfg.Backup, flagSvc),
		Job:          NewJobService(repos, auditSvc),
		Dashboard:    NewDashboardService(repos),
	}
}
