// Package sync provides synchronization and coalescence services.
package sync

import (
	"context"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// Dispatcher routes incoming events to syncservice handlers for coalescence.
// This is the internal dispatcher owned by SyncService.
type Dispatcher struct {
	service *SyncService
	ctx     context.Context
	log     waLog.Logger
}

// NewDispatcher creates a new sync dispatcher.
func NewDispatcher(service *SyncService, ctx context.Context, log waLog.Logger) *Dispatcher {
	return &Dispatcher{
		service: service,
		ctx:     ctx,
		log:     log.Sub("SyncDispatcher"),
	}
}

// Handle routes an event to the appropriate coalescence handler.
// All handlers are called asynchronously (goroutines) to avoid blocking.
func (d *Dispatcher) Handle(evt interface{}) {
	switch e := evt.(type) {
	case *events.Connected:
		// Trigger full sync on connect
		go func() {
			if err := d.service.FullSync(d.ctx); err != nil {
				d.log.Warnf("Initial sync failed: %v", err)
			}
			// Start periodic scheduler
			d.service.StartScheduler(DefaultSchedulerConfig())
		}()

	case *events.PairSuccess:
		d.log.Infof("Paired successfully as %s", e.ID)

	case *events.Message:
		// Coalescence: sync sender + group if unknown
		go d.service.OnNewMessage(d.ctx, e.Info.Chat, e.Info.Sender, e.Info.IsGroup)

	case *events.Receipt:
		// Coalescence: sync receipt sender
		go d.service.OnNewContact(d.ctx, e.Sender)

	case *events.Presence:
		// Coalescence: sync contact from presence
		go d.service.OnPresenceUpdate(d.ctx, e.From)

	case *events.ChatPresence:
		// Coalescence: sync sender from typing status
		go d.service.OnChatPresenceUpdate(d.ctx, e.Chat, e.Sender)

	case *events.PushName:
		// Coalescence: sync profile pic on push name update
		go d.service.OnPushNameUpdate(d.ctx, e.JID)

	case *events.Picture:
		// Coalescence: fetch full picture info
		go d.service.OnPictureUpdate(d.ctx, e.JID, e.PictureID)

	case *events.JoinedGroup:
		// Coalescence: full sync for new group
		go d.service.OnGroupJoined(d.ctx, e.JID)

	case *events.GroupInfo:
		// Coalescence: sync group info changes + participants
		go func() {
			d.service.OnGroupInfoChange(d.ctx, e.JID)
			// Sync all mentioned participants
			var participantJIDs []types.JID
			if e.Join != nil {
				participantJIDs = append(participantJIDs, e.Join...)
			}
			if e.Leave != nil {
				participantJIDs = append(participantJIDs, e.Leave...)
			}
			if len(participantJIDs) > 0 {
				d.service.OnGroupParticipantsChange(d.ctx, e.JID, participantJIDs)
			}
		}()

	case *events.HistorySync:
		// Coalescence: batch sync contacts/groups from history
		go func() {
			var contactJIDs []types.JID
			var groupJIDs []types.JID
			if e.Data != nil && e.Data.Conversations != nil {
				for _, conv := range e.Data.Conversations {
					if conv.ID != nil {
						jid, _ := types.ParseJID(*conv.ID)
						switch jid.Server {
						case types.GroupServer:
							groupJIDs = append(groupJIDs, jid)
						case types.HiddenUserServer, types.DefaultUserServer:
							contactJIDs = append(contactJIDs, jid)
						}
					}
				}
			}
			d.service.OnHistorySyncContacts(d.ctx, contactJIDs)
			d.service.OnHistorySyncGroups(d.ctx, groupJIDs)
		}()

	case *events.CallOffer:
		// Coalescence: sync caller info
		go d.service.OnCallReceived(d.ctx, e.CallCreator)

	case *events.Blocklist:
		// Coalescence: refresh blocklist
		go d.service.OnBlocklistChange(d.ctx)

	case *events.PrivacySettings:
		// Coalescence: refresh privacy settings
		go d.service.OnPrivacySettingsChange(d.ctx)

	case *events.BusinessName:
		// Coalescence: sync contact with business name
		go d.service.OnNewContact(d.ctx, e.JID)

	case *events.Contact:
		// Coalescence: sync full contact info
		go d.service.OnNewContact(d.ctx, e.JID)

	case *events.NewsletterJoin:
		// Coalescence: newsletter info
		go d.service.OnNewsletterMessage(d.ctx, e.ID)
	}
}
