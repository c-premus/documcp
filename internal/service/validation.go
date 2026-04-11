package service

import (
	"errors"
	"fmt"
)

// Tag validation bounds. Applied at the service layer so both REST and MCP
// code paths share a single enforcement point. Bounds match the memory bank's
// "MCP Tool Input Validation" entry (50 tags × 100 chars).
const (
	MaxTagsPerDocument = 50
	MaxTagLength       = 100
)

// ErrTagValidation indicates that a document's tags failed validation.
// Handlers should check errors.Is(err, ErrTagValidation) and return 400.
var ErrTagValidation = errors.New("invalid tags")

// validateTags enforces count and per-tag length bounds. It is called by
// DocumentService.Create, DocumentService.Update, and DocumentPipeline.Upload
// before any database or filesystem writes happen.
func validateTags(tags []string) error {
	if len(tags) > MaxTagsPerDocument {
		return fmt.Errorf("%w: maximum %d tags allowed, got %d",
			ErrTagValidation, MaxTagsPerDocument, len(tags))
	}
	for i, tag := range tags {
		if len(tag) > MaxTagLength {
			return fmt.Errorf("%w: tag at index %d exceeds %d characters (length %d)",
				ErrTagValidation, i, MaxTagLength, len(tag))
		}
	}
	return nil
}
