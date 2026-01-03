// Package event provides event handling services for WhatsApp data.
package event

import (
	"context"

	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// Dispatcher routes incoming events to EventService handlers.
// This is the internal dispatcher owned by EventService.
type Dispatcher struct {
	service *EventService
	ctx     context.Context
	log     waLog.Logger
}

// NewDispatcher creates a new event dispatcher.
func NewDispatcher(service *EventService, ctx context.Context, log waLog.Logger) *Dispatcher {
	return &Dispatcher{
		service: service,
		ctx:     ctx,
		log:     log.Sub("EventDispatcher"),
	}
}

// Handle routes an event to the appropriate EventService handler.
func (d *Dispatcher) Handle(evt interface{}) {
	if d.ctx.Err() != nil {
		return
	}

	switch e := evt.(type) {
	// Message events
	case *events.Message:
		d.log.Debugf("Message from %s in %s", e.Info.Sender, e.Info.Chat)
		d.service.OnMessage(e)
	case *events.Receipt:
		d.log.Debugf("Receipt %s for %d messages", e.Type, len(e.MessageIDs))
		d.service.OnReceipt(e)
	case *events.UndecryptableMessage:
		d.log.Warnf("Undecryptable message from %s", e.Info.Sender)
		d.service.OnUndecryptableMessage(e)

	// Presence events
	case *events.Presence:
		d.service.OnPresence(e)
	case *events.ChatPresence:
		d.service.OnChatPresence(e)

	// Contact events
	case *events.Contact:
		d.service.OnContact(e)
	case *events.PushName:
		d.service.OnPushName(e)
	case *events.BusinessName:
		d.service.OnBusinessName(e)
	case *events.Picture:
		d.service.OnPicture(e)

	// Group events
	case *events.GroupInfo:
		d.log.Debugf("Group info update for %s", e.JID)
		d.service.OnGroupInfo(e)
	case *events.JoinedGroup:
		d.log.Infof("Joined group %s", e.JID)
		d.service.OnJoinedGroup(e)

	// Newsletter events
	case *events.NewsletterJoin:
		d.log.Infof("Joined newsletter %s", e.ID)
		d.service.OnNewsletterJoin(e)
	case *events.NewsletterLeave:
		d.log.Infof("Left newsletter %s", e.ID)
		d.service.OnNewsletterLeave(e)
	case *events.NewsletterLiveUpdate:
		d.service.OnNewsletterLiveUpdate(e)
	case *events.NewsletterMuteChange:
		d.service.OnNewsletterMuteChange(e)

	// Chat state events
	case *events.Pin:
		d.service.OnPinChat(e)
	case *events.Mute:
		d.service.OnMuteChat(e)
	case *events.Archive:
		d.service.OnArchiveChat(e)
	case *events.Star:
		d.service.OnStarMessage(e)
	case *events.DeleteForMe:
		d.service.OnDeleteForMe(e)
	case *events.MarkChatAsRead:
		d.service.OnMarkChatAsRead(e)
	case *events.ClearChat:
		d.service.OnClearChat(e)
	case *events.DeleteChat:
		d.service.OnDeleteChat(e)

	// Label events
	case *events.LabelEdit:
		d.service.OnLabelEdit(e)
	case *events.LabelAssociationChat:
		d.service.OnLabelAssociationChat(e)
	case *events.LabelAssociationMessage:
		d.service.OnLabelAssociationMessage(e)

	// Privacy events
	case *events.Blocklist:
		d.service.OnBlocklist(e)
	case *events.PrivacySettings:
		d.service.OnPrivacySettings(e)

	// Call events
	case *events.CallOffer:
		d.log.Debugf("Incoming call from %s", e.From)
		d.service.OnCallOffer(e)
	case *events.CallAccept:
		d.service.OnCallAccept(e)
	case *events.CallTerminate:
		d.service.OnCallTerminate(e)

	// Identity events
	case *events.IdentityChange:
		d.log.Infof("Identity changed for %s", e.JID)
		d.service.OnIdentityChange(e)

	// History sync events
	case *events.HistorySync:
		d.log.Infof("History sync: %d conversations", len(e.Data.GetConversations()))
		d.service.OnHistorySync(e)
	case *events.AppStateSyncComplete:
		d.log.Infof("App state sync complete: %s", e.Name)
		d.service.OnAppStateSyncComplete(e)

	// Offline sync events
	case *events.OfflineSyncPreview:
		d.log.Infof("Offline sync preview: %d total, %d messages", e.Total, e.Messages)
		d.service.OnOfflineSyncPreview(e)
	case *events.OfflineSyncCompleted:
		d.log.Infof("Offline sync completed: %d events", e.Count)
		d.service.OnOfflineSyncCompleted(e)

	default:
		d.log.Debugf("Unhandled event type: %T", evt)
	}
}
