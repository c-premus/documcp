package git

import (
	"fmt"
	"os"
)

// writeCredentialScript creates a temporary shell script for GIT_ASKPASS
// that echoes the token. Returns the script path and a cleanup function.
// The caller must defer cleanup() immediately after checking the error.
func writeCredentialScript(token string) (scriptPath string, cleanup func(), err error) {
	f, err := os.CreateTemp("", "git-askpass-*.sh")
	if err != nil {
		return "", nil, fmt.Errorf("creating credential script: %w", err)
	}

	script := fmt.Sprintf("#!/bin/sh\necho '%s'\n", token)
	if _, err := f.WriteString(script); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", nil, fmt.Errorf("writing credential script: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", nil, fmt.Errorf("closing credential script: %w", err)
	}

	if err := os.Chmod(f.Name(), 0o700); err != nil {
		_ = os.Remove(f.Name())
		return "", nil, fmt.Errorf("making credential script executable: %w", err)
	}

	cleanup = func() {
		_ = os.Remove(f.Name())
	}

	return f.Name(), cleanup, nil
}
