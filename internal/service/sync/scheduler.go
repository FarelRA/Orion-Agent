package sync

import (
	"context"
	"time"
)

// =============================================================================
// Scheduler - Periodic Sync
// =============================================================================

// SchedulerConfig configures the sync scheduler intervals.
type SchedulerConfig struct {
	BlocklistInterval        time.Duration
	ContactPicturesInterval  time.Duration
	GroupsInterval           time.Duration
	NewslettersInterval      time.Duration
	PrivacyInterval          time.Duration
	BusinessProfilesInterval time.Duration
}

// DefaultSchedulerConfig returns default scheduler configuration.
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		BlocklistInterval:        1 * time.Hour,
		ContactPicturesInterval:  6 * time.Hour,
		GroupsInterval:           6 * time.Hour,
		NewslettersInterval:      24 * time.Hour,
		PrivacyInterval:          24 * time.Hour,
		BusinessProfilesInterval: 24 * time.Hour,
	}
}

// StartScheduler starts the periodic sync scheduler.
func (s *SyncService) StartScheduler(cfg SchedulerConfig) {
	if s.schedulerCancel != nil {
		s.log.Warnf("Scheduler already running")
		return
	}

	s.schedulerCtx, s.schedulerCancel = context.WithCancel(context.Background())
	s.log.Infof("Starting sync scheduler...")

	// Blocklist - every hour
	s.schedulerWg.Add(1)
	go s.runPeriodic("blocklist", cfg.BlocklistInterval, func(ctx context.Context) error {
		return s.SyncBlocklist(ctx)
	})

	// Contact pictures - every 6 hours
	s.schedulerWg.Add(1)
	go s.runPeriodic("contact_pictures", cfg.ContactPicturesInterval, func(ctx context.Context) error {
		return s.SyncAllContactPictures(ctx)
	})

	// Groups - every 6 hours
	s.schedulerWg.Add(1)
	go s.runPeriodic("groups", cfg.GroupsInterval, func(ctx context.Context) error {
		return s.SyncAllGroups(ctx)
	})

	// Newsletters - every 24 hours
	s.schedulerWg.Add(1)
	go s.runPeriodic("newsletters", cfg.NewslettersInterval, func(ctx context.Context) error {
		return s.SyncAllNewsletters(ctx)
	})

	// Privacy - every 24 hours
	s.schedulerWg.Add(1)
	go s.runPeriodic("privacy", cfg.PrivacyInterval, func(ctx context.Context) error {
		return s.SyncPrivacySettings(ctx)
	})
}

// StopScheduler stops the periodic sync scheduler.
func (s *SyncService) StopScheduler() {
	if s.schedulerCancel != nil {
		s.log.Infof("Stopping sync scheduler...")
		s.schedulerCancel()
		s.schedulerWg.Wait()
		s.schedulerCancel = nil
		s.log.Infof("Sync scheduler stopped")
	}
}

// runPeriodic runs a sync function periodically.
func (s *SyncService) runPeriodic(name string, interval time.Duration, syncFn func(context.Context) error) {
	defer s.schedulerWg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.schedulerCtx.Done():
			return
		case <-ticker.C:
			if s.shouldSync(name, interval) {
				s.log.Infof("Running periodic sync: %s", name)
				if err := syncFn(s.schedulerCtx); err != nil {
					s.log.Warnf("Periodic sync %s failed: %v", name, err)
				}
			}
		}
	}
}
