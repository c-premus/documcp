// Package config loads and validates application configuration from environment
// variables and optional YAML config files. It supports both unprefixed env vars
// (DB_HOST) matching the PHP convention and DOCUMCP_-prefixed variants.
package config

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds the complete application configuration.
type Config struct {
	App         AppConfig
	Server      ServerConfig
	Database    DatabaseConfig
	Meilisearch MeilisearchConfig
	OIDC        OIDCConfig
	OAuth       OAuthConfig
	Storage     StorageConfig
	OTEL        OTELConfig
	DocuMCP     DocuMCPConfig
	Scheduler   SchedulerConfig
}

// SchedulerConfig holds cron schedule expressions for background sync jobs.
// Empty schedule strings disable the corresponding job.
type SchedulerConfig struct {
	Enabled                 bool   `mapstructure:"scheduler_enabled"`
	KiwixSchedule           string `mapstructure:"scheduler_kiwix_schedule"`
	ConfluenceSchedule      string `mapstructure:"scheduler_confluence_schedule"`
	GitSchedule             string `mapstructure:"scheduler_git_schedule"`
	OAuthCleanupSchedule    string `mapstructure:"scheduler_oauth_cleanup_schedule"`
	OrphanedFilesSchedule   string `mapstructure:"scheduler_orphaned_files_schedule"`
	SearchVerifySchedule    string `mapstructure:"scheduler_search_verify_schedule"`
	SoftDeletePurgeSchedule string `mapstructure:"scheduler_soft_delete_purge_schedule"`
	ZimCleanupSchedule      string `mapstructure:"scheduler_zim_cleanup_schedule"`
	HealthCheckSchedule     string `mapstructure:"scheduler_health_check_schedule"`
}

// AppConfig holds general application settings.
type AppConfig struct {
	Name             string `mapstructure:"app_name"`
	Env              string `mapstructure:"app_env"`
	Debug            bool   `mapstructure:"app_debug"`
	URL              string `mapstructure:"app_url"`
	Timezone         string `mapstructure:"app_timezone"`
	InternalAPIToken string `mapstructure:"internal_api_token"`
	EncryptionKey    string `mapstructure:"encryption_key"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host              string        `mapstructure:"server_host"`
	Port              int           `mapstructure:"server_port"`
	TrustedProxies    []string      `mapstructure:"trusted_proxies"`
	ReadTimeout       time.Duration `mapstructure:"server_read_timeout"`
	WriteTimeout      time.Duration `mapstructure:"server_write_timeout"`
	IdleTimeout       time.Duration `mapstructure:"server_idle_timeout"`
	ReadHeaderTimeout time.Duration `mapstructure:"server_read_header_timeout"`
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

// MeilisearchConfig holds Meilisearch connection settings.
type MeilisearchConfig struct {
	Host string `mapstructure:"meilisearch_host"`
	Key  string `mapstructure:"meilisearch_key"`
}

// OIDCConfig holds OpenID Connect provider settings.
// When AuthorizationURL and TokenURL are set, auto-discovery is skipped and
// endpoints are configured manually (REQ-AUTH-003).
type OIDCConfig struct {
	ProviderURL      string   `mapstructure:"oidc_provider_url"`
	ClientID         string   `mapstructure:"oidc_client_id"`
	ClientSecret     string   `mapstructure:"oidc_client_secret"`
	RedirectURL      string   `mapstructure:"oidc_redirect_uri"`
	Scopes           []string `mapstructure:"oidc_scopes"`
	AdminGroups      []string `mapstructure:"oidc_admin_groups"`
	AuthorizationURL string   `mapstructure:"oidc_authorization_url"`
	TokenURL         string   `mapstructure:"oidc_token_url"`
	UserinfoURL      string   `mapstructure:"oidc_userinfo_url"`
	JWKSURL          string   `mapstructure:"oidc_jwks_url"`
}

// ManualEndpoints returns true when manual OIDC endpoint configuration is active.
func (c OIDCConfig) ManualEndpoints() bool {
	return c.AuthorizationURL != "" && c.TokenURL != ""
}

// OAuthConfig holds OAuth 2.1 authorization server settings.
type OAuthConfig struct {
	AuthCodeLifetime        time.Duration `mapstructure:"oauth_authorization_code_lifetime"`
	AccessTokenLifetime     time.Duration `mapstructure:"oauth_access_token_lifetime"`
	RefreshTokenLifetime    time.Duration `mapstructure:"oauth_refresh_token_lifetime"`
	DeviceCodeLifetime      time.Duration `mapstructure:"oauth_device_code_lifetime"`
	DeviceCodeInterval      time.Duration `mapstructure:"oauth_device_polling_interval"`
	RequirePKCE             bool          `mapstructure:"oauth_pkce_required"`
	SessionSecret           string        `mapstructure:"oauth_session_secret"`
	RegistrationEnabled     bool          `mapstructure:"oauth_registration_enabled"`
	RegistrationRequireAuth bool          `mapstructure:"oauth_registration_require_auth"`
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
	v.SetDefault("app_env", "development")
	v.SetDefault("app_debug", false)
	v.SetDefault("app_url", "http://localhost")
	v.SetDefault("app_timezone", "UTC")
	v.SetDefault("internal_api_token", "")

	// Server
	v.SetDefault("server_host", "0.0.0.0")
	v.SetDefault("server_port", 8080)
	v.SetDefault("trusted_proxies", []string{})
	v.SetDefault("server_read_timeout", 30*time.Second)
	v.SetDefault("server_write_timeout", 30*time.Second)
	v.SetDefault("server_idle_timeout", 120*time.Second)
	v.SetDefault("server_read_header_timeout", 5*time.Second)

	// Database
	v.SetDefault("db_host", "127.0.0.1")
	v.SetDefault("db_port", 5432)
	v.SetDefault("db_database", "")
	v.SetDefault("db_username", "")
	v.SetDefault("db_password", "")
	v.SetDefault("db_sslmode", "require")
	v.SetDefault("db_max_open_conns", 25)
	v.SetDefault("db_max_idle_conns", 5)
	v.SetDefault("db_max_lifetime", 5*time.Minute)

	// Meilisearch
	v.SetDefault("meilisearch_host", "http://localhost:7700")
	v.SetDefault("meilisearch_key", "")

	// OIDC
	v.SetDefault("oidc_provider_url", "")
	v.SetDefault("oidc_client_id", "")
	v.SetDefault("oidc_client_secret", "")
	v.SetDefault("oidc_redirect_uri", "")
	v.SetDefault("oidc_scopes", []string{"openid", "profile", "email"})
	v.SetDefault("oidc_admin_groups", []string{})
	v.SetDefault("oidc_authorization_url", "")
	v.SetDefault("oidc_token_url", "")
	v.SetDefault("oidc_userinfo_url", "")
	v.SetDefault("oidc_jwks_url", "")

	// OAuth
	v.SetDefault("oauth_authorization_code_lifetime", 10*time.Minute)
	v.SetDefault("oauth_access_token_lifetime", 1*time.Hour)
	v.SetDefault("oauth_refresh_token_lifetime", 30*24*time.Hour)
	v.SetDefault("oauth_device_code_lifetime", 10*time.Minute)
	v.SetDefault("oauth_device_polling_interval", 5*time.Second)
	v.SetDefault("oauth_pkce_required", true)
	v.SetDefault("oauth_session_secret", "")
	v.SetDefault("oauth_registration_enabled", true)
	v.SetDefault("oauth_registration_require_auth", true)

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

	// Scheduler
	v.SetDefault("scheduler_enabled", false)
	v.SetDefault("scheduler_kiwix_schedule", "0 */6 * * *")           // every 6 hours
	v.SetDefault("scheduler_confluence_schedule", "0 */4 * * *")      // every 4 hours
	v.SetDefault("scheduler_git_schedule", "0 * * * *")               // every hour
	v.SetDefault("scheduler_oauth_cleanup_schedule", "0 * * * *")     // hourly
	v.SetDefault("scheduler_orphaned_files_schedule", "0 2 * * *")    // daily 2 AM
	v.SetDefault("scheduler_search_verify_schedule", "0 3 * * *")     // daily 3 AM
	v.SetDefault("scheduler_soft_delete_purge_schedule", "0 4 * * *") // daily 4 AM
	v.SetDefault("scheduler_zim_cleanup_schedule", "0 5 * * *")       // daily 5 AM
	v.SetDefault("scheduler_health_check_schedule", "*/15 * * * *")   // every 15 min
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
		Name:             v.GetString("app_name"),
		Env:              v.GetString("app_env"),
		Debug:            v.GetBool("app_debug"),
		URL:              v.GetString("app_url"),
		Timezone:         v.GetString("app_timezone"),
		InternalAPIToken: v.GetString("internal_api_token"),
		EncryptionKey:    v.GetString("encryption_key"),
	}

	cfg.Server = ServerConfig{
		Host:              v.GetString("server_host"),
		Port:              v.GetInt("server_port"),
		TrustedProxies:    v.GetStringSlice("trusted_proxies"),
		ReadTimeout:       v.GetDuration("server_read_timeout"),
		WriteTimeout:      v.GetDuration("server_write_timeout"),
		IdleTimeout:       v.GetDuration("server_idle_timeout"),
		ReadHeaderTimeout: v.GetDuration("server_read_header_timeout"),
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

	cfg.Meilisearch = MeilisearchConfig{
		Host: v.GetString("meilisearch_host"),
		Key:  v.GetString("meilisearch_key"),
	}

	cfg.OIDC = OIDCConfig{
		ProviderURL:      v.GetString("oidc_provider_url"),
		ClientID:         v.GetString("oidc_client_id"),
		ClientSecret:     v.GetString("oidc_client_secret"),
		RedirectURL:      v.GetString("oidc_redirect_uri"),
		Scopes:           v.GetStringSlice("oidc_scopes"),
		AdminGroups:      v.GetStringSlice("oidc_admin_groups"),
		AuthorizationURL: v.GetString("oidc_authorization_url"),
		TokenURL:         v.GetString("oidc_token_url"),
		UserinfoURL:      v.GetString("oidc_userinfo_url"),
		JWKSURL:          v.GetString("oidc_jwks_url"),
	}

	cfg.OAuth = OAuthConfig{
		AuthCodeLifetime:        v.GetDuration("oauth_authorization_code_lifetime"),
		AccessTokenLifetime:     v.GetDuration("oauth_access_token_lifetime"),
		RefreshTokenLifetime:    v.GetDuration("oauth_refresh_token_lifetime"),
		DeviceCodeLifetime:      v.GetDuration("oauth_device_code_lifetime"),
		DeviceCodeInterval:      v.GetDuration("oauth_device_polling_interval"),
		RequirePKCE:             v.GetBool("oauth_pkce_required"),
		SessionSecret:           v.GetString("oauth_session_secret"),
		RegistrationEnabled:     v.GetBool("oauth_registration_enabled"),
		RegistrationRequireAuth: v.GetBool("oauth_registration_require_auth"),
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

	cfg.Scheduler = SchedulerConfig{
		Enabled:                 v.GetBool("scheduler_enabled"),
		KiwixSchedule:           v.GetString("scheduler_kiwix_schedule"),
		ConfluenceSchedule:      v.GetString("scheduler_confluence_schedule"),
		GitSchedule:             v.GetString("scheduler_git_schedule"),
		OAuthCleanupSchedule:    v.GetString("scheduler_oauth_cleanup_schedule"),
		OrphanedFilesSchedule:   v.GetString("scheduler_orphaned_files_schedule"),
		SearchVerifySchedule:    v.GetString("scheduler_search_verify_schedule"),
		SoftDeletePurgeSchedule: v.GetString("scheduler_soft_delete_purge_schedule"),
		ZimCleanupSchedule:      v.GetString("scheduler_zim_cleanup_schedule"),
		HealthCheckSchedule:     v.GetString("scheduler_health_check_schedule"),
	}

	return cfg, nil
}

// Validate checks that all required configuration fields are set.
func (c *Config) Validate() error {
	var errs []string

	// --- Always required ---
	if c.Database.Host == "" {
		errs = append(errs, "database host is required (DB_HOST)")
	}
	if c.Database.Database == "" {
		errs = append(errs, "database name is required (DB_DATABASE)")
	}
	if c.Database.Username == "" {
		errs = append(errs, "database username is required (DB_USERNAME)")
	}
	if c.Meilisearch.Host == "" {
		errs = append(errs, "meilisearch host is required (MEILISEARCH_HOST)")
	}

	// --- Conditional validation ---
	if c.App.Env != "" && c.App.Env != "development" && c.App.Env != "staging" &&
		c.App.Env != "production" && c.App.Env != "testing" {
		errs = append(errs, "APP_ENV must be one of: development, staging, production, testing")
	}

	if c.App.EncryptionKey != "" && len(c.App.EncryptionKey) != 32 {
		errs = append(errs, "ENCRYPTION_KEY must be exactly 32 bytes for AES-256-GCM")
	}

	if c.OTEL.Enabled && c.OTEL.Endpoint == "" {
		errs = append(errs, "OTEL_EXPORTER_OTLP_ENDPOINT is required when OTEL_ENABLED=true")
	}

	// --- Production requirements ---
	isProd := c.App.Env == "production"

	if isProd && c.OAuth.SessionSecret == "" {
		errs = append(errs, "OAUTH_SESSION_SECRET is required in production")
	}
	if isProd && c.Database.Password == "" {
		errs = append(errs, "DB_PASSWORD is required in production")
	}
	if isProd && c.App.EncryptionKey == "" {
		errs = append(errs, "ENCRYPTION_KEY is required in production (git tokens stored in plaintext without it)")
	}
	if isProd && c.App.URL == "http://localhost" {
		errs = append(errs, "APP_URL must be set to the actual URL in production (currently http://localhost)")
	}
	if isProd && c.App.InternalAPIToken == "" {
		errs = append(errs, "INTERNAL_API_TOKEN is required in production")
	}
	if isProd && c.App.Debug {
		errs = append(errs, "APP_DEBUG should not be enabled in production")
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed: %s", strings.Join(errs, "; "))
	}

	return nil
}

// ParseCIDRs parses a list of CIDR strings into net.IPNet values.
// Bare IPs without a prefix length are treated as /32 (IPv4) or /128 (IPv6).
func ParseCIDRs(cidrs []string) ([]*net.IPNet, error) {
	if len(cidrs) == 0 {
		return nil, nil
	}
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, entry := range cidrs {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if !strings.Contains(entry, "/") {
			ip := net.ParseIP(entry)
			if ip == nil {
				return nil, fmt.Errorf("invalid IP: %q", entry)
			}
			if ip.To4() != nil {
				entry += "/32"
			} else {
				entry += "/128"
			}
		}
		_, cidr, err := net.ParseCIDR(entry)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR: %q: %w", entry, err)
		}
		nets = append(nets, cidr)
	}
	return nets, nil
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
		escaped := strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(c.Database.Password)
		dsn += fmt.Sprintf(" password='%s'", escaped)
	}

	return dsn
}
