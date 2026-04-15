package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Session  SessionConfig
	Facility FacilityConfig
	Backup   BackupConfig
	Storage  StorageConfig
	Payment  PaymentConfig
}

type ServerConfig struct {
	Host string
	Port int
	Mode string // "debug", "release", "test"
}

type DatabaseConfig struct {
	URL string
}

type SessionConfig struct {
	Secret    string
	MaxAge    time.Duration
	CookieSecure bool
}

type FacilityConfig struct {
	Timezone *time.Location
	Name     string
}

type BackupConfig struct {
	Dir           string
	EncryptionKey string
}

type StorageConfig struct {
	UploadDir      string
	ExportDir      string
	MaxUploadBytes int64
}

type PaymentConfig struct {
	MerchantKey string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	port, _ := strconv.Atoi(getEnv("SERVER_PORT", "8080"))
	maxAge, _ := strconv.Atoi(getEnv("SESSION_MAX_AGE_HOURS", "8"))
	maxUploadMB, _ := strconv.Atoi(getEnv("MAX_UPLOAD_SIZE_MB", "25"))

	tzName := getEnv("FACILITY_TIMEZONE", "Asia/Shanghai")
	tz, err := time.LoadLocation(tzName)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone %q: %w", tzName, err)
	}

	dbURL := getEnv("DATABASE_URL", "")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	secret := getEnv("SESSION_SECRET", "")
	if secret == "" {
		return nil, fmt.Errorf("SESSION_SECRET is required")
	}

	cfg := &Config{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: port,
			Mode: getEnv("GIN_MODE", "release"),
		},
		Database: DatabaseConfig{
			URL: dbURL,
		},
		Session: SessionConfig{
			Secret:       secret,
			MaxAge:       time.Duration(maxAge) * time.Hour,
			CookieSecure: strings.ToLower(getEnv("SESSION_COOKIE_SECURE", "false")) == "true",
		},
		Facility: FacilityConfig{
			Timezone: tz,
			Name:     getEnv("FACILITY_NAME", "CampusRec Center"),
		},
		Backup: BackupConfig{
			Dir:           getEnv("BACKUP_DIR", "./backups"),
			EncryptionKey: getEnv("BACKUP_ENCRYPTION_KEY", ""),
		},
		Storage: StorageConfig{
			UploadDir:      getEnv("UPLOAD_DIR", "./uploads"),
			ExportDir:      getEnv("EXPORT_DIR", "./exports"),
			MaxUploadBytes: int64(maxUploadMB) * 1024 * 1024,
		},
		Payment: PaymentConfig{
			MerchantKey: getEnv("PAYMENT_MERCHANT_KEY", ""),
		},
	}

	// Validate: payment callback flow requires merchant key
	if cfg.Payment.MerchantKey == "" {
		return nil, fmt.Errorf("PAYMENT_MERCHANT_KEY is required: payment callback signature verification will fail without it")
	}

	return cfg, nil
}

// Addr returns the listen address string.
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
