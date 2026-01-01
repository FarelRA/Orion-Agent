// Package sync provides comprehensive synchronization services for WhatsApp data.
// It implements all available whatsmeow sync methods and provides event-sync coalescence.
package sync

import (
	"context"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	waLog "go.mau.fi/whatsmeow/util/log"

	"orion-agent/internal/service"
	"orion-agent/internal/store"
)

// SyncService coordinates all synchronization operations.
type SyncService struct {
	client  *whatsmeow.Client
	service *service.JIDService
	log     waLog.Logger

	// Stores - ALL data is persisted
	contacts    *store.ContactStore
	groups      *store.GroupStore
	chats       *store.ChatStore
	blocklist   *store.BlocklistStore
	privacy     *store.PrivacyStore
	newsletters *store.NewsletterStore

	// Sync state
	lastSync map[string]time.Time

	// Scheduler
	schedulerCtx    context.Context
	schedulerCancel context.CancelFunc
	schedulerWg     sync.WaitGroup
}

// NewSyncService creates a new SyncService.
func NewSyncService(
	client *whatsmeow.Client,
	service *service.JIDService,
	contacts *store.ContactStore,
	groups *store.GroupStore,
	chats *store.ChatStore,
	blocklist *store.BlocklistStore,
	privacy *store.PrivacyStore,
	newsletters *store.NewsletterStore,
	log waLog.Logger,
) *SyncService {
	return &SyncService{
		client:      client,
		service:     service,
		contacts:    contacts,
		groups:      groups,
		chats:       chats,
		blocklist:   blocklist,
		privacy:     privacy,
		newsletters: newsletters,
		log:         log.Sub("SyncService"),
		lastSync:    make(map[string]time.Time),
	}
}

// SetClient sets the whatsmeow client (for delayed initialization).
func (s *SyncService) SetClient(client *whatsmeow.Client) {
	s.client = client
}

// FullSync performs a complete sync of all data types.
func (s *SyncService) FullSync(ctx context.Context) error {
	s.log.Infof("Starting full sync...")
	start := time.Now()

	var errors []error

	// Sync groups
	if err := s.SyncAllGroups(ctx); err != nil {
		s.log.Warnf("Failed to sync groups: %v", err)
		errors = append(errors, err)
	}

	// Sync blocklist
	if err := s.SyncBlocklist(ctx); err != nil {
		s.log.Warnf("Failed to sync blocklist: %v", err)
		errors = append(errors, err)
	}

	// Sync privacy settings
	if err := s.SyncPrivacySettings(ctx); err != nil {
		s.log.Warnf("Failed to sync privacy: %v", err)
		errors = append(errors, err)
	}

	// Sync newsletters
	if err := s.SyncAllNewsletters(ctx); err != nil {
		s.log.Warnf("Failed to sync newsletters: %v", err)
		errors = append(errors, err)
	}

	// Sync all contact pictures
	if err := s.SyncAllContactPictures(ctx); err != nil {
		s.log.Warnf("Failed to sync contact pictures: %v", err)
		errors = append(errors, err)
	}

	// Sync all group pictures
	if err := s.SyncAllGroupPictures(ctx); err != nil {
		s.log.Warnf("Failed to sync group pictures: %v", err)
		errors = append(errors, err)
	}

	s.log.Infof("Full sync completed in %v", time.Since(start))

	if len(errors) > 0 {
		return errors[0] // Return first error
	}
	return nil
}

// recordSync records when a sync type was last performed.
func (s *SyncService) recordSync(syncType string) {
	s.lastSync[syncType] = time.Now()
}

// lastSyncTime returns when a sync type was last performed.
func (s *SyncService) lastSyncTime(syncType string) time.Time {
	return s.lastSync[syncType]
}

// shouldSync returns true if enough time has passed since last sync.
func (s *SyncService) shouldSync(syncType string, interval time.Duration) bool {
	last := s.lastSyncTime(syncType)
	return time.Since(last) > interval
}
