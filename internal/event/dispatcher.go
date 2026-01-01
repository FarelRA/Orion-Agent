package event

import (
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// Dispatcher routes WhatsApp events to registered handlers.
type Dispatcher struct {
	handlers []Handler
	log      waLog.Logger
}

// NewDispatcher creates a new Dispatcher.
func NewDispatcher(log waLog.Logger) *Dispatcher {
	return &Dispatcher{
		handlers: make([]Handler, 0),
		log:      log.Sub("Dispatcher"),
	}
}

// Register adds a handler to the dispatcher.
func (d *Dispatcher) Register(h Handler) {
	d.handlers = append(d.handlers, h)
}

// Handle processes a WhatsApp event and routes it to all registered handlers.
func (d *Dispatcher) Handle(evt interface{}) {
	switch e := evt.(type) {
	// Connection events
	case *events.Connected:
		d.log.Debugf("Connected event")
		for _, h := range d.handlers {
			h.OnConnected(e)
		}
	case *events.Disconnected:
		d.log.Debugf("Disconnected event")
		for _, h := range d.handlers {
			h.OnDisconnected(e)
		}
	case *events.LoggedOut:
		d.log.Infof("Logged out: %v", e.Reason)
		for _, h := range d.handlers {
			h.OnLoggedOut(e)
		}
	case *events.StreamReplaced:
		d.log.Infof("Stream replaced")
		for _, h := range d.handlers {
			h.OnStreamReplaced(e)
		}
	case *events.StreamError:
		d.log.Errorf("Stream error: %v", e.Code)
		for _, h := range d.handlers {
			h.OnStreamError(e)
		}
	case *events.ConnectFailure:
		d.log.Errorf("Connect failure: %s - %s", e.Reason, e.Message)
		for _, h := range d.handlers {
			h.OnConnectFailure(e)
		}
	case *events.ClientOutdated:
		d.log.Errorf("Client outdated - update required")
		for _, h := range d.handlers {
			h.OnClientOutdated(e)
		}
	case *events.TemporaryBan:
		d.log.Warnf("Temporary ban: code=%d, expires=%s", e.Code, e.Expire)
		for _, h := range d.handlers {
			h.OnTemporaryBan(e)
		}
	case *events.KeepAliveTimeout:
		d.log.Warnf("Keep alive timeout, last success: %s", e.LastSuccess)
		for _, h := range d.handlers {
			h.OnKeepAliveTimeout(e)
		}
	case *events.KeepAliveRestored:
		d.log.Infof("Keep alive restored")
		for _, h := range d.handlers {
			h.OnKeepAliveRestored(e)
		}

	// Message events
	case *events.Message:
		d.log.Debugf("Message from %s in %s", e.Info.Sender, e.Info.Chat)
		for _, h := range d.handlers {
			h.OnMessage(e)
		}
	case *events.Receipt:
		d.log.Debugf("Receipt %s for %d messages", e.Type, len(e.MessageIDs))
		for _, h := range d.handlers {
			h.OnReceipt(e)
		}
	case *events.UndecryptableMessage:
		d.log.Warnf("Undecryptable message from %s", e.Info.Sender)
		for _, h := range d.handlers {
			h.OnUndecryptableMessage(e)
		}
	case *events.MediaRetry:
		d.log.Debugf("Media retry for %s", e.MessageID)
		for _, h := range d.handlers {
			h.OnMediaRetry(e)
		}

	// Presence events
	case *events.Presence:
		for _, h := range d.handlers {
			h.OnPresence(e)
		}
	case *events.ChatPresence:
		for _, h := range d.handlers {
			h.OnChatPresence(e)
		}

	// Group events
	case *events.GroupInfo:
		d.log.Debugf("Group info update for %s", e.JID)
		for _, h := range d.handlers {
			h.OnGroupInfo(e)
		}
	case *events.JoinedGroup:
		d.log.Infof("Joined group %s", e.JID)
		for _, h := range d.handlers {
			h.OnJoinedGroup(e)
		}
	case *events.Picture:
		for _, h := range d.handlers {
			h.OnPicture(e)
		}

	// Newsletter events
	case *events.NewsletterJoin:
		d.log.Infof("Joined newsletter %s", e.ID)
		for _, h := range d.handlers {
			h.OnNewsletterJoin(e)
		}
	case *events.NewsletterLeave:
		d.log.Infof("Left newsletter %s", e.ID)
		for _, h := range d.handlers {
			h.OnNewsletterLeave(e)
		}
	case *events.NewsletterLiveUpdate:
		for _, h := range d.handlers {
			h.OnNewsletterLiveUpdate(e)
		}
	case *events.NewsletterMuteChange:
		for _, h := range d.handlers {
			h.OnNewsletterMuteChange(e)
		}

	// AppState events
	case *events.Contact:
		for _, h := range d.handlers {
			h.OnContact(e)
		}
	case *events.PushName:
		for _, h := range d.handlers {
			h.OnPushName(e)
		}
	case *events.BusinessName:
		for _, h := range d.handlers {
			h.OnBusinessName(e)
		}
	case *events.Pin:
		for _, h := range d.handlers {
			h.OnPinChat(e)
		}
	case *events.Mute:
		for _, h := range d.handlers {
			h.OnMuteChat(e)
		}
	case *events.Archive:
		for _, h := range d.handlers {
			h.OnArchiveChat(e)
		}
	case *events.Star:
		for _, h := range d.handlers {
			h.OnStarMessage(e)
		}
	case *events.DeleteForMe:
		for _, h := range d.handlers {
			h.OnDeleteForMe(e)
		}
	case *events.MarkChatAsRead:
		for _, h := range d.handlers {
			h.OnMarkChatAsRead(e)
		}
	case *events.ClearChat:
		for _, h := range d.handlers {
			h.OnClearChat(e)
		}
	case *events.DeleteChat:
		for _, h := range d.handlers {
			h.OnDeleteChat(e)
		}
	case *events.HistorySync:
		d.log.Infof("History sync: %d conversations", len(e.Data.GetConversations()))
		for _, h := range d.handlers {
			h.OnHistorySync(e)
		}
	case *events.AppStateSyncComplete:
		d.log.Infof("App state sync complete: %s", e.Name)
		for _, h := range d.handlers {
			h.OnAppStateSyncComplete(e)
		}

	// Label events
	case *events.LabelEdit:
		for _, h := range d.handlers {
			h.OnLabelEdit(e)
		}
	case *events.LabelAssociationChat:
		for _, h := range d.handlers {
			h.OnLabelAssociationChat(e)
		}
	case *events.LabelAssociationMessage:
		for _, h := range d.handlers {
			h.OnLabelAssociationMessage(e)
		}

	// Privacy events
	case *events.Blocklist:
		for _, h := range d.handlers {
			h.OnBlocklist(e)
		}
	case *events.PrivacySettings:
		for _, h := range d.handlers {
			h.OnPrivacySettings(e)
		}

	// Call events
	case *events.CallOffer:
		d.log.Debugf("Incoming call from %s", e.From)
		for _, h := range d.handlers {
			h.OnCallOffer(e)
		}
	case *events.CallAccept:
		for _, h := range d.handlers {
			h.OnCallAccept(e)
		}
	case *events.CallTerminate:
		for _, h := range d.handlers {
			h.OnCallTerminate(e)
		}

	// Identity events
	case *events.IdentityChange:
		d.log.Infof("Identity changed for %s", e.JID)
		for _, h := range d.handlers {
			h.OnIdentityChange(e)
		}

	// Offline sync events
	case *events.OfflineSyncPreview:
		d.log.Infof("Offline sync preview: %d total, %d messages", e.Total, e.Messages)
		for _, h := range d.handlers {
			h.OnOfflineSyncPreview(e)
		}
	case *events.OfflineSyncCompleted:
		d.log.Infof("Offline sync completed: %d events", e.Count)
		for _, h := range d.handlers {
			h.OnOfflineSyncCompleted(e)
		}

	default:
		// Log unknown events for debugging
		d.log.Debugf("Unhandled event type: %T", evt)
	}
}
