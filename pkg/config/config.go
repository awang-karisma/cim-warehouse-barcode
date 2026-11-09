// Package config handles application configuration loading and validation
// using Viper and godotenv with support for environment variable overrides.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strings"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config holds all server, database, auth, and Accurate Online configuration parameters.
type Config struct {
	Port                       string `mapstructure:"server_port" json:"port"`
	DBPath                     string `mapstructure:"db_path" json:"db_path"`
	GoogleClientID             string `mapstructure:"google_client_id" json:"google_client_id"`
	JWTSecret                  string `mapstructure:"jwt_secret" json:"jwt_secret"`
	SuperadminEmails           string `mapstructure:"superadmin_emails" json:"superadmin_emails"`
	AccurateProvider           string `mapstructure:"accurate_provider" json:"accurate_provider"`
	AccurateEmail              string `mapstructure:"accurate_email" json:"-"`
	AccuratePassword           string `mapstructure:"accurate_password" json:"-"`
	AccurateAPIToken           string `mapstructure:"accurate_api_token" json:"-"`
	AccurateAPISignatureSecret string `mapstructure:"accurate_api_signature_secret" json:"-"`
}

// DefaultConfig returns a Config struct with default configuration values.
func DefaultConfig() Config {
	return Config{
		Port:                       "8080",
		DBPath:                     "data/barcode.db",
		GoogleClientID:             "",
		JWTSecret:                  "",
		SuperadminEmails:           "",
		AccurateProvider:           "",
		AccurateEmail:              "",
		AccuratePassword:           "",
		AccurateAPIToken:           "",
		AccurateAPISignatureSecret: "",
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
	v.SetDefault("google_client_id", defaults.GoogleClientID)
	v.SetDefault("jwt_secret", defaults.JWTSecret)
	v.SetDefault("superadmin_emails", defaults.SuperadminEmails)
	v.SetDefault("accurate_provider", defaults.AccurateProvider)
	v.SetDefault("accurate_email", defaults.AccurateEmail)
	v.SetDefault("accurate_password", defaults.AccuratePassword)
	v.SetDefault("accurate_api_token", defaults.AccurateAPIToken)
	v.SetDefault("accurate_api_signature_secret", defaults.AccurateAPISignatureSecret)

	// Bind environment keys
	_ = v.BindEnv("server_port", "SERVER_PORT", "PORT")
	_ = v.BindEnv("db_path", "DB_PATH")
	_ = v.BindEnv("google_client_id", "GOOGLE_CLIENT_ID")
	_ = v.BindEnv("jwt_secret", "JWT_SECRET")
	_ = v.BindEnv("superadmin_emails", "SUPERADMIN_EMAILS")
	_ = v.BindEnv("accurate_provider", "ACCURATE_PROVIDER")
	_ = v.BindEnv("accurate_email", "ACCURATE_EMAIL")
	_ = v.BindEnv("accurate_password", "ACCURATE_PASSWORD")
	_ = v.BindEnv("accurate_api_token", "ACCURATE_API_TOKEN")
	_ = v.BindEnv("accurate_api_signature_secret", "ACCURATE_API_SIGNATURE_SECRET")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if cfg.JWTSecret == "" {
		bytes := make([]byte, 32)
		if _, err := rand.Read(bytes); err != nil {
			return nil, fmt.Errorf("failed to generate random jwt secret: %w", err)
		}
		cfg.JWTSecret = hex.EncodeToString(bytes)
		log.Println("WARNING: JWT_SECRET not set; generated random secret. Sessions will not survive server restarts.")
	}

	prov := strings.ToLower(strings.TrimSpace(cfg.AccurateProvider))
	switch accurate.ProviderKind(prov) {
	case "":
	case accurate.ProviderWeb:
		if strings.TrimSpace(cfg.AccurateEmail) == "" || strings.TrimSpace(cfg.AccuratePassword) == "" {
			return nil, fmt.Errorf("ACCURATE_EMAIL and ACCURATE_PASSWORD are required when ACCURATE_PROVIDER=web")
		}
	case accurate.ProviderOfficial:
		if strings.TrimSpace(cfg.AccurateAPIToken) == "" {
			return nil, fmt.Errorf("ACCURATE_API_TOKEN is required when ACCURATE_PROVIDER=official")
		}
	default:
		return nil, fmt.Errorf("invalid ACCURATE_PROVIDER %q: must be 'web', 'official', or empty", cfg.AccurateProvider)
	}
	cfg.AccurateProvider = prov

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
