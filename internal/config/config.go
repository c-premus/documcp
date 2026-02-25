// Package config loads and validates application configuration from environment
// variables and optional YAML config files. It supports both unprefixed env vars
// (DB_HOST, REDIS_HOST) matching the PHP convention and DOCUMCP_-prefixed variants.
package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds the complete application configuration.
type Config struct {
	App         AppConfig
	Server      ServerConfig
	Database    DatabaseConfig
	Redis       RedisConfig
	Meilisearch MeilisearchConfig
	OIDC        OIDCConfig
	OAuth       OAuthConfig
	Storage     StorageConfig
	OTEL        OTELConfig
	DocuMCP     DocuMCPConfig
}

// AppConfig holds general application settings.
type AppConfig struct {
	Name     string `mapstructure:"app_name"`
	Env      string `mapstructure:"app_env"`
	Debug    bool   `mapstructure:"app_debug"`
	URL      string `mapstructure:"app_url"`
	Timezone string `mapstructure:"app_timezone"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host           string        `mapstructure:"server_host"`
	Port           int           `mapstructure:"server_port"`
	TrustedProxies []string      `mapstructure:"trusted_proxies"`
	ReadTimeout    time.Duration `mapstructure:"server_read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"server_write_timeout"`
	IdleTimeout    time.Duration `mapstructure:"server_idle_timeout"`
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host         string        `mapstructure:"db_host"`
	Port         int           `mapstructure:"db_port"`
	Database     string        `mapstructure:"db_database"`
	Username     string        `mapstructure:"db_username"`
	Password     string        `mapstructure:"db_password"`
	SSLMode      string        `mapstructure:"db_sslmode"`
	MaxOpenConns int           `mapstructure:"db_max_open_conns"`
	MaxIdleConns int           `mapstructure:"db_max_idle_conns"`
	MaxLifetime  time.Duration `mapstructure:"db_max_lifetime"`
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Host     string `mapstructure:"redis_host"`
	Port     int    `mapstructure:"redis_port"`
	Password string `mapstructure:"redis_password"`
	DB       int    `mapstructure:"redis_db"`
}

// MeilisearchConfig holds Meilisearch connection settings.
type MeilisearchConfig struct {
	Host string `mapstructure:"meilisearch_host"`
	Key  string `mapstructure:"meilisearch_key"`
}

// OIDCConfig holds OpenID Connect provider settings.
type OIDCConfig struct {
	ProviderURL  string   `mapstructure:"oidc_provider_url"`
	ClientID     string   `mapstructure:"oidc_client_id"`
	ClientSecret string   `mapstructure:"oidc_client_secret"`
	RedirectURL  string   `mapstructure:"oidc_redirect_uri"`
	Scopes       []string `mapstructure:"oidc_scopes"`
}

// OAuthConfig holds OAuth 2.1 authorization server settings.
type OAuthConfig struct {
	AuthCodeLifetime    time.Duration `mapstructure:"oauth_authorization_code_lifetime"`
	AccessTokenLifetime time.Duration `mapstructure:"oauth_access_token_lifetime"`
	RefreshTokenLifetime time.Duration `mapstructure:"oauth_refresh_token_lifetime"`
	DeviceCodeLifetime  time.Duration `mapstructure:"oauth_device_code_lifetime"`
	DeviceCodeInterval  time.Duration `mapstructure:"oauth_device_polling_interval"`
	RequirePKCE         bool          `mapstructure:"oauth_pkce_required"`
}

// StorageConfig holds file storage settings.
type StorageConfig struct {
	Driver       string `mapstructure:"storage_driver"`
	BasePath     string `mapstructure:"storage_base_path"`
	DocumentPath string `mapstructure:"storage_document_path"`
	TempPath     string `mapstructure:"storage_temp_path"`
}

// OTELConfig holds OpenTelemetry observability settings.
type OTELConfig struct {
	Enabled     bool   `mapstructure:"otel_enabled"`
	Endpoint    string `mapstructure:"otel_exporter_otlp_endpoint"`
	ServiceName string `mapstructure:"otel_service_name"`
	Insecure    bool   `mapstructure:"otel_insecure"`
}

// DocuMCPConfig holds MCP server-specific settings.
type DocuMCPConfig struct {
	Endpoint      string `mapstructure:"documcp_endpoint"`
	ServerName    string `mapstructure:"documcp_name"`
	ServerVersion string `mapstructure:"documcp_version"`
}

// setDefaults registers all default values with viper.
func setDefaults(v *viper.Viper) {
	// App
	v.SetDefault("app_name", "DocuMCP")
	v.SetDefault("app_env", "production")
	v.SetDefault("app_debug", false)
	v.SetDefault("app_url", "http://localhost")
	v.SetDefault("app_timezone", "UTC")

	// Server
	v.SetDefault("server_host", "0.0.0.0")
	v.SetDefault("server_port", 8080)
	v.SetDefault("trusted_proxies", []string{})
	v.SetDefault("server_read_timeout", 30*time.Second)
	v.SetDefault("server_write_timeout", 30*time.Second)
	v.SetDefault("server_idle_timeout", 120*time.Second)

	// Database
	v.SetDefault("db_host", "127.0.0.1")
	v.SetDefault("db_port", 5432)
	v.SetDefault("db_database", "")
	v.SetDefault("db_username", "")
	v.SetDefault("db_password", "")
	v.SetDefault("db_sslmode", "disable")
	v.SetDefault("db_max_open_conns", 25)
	v.SetDefault("db_max_idle_conns", 5)
	v.SetDefault("db_max_lifetime", 5*time.Minute)

	// Redis
	v.SetDefault("redis_host", "localhost")
	v.SetDefault("redis_port", 6379)
	v.SetDefault("redis_password", "")
	v.SetDefault("redis_db", 0)

	// Meilisearch
	v.SetDefault("meilisearch_host", "http://localhost:7700")
	v.SetDefault("meilisearch_key", "")

	// OIDC
	v.SetDefault("oidc_provider_url", "")
	v.SetDefault("oidc_client_id", "")
	v.SetDefault("oidc_client_secret", "")
	v.SetDefault("oidc_redirect_uri", "")
	v.SetDefault("oidc_scopes", []string{"openid", "profile", "email"})

	// OAuth
	v.SetDefault("oauth_authorization_code_lifetime", 10*time.Minute)
	v.SetDefault("oauth_access_token_lifetime", 1*time.Hour)
	v.SetDefault("oauth_refresh_token_lifetime", 30*24*time.Hour)
	v.SetDefault("oauth_device_code_lifetime", 10*time.Minute)
	v.SetDefault("oauth_device_polling_interval", 5*time.Second)
	v.SetDefault("oauth_pkce_required", true)

	// Storage
	v.SetDefault("storage_driver", "local")
	v.SetDefault("storage_base_path", "./storage")
	v.SetDefault("storage_document_path", "documents")
	v.SetDefault("storage_temp_path", "tmp")

	// OTEL
	v.SetDefault("otel_enabled", false)
	v.SetDefault("otel_exporter_otlp_endpoint", "")
	v.SetDefault("otel_service_name", "documcp")
	v.SetDefault("otel_insecure", false)

	// DocuMCP
	v.SetDefault("documcp_endpoint", "/documcp")
	v.SetDefault("documcp_name", "DocuMCP")
	v.SetDefault("documcp_version", "0.1.0")
}

// Load reads configuration from environment variables and an optional YAML
// config file. The config file path is determined by the DOCUMCP_CONFIG_PATH
// env var, falling back to ./config.yaml if present.
func Load() (*Config, error) {
	v := viper.New()

	setDefaults(v)

	// Allow reading env vars automatically.
	v.AutomaticEnv()

	// Determine config file path from DOCUMCP_CONFIG_PATH or default.
	configPath := v.GetString("DOCUMCP_CONFIG_PATH")
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
	}

	// Read config file if it exists; missing file is not an error.
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			// If a specific path was requested, a missing file is an error.
			if configPath != "" {
				return nil, fmt.Errorf("reading config file: %w", err)
			}
			// For the default ./config.yaml, ignore file-not-found from the OS.
			// viper may return a raw os.PathError when AddConfigPath is used.
		}
	}

	cfg := &Config{}

	// Populate each section by binding env vars and unmarshalling.
	cfg.App = AppConfig{
		Name:     v.GetString("app_name"),
		Env:      v.GetString("app_env"),
		Debug:    v.GetBool("app_debug"),
		URL:      v.GetString("app_url"),
		Timezone: v.GetString("app_timezone"),
	}

	cfg.Server = ServerConfig{
		Host:           v.GetString("server_host"),
		Port:           v.GetInt("server_port"),
		TrustedProxies: v.GetStringSlice("trusted_proxies"),
		ReadTimeout:    v.GetDuration("server_read_timeout"),
		WriteTimeout:   v.GetDuration("server_write_timeout"),
		IdleTimeout:    v.GetDuration("server_idle_timeout"),
	}

	cfg.Database = DatabaseConfig{
		Host:         v.GetString("db_host"),
		Port:         v.GetInt("db_port"),
		Database:     v.GetString("db_database"),
		Username:     v.GetString("db_username"),
		Password:     v.GetString("db_password"),
		SSLMode:      v.GetString("db_sslmode"),
		MaxOpenConns: v.GetInt("db_max_open_conns"),
		MaxIdleConns: v.GetInt("db_max_idle_conns"),
		MaxLifetime:  v.GetDuration("db_max_lifetime"),
	}

	cfg.Redis = RedisConfig{
		Host:     v.GetString("redis_host"),
		Port:     v.GetInt("redis_port"),
		Password: v.GetString("redis_password"),
		DB:       v.GetInt("redis_db"),
	}

	cfg.Meilisearch = MeilisearchConfig{
		Host: v.GetString("meilisearch_host"),
		Key:  v.GetString("meilisearch_key"),
	}

	cfg.OIDC = OIDCConfig{
		ProviderURL:  v.GetString("oidc_provider_url"),
		ClientID:     v.GetString("oidc_client_id"),
		ClientSecret: v.GetString("oidc_client_secret"),
		RedirectURL:  v.GetString("oidc_redirect_uri"),
		Scopes:       v.GetStringSlice("oidc_scopes"),
	}

	cfg.OAuth = OAuthConfig{
		AuthCodeLifetime:     v.GetDuration("oauth_authorization_code_lifetime"),
		AccessTokenLifetime:  v.GetDuration("oauth_access_token_lifetime"),
		RefreshTokenLifetime: v.GetDuration("oauth_refresh_token_lifetime"),
		DeviceCodeLifetime:   v.GetDuration("oauth_device_code_lifetime"),
		DeviceCodeInterval:   v.GetDuration("oauth_device_polling_interval"),
		RequirePKCE:          v.GetBool("oauth_pkce_required"),
	}

	cfg.Storage = StorageConfig{
		Driver:       v.GetString("storage_driver"),
		BasePath:     v.GetString("storage_base_path"),
		DocumentPath: v.GetString("storage_document_path"),
		TempPath:     v.GetString("storage_temp_path"),
	}

	cfg.OTEL = OTELConfig{
		Enabled:     v.GetBool("otel_enabled"),
		Endpoint:    v.GetString("otel_exporter_otlp_endpoint"),
		ServiceName: v.GetString("otel_service_name"),
		Insecure:    v.GetBool("otel_insecure"),
	}

	cfg.DocuMCP = DocuMCPConfig{
		Endpoint:      v.GetString("documcp_endpoint"),
		ServerName:    v.GetString("documcp_name"),
		ServerVersion: v.GetString("documcp_version"),
	}

	return cfg, nil
}

// Validate checks that all required configuration fields are set.
func (c *Config) Validate() error {
	var errs []string

	if c.Database.Host == "" {
		errs = append(errs, "database host is required (DB_HOST)")
	}
	if c.Database.Database == "" {
		errs = append(errs, "database name is required (DB_DATABASE)")
	}
	if c.Database.Username == "" {
		errs = append(errs, "database username is required (DB_USERNAME)")
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed: %s", strings.Join(errs, "; "))
	}

	return nil
}

// DatabaseDSN builds a PostgreSQL connection string from the database config.
func (c *Config) DatabaseDSN() string {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s dbname=%s sslmode=%s",
		c.Database.Host,
		c.Database.Port,
		c.Database.Username,
		c.Database.Database,
		c.Database.SSLMode,
	)

	if c.Database.Password != "" {
		dsn += fmt.Sprintf(" password=%s", c.Database.Password)
	}

	return dsn
}
