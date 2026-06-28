package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	EnvLocal   = "local"
	EnvStaging = "staging"
	EnvProd    = "prod"
)

var validEnvironments = map[string]struct{}{
	EnvLocal:   {},
	EnvStaging: {},
	EnvProd:    {},
}

var validAssetStorageBackends = map[string]struct{}{
	"local": {},
	"minio": {},
	"s3":    {},
}

// Config stores runtime configuration loaded from environment variables.
type Config struct {
	Env                   string
	ServerAddr            string
	ServiceName           string
	LogLevel              string
	PostgresURL           string
	RedisAddr             string
	AssetStorageBackend   string
	AssetStorageBucket    string
	MinioEndpoint         string
	MinioAccess           string
	MinioSecret           string
	MinioUseSSL           bool
	MailhogSMTP           string
	JWTSecret             string
	AccessTokenTTLMinutes int
	RefreshTokenTTLHours  int
	USDAAPIBaseURL        string
	USDAAPIKey            string
	EdamamAPIBaseURL      string
	EdamamAppID           string
	EdamamAppKey          string
	FoodDetailsCacheTTLM  int
	ProAllUsers           bool
	ProUserEmails         []string
}

func Load() (Config, error) {
	loadDotenvIfNeeded()

	env := normalizeEnv(os.Getenv("APP_ENV"))
	validation := &configValidationErrors{environment: env}
	if _, ok := validEnvironments[env]; !ok {
		validation.addf("APP_ENV must be one of %q, %q, %q (got %q)", EnvLocal, EnvStaging, EnvProd, env)
	}

	cfg := Config{
		Env:                   env,
		ServerAddr:            readString("API_SERVER_ADDR", env, ":8080", true, validation),
		ServiceName:           readString("APP_NAME", env, "atlas-api", true, validation),
		LogLevel:              readString("LOG_LEVEL", env, "info", true, validation),
		PostgresURL:           readString("POSTGRES_URL", env, "postgres://atlas:atlas@localhost:5432/atlas?sslmode=disable", true, validation),
		RedisAddr:             readString("REDIS_ADDR", env, "localhost:6379", true, validation),
		AssetStorageBackend:   strings.ToLower(readString("ASSET_STORAGE_BACKEND", env, "local", true, validation)),
		MailhogSMTP:           readString("MAILHOG_SMTP_ADDR", env, "localhost:1025", false, validation),
		JWTSecret:             readString("JWT_SECRET", env, "atlas-local-dev-secret", true, validation),
		AccessTokenTTLMinutes: readInt("ACCESS_TOKEN_TTL_MINUTES", env, 15, true, validation),
		RefreshTokenTTLHours:  readInt("REFRESH_TOKEN_TTL_HOURS", env, 720, true, validation),
		USDAAPIBaseURL:        readString("USDA_API_BASE_URL", env, "https://api.nal.usda.gov/fdc/v1", true, validation),
		USDAAPIKey:            readString("USDA_API_KEY", env, "DEMO_KEY", true, validation),
		EdamamAPIBaseURL:      readString("EDAMAM_API_BASE_URL", env, "https://api.edamam.com", true, validation),
		EdamamAppID:           strings.TrimSpace(readString("EDAMAM_APP_ID", env, "", env != EnvLocal, validation)),
		EdamamAppKey:          strings.TrimSpace(readString("EDAMAM_APP_KEY", env, "", env != EnvLocal, validation)),
		FoodDetailsCacheTTLM:  readInt("FOOD_DETAILS_CACHE_TTL_MINUTES", env, 60, true, validation),
		ProAllUsers:           readBool("PRO_ALL_USERS", env, false, false, validation),
		ProUserEmails:         parseCSVEnv("PRO_USER_EMAILS"),
	}

	if cfg.AssetStorageBackend != "" {
		if _, ok := validAssetStorageBackends[cfg.AssetStorageBackend]; !ok {
			validation.addf("ASSET_STORAGE_BACKEND must be one of \"local\", \"minio\", \"s3\" (got %q)", cfg.AssetStorageBackend)
		}
	} else if env == EnvLocal {
		cfg.AssetStorageBackend = "local"
	}

	needsObjectStorageCreds := cfg.AssetStorageBackend == "minio" || cfg.AssetStorageBackend == "s3"
	cfg.AssetStorageBucket = readString("ASSET_STORAGE_BUCKET", env, "atlas-assets", needsObjectStorageCreds, validation)
	cfg.MinioEndpoint = readString("MINIO_ENDPOINT", env, "localhost:9000", needsObjectStorageCreds, validation)
	cfg.MinioAccess = readString("MINIO_ROOT_USER", env, "atlasminio", needsObjectStorageCreds, validation)
	cfg.MinioSecret = readString("MINIO_ROOT_PASSWORD", env, "atlasminio", needsObjectStorageCreds, validation)
	cfg.MinioUseSSL = readBool("MINIO_USE_SSL", env, false, needsObjectStorageCreds, validation)

	accessTTLWasSet := env == EnvLocal
	if _, ok := lookupEnvTrimmed("ACCESS_TOKEN_TTL_MINUTES"); ok {
		accessTTLWasSet = true
	}
	refreshTTLWasSet := env == EnvLocal
	if _, ok := lookupEnvTrimmed("REFRESH_TOKEN_TTL_HOURS"); ok {
		refreshTTLWasSet = true
	}
	cacheTTLWasSet := env == EnvLocal
	if _, ok := lookupEnvTrimmed("FOOD_DETAILS_CACHE_TTL_MINUTES"); ok {
		cacheTTLWasSet = true
	}

	if accessTTLWasSet && cfg.AccessTokenTTLMinutes <= 0 {
		validation.addf("ACCESS_TOKEN_TTL_MINUTES must be greater than 0")
	}
	if refreshTTLWasSet && cfg.RefreshTokenTTLHours <= 0 {
		validation.addf("REFRESH_TOKEN_TTL_HOURS must be greater than 0")
	}
	if cacheTTLWasSet && cfg.FoodDetailsCacheTTLM < 0 {
		validation.addf("FOOD_DETAILS_CACHE_TTL_MINUTES must be 0 or greater")
	}

	if env != EnvLocal {
		if cfg.JWTSecret == "atlas-local-dev-secret" {
			validation.addf("JWT_SECRET must not use the local default when APP_ENV=%s", env)
		}
		if strings.EqualFold(cfg.USDAAPIKey, "DEMO_KEY") {
			validation.addf("USDA_API_KEY must not use DEMO_KEY when APP_ENV=%s", env)
		}
	}

	if err := validation.err(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) AccessTokenTTL() time.Duration {
	return time.Duration(c.AccessTokenTTLMinutes) * time.Minute
}

func (c Config) RefreshTokenTTL() time.Duration {
	return time.Duration(c.RefreshTokenTTLHours) * time.Hour
}

func (c Config) FoodDetailsCacheTTL() time.Duration {
	if c.FoodDetailsCacheTTLM <= 0 {
		return 0
	}
	return time.Duration(c.FoodDetailsCacheTTLM) * time.Minute
}

func (c Config) Redacted() Config {
	redacted := c
	redacted.JWTSecret = redactSecret(redacted.JWTSecret)
	redacted.MinioSecret = redactSecret(redacted.MinioSecret)
	redacted.USDAAPIKey = redactSecret(redacted.USDAAPIKey)
	redacted.EdamamAppID = redactSecret(redacted.EdamamAppID)
	redacted.EdamamAppKey = redactSecret(redacted.EdamamAppKey)
	return redacted
}

func redactSecret(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return "***REDACTED***"
}

func normalizeEnv(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return EnvLocal
	}
	return trimmed
}

type configValidationErrors struct {
	environment string
	issues      []string
}

func (v *configValidationErrors) addf(format string, args ...interface{}) {
	v.issues = append(v.issues, fmt.Sprintf(format, args...))
}

func (v *configValidationErrors) err() error {
	if len(v.issues) == 0 {
		return nil
	}
	return fmt.Errorf(
		"invalid configuration for APP_ENV=%s:\n - %s",
		v.environment,
		strings.Join(v.issues, "\n - "),
	)
}

func parseBool(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "t", "true", "yes", "y", "on":
		return true, true
	case "0", "f", "false", "no", "n", "off":
		return false, true
	default:
		return false, false
	}
}

func readString(key, env, localDefault string, requiredOutsideLocal bool, validation *configValidationErrors) string {
	if value, ok := lookupEnvTrimmed(key); ok {
		return value
	}

	if env == EnvLocal {
		return localDefault
	}
	if requiredOutsideLocal {
		validation.addf("%s is required when APP_ENV=%s", key, env)
	}
	return ""
}

func readInt(key, env string, localDefault int, requiredOutsideLocal bool, validation *configValidationErrors) int {
	value, ok := lookupEnvTrimmed(key)
	if !ok {
		if env == EnvLocal {
			return localDefault
		}
		if requiredOutsideLocal {
			validation.addf("%s is required when APP_ENV=%s", key, env)
		}
		return 0
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		validation.addf("%s must be an integer (got %q)", key, value)
		return 0
	}
	return parsed
}

func readBool(key, env string, localDefault bool, requiredOutsideLocal bool, validation *configValidationErrors) bool {
	value, ok := lookupEnvTrimmed(key)
	if !ok {
		if env == EnvLocal {
			return localDefault
		}
		if requiredOutsideLocal {
			validation.addf("%s is required when APP_ENV=%s", key, env)
		}
		return false
	}

	parsed, valid := parseBool(value)
	if !valid {
		validation.addf("%s must be a boolean (got %q)", key, value)
		return false
	}
	return parsed
}

func lookupEnvTrimmed(key string) (string, bool) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return "", false
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}
	return trimmed, true
}

func loadDotenvIfNeeded() {
	if value, ok := lookupEnvTrimmed("LOAD_DOTENV"); ok {
		if parsed, valid := parseBool(value); valid && parsed {
			_ = godotenv.Load()
			return
		}
	}

	// Local defaults are intentionally easy to bootstrap from .env.
	if normalizeEnv(os.Getenv("APP_ENV")) == EnvLocal {
		_ = godotenv.Load()
	}
}

func parseCSVEnv(key string) []string {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return []string{}
	}

	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized := strings.TrimSpace(strings.ToLower(part))
		if normalized == "" {
			continue
		}
		values = append(values, normalized)
	}
	return values
}
