// Package event provides event handling services for WhatsApp data.
// It routes incoming WhatsApp events to handlers for persistence and processing.
package event

import (
	"context"

	waLog "go.mau.fi/whatsmeow/util/log"

	"orion-agent/internal/data/store"
	"orion-agent/internal/service/media"
	"orion-agent/internal/utils"
)

// AgentProcessor handles AI message processing.
// This interface avoids import cycles with the agent package.
type AgentProcessor interface {
	HandleMessage(ctx context.Context, msg *store.Message)
}

// EventService manages event handling and data persistence.
// All JIDs are normalized to LID form before saving.
type EventService struct {
	ctx   context.Context
	log   waLog.Logger
	utils *utils.Utils
	agent AgentProcessor
	media *media.MediaService

	// Internal dispatcher
	dispatcher *Dispatcher

	// Stores - ALL data is persisted
	messages    *store.MessageStore
	contacts    *store.ContactStore
	chats       *store.ChatStore
	groups      *store.GroupStore
	newsletters *store.NewsletterStore
	receipts    *store.ReceiptStore
	reactions   *store.ReactionStore
	calls       *store.CallStore
	polls       *store.PollStore
	labels      *store.LabelStore
	privacy     *store.PrivacyStore
	blocklist   *store.BlocklistStore
}

// NewEventService creates a new EventService.
func NewEventService(
	log waLog.Logger,
	utils *utils.Utils,
	agent AgentProcessor,
	media *media.MediaService,
	messages *store.MessageStore,
	contacts *store.ContactStore,
	chats *store.ChatStore,
	groups *store.GroupStore,
	newsletters *store.NewsletterStore,
	receipts *store.ReceiptStore,
	reactions *store.ReactionStore,
	calls *store.CallStore,
	polls *store.PollStore,
	labels *store.LabelStore,
	privacy *store.PrivacyStore,
	blocklist *store.BlocklistStore,
) *EventService {
	return &EventService{
		log:         log.Sub("EventService"),
		utils:       utils,
		agent:       agent,
		media:       media,
		messages:    messages,
		contacts:    contacts,
		chats:       chats,
		groups:      groups,
		newsletters: newsletters,
		receipts:    receipts,
		reactions:   reactions,
		calls:       calls,
		polls:       polls,
		labels:      labels,
		privacy:     privacy,
		blocklist:   blocklist,
	}
}

// SetDispatcher sets up the internal dispatcher with context.
func (s *EventService) SetDispatcher(ctx context.Context) {
	s.ctx = ctx
	s.dispatcher = NewDispatcher(s, ctx, s.log)
}

// Handle routes an event through the internal dispatcher.
func (s *EventService) Handle(evt interface{}) {
	if s.dispatcher != nil {
		s.dispatcher.Handle(evt)
	}
}
