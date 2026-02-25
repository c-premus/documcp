package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

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
		"REDIS_HOST", "REDIS_PORT", "REDIS_PASSWORD", "REDIS_DB",
		"MEILISEARCH_HOST", "MEILISEARCH_KEY",
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
		{"App.Env", cfg.App.Env, "production"},
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
		{"Database.SSLMode", cfg.Database.SSLMode, "disable"},
		{"Database.MaxOpenConns", cfg.Database.MaxOpenConns, 25},
		{"Database.MaxIdleConns", cfg.Database.MaxIdleConns, 5},
		{"Database.MaxLifetime", cfg.Database.MaxLifetime, 5 * time.Minute},

		// Redis
		{"Redis.Host", cfg.Redis.Host, "localhost"},
		{"Redis.Port", cfg.Redis.Port, 6379},
		{"Redis.DB", cfg.Redis.DB, 0},

		// Meilisearch
		{"Meilisearch.Host", cfg.Meilisearch.Host, "http://localhost:7700"},

		// OAuth
		{"OAuth.AuthCodeLifetime", cfg.OAuth.AuthCodeLifetime, 10 * time.Minute},
		{"OAuth.AccessTokenLifetime", cfg.OAuth.AccessTokenLifetime, 1 * time.Hour},
		{"OAuth.RefreshTokenLifetime", cfg.OAuth.RefreshTokenLifetime, 30 * 24 * time.Hour},
		{"OAuth.DeviceCodeLifetime", cfg.OAuth.DeviceCodeLifetime, 10 * time.Minute},
		{"OAuth.DeviceCodeInterval", cfg.OAuth.DeviceCodeInterval, 5 * time.Second},
		{"OAuth.RequirePKCE", cfg.OAuth.RequirePKCE, true},

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
	setEnv(t, "REDIS_PORT", "6380")
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
		{"Redis.Port", cfg.Redis.Port, 6380},
		{"Server.Port", cfg.Server.Port, 9090},
		{"OTEL.Enabled", cfg.OTEL.Enabled, true},
		{"OAuth.RequirePKCE", cfg.OAuth.RequirePKCE, false},
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

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			cfg: Config{
				Database: DatabaseConfig{
					Host:     "localhost",
					Database: "mydb",
					Username: "admin",
				},
			},
			wantErr: false,
		},
		{
			name: "missing host",
			cfg: Config{
				Database: DatabaseConfig{
					Database: "mydb",
					Username: "admin",
				},
			},
			wantErr: true,
			errMsg:  "database host is required",
		},
		{
			name: "missing database",
			cfg: Config{
				Database: DatabaseConfig{
					Host:     "localhost",
					Username: "admin",
				},
			},
			wantErr: true,
			errMsg:  "database name is required",
		},
		{
			name: "missing username",
			cfg: Config{
				Database: DatabaseConfig{
					Host:     "localhost",
					Database: "mydb",
				},
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
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
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
			want: "host=db.example.com port=5432 user=admin dbname=mydb sslmode=require password=secret",
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
			want: "host=10.0.0.1 port=5433 user=app dbname=production sslmode=verify-full password=p@ss",
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

