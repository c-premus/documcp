package kiwix

import "encoding/xml"

// CatalogEntry represents a ZIM archive from the OPDS catalog feed.
type CatalogEntry struct {
	ID               string   // Kiwix catalog ID
	Title            string   // Human-readable title
	Description      string   // Summary of archive contents
	Language         string   // ISO 639 language code
	Category         string   // devdocs, wikipedia, stack_exchange, other
	Creator          string   // Content creator
	Publisher        string   // Archive publisher
	Favicon          string   // URL to favicon
	ArticleCount     int64    // Number of articles in the archive
	MediaCount       int64    // Number of media items in the archive
	FileSize         int64    // Size in bytes
	Tags             []string // Descriptive tags
	Name             string   // Archive name (derived from ID)
	ContentID        string   // Versioned content ID for search/article reading (e.g. "gobyexample.com_en_all_2025-11")
	HasFulltextIndex bool     // Whether fulltext search index is available
}

// SearchResult represents a single search result from Kiwix.
type SearchResult struct {
	Title   string  `json:"title"`
	Path    string  `json:"path"`
	Snippet string  `json:"snippet,omitempty"`
	Score   float64 `json:"score,omitempty"`
}

// Article represents an article read from a ZIM archive.
type Article struct {
	Title    string // Article title, extracted from HTML <title> or path
	Content  string // Plain text content (HTML stripped)
	MIMEType string // Response MIME type
}

// opdsFeed is the top-level OPDS catalog XML structure.
type opdsFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Entries []opdsEntry `xml:"entry"`
}

// opdsEntry is a single entry in the OPDS catalog feed.
type opdsEntry struct {
	ID       string     `xml:"id"`
	Title    string     `xml:"title"`
	Summary  string     `xml:"summary"`
	Language string     `xml:"language"`
	Name     string     `xml:"name"`
	Tags     string     `xml:"tags"`
	Creator  string     `xml:"author>name"`
	Links    []opdsLink `xml:"link"`

	// Kiwix-specific metadata fields.
	ArticleCount int64  `xml:"articleCount"`
	MediaCount   int64  `xml:"mediaCount"`
	Size         int64  `xml:"size"`
	Favicon      string `xml:"favicon"`
}

// opdsLink is a link element in an OPDS entry.
type opdsLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
	Type string `xml:"type,attr"`
}

// suggestResult is the JSON structure returned by the Kiwix suggest API.
type suggestResult struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Path  string `json:"path"`
}
