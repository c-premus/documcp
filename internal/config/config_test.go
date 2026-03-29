package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseCIDRs(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    int // number of nets returned
		wantErr bool
	}{
		{"nil input", nil, 0, false},
		{"empty slice", []string{}, 0, false},
		{"single CIDR", []string{"10.0.0.0/8"}, 1, false},
		{"multiple CIDRs", []string{"10.0.0.0/8", "172.16.0.0/12"}, 2, false},
		{"bare IPv4 auto-promotes to /32", []string{"10.0.0.1"}, 1, false},
		{"bare IPv6 auto-promotes to /128", []string{"fd00::1"}, 1, false},
		{"mixed bare and CIDR", []string{"10.0.0.1", "172.16.0.0/12"}, 2, false},
		{"whitespace trimmed", []string{"  10.0.0.0/8  ", " 172.16.0.0/12 "}, 2, false},
		{"empty entries skipped", []string{"10.0.0.0/8", "", "  ", "172.16.0.0/12"}, 2, false},
		{"invalid IP", []string{"not-an-ip"}, 0, true},
		{"invalid CIDR", []string{"10.0.0.0/99"}, 0, true},
		{"IPv6 CIDR", []string{"fd00::/8"}, 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCIDRs(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tt.want {
				t.Errorf("ParseCIDRs() returned %d nets, want %d", len(got), tt.want)
			}
		})
	}
}

func TestParseCIDRs_BareIPContains(t *testing.T) {
	// A bare IPv4 should produce a /32 that contains only that exact IP.
	nets, err := ParseCIDRs([]string{"10.0.0.1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nets) != 1 {
		t.Fatalf("expected 1 net, got %d", len(nets))
	}
	if nets[0].String() != "10.0.0.1/32" {
		t.Errorf("got %s, want 10.0.0.1/32", nets[0].String())
	}
}

// setEnv is a test helper that sets an environment variable and registers
// cleanup to restore the original value after the test completes.
func setEnv(t *testing.T, key, value string) {
	t.Helper()
	prev, existed := os.LookupEnv(key)
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, prev)
		} else {
			_ = os.Unsetenv(key)
		}
	})
	_ = os.Setenv(key, value)
}

func TestLoad_Defaults(t *testing.T) {
	// Ensure no env vars interfere with default tests.
	for _, key := range []string{
		"APP_NAME", "APP_ENV", "APP_DEBUG", "APP_URL", "APP_TIMEZONE",
		"SERVER_HOST", "SERVER_PORT",
		"DB_HOST", "DB_PORT", "DB_DATABASE", "DB_USERNAME", "DB_PASSWORD", "DB_SSLMODE",
		"DB_MAX_OPEN_CONNS", "DB_MAX_IDLE_CONNS", "DB_MAX_LIFETIME",
		"OTEL_ENABLED", "OTEL_SERVICE_NAME", "OTEL_INSECURE",
		"OAUTH_PKCE_REQUIRED",
		"STORAGE_DRIVER", "STORAGE_BASE_PATH", "STORAGE_DOCUMENT_PATH", "STORAGE_TEMP_PATH",
		"DOCUMCP_ENDPOINT", "DOCUMCP_NAME", "DOCUMCP_VERSION",
	} {
		setEnv(t, key, "")
		_ = os.Unsetenv(key)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	tests := []struct {
		name string
		got  any
		want any
	}{
		// App
		{"App.Name", cfg.App.Name, "DocuMCP"},
		{"App.Env", cfg.App.Env, "development"},
		{"App.Debug", cfg.App.Debug, false},
		{"App.URL", cfg.App.URL, "http://localhost"},
		{"App.Timezone", cfg.App.Timezone, "UTC"},

		// Server
		{"Server.Host", cfg.Server.Host, "0.0.0.0"},
		{"Server.Port", cfg.Server.Port, 8080},
		{"Server.ReadTimeout", cfg.Server.ReadTimeout, 30 * time.Second},
		{"Server.WriteTimeout", cfg.Server.WriteTimeout, 30 * time.Second},
		{"Server.IdleTimeout", cfg.Server.IdleTimeout, 120 * time.Second},

		// Database
		{"Database.Host", cfg.Database.Host, "127.0.0.1"},
		{"Database.Port", cfg.Database.Port, 5432},
		{"Database.SSLMode", cfg.Database.SSLMode, "require"},
		{"Database.MaxOpenConns", cfg.Database.MaxOpenConns, 25},
		{"Database.MaxIdleConns", cfg.Database.MaxIdleConns, 5},
		{"Database.MaxLifetime", cfg.Database.MaxLifetime, 5 * time.Minute},

		// OAuth
		{"OAuth.AuthCodeLifetime", cfg.OAuth.AuthCodeLifetime, 10 * time.Minute},
		{"OAuth.AccessTokenLifetime", cfg.OAuth.AccessTokenLifetime, 1 * time.Hour},
		{"OAuth.RefreshTokenLifetime", cfg.OAuth.RefreshTokenLifetime, 30 * 24 * time.Hour},
		{"OAuth.DeviceCodeLifetime", cfg.OAuth.DeviceCodeLifetime, 10 * time.Minute},
		{"OAuth.DeviceCodeInterval", cfg.OAuth.DeviceCodeInterval, 5 * time.Second},

		// Storage
		{"Storage.Driver", cfg.Storage.Driver, "local"},
		{"Storage.BasePath", cfg.Storage.BasePath, "./storage"},
		{"Storage.DocumentPath", cfg.Storage.DocumentPath, "documents"},
		{"Storage.TempPath", cfg.Storage.TempPath, "tmp"},

		// OTEL
		{"OTEL.Enabled", cfg.OTEL.Enabled, false},
		{"OTEL.ServiceName", cfg.OTEL.ServiceName, "documcp"},
		{"OTEL.Insecure", cfg.OTEL.Insecure, false},

		// DocuMCP
		{"DocuMCP.Endpoint", cfg.DocuMCP.Endpoint, "/documcp"},
		{"DocuMCP.ServerName", cfg.DocuMCP.ServerName, "DocuMCP"},
		{"DocuMCP.ServerVersion", cfg.DocuMCP.ServerVersion, "0.1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %v (%T), want %v (%T)", tt.got, tt.got, tt.want, tt.want)
			}
		})
	}

	// Check OIDC scopes separately since slices are not comparable with !=.
	t.Run("OIDC.Scopes", func(t *testing.T) {
		want := []string{"openid", "profile", "email"}
		if len(cfg.OIDC.Scopes) != len(want) {
			t.Fatalf("got %v, want %v", cfg.OIDC.Scopes, want)
		}
		for i, s := range cfg.OIDC.Scopes {
			if s != want[i] {
				t.Errorf("scope[%d] = %q, want %q", i, s, want[i])
			}
		}
	})
}

func TestLoad_EnvOverrides(t *testing.T) {
	setEnv(t, "APP_NAME", "TestApp")
	setEnv(t, "APP_ENV", "testing")
	setEnv(t, "APP_DEBUG", "true")
	setEnv(t, "DB_HOST", "db.example.com")
	setEnv(t, "DB_PORT", "5433")
	setEnv(t, "DB_DATABASE", "testdb")
	setEnv(t, "DB_USERNAME", "testuser")
	setEnv(t, "DB_PASSWORD", "secret")
	setEnv(t, "SERVER_PORT", "9090")
	setEnv(t, "OTEL_ENABLED", "true")
	setEnv(t, "OAUTH_PKCE_REQUIRED", "false")
	setEnv(t, "DOCUMCP_NAME", "MyMCP")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"App.Name", cfg.App.Name, "TestApp"},
		{"App.Env", cfg.App.Env, "testing"},
		{"App.Debug", cfg.App.Debug, true},
		{"Database.Host", cfg.Database.Host, "db.example.com"},
		{"Database.Port", cfg.Database.Port, 5433},
		{"Database.Database", cfg.Database.Database, "testdb"},
		{"Database.Username", cfg.Database.Username, "testuser"},
		{"Database.Password", cfg.Database.Password, "secret"},
		{"Server.Port", cfg.Server.Port, 9090},
		{"OTEL.Enabled", cfg.OTEL.Enabled, true},
		{"DocuMCP.ServerName", cfg.DocuMCP.ServerName, "MyMCP"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %v (%T), want %v (%T)", tt.got, tt.got, tt.want, tt.want)
			}
		})
	}
}

// validBaseConfig returns a Config with all always-required fields set.
func validBaseConfig() Config {
	return Config{
		Server: ServerConfig{
			Port:        8080,
			MaxBodySize: 1048576,
		},
		Database: DatabaseConfig{
			Host:         "localhost",
			Database:     "mydb",
			Username:     "admin",
			MaxOpenConns: 25,
			MaxIdleConns: 10,
		},
		Git: GitConfig{
			MaxFileSize:  10 * 1024 * 1024,
			MaxTotalSize: 50 * 1024 * 1024,
		},
	}
}

// validProdConfig returns a Config valid for production.
func validProdConfig() Config {
	cfg := validBaseConfig()
	cfg.App = AppConfig{
		Env:              "production",
		URL:              "https://documcp.example.com",
		InternalAPIToken: "secure-token-here",
		EncryptionKey:    "01234567890123456789012345678901", // 32 bytes
	}
	cfg.Database.Password = "secret"
	cfg.OAuth.SessionSecret = "my-session-secret-that-is-long-enough-for-production"
	return cfg
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid config",
			cfg:     validBaseConfig(),
			wantErr: false,
		},
		{
			name:    "valid production config",
			cfg:     validProdConfig(),
			wantErr: false,
		},
		{
			name: "missing host",
			cfg: Config{
				Database: DatabaseConfig{Database: "mydb", Username: "admin"},
			},
			wantErr: true,
			errMsg:  "database host is required",
		},
		{
			name: "missing database",
			cfg: Config{
				Database: DatabaseConfig{Host: "localhost", Username: "admin"},
			},
			wantErr: true,
			errMsg:  "database name is required",
		},
		{
			name: "missing username",
			cfg: Config{
				Database: DatabaseConfig{Host: "localhost", Database: "mydb"},
			},
			wantErr: true,
			errMsg:  "database username is required",
		},
		{
			name:    "all missing",
			cfg:     Config{},
			wantErr: true,
			errMsg:  "database host is required",
		},
		// APP_ENV validation
		{
			name: "invalid app env",
			cfg: func() Config {
				c := validBaseConfig()
				c.App.Env = "invalid"
				return c
			}(),
			wantErr: true,
			errMsg:  "APP_ENV must be one of",
		},
		{
			name: "valid app env development",
			cfg: func() Config {
				c := validBaseConfig()
				c.App.Env = "development"
				return c
			}(),
			wantErr: false,
		},
		{
			name: "valid app env staging",
			cfg: func() Config {
				c := validBaseConfig()
				c.App.Env = "staging"
				return c
			}(),
			wantErr: false,
		},
		{
			name: "valid app env testing",
			cfg: func() Config {
				c := validBaseConfig()
				c.App.Env = "testing"
				return c
			}(),
			wantErr: false,
		},
		{
			name: "empty app env is valid",
			cfg: func() Config {
				c := validBaseConfig()
				c.App.Env = ""
				return c
			}(),
			wantErr: false,
		},
		// ENCRYPTION_KEY length validation
		{
			name: "encryption key wrong length",
			cfg: func() Config {
				c := validBaseConfig()
				c.App.EncryptionKey = "too-short"
				return c
			}(),
			wantErr: true,
			errMsg:  "ENCRYPTION_KEY must be exactly 32 bytes",
		},
		{
			name: "encryption key correct length",
			cfg: func() Config {
				c := validBaseConfig()
				c.App.EncryptionKey = "01234567890123456789012345678901"
				return c
			}(),
			wantErr: false,
		},
		{
			name: "empty encryption key is valid in non-prod",
			cfg: func() Config {
				c := validBaseConfig()
				c.App.EncryptionKey = ""
				return c
			}(),
			wantErr: false,
		},
		// OTEL validation
		{
			name: "otel enabled without endpoint",
			cfg: func() Config {
				c := validBaseConfig()
				c.OTEL.Enabled = true
				c.OTEL.Endpoint = ""
				return c
			}(),
			wantErr: true,
			errMsg:  "OTEL_EXPORTER_OTLP_ENDPOINT is required when OTEL_ENABLED=true",
		},
		{
			name: "otel enabled with endpoint",
			cfg: func() Config {
				c := validBaseConfig()
				c.OTEL.Enabled = true
				c.OTEL.Endpoint = "http://localhost:4318"
				return c
			}(),
			wantErr: false,
		},
		// Production-only requirements
		{
			name: "production missing session secret",
			cfg: func() Config {
				c := validProdConfig()
				c.OAuth.SessionSecret = ""
				return c
			}(),
			wantErr: true,
			errMsg:  "OAUTH_SESSION_SECRET is required in production",
		},
		{
			name: "production missing db password",
			cfg: func() Config {
				c := validProdConfig()
				c.Database.Password = ""
				return c
			}(),
			wantErr: true,
			errMsg:  "DB_PASSWORD is required in production",
		},
		{
			name: "production missing encryption key",
			cfg: func() Config {
				c := validProdConfig()
				c.App.EncryptionKey = ""
				return c
			}(),
			wantErr: true,
			errMsg:  "ENCRYPTION_KEY is required in production",
		},
		{
			name: "production default app url",
			cfg: func() Config {
				c := validProdConfig()
				c.App.URL = "http://localhost"
				return c
			}(),
			wantErr: true,
			errMsg:  "APP_URL must be set to the actual URL in production",
		},
		{
			name: "production missing internal api token",
			cfg: func() Config {
				c := validProdConfig()
				c.App.InternalAPIToken = ""
				return c
			}(),
			wantErr: true,
			errMsg:  "INTERNAL_API_TOKEN is required in production",
		},
		// Non-production does not require production fields
		{
			name: "development allows missing password",
			cfg: func() Config {
				c := validBaseConfig()
				c.App.Env = "development"
				return c
			}(),
			wantErr: false,
		},
		// Numeric range checks
		{
			name: "port zero is invalid",
			cfg: func() Config {
				c := validBaseConfig()
				c.Server.Port = 0
				return c
			}(),
			wantErr: true,
			errMsg:  "SERVER_PORT must be between 1 and 65535",
		},
		{
			name: "port exceeds max",
			cfg: func() Config {
				c := validBaseConfig()
				c.Server.Port = 70000
				return c
			}(),
			wantErr: true,
			errMsg:  "SERVER_PORT must be between 1 and 65535",
		},
		{
			name: "max body size zero is invalid",
			cfg: func() Config {
				c := validBaseConfig()
				c.Server.MaxBodySize = 0
				return c
			}(),
			wantErr: true,
			errMsg:  "SERVER_MAX_BODY_SIZE must be positive",
		},
		{
			name: "idle conns exceeds open conns",
			cfg: func() Config {
				c := validBaseConfig()
				c.Database.MaxOpenConns = 10
				c.Database.MaxIdleConns = 20
				return c
			}(),
			wantErr: true,
			errMsg:  "DB_MAX_IDLE_CONNS must not exceed DB_MAX_OPEN_CONNS",
		},
		{
			name: "idle conns equals open conns is valid",
			cfg: func() Config {
				c := validBaseConfig()
				c.Database.MaxOpenConns = 10
				c.Database.MaxIdleConns = 10
				return c
			}(),
			wantErr: false,
		},
		{
			name: "git max file size zero is invalid",
			cfg: func() Config {
				c := validBaseConfig()
				c.Git.MaxFileSize = 0
				return c
			}(),
			wantErr: true,
			errMsg:  "GIT_MAX_FILE_SIZE must be positive",
		},
		{
			name: "git max total size zero is invalid",
			cfg: func() Config {
				c := validBaseConfig()
				c.Git.MaxTotalSize = 0
				return c
			}(),
			wantErr: true,
			errMsg:  "GIT_MAX_TOTAL_SIZE must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestConfig_DatabaseDSN(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "with password",
			cfg: Config{
				Database: DatabaseConfig{
					Host:     "db.example.com",
					Port:     5432,
					Username: "admin",
					Password: "secret",
					Database: "mydb",
					SSLMode:  "require",
				},
			},
			want: "host=db.example.com port=5432 user=admin dbname=mydb sslmode=require password='secret'",
		},
		{
			name: "without password",
			cfg: Config{
				Database: DatabaseConfig{
					Host:     "localhost",
					Port:     5432,
					Username: "postgres",
					Database: "testdb",
					SSLMode:  "disable",
				},
			},
			want: "host=localhost port=5432 user=postgres dbname=testdb sslmode=disable",
		},
		{
			name: "custom port",
			cfg: Config{
				Database: DatabaseConfig{
					Host:     "10.0.0.1",
					Port:     5433,
					Username: "app",
					Password: "p@ss",
					Database: "production",
					SSLMode:  "verify-full",
				},
			},
			want: "host=10.0.0.1 port=5433 user=app dbname=production sslmode=verify-full password='p@ss'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.DatabaseDSN()
			if got != tt.want {
				t.Errorf("DatabaseDSN() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfig_Validate_MultipleErrors(t *testing.T) {
	cfg := Config{
		Database: DatabaseConfig{},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "database host is required") {
		t.Errorf("error should mention missing host: %s", msg)
	}
	if !strings.Contains(msg, "database name is required") {
		t.Errorf("error should mention missing database: %s", msg)
	}
	if !strings.Contains(msg, "database username is required") {
		t.Errorf("error should mention missing username: %s", msg)
	}
}

func TestConfig_Validate_ErrorMessageFormat(t *testing.T) {
	cfg := Config{
		Database: DatabaseConfig{
			Host:     "localhost",
			Username: "admin",
			// Database is missing.
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	msg := err.Error()
	if !strings.HasPrefix(msg, "config validation failed:") {
		t.Errorf("error message should start with 'config validation failed:', got: %s", msg)
	}

	// Should contain DB_DATABASE hint.
	if !strings.Contains(msg, "DB_DATABASE") {
		t.Errorf("error message should mention env var hint DB_DATABASE: %s", msg)
	}
}

func TestConfig_Validate_SingleFieldMissing(t *testing.T) {
	tests := []struct {
		name   string
		cfg    Config
		errMsg string
	}{
		{
			name: "only host missing",
			cfg: Config{
				Database: DatabaseConfig{Database: "mydb", Username: "admin"},
			},
			errMsg: "database host is required",
		},
		{
			name: "only database missing",
			cfg: Config{
				Database: DatabaseConfig{Host: "localhost", Username: "admin"},
			},
			errMsg: "database name is required",
		},
		{
			name: "only username missing",
			cfg: Config{
				Database: DatabaseConfig{Host: "localhost", Database: "mydb"},
			},
			errMsg: "database username is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestConfig_DatabaseDSN_ZeroPort(t *testing.T) {
	cfg := Config{
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     0,
			Username: "user",
			Database: "db",
			SSLMode:  "disable",
		},
	}

	dsn := cfg.DatabaseDSN()
	if !strings.Contains(dsn, "port=0") {
		t.Errorf("DatabaseDSN() should contain port=0 when port is zero, got: %q", dsn)
	}
}

func TestConfig_DatabaseDSN_EmptyPassword(t *testing.T) {
	cfg := Config{
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			Username: "user",
			Password: "",
			Database: "db",
			SSLMode:  "disable",
		},
	}

	dsn := cfg.DatabaseDSN()
	if strings.Contains(dsn, "password=") {
		t.Errorf("DatabaseDSN() should not contain password when empty, got: %q", dsn)
	}
}

func TestConfig_Validate_ValidWithMinimumFields(t *testing.T) {
	cfg := Config{
		Server: ServerConfig{
			Port:        8080,
			MaxBodySize: 1,
		},
		Database: DatabaseConfig{
			Host:     "h",
			Database: "d",
			Username: "u",
		},
		Git: GitConfig{
			MaxFileSize:  1,
			MaxTotalSize: 1,
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}
}

func TestConfig_Validate_ProductionMultipleErrors(t *testing.T) {
	cfg := Config{
		Server: ServerConfig{
			Port:        8080,
			MaxBodySize: 1,
		},
		App: AppConfig{
			Env: "production",
			URL: "http://localhost",
		},
		Database: DatabaseConfig{
			Host:     "localhost",
			Database: "mydb",
			Username: "admin",
		},
		Git: GitConfig{
			MaxFileSize:  1,
			MaxTotalSize: 1,
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	msg := err.Error()
	expected := []string{
		"OAUTH_SESSION_SECRET is required in production",
		"DB_PASSWORD is required in production",
		"ENCRYPTION_KEY is required in production",
		"APP_URL must be set to the actual URL in production",
		"INTERNAL_API_TOKEN is required in production",
	}
	for _, e := range expected {
		if !strings.Contains(msg, e) {
			t.Errorf("error should contain %q, got: %s", e, msg)
		}
	}
}

func TestLoad_FromYAMLConfigFile(t *testing.T) {
	// Write a temporary YAML config file and point DOCUMCP_CONFIG_PATH to it.
	dir := t.TempDir()
	configFile := dir + "/test-config.yaml"
	yamlContent := `
app_name: YAMLApp
app_env: staging
server_port: 3000
db_host: yaml-db-host
db_database: yamldb
db_username: yamluser
`
	if err := os.WriteFile(configFile, []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	setEnv(t, "DOCUMCP_CONFIG_PATH", configFile)
	// Clear env vars that might override YAML values.
	for _, key := range []string{"APP_NAME", "APP_ENV", "SERVER_PORT", "DB_HOST", "DB_DATABASE", "DB_USERNAME"} {
		prev, existed := os.LookupEnv(key)
		if existed {
			t.Cleanup(func() { _ = os.Setenv(key, prev) })
		} else {
			t.Cleanup(func() { _ = os.Unsetenv(key) })
		}
		_ = os.Unsetenv(key)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.App.Name != "YAMLApp" {
		t.Errorf("App.Name = %q, want %q", cfg.App.Name, "YAMLApp")
	}
	if cfg.App.Env != "staging" {
		t.Errorf("App.Env = %q, want %q", cfg.App.Env, "staging")
	}
	if cfg.Server.Port != 3000 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 3000)
	}
	if cfg.Database.Host != "yaml-db-host" {
		t.Errorf("Database.Host = %q, want %q", cfg.Database.Host, "yaml-db-host")
	}
}

func TestLoad_NonExistentExplicitConfigFileReturnsError(t *testing.T) {
	setEnv(t, "DOCUMCP_CONFIG_PATH", "/nonexistent/path/config.yaml")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for non-existent explicit config file, got nil")
	}
}
