package queue

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c-premus/documcp/internal/config"
)

func TestBuildPeriodicJobs_allSchedulesConfigured(t *testing.T) {
	t.Parallel()

	cfg := config.SchedulerConfig{
		KiwixSchedule:           "0 */6 * * *",
		GitSchedule:             "0 * * * *",
		OAuthCleanupSchedule:    "0 * * * *",
		OrphanedFilesSchedule:   "0 2 * * *",
		SearchVerifySchedule:    "0 3 * * *",
		SoftDeletePurgeSchedule: "0 4 * * *",
		ZimCleanupSchedule:      "0 5 * * *",
		HealthCheckSchedule:     "*/15 * * * *",
	}

	jobs := BuildPeriodicJobs(cfg, discardLogger())

	assert.Len(t, jobs, 8, "all 8 periodic jobs should be registered")
}

func TestBuildPeriodicJobs_emptySchedulesSkipped(t *testing.T) {
	t.Parallel()

	cfg := config.SchedulerConfig{
		KiwixSchedule:           "0 */6 * * *",
		GitSchedule:             "", // disabled
		OAuthCleanupSchedule:    "0 * * * *",
		OrphanedFilesSchedule:   "", // disabled
		SearchVerifySchedule:    "", // disabled
		SoftDeletePurgeSchedule: "", // disabled
		ZimCleanupSchedule:      "", // disabled
		HealthCheckSchedule:     "", // disabled
	}

	jobs := BuildPeriodicJobs(cfg, discardLogger())

	assert.Len(t, jobs, 2, "only 2 non-empty schedules should produce jobs")
}

func TestBuildPeriodicJobs_allEmpty(t *testing.T) {
	t.Parallel()

	cfg := config.SchedulerConfig{}

	jobs := BuildPeriodicJobs(cfg, discardLogger())

	assert.Empty(t, jobs, "no jobs should be registered when all schedules are empty")
}

func TestBuildPeriodicJobs_invalidCronSkipped(t *testing.T) {
	t.Parallel()

	cfg := config.SchedulerConfig{
		KiwixSchedule:           "not-a-cron",
		GitSchedule:             "invalid expression here",
		OAuthCleanupSchedule:    "0 * * * *",
		OrphanedFilesSchedule:   "***",
		SearchVerifySchedule:    "0 3 * * *",
		SoftDeletePurgeSchedule: "",
		ZimCleanupSchedule:      "0 5 * * *",
		HealthCheckSchedule:     "*/15 * * * *",
	}

	jobs := BuildPeriodicJobs(cfg, discardLogger())

	// 2 invalid + 1 empty = 3 skipped, so 4 valid jobs.
	assert.Len(t, jobs, 4, "invalid cron expressions should be skipped")
}

func TestBuildPeriodicJobs_nilLogger(t *testing.T) {
	t.Parallel()

	t.Run("valid and invalid schedules with nil logger", func(t *testing.T) {
		t.Parallel()

		cfg := config.SchedulerConfig{
			KiwixSchedule: "0 */6 * * *",
			GitSchedule:   "invalid", // triggers error log path with nil logger
		}

		// Should not panic with nil logger.
		assert.NotPanics(t, func() {
			jobs := BuildPeriodicJobs(cfg, nil)
			assert.Len(t, jobs, 1)
		})
	})

	t.Run("empty schedule with nil logger", func(t *testing.T) {
		t.Parallel()

		cfg := config.SchedulerConfig{
			KiwixSchedule: "",
			GitSchedule:   "0 * * * *",
		}

		// nil logger on empty schedule branch should not panic.
		assert.NotPanics(t, func() {
			jobs := BuildPeriodicJobs(cfg, nil)
			assert.Len(t, jobs, 1)
		})
	})

	t.Run("successful registration with nil logger", func(t *testing.T) {
		t.Parallel()

		cfg := config.SchedulerConfig{
			KiwixSchedule: "0 */6 * * *",
		}

		// nil logger on success path should not panic.
		assert.NotPanics(t, func() {
			jobs := BuildPeriodicJobs(cfg, nil)
			assert.Len(t, jobs, 1)
		})
	})
}

func TestBuildPeriodicJobs_singleValidSchedule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cfg       config.SchedulerConfig
		wantCount int
	}{
		{
			name:      "only_kiwix",
			cfg:       config.SchedulerConfig{KiwixSchedule: "0 */6 * * *"},
			wantCount: 1,
		},
		{
			name:      "only_git",
			cfg:       config.SchedulerConfig{GitSchedule: "0 * * * *"},
			wantCount: 1,
		},
		{
			name:      "only_oauth_cleanup",
			cfg:       config.SchedulerConfig{OAuthCleanupSchedule: "0 * * * *"},
			wantCount: 1,
		},
		{
			name:      "only_orphaned_files",
			cfg:       config.SchedulerConfig{OrphanedFilesSchedule: "0 2 * * *"},
			wantCount: 1,
		},
		{
			name:      "only_search_verify",
			cfg:       config.SchedulerConfig{SearchVerifySchedule: "0 3 * * *"},
			wantCount: 1,
		},
		{
			name:      "only_soft_delete_purge",
			cfg:       config.SchedulerConfig{SoftDeletePurgeSchedule: "0 4 * * *"},
			wantCount: 1,
		},
		{
			name:      "only_zim_cleanup",
			cfg:       config.SchedulerConfig{ZimCleanupSchedule: "0 5 * * *"},
			wantCount: 1,
		},
		{
			name:      "only_health_check",
			cfg:       config.SchedulerConfig{HealthCheckSchedule: "*/15 * * * *"},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			jobs := BuildPeriodicJobs(tt.cfg, discardLogger())
			assert.Len(t, jobs, tt.wantCount)
		})
	}
}

func TestBuildPeriodicJobs_mixedValidAndInvalid(t *testing.T) {
	t.Parallel()

	cfg := config.SchedulerConfig{
		KiwixSchedule:           "0 */6 * * *", // valid
		GitSchedule:             "",            // empty (disabled)
		OAuthCleanupSchedule:    "0 * * * *",   // valid
		OrphanedFilesSchedule:   "also bad",    // invalid
		SearchVerifySchedule:    "0 3 * * *",   // valid
		SoftDeletePurgeSchedule: "",            // empty (disabled)
		ZimCleanupSchedule:      "0 5 * * *",   // valid
		HealthCheckSchedule:     "",            // empty (disabled)
	}

	jobs := BuildPeriodicJobs(cfg, discardLogger())

	// 4 valid, 1 invalid, 3 empty = 4 jobs.
	assert.Len(t, jobs, 4)
}

func TestBuildPeriodicJobs_returnsNilForNoJobs(t *testing.T) {
	t.Parallel()

	cfg := config.SchedulerConfig{}

	jobs := BuildPeriodicJobs(cfg, discardLogger())

	assert.Nil(t, jobs, "should return nil slice when no jobs are built")
}

func TestBuildPeriodicJobs_allInvalidCron(t *testing.T) {
	t.Parallel()

	cfg := config.SchedulerConfig{
		KiwixSchedule:           "bad",
		GitSchedule:             "nope",
		OAuthCleanupSchedule:    "!!!",
		OrphanedFilesSchedule:   "abc def",
		SearchVerifySchedule:    "x x x x x x",
		SoftDeletePurgeSchedule: "???",
		ZimCleanupSchedule:      "@invalid",
		HealthCheckSchedule:     "1 2 3 4 5 6 7",
	}

	jobs := BuildPeriodicJobs(cfg, discardLogger())

	assert.Empty(t, jobs, "all invalid cron expressions should result in no jobs")
}
