package service

import "errors"

// Sentinel errors for service-layer operations. Handlers should use errors.Is()
// to classify these and map to appropriate HTTP status codes.
var (
	// ErrNotFound indicates the requested resource does not exist.
	ErrNotFound = errors.New("not found")

	// ErrUnsupportedFileType indicates the uploaded file type is not supported.
	ErrUnsupportedFileType = errors.New("unsupported file type")

	// ErrFileTooLarge indicates the uploaded file exceeds the maximum size.
	ErrFileTooLarge = errors.New("file exceeds maximum size")

	// ErrEnvManaged indicates the resource is managed by environment configuration
	// and cannot be modified or deleted via the API.
	ErrEnvManaged = errors.New("env-managed resource cannot be modified")
)
