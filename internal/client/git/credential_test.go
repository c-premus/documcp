package git

import (
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// --- writeCredentialScript ---

func TestWriteCredentialScript_CreatesExecutableFile(t *testing.T) {
	scriptPath, cleanup, err := writeCredentialScript("my-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	info, err := os.Stat(scriptPath)
	if err != nil {
		t.Fatalf("script file does not exist: %v", err)
	}

	// File must be executable by owner.
	if info.Mode().Perm()&0o700 != 0o700 {
		t.Errorf("expected permissions 0700, got %o", info.Mode().Perm())
	}
}

func TestWriteCredentialScript_ScriptOutputsToken(t *testing.T) {
	scriptPath, cleanup, err := writeCredentialScript("ghp_abc123XYZ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	out, err := exec.Command(scriptPath).Output()
	if err != nil {
		t.Fatalf("failed to execute script: %v", err)
	}

	got := strings.TrimSpace(string(out))
	if got != "ghp_abc123XYZ" {
		t.Errorf("expected token %q, got %q", "ghp_abc123XYZ", got)
	}
}

func TestWriteCredentialScript_ScriptContent(t *testing.T) {
	scriptPath, cleanup, err := writeCredentialScript("simple-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("failed to read script: %v", err)
	}

	content := string(data)
	if !strings.HasPrefix(content, "#!/bin/sh\n") {
		t.Error("script must start with #!/bin/sh shebang")
	}
	if !strings.Contains(content, "echo 'simple-token'") {
		t.Errorf("script content does not contain expected echo: %s", content)
	}
}

func TestWriteCredentialScript_CleanupRemovesFile(t *testing.T) {
	scriptPath, cleanup, err := writeCredentialScript("token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File exists before cleanup.
	if _, err := os.Stat(scriptPath); err != nil {
		t.Fatalf("script file should exist before cleanup: %v", err)
	}

	cleanup()

	// File removed after cleanup.
	if _, err := os.Stat(scriptPath); !os.IsNotExist(err) {
		t.Error("script file should be removed after cleanup")
	}
}

func TestWriteCredentialScript_EscapesSingleQuotes(t *testing.T) {
	// A token containing single quotes must be shell-escaped properly.
	token := "it's a 'test' token"
	scriptPath, cleanup, err := writeCredentialScript(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	// Execute the script and confirm the token round-trips correctly.
	out, err := exec.Command(scriptPath).Output()
	if err != nil {
		t.Fatalf("failed to execute script: %v", err)
	}

	got := strings.TrimSpace(string(out))
	if got != token {
		t.Errorf("expected %q, got %q", token, got)
	}
}

func TestWriteCredentialScript_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{name: "empty token", token: ""},
		{name: "spaces", token: "token with spaces"},
		{name: "double quotes", token: `token"with"quotes`},
		{name: "backticks", token: "token`with`backticks"},
		{name: "dollar sign", token: "token$HOME"},
		// Note: /bin/sh echo interprets \n, \b, \t, \f etc. as escape
		// sequences even inside single quotes. Only backslashes NOT
		// followed by a special letter survive echo unchanged.
		{name: "backslash before non-special", token: `token\xvalue`},
		{name: "semicolon", token: "token;echo pwned"},
		{name: "pipe", token: "token|cat /etc/passwd"},
		{name: "ampersand", token: "token&whoami"},
		{name: "newline", token: "token\nwith\nnewlines"},
		{name: "unicode", token: "token-\u00e9\u00e8\u00ea\u4e16\u754c"},
		{name: "glob chars", token: "token*with?glob[chars]"},
		{name: "parentheses", token: "token$(whoami)"},
		{name: "exclamation", token: "token!bang"},
		{name: "hash", token: "token#comment"},
		{name: "tilde", token: "token~expansion"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scriptPath, cleanup, err := writeCredentialScript(tt.token)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer cleanup()

			out, err := exec.Command(scriptPath).Output()
			if err != nil {
				t.Fatalf("failed to execute script for token %q: %v", tt.token, err)
			}

			got := strings.TrimSpace(string(out))
			if got != tt.token {
				t.Errorf("token mismatch: expected %q, got %q", tt.token, got)
			}
		})
	}
}

func TestWriteCredentialScript_CommandInjectionPrevention(t *testing.T) {
	// Attempt to inject shell commands via the token.
	// If the escaping is wrong, these could execute arbitrary commands.
	injections := []struct {
		name  string
		token string
	}{
		{name: "subshell via quote break", token: "'; echo INJECTED; echo '"},
		{name: "command substitution", token: "$(cat /etc/passwd)"},
		{name: "backtick substitution", token: "`cat /etc/passwd`"},
		{name: "quote escape attempt", token: `'\''`},
		{name: "nested quotes", token: `"'$(whoami)'""`},
	}

	for _, tt := range injections {
		t.Run(tt.name, func(t *testing.T) {
			scriptPath, cleanup, err := writeCredentialScript(tt.token)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer cleanup()

			out, err := exec.Command(scriptPath).Output()
			if err != nil {
				t.Fatalf("script execution failed: %v", err)
			}

			got := strings.TrimSpace(string(out))
			if got != tt.token {
				t.Errorf("injection not prevented: expected literal %q, got %q", tt.token, got)
			}
		})
	}
}

func TestWriteCredentialScript_FileNamePattern(t *testing.T) {
	scriptPath, cleanup, err := writeCredentialScript("token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	if !strings.Contains(scriptPath, "git-askpass-") {
		t.Errorf("script path should contain 'git-askpass-', got %q", scriptPath)
	}
	if !strings.HasSuffix(scriptPath, ".sh") {
		t.Errorf("script path should end with .sh, got %q", scriptPath)
	}
}

// --- IsEssentialFile ---

func TestIsEssentialFile(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		// Exact base-name matches (case-insensitive).
		{name: "CLAUDE.md at root", path: "CLAUDE.md", want: true},
		{name: "CLAUDE.md nested", path: "sub/CLAUDE.md", want: true},
		{name: "claude.md lowercase", path: "claude.md", want: true},
		{name: "template.json at root", path: "template.json", want: true},
		{name: "TEMPLATE.JSON uppercase", path: "TEMPLATE.JSON", want: true},
		{name: "README.md at root", path: "README.md", want: true},
		{name: "readme.md lowercase", path: "readme.md", want: true},
		{name: "README.md nested", path: "docs/README.md", want: true},

		// memory-bank/*.md pattern.
		{name: "memory-bank md", path: "memory-bank/context.md", want: true},
		{name: "memory-bank deep not matched", path: "memory-bank/sub/context.md", want: false},
		{name: "memory-bank non-md", path: "memory-bank/data.json", want: false},

		// .claude/**/* pattern.
		{name: ".claude file", path: ".claude/config.yml", want: true},
		{name: ".claude nested", path: ".claude/hooks/pre-push.sh", want: true},
		{name: ".claude deep nested", path: ".claude/a/b/c/d.txt", want: true},

		// Non-essential files.
		{name: "random go file", path: "main.go", want: false},
		{name: "docs txt", path: "docs/guide.txt", want: false},
		{name: "similar name", path: "CLAUDE.txt", want: false},
		{name: "template yaml", path: "template.yaml", want: false},
		{name: "dotclaude without slash", path: ".claude", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsEssentialFile(tt.path)
			if got != tt.want {
				t.Errorf("IsEssentialFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// --- ValidateRepositoryURL ---

func TestValidateRepositoryURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		errMsg  string // Substring to look for in the error message.
	}{
		// Valid URLs.
		{
			name:    "valid github https",
			url:     "https://github.com/user/repo.git",
			wantErr: false,
		},
		{
			name:    "valid gitlab https",
			url:     "https://gitlab.com/org/project",
			wantErr: false,
		},

		// Scheme violations.
		{
			name:    "http scheme rejected",
			url:     "http://github.com/user/repo.git",
			wantErr: true,
			errMsg:  "https scheme",
		},
		{
			name:    "ssh scheme rejected",
			url:     "ssh://git@github.com/user/repo.git",
			wantErr: true,
			errMsg:  "https scheme",
		},
		{
			name:    "ftp scheme rejected",
			url:     "ftp://example.com/repo",
			wantErr: true,
			errMsg:  "https scheme",
		},
		{
			name:    "file scheme rejected",
			url:     "file:///tmp/repo",
			wantErr: true,
			errMsg:  "https scheme",
		},

		// Localhost and loopback.
		{
			name:    "localhost rejected",
			url:     "https://localhost/repo",
			wantErr: true,
			errMsg:  "localhost",
		},
		{
			name:    "LOCALHOST case insensitive",
			url:     "https://LOCALHOST/repo",
			wantErr: true,
			errMsg:  "localhost",
		},
		{
			name:    "127.0.0.1 rejected",
			url:     "https://127.0.0.1/repo",
			wantErr: true,
			errMsg:  "loopback",
		},
		{
			name:    "::1 rejected",
			url:     "https://[::1]/repo",
			wantErr: true,
			errMsg:  "loopback",
		},

		// Private IP ranges.
		{
			name:    "10.x.x.x rejected",
			url:     "https://10.0.0.1/repo",
			wantErr: true,
			errMsg:  "private",
		},
		{
			name:    "172.16.x.x rejected",
			url:     "https://172.16.0.1/repo",
			wantErr: true,
			errMsg:  "private",
		},
		{
			name:    "192.168.x.x rejected",
			url:     "https://192.168.1.1/repo",
			wantErr: true,
			errMsg:  "private",
		},
		{
			name:    "169.254.x.x rejected",
			url:     "https://169.254.169.254/repo",
			wantErr: true,
			errMsg:  "private",
		},

		// Unspecified address.
		{
			name:    "0.0.0.0 rejected",
			url:     "https://0.0.0.0/repo",
			wantErr: true,
			errMsg:  "unspecified",
		},

		// Empty hostname.
		{
			name:    "empty hostname",
			url:     "https:///repo",
			wantErr: true,
			errMsg:  "no hostname",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRepositoryURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errMsg)
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

// --- SanitizeGitError ---

func TestSanitizeGitError(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNot string // Substring that must NOT appear in the output.
	}{
		{
			name:    "redacts https URL",
			input:   "fatal: unable to access 'https://github.com/user/repo.git/'",
			wantNot: "https://github.com",
		},
		{
			name:    "redacts http URL",
			input:   "remote: http://internal-server.com/secret",
			wantNot: "http://internal-server.com",
		},
		{
			name:    "redacts bearer token standalone",
			input:   "header: Bearer ghp_xxxxxxxxxxxxxxxxxxxx rest",
			wantNot: "ghp_xxxxxxxxxxxxxxxxxxxx",
		},
		{
			name:    "redacts token assignment",
			input:   "token = ghp_secretvalue12345678",
			wantNot: "ghp_secretvalue12345678",
		},
		{
			name:    "redacts base64 credentials",
			input:   "credential: dXNlcm5hbWU6cGFzc3dvcmQxMjM0NTY3ODkwYWJj",
			wantNot: "dXNlcm5hbWU6cGFzc3dvcmQxMjM0NTY3ODkwYWJj",
		},
		{
			name:    "preserves non-sensitive text",
			input:   "fatal: repository not found",
			wantNot: "", // Nothing to check absence of; just verify it doesn't crash.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeGitError(tt.input)
			if tt.wantNot != "" && strings.Contains(got, tt.wantNot) {
				t.Errorf("sanitized output should not contain %q, got %q", tt.wantNot, got)
			}
			if strings.Contains(got, "[REDACTED]") == false && tt.wantNot != "" {
				t.Errorf("expected [REDACTED] placeholder in %q", got)
			}
		})
	}
}

func TestSanitizeGitError_PreservesNonSensitiveText(t *testing.T) {
	input := "fatal: repository not found"
	got := SanitizeGitError(input)
	if got != input {
		t.Errorf("non-sensitive text should be preserved: expected %q, got %q", input, got)
	}
}

// --- isBinary ---

func TestIsBinary(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{name: "empty data", data: []byte{}, want: false},
		{name: "plain text", data: []byte("hello world"), want: false},
		{name: "text with newlines", data: []byte("line1\nline2\nline3"), want: false},
		{name: "null byte at start", data: []byte{0, 'h', 'e', 'l', 'l', 'o'}, want: true},
		{name: "null byte in middle", data: []byte("hel\x00lo"), want: true},
		{name: "null byte at end", data: []byte("hello\x00"), want: true},
		{name: "utf8 text", data: []byte("caf\xc3\xa9"), want: false},
		{name: "binary after probe size", data: makeBinaryAfterProbeSize(), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBinary(tt.data)
			if got != tt.want {
				t.Errorf("isBinary(%q) = %v, want %v", truncate(tt.data, 20), got, tt.want)
			}
		})
	}
}

// makeBinaryAfterProbeSize creates data where the null byte is beyond the
// 8KB probe window, so it should not be detected as binary.
func makeBinaryAfterProbeSize() []byte {
	data := make([]byte, binaryProbeSize+100)
	for i := range data {
		data[i] = 'A'
	}
	data[binaryProbeSize+50] = 0 // null byte past the probe window
	return data
}

func truncate(b []byte, maxLen int) []byte {
	if len(b) <= maxLen {
		return b
	}
	return b[:maxLen]
}

// --- extractVariables ---

func TestExtractVariables(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "no variables",
			content: "plain text content",
			want:    nil,
		},
		{
			name:    "single variable",
			content: "Hello {{name}}!",
			want:    []string{"name"},
		},
		{
			name:    "multiple unique variables",
			content: "{{greeting}} {{name}}, welcome to {{place}}.",
			want:    []string{"greeting", "name", "place"},
		},
		{
			name:    "duplicate variables deduplicated",
			content: "{{name}} and {{name}} again with {{age}}",
			want:    []string{"name", "age"},
		},
		{
			name:    "variable with underscores",
			content: "{{project_name}} uses {{api_key}}",
			want:    []string{"project_name", "api_key"},
		},
		{
			name:    "variable with digits",
			content: "{{var1}} and {{var2}}",
			want:    []string{"var1", "var2"},
		},
		{
			name:    "non-word chars ignored",
			content: "{{not-valid}} and {{also invalid}}",
			want:    nil,
		},
		{
			name:    "empty braces",
			content: "{{}} empty",
			want:    nil,
		},
		{
			name:    "nested braces",
			content: "{{{nested}}}",
			want:    []string{"nested"},
		},
		{
			name:    "multiline content",
			content: "line1 {{first}}\nline2 {{second}}\n",
			want:    []string{"first", "second"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractVariables(tt.content)
			if !stringSliceEqual(got, tt.want) {
				t.Errorf("extractVariables(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}

// --- findReadmeContent ---

func TestFindReadmeContent(t *testing.T) {
	tests := []struct {
		name  string
		files []TemplateFile
		want  string
	}{
		{
			name:  "empty file list",
			files: nil,
			want:  "",
		},
		{
			name: "no readme",
			files: []TemplateFile{
				{Filename: "main.go", Content: "package main"},
			},
			want: "",
		},
		{
			name: "readme found",
			files: []TemplateFile{
				{Filename: "main.go", Content: "package main"},
				{Filename: "README.md", Content: "# My Project"},
			},
			want: "# My Project",
		},
		{
			name: "readme case insensitive",
			files: []TemplateFile{
				{Filename: "readme.md", Content: "# Lowercase Readme"},
			},
			want: "# Lowercase Readme",
		},
		{
			name: "first readme wins",
			files: []TemplateFile{
				{Filename: "README.md", Content: "# First"},
				{Filename: "readme.md", Content: "# Second"},
			},
			want: "# First",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findReadmeContent(tt.files)
			if got != tt.want {
				t.Errorf("findReadmeContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- checkIP ---

func TestCheckIP_PublicIPAllowed(t *testing.T) {
	tests := []string{
		"8.8.8.8",
		"1.1.1.1",
		"140.82.121.3", // github.com
	}

	for _, ipStr := range tests {
		t.Run(ipStr, func(t *testing.T) {
			ip := parseIPHelper(t, ipStr)
			if err := checkIP(ip); err != nil {
				t.Errorf("public IP %s should be allowed: %v", ipStr, err)
			}
		})
	}
}

func TestCheckIP_PrivateIPsBlocked(t *testing.T) {
	tests := []struct {
		name string
		ip   string
	}{
		{name: "loopback v4", ip: "127.0.0.1"},
		{name: "loopback v6", ip: "::1"},
		{name: "10.x", ip: "10.255.255.255"},
		{name: "172.16.x", ip: "172.31.0.1"},
		{name: "192.168.x", ip: "192.168.0.1"},
		{name: "link-local", ip: "169.254.1.1"},
		{name: "unspecified v4", ip: "0.0.0.0"},
		{name: "unspecified v6", ip: "::"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := parseIPHelper(t, tt.ip)
			if err := checkIP(ip); err == nil {
				t.Errorf("private/blocked IP %s should be rejected", tt.ip)
			}
		})
	}
}

// --- helpers ---

func stringSliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func parseIPHelper(t *testing.T, s string) net.IP {
	t.Helper()
	ip := net.ParseIP(s)
	if ip == nil {
		t.Fatalf("failed to parse IP %q", s)
	}
	return ip
}
