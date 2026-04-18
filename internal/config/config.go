package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const minSecretKeyBytes = 32

const appEnvDevelopment = "development"
const appEnvProduction = "production"

type Config struct {
	AppEnv                 string
	AppPort                int
	MonsteraInstanceDomain string
	MonsteraServerURL      *url.URL
	MonsteraUIURL          *url.URL
	LogLevel               string

	DatabaseHost         string
	DatabasePort         string
	DatabaseName         string
	DatabaseUsername     string
	DatabasePassword     string
	DatabaseMaxOpenConns int
	DatabaseSSLMode      string

	NATSUrl       string
	NATSCredsFile string

	CacheDriver string

	MediaDriver     string
	MediaLocalPath  string
	MediaBaseURL    string
	MediaS3Bucket   string
	MediaS3Region   string
	MediaS3Endpoint string
	MediaCDNBase    string

	EmailDriver       string
	EmailFrom         string
	EmailFromName     string
	EmailSMTPHost     string
	EmailSMTPPort     int
	EmailSMTPUsername string
	EmailSMTPPassword string

	SecretKeyBase string
	MetricsToken  string

	VAPIDPrivateKey string
	VAPIDPublicKey  string

	FederationWorkerConcurrency int
	FederationInsecureSkipTLS   bool
	MaxStatusChars              int
	MaxRequestBodyBytes         int64
	MediaMaxBytes               int64
	Version                     string

	RateLimitAuthPerWindow   int
	RateLimitAuthWindow      time.Duration
	RateLimitPublicPerWindow int
	RateLimitPublicWindow    time.Duration

	BackfillMaxPages int
	BackfillCooldown time.Duration

	// AccountDeletionGracePeriod is how long a soft-deleted local account stays
	// recoverable before the scheduler purges it. Default 30 days.
	AccountDeletionGracePeriod time.Duration
}

func Load() (*Config, error) {
	var errs []string

	cfg := &Config{
		AppEnv:                 envString("APP_ENV", appEnvDevelopment),
		AppPort:                envInt("APP_PORT", 8080),
		MonsteraInstanceDomain: envStringRequired("MONSTERA_INSTANCE_DOMAIN", &errs),
		MonsteraUIURL:          envRequiredURL("MONSTERA_UI_URL", &errs),
		MonsteraServerURL:      envOptionalURL("MONSTERA_SERVER_URL", &errs),
		LogLevel:               envString("LOG_LEVEL", "info"),

		DatabaseHost:         envStringRequired("DATABASE_HOST", &errs),
		DatabasePort:         envString("DATABASE_PORT", "5432"),
		DatabaseName:         envString("DATABASE_NAME", "monstera"),
		DatabaseUsername:     envString("DATABASE_USERNAME", "monstera"),
		DatabasePassword:     envString("DATABASE_PASSWORD", "monstera"),
		DatabaseMaxOpenConns: envInt("DATABASE_MAX_OPEN_CONNS", 20),
		DatabaseSSLMode:      envString("DATABASE_SSL_MODE", "disable"),

		NATSUrl:       envStringRequired("NATS_URL", &errs),
		NATSCredsFile: envString("NATS_CREDS_FILE", ""),

		CacheDriver: envString("CACHE_DRIVER", "memory"),

		MediaDriver:     envString("MEDIA_DRIVER", "local"),
		MediaLocalPath:  envString("MEDIA_LOCAL_PATH", ""),
		MediaBaseURL:    envStringRequired("MEDIA_BASE_URL", &errs),
		MediaS3Bucket:   envString("MEDIA_S3_BUCKET", ""),
		MediaS3Region:   envString("MEDIA_S3_REGION", ""),
		MediaS3Endpoint: envString("MEDIA_S3_ENDPOINT", ""),
		MediaCDNBase:    envString("MEDIA_CDN_BASE", ""),

		EmailDriver:       envString("EMAIL_DRIVER", "noop"),
		EmailFrom:         envStringRequired("EMAIL_FROM", &errs),
		EmailFromName:     envString("EMAIL_FROM_NAME", "Monstera"),
		EmailSMTPHost:     envString("EMAIL_SMTP_HOST", ""),
		EmailSMTPPort:     envInt("EMAIL_SMTP_PORT", 587),
		EmailSMTPUsername: envString("EMAIL_SMTP_USERNAME", ""),
		EmailSMTPPassword: envString("EMAIL_SMTP_PASSWORD", ""),

		SecretKeyBase: envStringRequired("SECRET_KEY_BASE", &errs),
		MetricsToken:  envString("METRICS_TOKEN", ""),

		VAPIDPrivateKey: envString("VAPID_PRIVATE_KEY", ""),
		VAPIDPublicKey:  envString("VAPID_PUBLIC_KEY", ""),

		FederationWorkerConcurrency: envInt("FEDERATION_WORKER_CONCURRENCY", 5),
		// FederationInsecureSkipTLS defaults to true for development, false for production
		FederationInsecureSkipTLS: envBool("FEDERATION_INSECURE_SKIP_TLS_VERIFY", envString("APP_ENV", appEnvDevelopment) == appEnvDevelopment),
		MaxStatusChars:            envInt("MAX_STATUS_CHARS", 500),
		MaxRequestBodyBytes:       envInt64("MAX_REQUEST_BODY_BYTES", 1048576), // 1 MB
		MediaMaxBytes:             envInt64("MEDIA_MAX_BYTES", 10485760),

		RateLimitAuthPerWindow:   envInt("RATE_LIMIT_AUTH_PER_WINDOW", 300),
		RateLimitAuthWindow:      time.Duration(envInt("RATE_LIMIT_AUTH_WINDOW_SECONDS", 300)) * time.Second,
		RateLimitPublicPerWindow: envInt("RATE_LIMIT_PUBLIC_PER_WINDOW", 300),
		RateLimitPublicWindow:    time.Duration(envInt("RATE_LIMIT_PUBLIC_WINDOW_SECONDS", 300)) * time.Second,
		Version:                  envString("VERSION", "0.0.0-dev"),

		BackfillMaxPages: envInt("BACKFILL_MAX_PAGES", 2),
		BackfillCooldown: time.Duration(envInt("BACKFILL_COOLDOWN_HOURS", 24)) * time.Hour,

		AccountDeletionGracePeriod: time.Duration(envInt("ACCOUNT_DELETION_GRACE_PERIOD_HOURS", 24*30)) * time.Hour,
	}

	if cfg.MonsteraServerURL == nil && cfg.MonsteraInstanceDomain != "" {
		cfg.MonsteraServerURL, _ = url.Parse("https://" + cfg.MonsteraInstanceDomain)
	}

	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "; "))
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	var errs []string

	switch c.AppEnv {
	case appEnvDevelopment, appEnvProduction:
	default:
		errs = append(errs, "APP_ENV must be development or production")
	}

	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, "LOG_LEVEL must be debug, info, warn, or error")
	}

	if c.MediaDriver == "local" && c.MediaLocalPath == "" {
		errs = append(errs, "MEDIA_LOCAL_PATH is required when MEDIA_DRIVER=local")
	}

	if c.MediaDriver == "s3" {
		if c.MediaS3Bucket == "" {
			errs = append(errs, "MEDIA_S3_BUCKET is required when MEDIA_DRIVER=s3")
		}
		if c.MediaS3Region == "" {
			errs = append(errs, "MEDIA_S3_REGION is required when MEDIA_DRIVER=s3")
		}
	}

	if c.EmailDriver == "smtp" && c.EmailSMTPHost == "" {
		errs = append(errs, "EMAIL_SMTP_HOST is required when EMAIL_DRIVER=smtp")
	}

	keyBytes, err := decodeSecretKeyBase(c.SecretKeyBase)
	if err != nil {
		errs = append(errs, err.Error())
	} else if len(keyBytes) < minSecretKeyBytes {
		errs = append(errs, fmt.Sprintf("SECRET_KEY_BASE must be at least %d bytes (decode hex for 64 hex chars)", minSecretKeyBytes))
	}

	if c.MonsteraServerURL != nil && c.MonsteraServerURL.Scheme != "https" {
		errs = append(errs, "MONSTERA_SERVER_URL scheme must be https")
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}

func decodeSecretKeyBase(s string) ([]byte, error) {
	if s == "" {
		return nil, errors.New("SECRET_KEY_BASE is required")
	}
	if _, err := hex.DecodeString(s); err == nil && len(s)%2 == 0 {
		out, err := hex.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("decode SECRET_KEY_BASE hex: %w", err)
		}
		return out, nil
	}
	return []byte(s), nil
}

func (c *Config) IsDevelopment() bool {
	return c.AppEnv == appEnvDevelopment
}

// InstanceBaseURL returns the canonical base URL for this instance (e.g. "https://example.com").
func (c *Config) InstanceBaseURL() string {
	return strings.TrimSuffix(c.MonsteraServerURL.String(), "/")
}

// SecretKeyBytes returns the raw secret key bytes from SECRET_KEY_BASE (hex or raw string).
// Callers must ensure Config has been validated (Load() or Validate() succeeded).
func (c *Config) SecretKeyBytes() ([]byte, error) {
	return decodeSecretKeyBase(c.SecretKeyBase)
}

func envString(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envStringRequired(key string, errs *[]string) string {
	v := os.Getenv(key)
	if v == "" {
		*errs = append(*errs, key+" is required")
		return ""
	}
	return v
}

func envInt(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

func envInt64(key string, defaultVal int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return defaultVal
	}
	return n
}

func envBool(key string, defaultVal bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "true", "1":
		return true
	case "false", "0":
		return false
	default:
		return defaultVal
	}
}

func envRequiredURL(key string, errs *[]string) *url.URL {
	v := os.Getenv(key)
	if v == "" {
		*errs = append(*errs, key+" is required")
		return nil
	}
	u, err := url.Parse(v)
	if err != nil {
		*errs = append(*errs, key+" is invalid: "+err.Error())
		return nil
	}
	return u
}

func envOptionalURL(key string, errs *[]string) *url.URL {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	u, err := url.Parse(v)
	if err != nil {
		*errs = append(*errs, key+" is invalid: "+err.Error())
		return nil
	}
	return u
}
