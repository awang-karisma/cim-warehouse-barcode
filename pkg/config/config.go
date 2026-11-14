// Package config handles application configuration loading and validation
// using Viper and godotenv with support for environment variable overrides.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config holds all server, database, auth, and Accurate Online configuration parameters.
type Config struct {
	Port                  string        `mapstructure:"server_port" json:"port"`
	DBPath                string        `mapstructure:"db_path" json:"db_path"`
	DatabaseURL           string        `mapstructure:"database_url" json:"-"`
	GoogleClientID        string        `mapstructure:"google_client_id" json:"google_client_id"`
	JWTSecret             string        `mapstructure:"jwt_secret" json:"jwt_secret"`
	SuperadminEmails      string        `mapstructure:"superadmin_emails" json:"superadmin_emails"`
	AccurateEncryptionKey string        `mapstructure:"accurate_encryption_key" json:"-"`
	RBACCacheTTL          time.Duration `mapstructure:"rbac_cache_ttl" json:"rbac_cache_ttl"`
}

// DefaultConfig returns a Config struct with default configuration values.
func DefaultConfig() Config {
	return Config{
		Port:                  "8080",
		DBPath:                "data/barcode.db",
		DatabaseURL:           "",
		GoogleClientID:        "",
		JWTSecret:             "",
		SuperadminEmails:      "",
		AccurateEncryptionKey: "",
		RBACCacheTTL:          5 * time.Minute,
	}
}

// Load initializes application configuration by loading .env (if present)
// and overriding values with system environment variables.
func Load(envFiles ...string) (*Config, error) {
	if len(envFiles) > 0 {
		_ = godotenv.Load(envFiles...)
	} else {
		_ = godotenv.Load()
	}

	v := viper.New()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Register sensible defaults
	defaults := DefaultConfig()
	v.SetDefault("server_port", defaults.Port)
	v.SetDefault("db_path", defaults.DBPath)
	v.SetDefault("database_url", defaults.DatabaseURL)
	v.SetDefault("google_client_id", defaults.GoogleClientID)
	v.SetDefault("jwt_secret", defaults.JWTSecret)
	v.SetDefault("superadmin_emails", defaults.SuperadminEmails)
	v.SetDefault("accurate_encryption_key", defaults.AccurateEncryptionKey)
	v.SetDefault("rbac_cache_ttl", "5m")
	// Bind environment keys
	_ = v.BindEnv("server_port", "SERVER_PORT", "PORT")
	_ = v.BindEnv("db_path", "DB_PATH")
	_ = v.BindEnv("database_url", "DATABASE_URL")
	_ = v.BindEnv("google_client_id", "GOOGLE_CLIENT_ID")
	_ = v.BindEnv("jwt_secret", "JWT_SECRET")
	_ = v.BindEnv("superadmin_emails", "SUPERADMIN_EMAILS")
	_ = v.BindEnv("accurate_encryption_key", "ACCURATE_ENCRYPTION_KEY")
	_ = v.BindEnv("rbac_cache_ttl", "RBAC_CACHE_TTL")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// Parse RBACCacheTTL duration
	if ttlStr := v.GetString("rbac_cache_ttl"); ttlStr != "" {
		if d, err := time.ParseDuration(ttlStr); err == nil && d > 0 {
			cfg.RBACCacheTTL = d
		} else {
			cfg.RBACCacheTTL = 5 * time.Minute
		}
	} else {
		cfg.RBACCacheTTL = 5 * time.Minute
	}

	cfg.DatabaseURL = strings.TrimSpace(cfg.DatabaseURL)
	if cfg.DatabaseURL != "" && !strings.HasPrefix(strings.ToLower(cfg.DatabaseURL), "postgres://") && !strings.HasPrefix(strings.ToLower(cfg.DatabaseURL), "postgresql://") {
		return nil, fmt.Errorf("DATABASE_URL must use postgres:// or postgresql://")
	}

	if cfg.JWTSecret == "" {
		bytes := make([]byte, 32)
		if _, err := rand.Read(bytes); err != nil {
			return nil, fmt.Errorf("failed to generate random jwt secret: %w", err)
		}
		cfg.JWTSecret = hex.EncodeToString(bytes)
		log.Println("WARNING: JWT_SECRET not set; generated random secret. Sessions will not survive server restarts.")
	}

	if cfg.AccurateEncryptionKey != "" {
		key, err := hex.DecodeString(cfg.AccurateEncryptionKey)
		if err != nil || len(key) != 32 || len(cfg.AccurateEncryptionKey) != 64 {
			return nil, fmt.Errorf("ACCURATE_ENCRYPTION_KEY must be exactly 64 hexadecimal characters (32 bytes)")
		}
	}

	return &cfg, nil
}

// IsSuperadmin reports whether the given email is listed in SuperadminEmails.
func (c *Config) IsSuperadmin(email string) bool {
	trimmedEmail := strings.TrimSpace(email)
	if trimmedEmail == "" || c.SuperadminEmails == "" {
		return false
	}
	for _, admin := range strings.Split(c.SuperadminEmails, ",") {
		if strings.EqualFold(strings.TrimSpace(admin), trimmedEmail) {
			return true
		}
	}
	return false
}
