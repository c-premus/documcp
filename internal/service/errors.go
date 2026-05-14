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

	// ErrInvalidURL indicates the provided URL failed validation (e.g. SSRF check).
	ErrInvalidURL = errors.New("invalid URL")

	// ErrNotDeleted indicates the caller tried to restore a document that is
	// not in the soft-deleted state.
	ErrNotDeleted = errors.New("document is not deleted")

	// ErrFileBackedDocument indicates the caller tried to replace inline content
	// on a document that was created via file upload. File-backed documents
	// must be updated via the REST POST /api/documents/{uuid}/content endpoint
	// so the extraction worker re-runs against the new blob.
	ErrFileBackedDocument = errors.New("document is file-backed; inline content replacement not supported")
)
