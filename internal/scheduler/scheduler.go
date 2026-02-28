// Package scheduler provides a cron-based scheduler for periodic sync of
// external services (Kiwix ZIM archives, Confluence spaces, Git templates).
package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"git.999.haus/chris/DocuMCP-go/internal/client/confluence"
	"git.999.haus/chris/DocuMCP-go/internal/client/git"
	"git.999.haus/chris/DocuMCP-go/internal/client/kiwix"
	"git.999.haus/chris/DocuMCP-go/internal/model"
	"git.999.haus/chris/DocuMCP-go/internal/repository"
	"git.999.haus/chris/DocuMCP-go/internal/search"
)

// ExternalServiceFinder retrieves enabled external services by type.
type ExternalServiceFinder interface {
	FindEnabledByType(ctx context.Context, serviceType string) ([]model.ExternalService, error)
}

// Config holds cron schedule expressions and a logger for the scheduler.
// Empty schedule strings disable the corresponding job.
type Config struct {
	KiwixSchedule      string // cron expression, e.g. "0 */6 * * *"
	ConfluenceSchedule string // e.g. "0 */4 * * *"
	GitSchedule        string // e.g. "0 * * * *"
	Logger             *slog.Logger
}

// Scheduler orchestrates periodic sync of Kiwix, Confluence, and Git external services.
type Scheduler struct {
	cron               *cron.Cron
	services           ExternalServiceFinder
	zimRepo            *repository.ZimArchiveRepository
	confRepo           *repository.ConfluenceSpaceRepository
	gitRepo            *repository.GitTemplateRepository
	indexer            *search.Indexer
	gitTempDir         string
	kiwixSchedule      string
	confluenceSchedule string
	gitSchedule        string
	logger             *slog.Logger
}

// New creates a Scheduler with the given dependencies. The indexer parameter is
// optional and may be nil. Schedule expressions in cfg control which jobs are
// registered; empty strings disable the corresponding job.
func New(cfg Config, services ExternalServiceFinder, zimRepo *repository.ZimArchiveRepository, confRepo *repository.ConfluenceSpaceRepository, gitRepo *repository.GitTemplateRepository, indexer *search.Indexer, gitTempDir string) *Scheduler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	c := cron.New(
		cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)),
	)

	return &Scheduler{
		cron:               c,
		services:           services,
		zimRepo:            zimRepo,
		confRepo:           confRepo,
		gitRepo:            gitRepo,
		indexer:            indexer,
		gitTempDir:         gitTempDir,
		kiwixSchedule:      cfg.KiwixSchedule,
		confluenceSchedule: cfg.ConfluenceSchedule,
		gitSchedule:        cfg.GitSchedule,
		logger:             logger,
	}
}

// Start registers cron jobs for each non-empty schedule and starts the cron runner.
func (s *Scheduler) Start() {
	if s == nil {
		return
	}

	s.addJob("kiwix", s.kiwixSchedule, s.syncKiwix)
	s.addJob("confluence", s.confluenceSchedule, s.syncConfluence)
	s.addJob("git", s.gitSchedule, s.syncGitTemplates)

	s.cron.Start()
	s.logger.Info("scheduler started")
}

// Stop signals the cron scheduler to stop and returns a context that completes
// when all running jobs have finished.
func (s *Scheduler) Stop() context.Context {
	if s == nil {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx
	}
	return s.cron.Stop()
}

// addJob registers a cron job if the given schedule expression is non-empty.
func (s *Scheduler) addJob(jobType, schedule string, fn func()) {
	if schedule == "" {
		s.logger.Info("schedule not configured, skipping", "job", jobType)
		return
	}

	_, err := s.cron.AddFunc(schedule, fn)
	if err != nil {
		s.logger.Error("failed to register cron job",
			"job", jobType,
			"schedule", schedule,
			"error", err,
		)
		return
	}

	s.logger.Info("cron job registered", "job", jobType, "schedule", schedule)
}

// syncKiwix fetches the catalog from each enabled Kiwix service and reconciles
// archives with the database and search index.
func (s *Scheduler) syncKiwix() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	logger := s.logger.With("job", "kiwix")
	logger.Info("starting kiwix sync")

	services, err := s.services.FindEnabledByType(ctx, "kiwix")
	if err != nil {
		logger.Error("finding enabled kiwix services", "error", err)
		return
	}

	if len(services) == 0 {
		logger.Info("no enabled kiwix services found")
		return
	}

	for _, svc := range services {
		svcLogger := logger.With("service_id", svc.ID, "base_url", svc.BaseURL)

		client, clientErr := kiwix.NewClient(svc.BaseURL, svcLogger)
		if clientErr != nil {
			svcLogger.Error("kiwix client URL rejected", "error", clientErr)
			continue
		}

		entries, err := client.FetchCatalog(ctx)
		if err != nil {
			svcLogger.Error("fetching kiwix catalog", "error", err)
			continue
		}

		var indexer kiwix.ArchiveIndexer
		if s.indexer != nil {
			indexer = &kiwixIndexerAdapter{indexer: s.indexer}
		}

		if err := kiwix.Sync(ctx, kiwix.SyncParams{
			ServiceID: svc.ID,
			Entries:   entries,
			Repo:      &kiwixRepoAdapter{repo: s.zimRepo},
			Indexer:   indexer,
			Logger:    svcLogger,
		}); err != nil {
			svcLogger.Error(fmt.Sprintf("syncing kiwix service %d: %v", svc.ID, err))
			continue
		}

		svcLogger.Info("kiwix service sync completed", "entries", len(entries))
	}
}

// syncConfluence fetches spaces from each enabled Confluence service and
// reconciles them with the database and search index.
func (s *Scheduler) syncConfluence() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	logger := s.logger.With("job", "confluence")
	logger.Info("starting confluence sync")

	services, err := s.services.FindEnabledByType(ctx, "confluence")
	if err != nil {
		logger.Error("finding enabled confluence services", "error", err)
		return
	}

	if len(services) == 0 {
		logger.Info("no enabled confluence services found")
		return
	}

	for _, svc := range services {
		svcLogger := logger.With("service_id", svc.ID, "base_url", svc.BaseURL)

		email, token, err := parseConfluenceCredentials(svc)
		if err != nil {
			svcLogger.Error("parsing confluence credentials", "error", err)
			continue
		}

		client, clientErr := confluence.NewClient(svc.BaseURL, email, token, svcLogger)
		if clientErr != nil {
			svcLogger.Error("confluence client URL rejected", "error", clientErr)
			continue
		}

		spaces, err := client.ListSpaces(ctx, "", "", 0)
		if err != nil {
			svcLogger.Error("listing confluence spaces", "error", err)
			continue
		}

		var indexer confluence.SpaceIndexer
		if s.indexer != nil {
			indexer = &confluenceIndexerAdapter{indexer: s.indexer}
		}

		if err := confluence.Sync(ctx, confluence.SyncParams{
			ServiceID: svc.ID,
			Spaces:    spaces,
			Repo:      &confluenceRepoAdapter{repo: s.confRepo},
			Indexer:   indexer,
			Logger:    svcLogger,
		}); err != nil {
			svcLogger.Error(fmt.Sprintf("syncing confluence service %d: %v", svc.ID, err))
			continue
		}

		svcLogger.Info("confluence service sync completed", "spaces", len(spaces))
	}
}

// syncGitTemplates fetches all enabled git templates and syncs each repository
// with the database and search index.
func (s *Scheduler) syncGitTemplates() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	logger := s.logger.With("job", "git")
	logger.Info("starting git template sync")

	templates, err := s.gitRepo.List(ctx, "", 100)
	if err != nil {
		logger.Error("listing git templates", "error", err)
		return
	}

	if len(templates) == 0 {
		logger.Info("no git templates found")
		return
	}

	client := git.NewClient(s.gitTempDir, logger)

	for _, t := range templates {
		tmplLogger := logger.With("template_id", t.ID, "slug", t.Slug)

		syncTmpl, err := toSyncTemplate(t)
		if err != nil {
			tmplLogger.Error("converting git template", "error", err)
			continue
		}

		var indexer git.TemplateIndexer
		if s.indexer != nil {
			indexer = &gitIndexerAdapter{indexer: s.indexer}
		}

		if err := git.Sync(ctx, git.SyncParams{
			Template: syncTmpl,
			Client:   client,
			Repo:     &gitRepoAdapter{repo: s.gitRepo},
			Indexer:  indexer,
			Logger:   tmplLogger,
		}); err != nil {
			tmplLogger.Error(fmt.Sprintf("syncing git template %d: %v", t.ID, err))
			continue
		}

		tmplLogger.Info("git template sync completed")
	}
}

// parseConfluenceCredentials extracts email and API token from the service's
// APIKey field, which stores them in "email:token" format.
func parseConfluenceCredentials(svc model.ExternalService) (email, token string, err error) {
	if !svc.APIKey.Valid || svc.APIKey.String == "" {
		return "", "", fmt.Errorf("service %d has no API key configured", svc.ID)
	}

	parts := strings.SplitN(svc.APIKey.String, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("service %d API key must be in email:token format", svc.ID)
	}

	return parts[0], parts[1], nil
}

// toSyncTemplate converts a model.GitTemplate to the git.SyncTemplate type
// used by the sync function.
func toSyncTemplate(t model.GitTemplate) (git.SyncTemplate, error) {
	var tags []string
	if t.Tags.Valid && t.Tags.String != "" {
		if err := json.Unmarshal([]byte(t.Tags.String), &tags); err != nil {
			return git.SyncTemplate{}, fmt.Errorf("parsing tags for template %d: %w", t.ID, err)
		}
	}

	description := ""
	if t.Description.Valid {
		description = t.Description.String
	}

	category := ""
	if t.Category.Valid {
		category = t.Category.String
	}

	token := ""
	if t.GitToken.Valid {
		token = t.GitToken.String
	}

	lastCommitSHA := ""
	if t.LastCommitSHA.Valid {
		lastCommitSHA = t.LastCommitSHA.String
	}

	return git.SyncTemplate{
		ID:            t.ID,
		UUID:          t.UUID,
		Name:          t.Name,
		Slug:          t.Slug,
		Description:   description,
		RepositoryURL: t.RepositoryURL,
		Branch:        t.Branch,
		Token:         token,
		Category:      category,
		Tags:          tags,
		LastCommitSHA: lastCommitSHA,
	}, nil
}
