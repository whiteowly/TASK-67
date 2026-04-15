package testutil

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/campusrec/campusrec/config"
	"github.com/campusrec/campusrec/db"
	"github.com/campusrec/campusrec/internal/repo"
	"github.com/campusrec/campusrec/internal/router"
	"github.com/campusrec/campusrec/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SetupTestDB creates a test database connection and runs migrations.
// It returns a cleanup function.
func SetupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, dbURL)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Run migrations
	if err := db.Migrate(dbURL, "up"); err != nil {
		pool.Close()
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Clean tables for test isolation
	cleanTables(ctx, pool, t)

	t.Cleanup(func() {
		cleanTables(ctx, pool, t)
		pool.Close()
	})

	return pool
}

func cleanTables(ctx context.Context, pool *pgxpool.Pool, t *testing.T) {
	tables := []string{
		"archive_lookup_projection",
		"archive_runs",
		"restore_runs",
		"backup_runs",
		"job_attempts",
		"scheduled_jobs",
		"job_queue",
		"import_rows",
		"import_jobs",
		"export_jobs",
		"file_artifacts",
		"ticket_sla_events",
		"ticket_comments",
		"ticket_assignments",
		"ticket_status_history",
		"tickets",
		"posting_rate_windows",
		"moderation_actions",
		"moderation_cases",
		"account_bans",
		"post_reports",
		"posts",
		"delivery_exceptions",
		"delivery_proofs",
		"shipment_status_history",
		"shipments",
		"refunds",
		"payments",
		"payment_requests",
		"order_status_history",
		"order_items",
		"orders",
		"cart_items",
		"carts",
		"occupancy_exceptions",
		"temporary_leave_events",
		"occupancy_sessions",
		"check_in_events",
		"session_waitlist_entries",
		"registration_status_history",
		"session_registrations",
		"session_policies",
		"audit_logs",
		"auth_sessions",
		"account_lockouts",
		"device_clients",
		"user_role_assignments",
		"password_history",
		"delivery_addresses",
		"session_seat_inventory",
		"product_inventory",
		"program_sessions",
		"products",
		"feature_flags",
		// system_config is seeded by migration, not by SeedAll — preserve it
	}
	for _, table := range tables {
		_, err := pool.Exec(ctx, fmt.Sprintf("DELETE FROM %s", table))
		if err != nil {
			t.Logf("Warning: failed to clean table %s: %v", table, err)
		}
	}
	// Clean users last (foreign keys)
	_, _ = pool.Exec(ctx, "DELETE FROM users")
	// Re-seed roles (they were created by migration, not deleted)
}

// SetupTestConfig returns a test configuration.
func SetupTestConfig() *config.Config {
	tz, _ := time.LoadLocation("Asia/Shanghai")
	return &config.Config{
		Server: config.ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
			Mode: "test",
		},
		Database: config.DatabaseConfig{
			URL: os.Getenv("DATABASE_URL"),
		},
		Session: config.SessionConfig{
			Secret:       "test-secret-at-least-32-characters-long-for-tests",
			MaxAge:       8 * time.Hour,
			CookieSecure: false,
		},
		Facility: config.FacilityConfig{
			Timezone: tz,
			Name:     "CampusRec Test",
		},
		Storage: config.StorageConfig{
			UploadDir:      "/tmp/campusrec-test-uploads",
			ExportDir:      "/tmp/campusrec-test-exports",
			MaxUploadBytes: 25 * 1024 * 1024,
		},
		Payment: config.PaymentConfig{
			MerchantKey: "test-merchant-key-for-testing-only",
		},
	}
}

// SetupTestRouter creates a fully wired test router with a real DB.
func SetupTestRouter(t *testing.T) (*gin.Engine, *service.Services) {
	r, _, svc := SetupTestRouterWithPool(t)
	return r, svc
}

// SetupTestRouterWithPool creates a fully wired test router and also returns
// the raw pgxpool.Pool for direct DB assertions in integration tests.
func SetupTestRouterWithPool(t *testing.T) (*gin.Engine, *pgxpool.Pool, *service.Services) {
	t.Helper()

	pool := SetupTestDB(t)
	cfg := SetupTestConfig()

	repos := repo.NewRepositories(pool)
	svc := service.NewServices(repos, cfg)

	gin.SetMode(gin.TestMode)
	r := router.Setup(svc, cfg)

	// Seed roles (ensure they exist)
	seeder := repo.NewSeeder(pool)
	ctx := context.Background()
	_ = seeder.SeedAll(ctx)

	return r, pool, svc
}
