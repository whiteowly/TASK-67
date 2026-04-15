package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/campusrec/campusrec/config"
	"github.com/campusrec/campusrec/db"
	"github.com/campusrec/campusrec/internal/repo"
	"github.com/campusrec/campusrec/internal/router"
	"github.com/campusrec/campusrec/internal/scheduler"
	"github.com/campusrec/campusrec/internal/service"
	"github.com/gin-gonic/gin"
)

func main() {
	migrateCmd := flag.String("migrate", "", "Run migration: up, down, status")
	seedCmd := flag.Bool("seed", false, "Run seed data")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Handle migration command
	if *migrateCmd != "" {
		if err := db.Migrate(cfg.Database.URL, *migrateCmd); err != nil {
			log.Fatalf("Migration %s failed: %v", *migrateCmd, err)
		}
		fmt.Printf("Migration %s completed successfully\n", *migrateCmd)
		return
	}

	// Connect to database
	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.Database.URL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Handle seed command
	if *seedCmd {
		seeder := repo.NewSeeder(pool)
		if err := seeder.SeedAll(ctx); err != nil {
			log.Fatalf("Seed failed: %v", err)
		}
		fmt.Println("Seed completed successfully")
		return
	}

	// Initialize repositories
	repos := repo.NewRepositories(pool)

	// Initialize services
	services := service.NewServices(repos, cfg)

	sched := scheduler.New()
	sched.Register("payment_expiry", 1*time.Minute, func(ctx context.Context) error {
		_, err := services.Payment.ExpirePayments(ctx)
		return err
	})
	sched.Register("noshow_cancel", 1*time.Minute, func(ctx context.Context) error {
		_, err := services.Attendance.DetectNoShows(ctx)
		return err
	})
	sched.Register("stale_occupancy", 5*time.Minute, func(ctx context.Context) error {
		_, err := services.Attendance.DetectStaleOccupancy(ctx)
		return err
	})
	sched.Register("sla_check", 15*time.Minute, func(ctx context.Context) error {
		_, err := services.Ticket.CheckSLABreaches(ctx)
		return err
	})
	sched.Register("waitlist_promotion", 30*time.Second, func(ctx context.Context) error {
		services.Registration.SweepWaitlistPromotions(ctx)
		return nil
	})
	// Nightly backup — runs every 24 hours
	sched.Register("nightly_backup", 24*time.Hour, func(ctx context.Context) error {
		_, err := services.Backup.RunBackup(ctx, nil)
		return err
	})
	// Nightly archive — runs every 24 hours
	sched.Register("nightly_archive", 24*time.Hour, func(ctx context.Context) error {
		_, err := services.Backup.RunArchive(ctx, "orders")
		if err != nil {
			return err
		}
		_, err = services.Backup.RunArchive(ctx, "tickets")
		return err
	})
	sched.Start()
	defer sched.Stop()

	// Setup Gin
	gin.SetMode(cfg.Server.Mode)
	r := router.Setup(services, cfg)

	// Create HTTP server
	srv := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		fmt.Printf("CampusRec server starting on %s\n", cfg.Addr())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	fmt.Println("Server stopped")
}

