package repo

import "github.com/jackc/pgx/v5/pgxpool"

// Repositories holds all data access objects.
type Repositories struct {
	User         *UserRepo
	Role         *RoleRepo
	Session      *SessionRepo
	Audit        *AuditRepo
	Catalog      *CatalogRepo
	Address      *AddressRepo
	Config       *ConfigRepo
	FeatureFlag  *FeatureFlagRepo
	Registration *RegistrationRepo
	Attendance   *AttendanceRepo
	Order        *OrderRepo
	Payment      *PaymentRepo
	Shipment     *ShipmentRepo
	Moderation   *ModerationRepo
	Ticket       *TicketRepo
	Import       *ImportRepo
	Job          *JobRepo
	Backup       *BackupRepo
}

func NewRepositories(pool *pgxpool.Pool) *Repositories {
	return &Repositories{
		User:         NewUserRepo(pool),
		Role:         NewRoleRepo(pool),
		Session:      NewSessionRepo(pool),
		Audit:        NewAuditRepo(pool),
		Catalog:      NewCatalogRepo(pool),
		Address:      NewAddressRepo(pool),
		Config:       NewConfigRepo(pool),
		FeatureFlag:  NewFeatureFlagRepo(pool),
		Registration: NewRegistrationRepo(pool),
		Attendance:   NewAttendanceRepo(pool),
		Order:        NewOrderRepo(pool),
		Payment:      NewPaymentRepo(pool),
		Shipment:     NewShipmentRepo(pool),
		Moderation:   NewModerationRepo(pool),
		Ticket:       NewTicketRepo(pool),
		Import:       NewImportRepo(pool),
		Job:          NewJobRepo(pool),
		Backup:       NewBackupRepo(pool),
	}
}
