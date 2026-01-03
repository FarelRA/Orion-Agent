// Package sync provides comprehensive synchronization services for WhatsApp data.
// It implements all available whatsmeow sync methods and provides event-sync coalescence.
package sync

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	waLog "go.mau.fi/whatsmeow/util/log"

	"orion-agent/internal/data/store"
	"orion-agent/internal/service/media"
	"orion-agent/internal/utils"
)

type usyncRequest struct {
	fn     func(context.Context) error
	result chan error
}

type SyncService struct {
	client *whatsmeow.Client
	utils  *utils.Utils
	media  *media.MediaService
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

	// Usync Queue
	usyncQueue chan usyncRequest
}

// NewSyncService creates a new SyncService.
func NewSyncService(
	client *whatsmeow.Client,
	utils *utils.Utils,
	media *media.MediaService,
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
		media:       media,
		contacts:    contacts,
		groups:      groups,
		chats:       chats,
		blocklist:   blocklist,
		privacy:     privacy,
		newsletters: newsletters,
		syncState:   syncState,
		log:         log.Sub("SyncService"),
		usyncQueue:  make(chan usyncRequest, 100),
	}
	// Start the worker immediately, it will block on channel receive
	go s.startUSyncWorker()
	return s
}

// performUSync adds a request to the queue and waits for it to complete.
func (s *SyncService) performUSync(ctx context.Context, fn func(context.Context) error) error {
	resultChan := make(chan error, 1)
	select {
	case s.usyncQueue <- usyncRequest{fn: fn, result: resultChan}:
		select {
		case err := <-resultChan:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	case <-ctx.Done():
		return ctx.Err()
	}
}

// startUSyncWorker processes usync requests sequentially with rate limiting.
func (s *SyncService) startUSyncWorker() {
	// Global delay between requests
	const globalDelay = 3 * time.Second

	for req := range s.usyncQueue {
		// Execute the request
		// We use a background context or the service's context if we had one global one,
		// but here we pass a fresh context or rely on the one inside 'fn' if strictly needed,
		// usually the 'fn' closure captures the context from the caller.
		// However, the caller is waiting on 'resultChan'.
		// We should respect the caller's context cancellation in 'performUSync', but here we just run it.
		// NOTE: The 'fn' passed usually uses 'ctx' captured from caller. If caller cancels, 'fn' might fail quickly.

		// Run the function
		err := req.fn(context.Background()) // We use background here as the func likely has the caller ctx embedded or we don't want to cancel the WORKER.
		req.result <- err

		// Handle rate limits / backoff
		backoff := 0
		if err != nil {
			var iqErr *whatsmeow.IQError
			if errors.As(err, &iqErr) && iqErr.Code == 429 {
				if iqErr.ErrorNode != nil {
					backoff = iqErr.ErrorNode.AttrGetter().OptionalInt("backoff")
				}
				if backoff == 0 && iqErr.RawNode != nil {
					backoff = iqErr.RawNode.AttrGetter().OptionalInt("backoff")
				}
			}
		}

		if backoff > 0 {
			s.log.Warnf("Global USync Worker: Rate limit hit. Sleeping for %d seconds...", backoff)
			time.Sleep(time.Duration(backoff) * time.Second)
		} else {
			// standard delay to be nice to the server
			time.Sleep(globalDelay)
		}
	}
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
	if s.client == nil || ctx.Err() != nil {
		return ctx.Err()
	}
	s.log.Infof("Starting full sync...")
	start := time.Now()

	var errors []error

	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err := s.SyncAllGroups(ctx); err != nil {
		s.log.Warnf("Failed to sync groups: %v", err)
		errors = append(errors, err)
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err := s.SyncBlocklist(ctx); err != nil {
		s.log.Warnf("Failed to sync blocklist: %v", err)
		errors = append(errors, err)
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err := s.SyncPrivacySettings(ctx); err != nil {
		s.log.Warnf("Failed to sync privacy: %v", err)
		errors = append(errors, err)
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err := s.SyncAllNewsletters(ctx); err != nil {
		s.log.Warnf("Failed to sync newsletters: %v", err)
		errors = append(errors, err)
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err := s.SyncAllContactPictures(ctx); err != nil {
		s.log.Warnf("Failed to sync contact pictures: %v", err)
		errors = append(errors, err)
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err := s.SyncAllGroupPictures(ctx); err != nil {
		s.log.Warnf("Failed to sync group pictures: %v", err)
		errors = append(errors, err)
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err := s.SyncAllNewsletterPictures(ctx); err != nil {
		s.log.Warnf("Failed to sync newsletter pictures: %v", err)
		errors = append(errors, err)
	}

	s.log.Infof("Full sync completed in %v", time.Since(start))

	if len(errors) > 0 {
		return errors[0]
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
