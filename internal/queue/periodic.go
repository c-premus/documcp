package queue

import (
	"log/slog"

	"github.com/riverqueue/river"
	"github.com/robfig/cron/v3"

	"git.999.haus/chris/DocuMCP-go/internal/config"
)

// BuildPeriodicJobs converts scheduler config cron expressions into River periodic jobs.
// Empty schedule strings are skipped (disables the job).
func BuildPeriodicJobs(cfg config.SchedulerConfig, logger *slog.Logger) []*river.PeriodicJob {
	type entry struct {
		name     string
		schedule string
		args     river.JobArgs
	}

	entries := []entry{
		{"kiwix", cfg.KiwixSchedule, SyncKiwixArgs{}},
		{"git", cfg.GitSchedule, SyncGitTemplatesArgs{}},
		{"oauth-cleanup", cfg.OAuthCleanupSchedule, CleanupOAuthTokensArgs{}},
		{"orphaned-files", cfg.OrphanedFilesSchedule, CleanupOrphanedFilesArgs{}},
		{"search-verify", cfg.SearchVerifySchedule, VerifySearchIndexArgs{}},
		{"soft-delete-purge", cfg.SoftDeletePurgeSchedule, PurgeSoftDeletedArgs{}},
		{"zim-cleanup", cfg.ZimCleanupSchedule, CleanupDisabledZimArgs{}},
		{"health-check", cfg.HealthCheckSchedule, HealthCheckServicesArgs{}},
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	var jobs []*river.PeriodicJob
	for _, e := range entries {
		if e.schedule == "" {
			if logger != nil {
				logger.Info("periodic job schedule not configured, skipping", "job", e.name)
			}
			continue
		}

		schedule, err := parser.Parse(e.schedule)
		if err != nil {
			if logger != nil {
				logger.Error("failed to parse cron schedule for periodic job",
					"job", e.name,
					"schedule", e.schedule,
					"error", err,
				)
			}
			continue
		}

		args := e.args
		jobs = append(jobs, river.NewPeriodicJob(
			schedule,
			func() (river.JobArgs, *river.InsertOpts) {
				return args, nil
			},
			&river.PeriodicJobOpts{RunOnStart: false},
		))

		if logger != nil {
			logger.Info("periodic job registered", "job", e.name, "schedule", e.schedule)
		}
	}

	return jobs
}
