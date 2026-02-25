package extractor

import "fmt"

// Registry holds all available extractors and selects the appropriate one
// based on MIME type.
type Registry struct {
	extractors []Extractor
}

// NewRegistry creates a Registry from the given extractors.
func NewRegistry(extractors ...Extractor) *Registry {
	return &Registry{extractors: extractors}
}

// ForMIMEType returns the first extractor that supports the given MIME type.
func (r *Registry) ForMIMEType(mimeType string) (Extractor, error) {
	for _, ext := range r.extractors {
		if ext.Supports(mimeType) {
			return ext, nil
		}
	}
	return nil, fmt.Errorf("no extractor for MIME type %q", mimeType)
}
