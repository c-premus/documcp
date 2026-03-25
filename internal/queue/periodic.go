package queue

import (
	"log/slog"

	"github.com/riverqueue/river"
	"github.com/robfig/cron/v3"

	"github.com/c-premus/documcp/internal/config"
)

// BuildPeriodicJobs converts scheduler config cron expressions into River periodic jobs.
// Empty schedule strings are skipped (disables the job).
func BuildPeriodicJobs(cfg config.SchedulerConfig, logger *slog.Logger) []*river.PeriodicJob {
	type entry struct {
		name       string
		schedule   string
		args       river.JobArgs
		runOnStart bool
	}

	entries := []entry{
		{"kiwix", cfg.KiwixSchedule, SyncKiwixArgs{}, false},
		{"git", cfg.GitSchedule, SyncGitTemplatesArgs{}, false},
		{"oauth-cleanup", cfg.OAuthCleanupSchedule, CleanupOAuthTokensArgs{}, false},
		{"orphaned-files", cfg.OrphanedFilesSchedule, CleanupOrphanedFilesArgs{}, false},
		{"search-verify", cfg.SearchVerifySchedule, VerifySearchIndexArgs{}, true},
		{"soft-delete-purge", cfg.SoftDeletePurgeSchedule, PurgeSoftDeletedArgs{}, false},
		{"zim-cleanup", cfg.ZimCleanupSchedule, CleanupDisabledZimArgs{}, false},
		{"health-check", cfg.HealthCheckSchedule, HealthCheckServicesArgs{}, false},
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
		runOnStart := e.runOnStart
		jobs = append(jobs, river.NewPeriodicJob(
			schedule,
			func() (river.JobArgs, *river.InsertOpts) {
				return args, nil
			},
			&river.PeriodicJobOpts{RunOnStart: runOnStart},
		))

		if logger != nil {
			logger.Info("periodic job registered", "job", e.name, "schedule", e.schedule)
		}
	}

	return jobs
}
