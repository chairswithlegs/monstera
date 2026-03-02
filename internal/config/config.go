package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/crypto/hkdf"
)

const minSecretKeyBytes = 32

const appEnvDevelopment = "development"
const appEnvProduction = "production"

type Config struct {
	AppEnv         string
	AppPort        int
	InstanceDomain string
	UIDomain       string // UIDomain defaults to InstanceDomain if not set
	InstanceName   string
	LogLevel       string

	DatabaseURL          string
	DatabaseMaxOpenConns int
	DatabaseMaxIdleConns int

	NATSUrl       string
	NATSCredsFile string

	CacheDriver   string
	CacheRedisURL string

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

	FederationWorkerConcurrency int
	FederationInsecureSkipTLS   bool
	MaxStatusChars              int
	MediaMaxBytes               int64
	Version                     string
}

func Load() (*Config, error) {
	var errs []string

	cfg := &Config{
		AppEnv:         envString("APP_ENV", appEnvDevelopment),
		AppPort:        envInt("APP_PORT", 8080),
		InstanceDomain: envStringRequired("INSTANCE_DOMAIN", &errs),
		UIDomain:       envString("UI_DOMAIN", ""),
		InstanceName:   envString("INSTANCE_NAME", "Monstera-fed"),
		LogLevel:       envString("LOG_LEVEL", "info"),

		DatabaseURL:          envStringRequired("DATABASE_URL", &errs),
		DatabaseMaxOpenConns: envInt("DATABASE_MAX_OPEN_CONNS", 20),
		DatabaseMaxIdleConns: envInt("DATABASE_MAX_IDLE_CONNS", 5),

		NATSUrl:       envStringRequired("NATS_URL", &errs),
		NATSCredsFile: envString("NATS_CREDS_FILE", ""),

		CacheDriver:   envString("CACHE_DRIVER", "memory"),
		CacheRedisURL: envString("CACHE_REDIS_URL", ""),

		MediaDriver:     envString("MEDIA_DRIVER", "local"),
		MediaLocalPath:  envString("MEDIA_LOCAL_PATH", ""),
		MediaBaseURL:    envStringRequired("MEDIA_BASE_URL", &errs),
		MediaS3Bucket:   envString("MEDIA_S3_BUCKET", ""),
		MediaS3Region:   envString("MEDIA_S3_REGION", ""),
		MediaS3Endpoint: envString("MEDIA_S3_ENDPOINT", ""),
		MediaCDNBase:    envString("MEDIA_CDN_BASE", ""),

		EmailDriver:       envString("EMAIL_DRIVER", "noop"),
		EmailFrom:         envStringRequired("EMAIL_FROM", &errs),
		EmailFromName:     envString("EMAIL_FROM_NAME", "Monstera-fed"),
		EmailSMTPHost:     envString("EMAIL_SMTP_HOST", ""),
		EmailSMTPPort:     envInt("EMAIL_SMTP_PORT", 587),
		EmailSMTPUsername: envString("EMAIL_SMTP_USERNAME", ""),
		EmailSMTPPassword: envString("EMAIL_SMTP_PASSWORD", ""),

		SecretKeyBase: envStringRequired("SECRET_KEY_BASE", &errs),
		MetricsToken:  envString("METRICS_TOKEN", ""),

		FederationWorkerConcurrency: envInt("FEDERATION_WORKER_CONCURRENCY", 5),
		// FederationInsecureSkipTLS defaults to true for development, false for production
		FederationInsecureSkipTLS: envBool("FEDERATION_INSECURE_SKIP_TLS_VERIFY", envString("APP_ENV", appEnvDevelopment) == appEnvDevelopment),
		MaxStatusChars:            envInt("MAX_STATUS_CHARS", 500),
		MediaMaxBytes:             envInt64("MEDIA_MAX_BYTES", 10485760),
		Version:                   envString("VERSION", "0.0.0-dev"),
	}

	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "; "))
	}

	if cfg.UIDomain == "" {
		cfg.UIDomain = cfg.InstanceDomain
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

	if c.CacheDriver == "redis" && c.CacheRedisURL == "" {
		errs = append(errs, "CACHE_REDIS_URL is required when CACHE_DRIVER=redis")
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

// SecretKeyBytes returns the raw secret key bytes from SECRET_KEY_BASE (hex or raw string).
// Callers must ensure Config has been validated (Load() or Validate() succeeded).
func (c *Config) SecretKeyBytes() ([]byte, error) {
	return decodeSecretKeyBase(c.SecretKeyBase)
}

func (c *Config) DeriveKey(purpose string, length int) []byte {
	keyBytes, _ := decodeSecretKeyBase(c.SecretKeyBase)
	if len(keyBytes) < minSecretKeyBytes {
		keyBytes = []byte(c.SecretKeyBase)
	}
	extractor := hkdf.Extract(sha256.New, keyBytes, nil)
	expander := hkdf.Expand(sha256.New, extractor, []byte(purpose))
	out := make([]byte, length)
	_, _ = expander.Read(out)
	return out
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
