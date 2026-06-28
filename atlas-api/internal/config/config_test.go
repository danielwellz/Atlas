package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/atlas/atlas-api/internal/config"
	"github.com/stretchr/testify/require"
)

var configKeys = []string{
	"LOAD_DOTENV",
	"APP_ENV",
	"API_SERVER_ADDR",
	"APP_NAME",
	"LOG_LEVEL",
	"POSTGRES_URL",
	"REDIS_ADDR",
	"ASSET_STORAGE_BACKEND",
	"ASSET_STORAGE_BUCKET",
	"MINIO_ENDPOINT",
	"MINIO_ROOT_USER",
	"MINIO_ROOT_PASSWORD",
	"MINIO_USE_SSL",
	"MAILHOG_SMTP_ADDR",
	"JWT_SECRET",
	"ACCESS_TOKEN_TTL_MINUTES",
	"REFRESH_TOKEN_TTL_HOURS",
	"USDA_API_BASE_URL",
	"USDA_API_KEY",
	"EDAMAM_API_BASE_URL",
	"EDAMAM_APP_ID",
	"EDAMAM_APP_KEY",
	"FOOD_DETAILS_CACHE_TTL_MINUTES",
	"PRO_ALL_USERS",
	"PRO_USER_EMAILS",
}

func TestLoadLocalDefaults(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_ENV", "local")

	cfg, err := config.Load()
	require.NoError(t, err)

	require.Equal(t, "local", cfg.Env)
	require.Equal(t, ":8080", cfg.ServerAddr)
	require.Equal(t, "atlas-api", cfg.ServiceName)
	require.Equal(t, "info", cfg.LogLevel)
	require.Equal(t, "postgres://atlas:atlas@localhost:5432/atlas?sslmode=disable", cfg.PostgresURL)
	require.Equal(t, "localhost:6379", cfg.RedisAddr)
	require.Equal(t, "local", cfg.AssetStorageBackend)
	require.Equal(t, "atlas-assets", cfg.AssetStorageBucket)
	require.Equal(t, "localhost:9000", cfg.MinioEndpoint)
	require.Equal(t, "atlasminio", cfg.MinioAccess)
	require.Equal(t, "atlasminio", cfg.MinioSecret)
	require.False(t, cfg.MinioUseSSL)
	require.Equal(t, "localhost:1025", cfg.MailhogSMTP)
	require.Equal(t, "atlas-local-dev-secret", cfg.JWTSecret)
	require.Equal(t, 15, cfg.AccessTokenTTLMinutes)
	require.Equal(t, 720, cfg.RefreshTokenTTLHours)
	require.Equal(t, "https://api.nal.usda.gov/fdc/v1", cfg.USDAAPIBaseURL)
	require.Equal(t, "DEMO_KEY", cfg.USDAAPIKey)
	require.Equal(t, "https://api.edamam.com", cfg.EdamamAPIBaseURL)
	require.Equal(t, 60, cfg.FoodDetailsCacheTTLM)
	require.Equal(t, 15*time.Minute, cfg.AccessTokenTTL())
	require.Equal(t, 720*time.Hour, cfg.RefreshTokenTTL())
}

func TestLoadRejectsInvalidEnv(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_ENV", "development")

	_, err := config.Load()
	require.Error(t, err)
	require.Contains(t, err.Error(), "APP_ENV must be one of")
}

func TestLoadRequiresVarsForStaging(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_ENV", "staging")

	_, err := config.Load()
	require.Error(t, err)
	require.Contains(t, err.Error(), "POSTGRES_URL is required when APP_ENV=staging")
	require.Contains(t, err.Error(), "JWT_SECRET is required when APP_ENV=staging")
	require.Contains(t, err.Error(), "APP_NAME is required when APP_ENV=staging")
}

func TestLoadStagingWithRequiredVars(t *testing.T) {
	clearConfigEnv(t)
	setNonLocalRequiredEnv(t, "staging")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, "staging", cfg.Env)
	require.Equal(t, "atlas-api", cfg.ServiceName)
	require.Equal(t, "s3", cfg.AssetStorageBackend)
	require.True(t, cfg.MinioUseSSL)
}

func TestLoadRejectsLocalSecretsInProd(t *testing.T) {
	clearConfigEnv(t)
	setNonLocalRequiredEnv(t, "prod")
	t.Setenv("JWT_SECRET", "atlas-local-dev-secret")
	t.Setenv("USDA_API_KEY", "DEMO_KEY")

	_, err := config.Load()
	require.Error(t, err)
	require.Contains(t, err.Error(), "JWT_SECRET must not use the local default")
	require.Contains(t, err.Error(), "USDA_API_KEY must not use DEMO_KEY")
}

func TestLoadRejectsInvalidNumericAndBoolValues(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_ENV", "local")
	t.Setenv("ACCESS_TOKEN_TTL_MINUTES", "abc")
	t.Setenv("MINIO_USE_SSL", "not-a-bool")

	_, err := config.Load()
	require.Error(t, err)
	require.Contains(t, err.Error(), "ACCESS_TOKEN_TTL_MINUTES must be an integer")
	require.Contains(t, err.Error(), "MINIO_USE_SSL must be a boolean")
}

func TestLoadDotenvForLocal(t *testing.T) {
	clearConfigEnv(t)
	unsetConfigKey(t, "APP_ENV")
	unsetConfigKey(t, "APP_NAME")
	unsetConfigKey(t, "POSTGRES_URL")
	tempDir := t.TempDir()
	err := os.WriteFile(
		filepath.Join(tempDir, ".env"),
		[]byte("APP_ENV=local\nAPP_NAME=dotenv-service\nPOSTGRES_URL=postgres://dotenv\n"),
		0o600,
	)
	require.NoError(t, err)

	originalWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(originalWD))
	})

	cfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, "local", cfg.Env)
	require.Equal(t, "dotenv-service", cfg.ServiceName)
	require.Equal(t, "postgres://dotenv", cfg.PostgresURL)
}

func TestLoadSkipsDotenvForStaging(t *testing.T) {
	clearConfigEnv(t)
	unsetConfigKey(t, "APP_NAME")
	tempDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tempDir, ".env"), []byte("APP_NAME=dotenv-service\n"), 0o600)
	require.NoError(t, err)

	originalWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(originalWD))
	})

	t.Setenv("APP_ENV", "staging")
	t.Setenv("API_SERVER_ADDR", ":8080")

	_, err = config.Load()
	require.Error(t, err)
	require.Contains(t, err.Error(), "APP_NAME is required when APP_ENV=staging")
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range configKeys {
		t.Setenv(key, "")
	}
}

func unsetConfigKey(t *testing.T, key string) {
	t.Helper()
	originalValue, hadValue := os.LookupEnv(key)
	require.NoError(t, os.Unsetenv(key))
	t.Cleanup(func() {
		if hadValue {
			require.NoError(t, os.Setenv(key, originalValue))
			return
		}
		require.NoError(t, os.Unsetenv(key))
	})
}

func setNonLocalRequiredEnv(t *testing.T, env string) {
	t.Helper()
	t.Setenv("APP_ENV", env)
	t.Setenv("API_SERVER_ADDR", ":8080")
	t.Setenv("APP_NAME", "atlas-api")
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("POSTGRES_URL", "postgres://atlas:atlas@staging-db:5432/atlas?sslmode=disable")
	t.Setenv("REDIS_ADDR", "staging-redis:6379")
	t.Setenv("ASSET_STORAGE_BACKEND", "s3")
	t.Setenv("ASSET_STORAGE_BUCKET", "atlas-assets")
	t.Setenv("MINIO_ENDPOINT", "s3.us-east-1.amazonaws.com")
	t.Setenv("MINIO_ROOT_USER", "atlas-access-key")
	t.Setenv("MINIO_ROOT_PASSWORD", "atlas-secret-key")
	t.Setenv("MINIO_USE_SSL", "true")
	t.Setenv("JWT_SECRET", "super-secure-production-secret")
	t.Setenv("ACCESS_TOKEN_TTL_MINUTES", "15")
	t.Setenv("REFRESH_TOKEN_TTL_HOURS", "720")
	t.Setenv("USDA_API_BASE_URL", "https://api.nal.usda.gov/fdc/v1")
	t.Setenv("USDA_API_KEY", "usda-live-key")
	t.Setenv("EDAMAM_API_BASE_URL", "https://api.edamam.com")
	t.Setenv("EDAMAM_APP_ID", "edamam-app-id")
	t.Setenv("EDAMAM_APP_KEY", "edamam-app-key")
	t.Setenv("FOOD_DETAILS_CACHE_TTL_MINUTES", "60")
}
