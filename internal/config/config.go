package config

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config holds all production standalone configuration.
// Not used when running under Encore locally (Encore manages infra config).
type Config struct {
	Server       ServerConfig
	Database     DatabaseConfig
	Redis        RedisConfig
	AuthProvider AuthProviderConfig
	CORS         CORSConfig
	OTel         OTelConfig
	RateLimit    RateLimitConfig
}

type ServerConfig struct {
	Port            int           `envconfig:"PORT" default:"5080"`
	Env             string        `envconfig:"ENV" default:"development"`
	ShutdownTimeout time.Duration `envconfig:"SHUTDOWN_TIMEOUT" default:"30s"`
}

type DatabaseConfig struct {
	URL      string `envconfig:"DATABASE_URL" required:"true"`
	MaxConns int    `envconfig:"DATABASE_MAX_CONNS" default:"25"`
	MinConns int    `envconfig:"DATABASE_MIN_CONNS" default:"5"`
}

type RedisConfig struct {
	URL string `envconfig:"REDIS_URL" required:"true"`
}

// AuthProviderConfig selects and configures the active identity provider (Vault).
// Local/dev: supabase or clerk. Production: keycloak.
type AuthProviderConfig struct {
	Provider string `envconfig:"AUTH_PROVIDER" default:"supabase"`
	JWKSURL  string `envconfig:"JWKS_URL" required:"true"`

	// Supabase (local/dev)
	SupabaseURL            string `envconfig:"SUPABASE_URL"`
	SupabaseAnonKey        string `envconfig:"SUPABASE_ANON_KEY"`
	SupabaseServiceRoleKey string `envconfig:"SUPABASE_SERVICE_ROLE_KEY"`

	// Clerk (local/dev alternative)
	ClerkSecretKey string `envconfig:"CLERK_SECRET_KEY"`

	// Keycloak (production)
	KeycloakURL               string `envconfig:"KEYCLOAK_URL"`
	KeycloakRealm             string `envconfig:"KEYCLOAK_REALM"`
	KeycloakClientID          string `envconfig:"KEYCLOAK_CLIENT_ID"`
	KeycloakAdminClientID     string `envconfig:"KEYCLOAK_ADMIN_CLIENT_ID"`
	KeycloakAdminClientSecret string `envconfig:"KEYCLOAK_ADMIN_CLIENT_SECRET"`
}

type CORSConfig struct {
	AllowedOrigins []string `envconfig:"CORS_ALLOWED_ORIGINS" default:"http://localhost:3000"`
}

type OTelConfig struct {
	Endpoint     string  `envconfig:"OTEL_EXPORTER_OTLP_ENDPOINT" default:"http://localhost:4317"`
	ServiceName  string  `envconfig:"OTEL_SERVICE_NAME" default:"vital-api"`
	SamplerRatio float64 `envconfig:"OTEL_TRACES_SAMPLER_ARG" default:"1.0"`
}

type RateLimitConfig struct {
	Auth    int `envconfig:"RATE_LIMIT_AUTH" default:"10"`
	General int `envconfig:"RATE_LIMIT_GENERAL" default:"100"`
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return &cfg, nil
}

// IsDevelopment returns true when running in the development environment.
func (c *Config) IsDevelopment() bool {
	return strings.EqualFold(c.Server.Env, "development")
}

// LogLevel returns the slog level appropriate for the current environment.
func (c *Config) LogLevel() slog.Level {
	if c.IsDevelopment() {
		return slog.LevelDebug
	}
	return slog.LevelInfo
}

// Addr returns the TCP address the HTTP server should bind to.
func (c *ServerConfig) Addr() string {
	return fmt.Sprintf(":%d", c.Port)
}
