package git

// CloneParams holds parameters for cloning a repository.
type CloneParams struct {
	URL    string
	Branch string
	Token  string // PAT for private repos (optional)
	Dest   string // Destination directory
}

// TemplateFile represents a file extracted from a git template repository.
type TemplateFile struct {
	Path        string
	Filename    string
	Extension   string
	Content     string
	SizeBytes   int64
	ContentHash string // SHA-256
	IsEssential bool
	Variables   []string // {{variable}} placeholders found
}
