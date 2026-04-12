// Package config loads and validates application configuration from environment
// variables and optional YAML config files. It supports both unprefixed env vars
// (DB_HOST) matching the PHP convention and DOCUMCP_-prefixed variants.
package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math"
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
	Redis       RedisConfig
	OIDC OIDCConfig
	OAuth       OAuthConfig
	Storage     StorageConfig
	OTEL        OTELConfig
	DocuMCP     DocuMCPConfig
	Sentry      SentryConfig
	Scheduler   SchedulerConfig
	Queue       QueueConfig
	Kiwix       KiwixConfig
	Git         GitConfig
}

// RedisConfig holds Redis connection settings used for distributed rate
// limiting and cross-instance event delivery (SSE). Supports Redis 6+ ACL
// authentication with username/password.
type RedisConfig struct {
	Addr            string        `mapstructure:"redis_addr"`
	Username        string        `mapstructure:"redis_username"`
	Password        string        `mapstructure:"redis_password"`
	DB              int           `mapstructure:"redis_db"`
	PoolSize        int           `mapstructure:"redis_pool_size"`
	MinIdleConns    int           `mapstructure:"redis_min_idle_conns"`
	MaxActiveConns  int           `mapstructure:"redis_max_active_conns"`
	ConnMaxIdleTime time.Duration `mapstructure:"redis_conn_max_idle_time"`
	DialTimeout     time.Duration `mapstructure:"redis_dial_timeout"`
	ReadTimeout     time.Duration `mapstructure:"redis_read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"redis_write_timeout"`
	MaxRetries      int           `mapstructure:"redis_max_retries"`
	TLSEnabled      bool          `mapstructure:"redis_tls_enabled"`
	TLSCAFile       string        `mapstructure:"redis_tls_ca_file"`
}

// QueueConfig holds River queue worker concurrency settings.
type QueueConfig struct {
	HighWorkers    int `mapstructure:"queue_high_workers"`
	DefaultWorkers int `mapstructure:"queue_default_workers"`
	LowWorkers     int `mapstructure:"queue_low_workers"`
	HealthPort     int `mapstructure:"worker_health_port"`
}

// SchedulerConfig holds cron schedule expressions for background sync jobs.
// Empty schedule strings disable the corresponding job.
type SchedulerConfig struct {
	Enabled                 bool   `mapstructure:"scheduler_enabled"`
	KiwixSchedule           string `mapstructure:"scheduler_kiwix_schedule"`
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
	Name              string        `mapstructure:"app_name"`
	Env               string        `mapstructure:"app_env"`
	Debug             bool          `mapstructure:"app_debug"`
	URL               string        `mapstructure:"app_url"`
	Timezone          string        `mapstructure:"app_timezone"`
	InternalAPIToken  string        `mapstructure:"internal_api_token"`
	EncryptionKey     string        `mapstructure:"encryption_key"`
	EncryptionKeyBytes []byte       // Decoded from EncryptionKey (hex); populated by Validate()
	QueueStopTimeout  time.Duration `mapstructure:"app_queue_stop_timeout"`
	TracerStopTimeout time.Duration `mapstructure:"app_tracer_stop_timeout"`
	SSRFDialerTimeout time.Duration `mapstructure:"ssrf_dialer_timeout"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host                 string        `mapstructure:"server_host"`
	Port                 int           `mapstructure:"server_port"`
	TrustedProxies       []string      `mapstructure:"trusted_proxies"`
	ReadTimeout          time.Duration `mapstructure:"server_read_timeout"`
	WriteTimeout         time.Duration `mapstructure:"server_write_timeout"`
	IdleTimeout          time.Duration `mapstructure:"server_idle_timeout"`
	ReadHeaderTimeout    time.Duration `mapstructure:"server_read_header_timeout"`
	ShutdownTimeout      time.Duration `mapstructure:"server_shutdown_timeout"`
	RequestTimeout       time.Duration `mapstructure:"server_request_timeout"`
	MaxBodySize          int64         `mapstructure:"server_max_body_size"`
	HSTSMaxAge           int           `mapstructure:"server_hsts_max_age"`
	SSEHeartbeatInterval time.Duration `mapstructure:"server_sse_heartbeat_interval"`
	TLSEnabled           bool          `mapstructure:"tls_enabled"`
	TLSPort              int           `mapstructure:"tls_port"`
	TLSCertFile          string        `mapstructure:"tls_cert_file"`
	TLSKeyFile           string        `mapstructure:"tls_key_file"`
}

// DatabaseConfig holds PostgreSQL connection settings.
//
// Pool sizing guidance:
//   - Serve-only mode: DB_MAX_OPEN_CONNS=25 is a reasonable default.
//   - Combined mode (serve --with-worker): HTTP handlers + River workers
//     (QUEUE_HIGH_WORKERS + QUEUE_DEFAULT_WORKERS + QUEUE_LOW_WORKERS = 17 by
//     default) share one pool. Set DB_MAX_OPEN_CONNS=40-50 to avoid workers
//     starving HTTP handlers under load.
//   - DB_PGX_MIN_CONNS keeps idle connections warm, reducing latency spikes
//     after quiet periods.
type DatabaseConfig struct {
	Host               string        `mapstructure:"db_host"`
	Port               int           `mapstructure:"db_port"`
	Database           string        `mapstructure:"db_database"`
	Username           string        `mapstructure:"db_username"`
	Password           string        `mapstructure:"db_password"`
	SSLMode            string        `mapstructure:"db_sslmode"`
	MaxOpenConns       int           `mapstructure:"db_max_open_conns"`
	MaxIdleConns       int           `mapstructure:"db_max_idle_conns"`
	MaxLifetime        time.Duration `mapstructure:"db_max_lifetime"`
	PgxMinConns        int32         `mapstructure:"db_pgx_min_conns"`
	PgxMaxConnLifetime time.Duration `mapstructure:"db_pgx_max_conn_lifetime"`
	PgxMaxConnIdleTime time.Duration `mapstructure:"db_pgx_max_conn_idle_time"`
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
	EndSessionURL    string   `mapstructure:"oidc_end_session_url"`
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
	SessionSecret           string        `mapstructure:"oauth_session_secret"`
	SessionSecretPrevious   string        `mapstructure:"oauth_session_secret_previous"`
	SessionMaxAge           time.Duration `mapstructure:"oauth_session_max_age"`
	HKDFSalt                string        `mapstructure:"hkdf_salt"`
	RegistrationEnabled     bool          `mapstructure:"oauth_registration_enabled"`
	RegistrationRequireAuth bool          `mapstructure:"oauth_registration_require_auth"`
	ClientTouchTimeout      time.Duration `mapstructure:"oauth_client_touch_timeout"`
	ScopeGrantTTL           time.Duration `mapstructure:"oauth_scope_grant_ttl"`
}

// KiwixConfig holds Kiwix external service client settings.
type KiwixConfig struct {
	CacheTTL           time.Duration `mapstructure:"kiwix_cache_ttl"`
	HTTPTimeout        time.Duration `mapstructure:"kiwix_http_timeout"`
	HealthCheckTimeout time.Duration `mapstructure:"kiwix_health_check_timeout"`

	// Federated search: fan-out to Kiwix archives during unified_search.
	FederatedSearchTimeout  time.Duration `mapstructure:"kiwix_federated_search_timeout"`
	FederatedMaxArchives    int           `mapstructure:"kiwix_federated_max_archives"`
	FederatedPerArchiveLimit int          `mapstructure:"kiwix_federated_per_archive_limit"`
}

// GitConfig holds Git template client settings.
type GitConfig struct {
	MaxFileSize  int64 `mapstructure:"git_max_file_size"`
	MaxTotalSize int64 `mapstructure:"git_max_total_size"`
}

// StorageConfig holds file storage settings.
type StorageConfig struct {
	// Driver selects the blob backend: "local"/"fs" (default) or "s3".
	// "local" writes to BasePath on disk. "s3" talks to any S3-compatible
	// service (AWS S3, Cloudflare R2, Backblaze B2, Wasabi, Garage, SeaweedFS)
	// via the S3 API. BasePath is still required in s3 mode for worker-local
	// scratch directories (git clones, extractor temp files).
	Driver       string `mapstructure:"storage_driver"`
	BasePath     string `mapstructure:"storage_base_path"`
	DocumentPath string `mapstructure:"storage_document_path"`
	TempPath     string `mapstructure:"storage_temp_path"`

	// Extraction limits — safety guards for document processing.
	MaxUploadSize    int64 `mapstructure:"storage_max_upload_size"`
	MaxExtractedText int64 `mapstructure:"storage_max_extracted_text"`
	MaxZIPFiles      int   `mapstructure:"storage_max_zip_files"`
	MaxSheets        int   `mapstructure:"storage_max_sheets"`

	// S3 driver fields — ignored when Driver != "s3".
	// Endpoint can be empty for AWS (uses the default regional endpoint) but
	// is required for self-hosted backends like Garage or SeaweedFS.
	S3Endpoint        string `mapstructure:"storage_s3_endpoint"`
	S3Bucket          string `mapstructure:"storage_s3_bucket"`
	S3Region          string `mapstructure:"storage_s3_region"`
	S3AccessKeyID     string `mapstructure:"storage_s3_access_key_id"`
	S3SecretAccessKey string `mapstructure:"storage_s3_secret_access_key"`
	// S3UsePathStyle defaults to true. Required for most self-hosted
	// S3-compatible services; AWS S3 and R2 accept both.
	S3UsePathStyle bool `mapstructure:"storage_s3_use_path_style"`
	// S3ForceSSL defaults to true — rejects plaintext endpoints at Open time.
	S3ForceSSL bool `mapstructure:"storage_s3_force_ssl"`
}

// OTELConfig holds OpenTelemetry observability settings.
type OTELConfig struct {
	Enabled     bool    `mapstructure:"otel_enabled"`
	Endpoint    string  `mapstructure:"otel_exporter_otlp_endpoint"`
	ServiceName string  `mapstructure:"otel_service_name"`
	Insecure    bool    `mapstructure:"otel_insecure"`
	SampleRate  float64 `mapstructure:"otel_sample_rate"`
	Environment string  `mapstructure:"otel_environment"`
	Version     string  `mapstructure:"otel_service_version"`
}

// DocuMCPConfig holds MCP server-specific settings.
type DocuMCPConfig struct {
	Endpoint      string `mapstructure:"documcp_endpoint"`
	ServerName    string `mapstructure:"documcp_name"`
	ServerVersion string `mapstructure:"documcp_version"`
}

// SentryConfig holds Sentry/GlitchTip error tracking settings.
type SentryConfig struct {
	DSN         string  `mapstructure:"sentry_dsn"`
	Environment string  `mapstructure:"sentry_environment"`
	Release     string  `mapstructure:"sentry_release"`
	SampleRate  float64 `mapstructure:"sentry_sample_rate"`
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
	v.SetDefault("app_queue_stop_timeout", 10*time.Second)
	v.SetDefault("app_tracer_stop_timeout", 5*time.Second)
	v.SetDefault("ssrf_dialer_timeout", 10*time.Second)

	// Server
	v.SetDefault("server_host", "0.0.0.0")
	v.SetDefault("server_port", 8080)
	v.SetDefault("trusted_proxies", "")
	v.SetDefault("server_read_timeout", 30*time.Second)
	v.SetDefault("server_write_timeout", 30*time.Second)
	v.SetDefault("server_idle_timeout", 120*time.Second)
	v.SetDefault("server_read_header_timeout", 5*time.Second)
	v.SetDefault("server_shutdown_timeout", 5*time.Second)
	v.SetDefault("server_request_timeout", 60*time.Second)
	v.SetDefault("server_max_body_size", int64(1*1024*1024))
	v.SetDefault("server_hsts_max_age", 63072000)
	v.SetDefault("server_sse_heartbeat_interval", 15*time.Second)
	v.SetDefault("tls_enabled", false)
	v.SetDefault("tls_port", 8443)
	v.SetDefault("tls_cert_file", "")
	v.SetDefault("tls_key_file", "")

	// Redis
	v.SetDefault("redis_addr", "")
	v.SetDefault("redis_username", "")
	v.SetDefault("redis_password", "")
	v.SetDefault("redis_db", 0)
	v.SetDefault("redis_pool_size", 10)
	v.SetDefault("redis_min_idle_conns", 2)
	v.SetDefault("redis_conn_max_idle_time", 5*time.Minute)
	v.SetDefault("redis_dial_timeout", 5*time.Second)
	v.SetDefault("redis_read_timeout", 5*time.Second)
	v.SetDefault("redis_write_timeout", 5*time.Second)
	v.SetDefault("redis_max_retries", 3)
	v.SetDefault("redis_max_active_conns", 0)
	v.SetDefault("redis_tls_enabled", false)
	v.SetDefault("redis_tls_ca_file", "")

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
	v.SetDefault("db_pgx_min_conns", int32(5))
	v.SetDefault("db_pgx_max_conn_lifetime", 30*time.Minute)
	v.SetDefault("db_pgx_max_conn_idle_time", 5*time.Minute)

	// OIDC
	v.SetDefault("oidc_provider_url", "")
	v.SetDefault("oidc_client_id", "")
	v.SetDefault("oidc_client_secret", "")
	v.SetDefault("oidc_redirect_uri", "")
	v.SetDefault("oidc_scopes", "openid,profile,email")
	v.SetDefault("oidc_admin_groups", "")
	v.SetDefault("oidc_authorization_url", "")
	v.SetDefault("oidc_token_url", "")
	v.SetDefault("oidc_userinfo_url", "")
	v.SetDefault("oidc_jwks_url", "")
	v.SetDefault("oidc_end_session_url", "")

	// OAuth
	v.SetDefault("oauth_authorization_code_lifetime", 10*time.Minute)
	v.SetDefault("oauth_access_token_lifetime", 1*time.Hour)
	v.SetDefault("oauth_refresh_token_lifetime", 30*24*time.Hour)
	v.SetDefault("oauth_device_code_lifetime", 10*time.Minute)
	v.SetDefault("oauth_device_polling_interval", 5*time.Second)

	v.SetDefault("oauth_session_secret", "")
	v.SetDefault("oauth_session_secret_previous", "")
	v.SetDefault("oauth_session_max_age", 30*24*time.Hour)
	v.SetDefault("hkdf_salt", "DocuMCP-go-v1")
	v.SetDefault("oauth_registration_enabled", true)
	v.SetDefault("oauth_registration_require_auth", true)
	v.SetDefault("oauth_client_touch_timeout", 3*time.Second)
	v.SetDefault("oauth_scope_grant_ttl", 30*24*time.Hour) // 30 days; 0 = no expiry

	// Storage
	v.SetDefault("storage_driver", "local")
	v.SetDefault("storage_base_path", "./storage")
	v.SetDefault("storage_document_path", "documents")
	v.SetDefault("storage_temp_path", "tmp")
	v.SetDefault("storage_max_upload_size", 50*1024*1024)    // 50 MiB
	v.SetDefault("storage_max_extracted_text", 50*1024*1024) // 50 MiB
	v.SetDefault("storage_max_zip_files", 100)
	v.SetDefault("storage_max_sheets", 100)
	v.SetDefault("storage_s3_use_path_style", true)
	v.SetDefault("storage_s3_force_ssl", true)

	// OTEL
	v.SetDefault("otel_enabled", false)
	v.SetDefault("otel_exporter_otlp_endpoint", "")
	v.SetDefault("otel_service_name", "documcp")
	v.SetDefault("otel_insecure", false)
	v.SetDefault("otel_sample_rate", 1.0)
	v.SetDefault("otel_environment", "")
	v.SetDefault("otel_service_version", "")

	// DocuMCP
	v.SetDefault("documcp_endpoint", "/documcp")
	v.SetDefault("documcp_name", "DocuMCP")
	v.SetDefault("documcp_version", "dev")

	// Sentry / GlitchTip
	v.SetDefault("sentry_dsn", "")
	v.SetDefault("sentry_environment", "")
	v.SetDefault("sentry_release", "")
	v.SetDefault("sentry_sample_rate", 1.0)

	// Kiwix
	v.SetDefault("kiwix_cache_ttl", 1*time.Hour)
	v.SetDefault("kiwix_http_timeout", 10*time.Second)
	v.SetDefault("kiwix_health_check_timeout", 5*time.Second)
	v.SetDefault("kiwix_federated_search_timeout", 3*time.Second)
	v.SetDefault("kiwix_federated_max_archives", 10)
	v.SetDefault("kiwix_federated_per_archive_limit", 3)

	// Git
	v.SetDefault("git_max_file_size", int64(1*1024*1024))
	v.SetDefault("git_max_total_size", int64(10*1024*1024))

	// Queue worker concurrency
	v.SetDefault("queue_high_workers", 10)
	v.SetDefault("queue_default_workers", 5)
	v.SetDefault("queue_low_workers", 2)
	v.SetDefault("worker_health_port", 9090)

	// Scheduler
	v.SetDefault("scheduler_enabled", false)
	v.SetDefault("scheduler_kiwix_schedule", "0 */6 * * *")           // every 6 hours
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
		Name:              v.GetString("app_name"),
		Env:               v.GetString("app_env"),
		Debug:             v.GetBool("app_debug"),
		URL:               v.GetString("app_url"),
		Timezone:          v.GetString("app_timezone"),
		InternalAPIToken:  v.GetString("internal_api_token"),
		EncryptionKey:     v.GetString("encryption_key"),
		QueueStopTimeout:  v.GetDuration("app_queue_stop_timeout"),
		TracerStopTimeout: v.GetDuration("app_tracer_stop_timeout"),
		SSRFDialerTimeout: v.GetDuration("ssrf_dialer_timeout"),
	}

	cfg.Server = ServerConfig{
		Host:                 v.GetString("server_host"),
		Port:                 v.GetInt("server_port"),
		TrustedProxies:       splitComma(v.GetString("trusted_proxies")),
		ReadTimeout:          v.GetDuration("server_read_timeout"),
		WriteTimeout:         v.GetDuration("server_write_timeout"),
		IdleTimeout:          v.GetDuration("server_idle_timeout"),
		ReadHeaderTimeout:    v.GetDuration("server_read_header_timeout"),
		ShutdownTimeout:      v.GetDuration("server_shutdown_timeout"),
		RequestTimeout:       v.GetDuration("server_request_timeout"),
		MaxBodySize:          v.GetInt64("server_max_body_size"),
		HSTSMaxAge:           v.GetInt("server_hsts_max_age"),
		SSEHeartbeatInterval: v.GetDuration("server_sse_heartbeat_interval"),
		TLSEnabled:           v.GetBool("tls_enabled"),
		TLSPort:              v.GetInt("tls_port"),
		TLSCertFile:          v.GetString("tls_cert_file"),
		TLSKeyFile:           v.GetString("tls_key_file"),
	}

	cfg.Redis = RedisConfig{
		Addr:            v.GetString("redis_addr"),
		Username:        v.GetString("redis_username"),
		Password:        v.GetString("redis_password"),
		DB:              v.GetInt("redis_db"),
		PoolSize:        v.GetInt("redis_pool_size"),
		MinIdleConns:    v.GetInt("redis_min_idle_conns"),
		MaxActiveConns:  v.GetInt("redis_max_active_conns"),
		ConnMaxIdleTime: v.GetDuration("redis_conn_max_idle_time"),
		DialTimeout:     v.GetDuration("redis_dial_timeout"),
		ReadTimeout:     v.GetDuration("redis_read_timeout"),
		WriteTimeout:    v.GetDuration("redis_write_timeout"),
		MaxRetries:      v.GetInt("redis_max_retries"),
	}

	cfg.Database = DatabaseConfig{
		Host:               v.GetString("db_host"),
		Port:               v.GetInt("db_port"),
		Database:           v.GetString("db_database"),
		Username:           v.GetString("db_username"),
		Password:           v.GetString("db_password"),
		SSLMode:            v.GetString("db_sslmode"),
		MaxOpenConns:       v.GetInt("db_max_open_conns"),
		MaxIdleConns:       v.GetInt("db_max_idle_conns"),
		MaxLifetime:        v.GetDuration("db_max_lifetime"),
		PgxMinConns:        clampInt32(v.GetInt("db_pgx_min_conns")),
		PgxMaxConnLifetime: v.GetDuration("db_pgx_max_conn_lifetime"),
		PgxMaxConnIdleTime: v.GetDuration("db_pgx_max_conn_idle_time"),
	}

	cfg.OIDC = OIDCConfig{
		ProviderURL:      v.GetString("oidc_provider_url"),
		ClientID:         v.GetString("oidc_client_id"),
		ClientSecret:     v.GetString("oidc_client_secret"),
		RedirectURL:      v.GetString("oidc_redirect_uri"),
		Scopes:           splitComma(v.GetString("oidc_scopes")),
		AdminGroups:      splitComma(v.GetString("oidc_admin_groups")),
		AuthorizationURL: v.GetString("oidc_authorization_url"),
		TokenURL:         v.GetString("oidc_token_url"),
		UserinfoURL:      v.GetString("oidc_userinfo_url"),
		JWKSURL:          v.GetString("oidc_jwks_url"),
		EndSessionURL:    v.GetString("oidc_end_session_url"),
	}

	cfg.OAuth = OAuthConfig{
		AuthCodeLifetime:        v.GetDuration("oauth_authorization_code_lifetime"),
		AccessTokenLifetime:     v.GetDuration("oauth_access_token_lifetime"),
		RefreshTokenLifetime:    v.GetDuration("oauth_refresh_token_lifetime"),
		DeviceCodeLifetime:      v.GetDuration("oauth_device_code_lifetime"),
		DeviceCodeInterval:      v.GetDuration("oauth_device_polling_interval"),
		SessionSecret:           v.GetString("oauth_session_secret"),
		SessionSecretPrevious:   v.GetString("oauth_session_secret_previous"),
		SessionMaxAge:           v.GetDuration("oauth_session_max_age"),
		HKDFSalt:                v.GetString("hkdf_salt"),
		RegistrationEnabled:     v.GetBool("oauth_registration_enabled"),
		RegistrationRequireAuth: v.GetBool("oauth_registration_require_auth"),
		ClientTouchTimeout:      v.GetDuration("oauth_client_touch_timeout"),
		ScopeGrantTTL:           v.GetDuration("oauth_scope_grant_ttl"),
	}

	cfg.Storage = StorageConfig{
		Driver:            v.GetString("storage_driver"),
		BasePath:          v.GetString("storage_base_path"),
		DocumentPath:      v.GetString("storage_document_path"),
		TempPath:          v.GetString("storage_temp_path"),
		MaxUploadSize:     v.GetInt64("storage_max_upload_size"),
		MaxExtractedText:  v.GetInt64("storage_max_extracted_text"),
		MaxZIPFiles:       v.GetInt("storage_max_zip_files"),
		MaxSheets:         v.GetInt("storage_max_sheets"),
		S3Endpoint:        v.GetString("storage_s3_endpoint"),
		S3Bucket:          v.GetString("storage_s3_bucket"),
		S3Region:          v.GetString("storage_s3_region"),
		S3AccessKeyID:     v.GetString("storage_s3_access_key_id"),
		S3SecretAccessKey: v.GetString("storage_s3_secret_access_key"),
		S3UsePathStyle:    v.GetBool("storage_s3_use_path_style"),
		S3ForceSSL:        v.GetBool("storage_s3_force_ssl"),
	}

	cfg.OTEL = OTELConfig{
		Enabled:     v.GetBool("otel_enabled"),
		Endpoint:    v.GetString("otel_exporter_otlp_endpoint"),
		ServiceName: v.GetString("otel_service_name"),
		Insecure:    v.GetBool("otel_insecure"),
		SampleRate:  v.GetFloat64("otel_sample_rate"),
		Environment: v.GetString("otel_environment"),
		Version:     v.GetString("otel_service_version"),
	}

	cfg.DocuMCP = DocuMCPConfig{
		Endpoint:      v.GetString("documcp_endpoint"),
		ServerName:    v.GetString("documcp_name"),
		ServerVersion: v.GetString("documcp_version"),
	}

	cfg.Sentry = SentryConfig{
		DSN:         v.GetString("sentry_dsn"),
		Environment: v.GetString("sentry_environment"),
		Release:     v.GetString("sentry_release"),
		SampleRate:  v.GetFloat64("sentry_sample_rate"),
	}

	cfg.Kiwix = KiwixConfig{
		CacheTTL:                 v.GetDuration("kiwix_cache_ttl"),
		HTTPTimeout:              v.GetDuration("kiwix_http_timeout"),
		HealthCheckTimeout:       v.GetDuration("kiwix_health_check_timeout"),
		FederatedSearchTimeout:   v.GetDuration("kiwix_federated_search_timeout"),
		FederatedMaxArchives:     v.GetInt("kiwix_federated_max_archives"),
		FederatedPerArchiveLimit: v.GetInt("kiwix_federated_per_archive_limit"),
	}

	cfg.Git = GitConfig{
		MaxFileSize:  v.GetInt64("git_max_file_size"),
		MaxTotalSize: v.GetInt64("git_max_total_size"),
	}

	cfg.Queue = QueueConfig{
		HighWorkers:    v.GetInt("queue_high_workers"),
		DefaultWorkers: v.GetInt("queue_default_workers"),
		LowWorkers:     v.GetInt("queue_low_workers"),
		HealthPort:     v.GetInt("worker_health_port"),
	}

	cfg.Scheduler = SchedulerConfig{
		Enabled:                 v.GetBool("scheduler_enabled"),
		KiwixSchedule:           v.GetString("scheduler_kiwix_schedule"),
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
func (c *Config) Validate() error { //nolint:gocyclo // validation is inherently branchy
	var errs []string

	// --- Always required ---
	if c.Redis.Addr == "" {
		errs = append(errs, "redis address is required (REDIS_ADDR)")
	}
	if c.Database.Host == "" {
		errs = append(errs, "database host is required (DB_HOST)")
	}
	if c.Database.Database == "" {
		errs = append(errs, "database name is required (DB_DATABASE)")
	}
	if c.Database.Username == "" {
		errs = append(errs, "database username is required (DB_USERNAME)")
	}
	// --- Conditional validation ---
	if c.App.Env != "" && c.App.Env != "development" && c.App.Env != "staging" &&
		c.App.Env != "production" && c.App.Env != "testing" {
		errs = append(errs, "APP_ENV must be one of: development, staging, production, testing")
	}

	if c.App.EncryptionKey != "" {
		keyBytes, hexErr := hex.DecodeString(c.App.EncryptionKey)
		switch {
		case hexErr != nil:
			errs = append(errs, "ENCRYPTION_KEY must be a valid hex string (generate with: openssl rand -hex 32)")
		case len(keyBytes) != 32:
			errs = append(errs, "ENCRYPTION_KEY must decode to exactly 32 bytes for AES-256-GCM (use a 64-character hex string)")
		default:
			c.App.EncryptionKeyBytes = keyBytes
		}
	}

	if c.OTEL.Enabled && c.OTEL.Endpoint == "" {
		errs = append(errs, "OTEL_EXPORTER_OTLP_ENDPOINT is required when OTEL_ENABLED=true")
	}
	if c.OTEL.SampleRate < 0 || c.OTEL.SampleRate > 1.0 {
		errs = append(errs, "OTEL_SAMPLE_RATE must be between 0.0 and 1.0")
	}

	// --- Numeric range validation ---
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		errs = append(errs, "SERVER_PORT must be between 1 and 65535")
	}
	if c.Server.MaxBodySize <= 0 {
		errs = append(errs, "SERVER_MAX_BODY_SIZE must be positive")
	}
	if (c.Server.TLSCertFile == "") != (c.Server.TLSKeyFile == "") {
		errs = append(errs, "TLS_CERT_FILE and TLS_KEY_FILE must both be set or both be empty")
	}
	if c.Server.TLSEnabled && (c.Server.TLSPort < 1 || c.Server.TLSPort > 65535) {
		errs = append(errs, "TLS_PORT must be between 1 and 65535")
	}
	if c.Server.TLSEnabled && c.Server.TLSPort == c.Server.Port {
		errs = append(errs, "TLS_PORT and SERVER_PORT must be different when TLS is enabled")
	}
	if c.Database.MaxOpenConns > 0 && c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		errs = append(errs, "DB_MAX_IDLE_CONNS must not exceed DB_MAX_OPEN_CONNS")
	}
	if c.Database.PgxMinConns > 0 && int(c.Database.PgxMinConns) > c.Database.MaxOpenConns {
		errs = append(errs, "DB_PGX_MIN_CONNS must not exceed DB_MAX_OPEN_CONNS")
	}
	if c.Redis.PoolSize < 0 {
		errs = append(errs, "REDIS_POOL_SIZE must be non-negative")
	}
	totalWorkers := c.Queue.HighWorkers + c.Queue.DefaultWorkers + c.Queue.LowWorkers
	if c.Database.MaxOpenConns > 0 && totalWorkers > 2*c.Database.MaxOpenConns {
		errs = append(errs, fmt.Sprintf(
			"total queue workers (%d) exceed 2× DB_MAX_OPEN_CONNS (%d); increase pool or reduce workers",
			totalWorkers, c.Database.MaxOpenConns))
	}
	if c.Git.MaxFileSize <= 0 {
		errs = append(errs, "GIT_MAX_FILE_SIZE must be positive")
	}
	if c.Git.MaxTotalSize <= 0 {
		errs = append(errs, "GIT_MAX_TOTAL_SIZE must be positive")
	}

	// --- Storage driver validation ---
	switch c.Storage.Driver {
	case "", "local", "fs":
		// Filesystem backend — no extra requirements beyond BasePath,
		// which has a default.
	case "s3":
		if c.Storage.S3Bucket == "" {
			errs = append(errs, "STORAGE_S3_BUCKET is required when STORAGE_DRIVER=s3")
		}
		if c.Storage.S3Region == "" {
			errs = append(errs, "STORAGE_S3_REGION is required when STORAGE_DRIVER=s3 (use \"us-east-1\" as a placeholder for Garage/SeaweedFS)")
		}
		if c.Storage.S3AccessKeyID == "" {
			errs = append(errs, "STORAGE_S3_ACCESS_KEY_ID is required when STORAGE_DRIVER=s3")
		}
		if c.Storage.S3SecretAccessKey == "" {
			errs = append(errs, "STORAGE_S3_SECRET_ACCESS_KEY is required when STORAGE_DRIVER=s3")
		}
		if c.Storage.S3ForceSSL && c.Storage.S3Endpoint != "" && !strings.HasPrefix(c.Storage.S3Endpoint, "https://") {
			errs = append(errs, "STORAGE_S3_ENDPOINT must use https:// when STORAGE_S3_FORCE_SSL=true (or set STORAGE_S3_FORCE_SSL=false for a plaintext endpoint)")
		}
	default:
		errs = append(errs, fmt.Sprintf("STORAGE_DRIVER=%q is not recognized (expected: local, fs, s3)", c.Storage.Driver))
	}

	// --- Production requirements ---
	isProd := c.App.Env == "production"

	if isProd && c.OAuth.SessionSecret == "" {
		errs = append(errs, "OAUTH_SESSION_SECRET is required in production")
	} else if isProd && len(c.OAuth.SessionSecret) < 32 {
		errs = append(errs, "OAUTH_SESSION_SECRET must be at least 32 characters in production")
	}
	if isProd && c.Database.Password == "" {
		errs = append(errs, "DB_PASSWORD is required in production")
	}
	if isProd && c.App.EncryptionKey == "" {
		errs = append(errs, "ENCRYPTION_KEY is required in production (secrets stored in plaintext without it)")
	}
	if isProd && c.App.URL == "http://localhost" {
		errs = append(errs, "APP_URL must be set to the actual URL in production (currently http://localhost)")
	}
	if isProd && !strings.HasPrefix(c.App.URL, "https://") {
		errs = append(errs, "APP_URL should use https:// in production (session cookies require Secure flag)")
	}
	if isProd && c.App.InternalAPIToken == "" {
		errs = append(errs, "INTERNAL_API_TOKEN is required in production")
	}
	if isProd && c.App.Debug {
		errs = append(errs, "APP_DEBUG should not be enabled in production")
	}
	if isProd && c.OAuth.HKDFSalt == "DocuMCP-go-v1" {
		errs = append(errs, "HKDF_SALT must be changed from the default value in production")
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed: %s", strings.Join(errs, "; "))
	}

	return nil
}

// splitComma splits a string on commas, trims whitespace, and drops empty
// elements. This replaces viper.GetStringSlice for env vars, which splits on
// spaces — unreliable when values contain colons (IPv6) or are injected via
// Docker env_file.
func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
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

// clampInt32 safely converts an int to int32, clamping to the int32 range.
func clampInt32(v int) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v) //nolint:gosec // clamped above
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
