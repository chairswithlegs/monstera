package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setEnv(t *testing.T, key, value string) {
	t.Helper()
	t.Setenv(key, value)
}

func setRequiredEnvs(t *testing.T) {
	t.Helper()
	setEnv(t, "INSTANCE_DOMAIN", "test.example.com")
	setEnv(t, "DATABASE_URL", "postgres://localhost/test")
	setEnv(t, "NATS_URL", "nats://localhost:4222")
	setEnv(t, "MEDIA_BASE_URL", "https://test.example.com/media")
	setEnv(t, "MEDIA_LOCAL_PATH", "/tmp/media")
	setEnv(t, "EMAIL_FROM", "noreply@test.example.com")
	setEnv(t, "SECRET_KEY_BASE", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef") // 32 bytes hex
}

func TestLoad_success(t *testing.T) {
	setRequiredEnvs(t)

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "development", cfg.AppEnv)
	assert.Equal(t, 8080, cfg.AppPort)
	assert.Equal(t, "test.example.com", cfg.InstanceDomain)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "memory", cfg.CacheDriver)
	assert.Equal(t, "local", cfg.MediaDriver)
	assert.Equal(t, 500, cfg.MaxStatusChars)
	assert.Equal(t, int64(10485760), cfg.MediaMaxBytes)
}

func TestLoad_defaults(t *testing.T) {
	setRequiredEnvs(t)
	setEnv(t, "APP_PORT", "3000")
	setEnv(t, "LOG_LEVEL", "debug")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 3000, cfg.AppPort)
	assert.Equal(t, "debug", cfg.LogLevel)
}

func TestLoad_missingRequired(t *testing.T) {
	setRequiredEnvs(t)
	setEnv(t, "SECRET_KEY_BASE", "") // clear the required secret

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SECRET_KEY_BASE")
}

func TestValidate_appEnv(t *testing.T) {
	setRequiredEnvs(t)
	setEnv(t, "APP_ENV", "staging")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "APP_ENV")
}

func TestValidate_logLevel(t *testing.T) {
	setRequiredEnvs(t)
	setEnv(t, "LOG_LEVEL", "trace")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LOG_LEVEL")
}

func TestValidate_cacheRedisRequired(t *testing.T) {
	setRequiredEnvs(t)
	setEnv(t, "CACHE_DRIVER", "redis")
	setEnv(t, "CACHE_REDIS_URL", "")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CACHE_REDIS_URL")
}

func TestValidate_mediaLocalPathRequired(t *testing.T) {
	setRequiredEnvs(t)
	setEnv(t, "MEDIA_DRIVER", "local")
	setEnv(t, "MEDIA_LOCAL_PATH", "")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MEDIA_LOCAL_PATH")
}

func TestValidate_mediaS3Required(t *testing.T) {
	setRequiredEnvs(t)
	setEnv(t, "MEDIA_DRIVER", "s3")
	setEnv(t, "MEDIA_S3_BUCKET", "")
	setEnv(t, "MEDIA_S3_REGION", "")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MEDIA_S3")
}

func TestValidate_emailSMTPRequired(t *testing.T) {
	setRequiredEnvs(t)
	setEnv(t, "EMAIL_DRIVER", "smtp")
	setEnv(t, "EMAIL_SMTP_HOST", "")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "EMAIL_SMTP_HOST")
}

func TestValidate_secretKeyBaseTooShort(t *testing.T) {
	setRequiredEnvs(t)
	setEnv(t, "SECRET_KEY_BASE", "short")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SECRET_KEY_BASE")
}

func TestIsDevelopment(t *testing.T) {
	setRequiredEnvs(t)

	setEnv(t, "APP_ENV", "development")
	cfg, err := Load()
	require.NoError(t, err)
	assert.True(t, cfg.IsDevelopment())

	setEnv(t, "APP_ENV", "production")
	cfg, err = Load()
	require.NoError(t, err)
	assert.False(t, cfg.IsDevelopment())
}

func TestDeriveKey_deterministic(t *testing.T) {
	setRequiredEnvs(t)
	cfg, err := Load()
	require.NoError(t, err)

	key1 := cfg.DeriveKey("monstera-fed-csrf", 32)
	key2 := cfg.DeriveKey("monstera-fed-csrf", 32)
	assert.Equal(t, key1, key2)
	assert.Len(t, key1, 32)
}

func TestDeriveKey_differentPurposeDifferentOutput(t *testing.T) {
	setRequiredEnvs(t)
	cfg, err := Load()
	require.NoError(t, err)

	key1 := cfg.DeriveKey("monstera-fed-csrf", 32)
	key2 := cfg.DeriveKey("monstera-fed-email-token", 32)
	assert.NotEqual(t, key1, key2)
}
