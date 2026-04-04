package testutil

import (
	"time"

	"github.com/c-premus/documcp/internal/model"
)

// GitTemplateOption configures a GitTemplate created by NewGitTemplate.
type GitTemplateOption func(*model.GitTemplate)

// NewGitTemplate returns a GitTemplate with sensible defaults.
func NewGitTemplate(opts ...GitTemplateOption) *model.GitTemplate {
	now := nullTime(time.Now())
	gt := &model.GitTemplate{
		ID:            1,
		UUID:          "test-template-uuid",
		Name:          "Test Template",
		Slug:          "test-template",
		RepositoryURL: "https://github.com/example/repo.git",
		Branch:        "main",
		IsPublic:      true,
		IsEnabled:     true,
		Status:        model.GitTemplateStatusSynced,
		FileCount:     5,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	for _, opt := range opts {
		opt(gt)
	}
	return gt
}

// WithGitTemplateID sets the git template ID on the builder.
func WithGitTemplateID(id int64) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.ID = id }
}

// WithGitTemplateUUID sets the git template UUID on the builder.
func WithGitTemplateUUID(uuid string) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.UUID = uuid }
}

// WithGitTemplateName sets the git template name on the builder.
func WithGitTemplateName(name string) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.Name = name }
}

// WithGitTemplateSlug sets the git template slug on the builder.
func WithGitTemplateSlug(slug string) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.Slug = slug }
}

// WithGitTemplateDescription sets the git template description on the builder.
func WithGitTemplateDescription(desc string) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.Description = nullString(desc) }
}

// WithGitTemplateRepositoryURL sets the git template repository URL on the builder.
func WithGitTemplateRepositoryURL(url string) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.RepositoryURL = url }
}

// WithGitTemplateBranch sets the git template branch on the builder.
func WithGitTemplateBranch(branch string) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.Branch = branch }
}

// WithGitTemplateUserID sets the git template user ID on the builder.
func WithGitTemplateUserID(uid int64) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.UserID = nullInt64(uid) }
}

// WithGitTemplateIsPublic sets the git template public visibility on the builder.
func WithGitTemplateIsPublic(public bool) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.IsPublic = public }
}

// WithGitTemplateIsEnabled sets the git template enabled flag on the builder.
func WithGitTemplateIsEnabled(enabled bool) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.IsEnabled = enabled }
}

// WithGitTemplateStatus sets the git template status on the builder.
func WithGitTemplateStatus(status model.GitTemplateStatus) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.Status = status }
}
