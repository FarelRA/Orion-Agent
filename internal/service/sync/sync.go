// Package sync provides comprehensive synchronization services for WhatsApp data.
// It implements all available whatsmeow sync methods and provides event-sync coalescence.
package sync

import (
	"context"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	waLog "go.mau.fi/whatsmeow/util/log"

	"orion-agent/internal/data/store"
	"orion-agent/internal/utils"
)

type SyncService struct {
	client *whatsmeow.Client
	utils  *utils.Utils
	log    waLog.Logger

	// Internal dispatcher for coalescence
	dispatcher *Dispatcher

	// Stores - ALL data is persisted
	contacts    *store.ContactStore
	groups      *store.GroupStore
	chats       *store.ChatStore
	blocklist   *store.BlocklistStore
	privacy     *store.PrivacyStore
	newsletters *store.NewsletterStore
	syncState   *store.SyncStateStore

	// Scheduler
	schedulerCtx    context.Context
	schedulerCancel context.CancelFunc
	schedulerWg     sync.WaitGroup
}

// NewSyncService creates a new SyncService.
func NewSyncService(
	client *whatsmeow.Client,
	utils *utils.Utils,
	contacts *store.ContactStore,
	groups *store.GroupStore,
	chats *store.ChatStore,
	blocklist *store.BlocklistStore,
	privacy *store.PrivacyStore,
	newsletters *store.NewsletterStore,
	syncState *store.SyncStateStore, // Add this
	log waLog.Logger,
) *SyncService {
	s := &SyncService{
		client:      client,
		utils:       utils,
		contacts:    contacts,
		groups:      groups,
		chats:       chats,
		blocklist:   blocklist,
		privacy:     privacy,
		newsletters: newsletters,
		syncState:   syncState,
		log:         log.Sub("SyncService"),
	}
	return s
}

// SetClient sets the whatsmeow client (for delayed initialization).
func (s *SyncService) SetClient(client *whatsmeow.Client) {
	s.client = client
}

// SetDispatcher sets the dispatcher with context (must be called after creation).
func (s *SyncService) SetDispatcher(ctx context.Context) {
	s.dispatcher = NewDispatcher(s, ctx, s.log)
}

// Handle routes an event through the sync dispatcher for coalescence.
func (s *SyncService) Handle(evt interface{}) {
	if s.dispatcher != nil {
		s.dispatcher.Handle(evt)
	}
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
	err := s.syncState.Put(&store.SyncState{
		SyncType:   syncType,
		LastSyncAt: time.Now(),
	})
	if err != nil {
		s.log.Warnf("Failed to persist sync state for %s: %v", syncType, err)
	}
}

// lastSyncTime returns when a sync type was last performed.
func (s *SyncService) lastSyncTime(syncType string) time.Time {
	state, err := s.syncState.Get(syncType)
	if err != nil || state == nil {
		s.log.Warnf("Failed to get sync state for %s: %v", syncType, err)
		return time.Time{}
	}
	return state.LastSyncAt
}

// shouldSync returns true if enough time has passed since last sync.
func (s *SyncService) shouldSync(syncType string, interval time.Duration) bool {
	last := s.lastSyncTime(syncType)
	return time.Since(last) > interval
}
