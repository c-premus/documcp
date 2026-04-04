package testutil

import (
	"time"

	"github.com/c-premus/documcp/internal/model"
)

// DocumentOption configures a Document created by NewDocument.
type DocumentOption func(*model.Document)

// NewDocument returns a Document with sensible defaults. Pass DocumentOption
// functions to override specific fields.
func NewDocument(opts ...DocumentOption) *model.Document {
	now := nullTime(time.Now())
	d := &model.Document{
		ID:        1,
		UUID:      "test-doc-uuid",
		Title:     "Test Document",
		FileType:  "pdf",
		FilePath:  "/tmp/test-document.pdf",
		FileSize:  1024,
		MIMEType:  "application/pdf",
		IsPublic:  false,
		Status:    model.DocumentStatusIndexed,
		CreatedAt: now,
		UpdatedAt: now,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// WithDocumentID sets the document ID on the builder.
func WithDocumentID(id int64) DocumentOption {
	return func(d *model.Document) { d.ID = id }
}

// WithDocumentUUID sets the document UUID on the builder.
func WithDocumentUUID(uuid string) DocumentOption {
	return func(d *model.Document) { d.UUID = uuid }
}

// WithDocumentTitle sets the document title on the builder.
func WithDocumentTitle(title string) DocumentOption {
	return func(d *model.Document) { d.Title = title }
}

// WithDocumentDescription sets the document description on the builder.
func WithDocumentDescription(desc string) DocumentOption {
	return func(d *model.Document) { d.Description = nullString(desc) }
}

// WithDocumentFileType sets the document file type on the builder.
func WithDocumentFileType(ft string) DocumentOption {
	return func(d *model.Document) { d.FileType = ft }
}

// WithDocumentFilePath sets the document file path on the builder.
func WithDocumentFilePath(fp string) DocumentOption {
	return func(d *model.Document) { d.FilePath = fp }
}

// WithDocumentFileSize sets the document file size on the builder.
func WithDocumentFileSize(size int64) DocumentOption {
	return func(d *model.Document) { d.FileSize = size }
}

// WithDocumentMIMEType sets the document MIME type on the builder.
func WithDocumentMIMEType(mime string) DocumentOption {
	return func(d *model.Document) { d.MIMEType = mime }
}

// WithDocumentContent sets the document content on the builder.
func WithDocumentContent(content string) DocumentOption {
	return func(d *model.Document) { d.Content = nullString(content) }
}

// WithDocumentUserID sets the document user ID on the builder.
func WithDocumentUserID(uid int64) DocumentOption {
	return func(d *model.Document) { d.UserID = nullInt64(uid) }
}

// WithDocumentIsPublic sets the document public visibility on the builder.
func WithDocumentIsPublic(public bool) DocumentOption {
	return func(d *model.Document) { d.IsPublic = public }
}

// WithDocumentStatus sets the document status on the builder.
func WithDocumentStatus(status model.DocumentStatus) DocumentOption {
	return func(d *model.Document) { d.Status = status }
}
