package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/campusrec/campusrec/internal/model"
	"github.com/campusrec/campusrec/internal/util"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Seeder struct {
	pool *pgxpool.Pool
}

func NewSeeder(pool *pgxpool.Pool) *Seeder {
	return &Seeder{pool: pool}
}

func (s *Seeder) SeedAll(ctx context.Context) error {
	if err := s.seedUsers(ctx); err != nil {
		return fmt.Errorf("seed users: %w", err)
	}
	if err := s.seedCatalog(ctx); err != nil {
		return fmt.Errorf("seed catalog: %w", err)
	}
	return nil
}

func (s *Seeder) seedUsers(ctx context.Context) error {
	repos := NewRepositories(s.pool)

	users := []struct {
		username string
		display  string
		role     string
	}{
		{"admin", "System Administrator", model.RoleAdministrator},
		{"staff1", "Staff User One", model.RoleStaff},
		{"mod1", "Moderator One", model.RoleModerator},
		{"member1", "Member One", model.RoleMember},
		{"member2", "Member Two", model.RoleMember},
	}

	// Default password for seed users: Seed@Pass1234
	hash, err := util.HashPassword("Seed@Pass1234")
	if err != nil {
		return err
	}

	for _, u := range users {
		existing, _ := repos.User.GetByUsername(ctx, u.username)

		var userID uuid.UUID
		if existing != nil {
			userID = existing.ID
		} else {
			now := time.Now().UTC()
			user := &model.User{
				ID:           uuid.New(),
				Username:     u.username,
				DisplayName:  u.display,
				PasswordHash: hash,
				IsActive:     true,
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			if err := repos.User.Create(ctx, user); err != nil {
				return fmt.Errorf("create user %s: %w", u.username, err)
			}
			userID = user.ID
		}

		// Always ensure role assignment (idempotent via ON CONFLICT DO NOTHING)
		role, err := repos.Role.GetByName(ctx, u.role)
		if err != nil {
			return fmt.Errorf("get role %s: %w", u.role, err)
		}
		if err := repos.Role.AssignRole(ctx, userID, role.ID, nil); err != nil {
			return fmt.Errorf("assign role %s: %w", u.role, err)
		}

		fmt.Printf("  Created user: %s (role: %s)\n", u.username, u.role)
	}
	return nil
}

func (s *Seeder) seedCatalog(ctx context.Context) error {
	repos := NewRepositories(s.pool)
	now := time.Now().UTC()

	// Seed sessions
	sessions := []model.ProgramSession{
		{
			ID:               uuid.New(),
			Title:            "Morning Yoga Flow",
			Description:      "A gentle morning yoga session suitable for all levels. Focus on breathing and flexibility.",
			ShortDescription: "Gentle morning yoga for all levels",
			Category:         strPtr("Yoga"),
			InstructorName:   strPtr("Coach Li"),
			Tags:             []string{"yoga", "morning", "beginner"},
			StartAt:          now.Add(48 * time.Hour),
			EndAt:            now.Add(49 * time.Hour),
			SeatCapacity:     30,
			PriceMinorUnits:  5000,
			Currency:         "CNY",
			RegistrationOpenAt:  timePtr(now),
			RegistrationCloseAt: timePtr(now.Add(46 * time.Hour)),
			AllowsWaitlist:   true,
			Status:           model.SessionStatusPublished,
			Location:         strPtr("Studio A"),
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               uuid.New(),
			Title:            "HIIT Bootcamp",
			Description:      "High-intensity interval training session. Bring water and a towel.",
			ShortDescription: "High-intensity interval training",
			Category:         strPtr("Fitness"),
			InstructorName:   strPtr("Coach Wang"),
			Tags:             []string{"fitness", "hiit", "advanced"},
			StartAt:          now.Add(72 * time.Hour),
			EndAt:            now.Add(73 * time.Hour),
			SeatCapacity:     20,
			PriceMinorUnits:  8000,
			Currency:         "CNY",
			RegistrationOpenAt:  timePtr(now),
			RegistrationCloseAt: timePtr(now.Add(70 * time.Hour)),
			AllowsWaitlist:   true,
			Status:           model.SessionStatusPublished,
			Location:         strPtr("Gym Floor"),
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               uuid.New(),
			Title:            "Swimming Basics",
			Description:      "Learn fundamental swimming techniques with a certified instructor.",
			ShortDescription: "Fundamental swimming techniques",
			Category:         strPtr("Aquatics"),
			InstructorName:   strPtr("Coach Zhang"),
			Tags:             []string{"swimming", "beginner", "aquatics"},
			StartAt:          now.Add(96 * time.Hour),
			EndAt:            now.Add(97 * time.Hour),
			SeatCapacity:     15,
			PriceMinorUnits:  10000,
			Currency:         "CNY",
			RegistrationOpenAt:  timePtr(now),
			RegistrationCloseAt: timePtr(now.Add(94 * time.Hour)),
			RequiresApproval: true,
			AllowsWaitlist:   true,
			Status:           model.SessionStatusPublished,
			Location:         strPtr("Indoor Pool"),
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}

	for _, sess := range sessions {
		if err := repos.Catalog.CreateSession(ctx, &sess); err != nil {
			fmt.Printf("  Session %s may already exist: %v\n", sess.Title, err)
			continue
		}
		fmt.Printf("  Created session: %s\n", sess.Title)
	}

	// Seed products
	products := []model.Product{
		{
			ID:               uuid.New(),
			Name:             "CampusRec T-Shirt",
			Description:      "Official CampusRec branded t-shirt. 100% cotton.",
			ShortDescription: "Official branded cotton t-shirt",
			Category:         strPtr("Merchandise"),
			SKU:              strPtr("TSHIRT-001"),
			PriceMinorUnits:  12900,
			Currency:         "CNY",
			IsShippable:      true,
			Status:           model.ProductStatusPublished,
			Tags:             []string{"clothing", "merchandise"},
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               uuid.New(),
			Name:             "Yoga Mat",
			Description:      "Premium non-slip yoga mat, 6mm thickness.",
			ShortDescription: "Premium non-slip yoga mat",
			Category:         strPtr("Equipment"),
			SKU:              strPtr("YOGAMAT-001"),
			PriceMinorUnits:  19900,
			Currency:         "CNY",
			IsShippable:      true,
			Status:           model.ProductStatusPublished,
			Tags:             []string{"yoga", "equipment"},
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               uuid.New(),
			Name:             "Water Bottle",
			Description:      "Stainless steel insulated water bottle, 750ml.",
			ShortDescription: "Insulated stainless steel water bottle",
			Category:         strPtr("Accessories"),
			SKU:              strPtr("BOTTLE-001"),
			PriceMinorUnits:  8900,
			Currency:         "CNY",
			IsShippable:      true,
			Status:           model.ProductStatusPublished,
			Tags:             []string{"accessories", "hydration"},
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}

	for _, prod := range products {
		if err := repos.Catalog.CreateProduct(ctx, &prod); err != nil {
			fmt.Printf("  Product %s may already exist: %v\n", prod.Name, err)
			continue
		}
		fmt.Printf("  Created product: %s\n", prod.Name)
	}

	// Set stock for all seeded products so checkout/buy-now flows work
	_, _ = s.pool.Exec(ctx, `UPDATE product_inventory SET stock_qty = 100`)

	// Seed feature flags for testing
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO feature_flags (id, key, enabled, description, cohort_percent, version)
		VALUES (gen_random_uuid(), 'test.flag', false, 'Test feature flag for verification', 100, 1)
		ON CONFLICT (key) DO NOTHING`)
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO feature_flags (id, key, enabled, description, cohort_percent, version)
		VALUES (gen_random_uuid(), 'enable_manual_backup', true, 'Enable manual backup trigger', 100, 1)
		ON CONFLICT (key) DO NOTHING`)
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO feature_flags (id, key, enabled, description, cohort_percent, version)
		VALUES (gen_random_uuid(), 'enable_ticket_assignment', true, 'Enable ticket assignment', 100, 1)
		ON CONFLICT (key) DO NOTHING`)

	return nil
}

func strPtr(s string) *string  { return &s }
func timePtr(t time.Time) *time.Time { return &t }
