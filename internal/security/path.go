package security

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SafeStoragePath validates that filePath resolves inside storagePath,
// preventing path-traversal attacks via manipulated DB values.
// Uses EvalSymlinks to resolve symlinks before the prefix check,
// preventing symlink-based escapes from the storage root.
// Falls back to Abs for non-existent paths — safe because the subsequent
// file operation (os.Open, os.Remove) will also fail on missing paths.
// Returns the joined absolute path or an error.
func SafeStoragePath(storagePath, filePath string) (string, error) {
	fullPath := filepath.Join(storagePath, filePath)

	absStorage, err := filepath.EvalSymlinks(storagePath)
	if err != nil {
		return "", fmt.Errorf("resolving storage root: %w", err)
	}

	// Resolve symlinks to catch symlink-based escapes. Fall back to Abs
	// when the path doesn't exist — no data can be read from a missing path.
	absPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		absPath, err = filepath.Abs(fullPath)
		if err != nil {
			return "", fmt.Errorf("resolving target path: %w", err)
		}
	}

	if !strings.HasPrefix(absPath, absStorage+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q escapes storage root", filePath)
	}
	return absPath, nil
}
