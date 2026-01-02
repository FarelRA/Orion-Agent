package event

import (
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// Handler defines the interface for handling WhatsApp events.
// Implement only the methods you need; unimplemented methods are no-ops.
type Handler interface {
	// Connection events
	OnConnected(*events.Connected)
	OnDisconnected(*events.Disconnected)
	OnLoggedOut(*events.LoggedOut)
	OnStreamReplaced(*events.StreamReplaced)
	OnStreamError(*events.StreamError)
	OnConnectFailure(*events.ConnectFailure)
	OnClientOutdated(*events.ClientOutdated)
	OnTemporaryBan(*events.TemporaryBan)
	OnKeepAliveTimeout(*events.KeepAliveTimeout)
	OnKeepAliveRestored(*events.KeepAliveRestored)

	// Message events
	OnMessage(*events.Message)
	OnReceipt(*events.Receipt)
	OnUndecryptableMessage(*events.UndecryptableMessage)
	OnMediaRetry(*events.MediaRetry)

	// Presence events
	OnPresence(*events.Presence)
	OnChatPresence(*events.ChatPresence)

	// Group events
	OnGroupInfo(*events.GroupInfo)
	OnJoinedGroup(*events.JoinedGroup)
	OnPicture(*events.Picture)

	// Newsletter events
	OnNewsletterJoin(*events.NewsletterJoin)
	OnNewsletterLeave(*events.NewsletterLeave)
	OnNewsletterLiveUpdate(*events.NewsletterLiveUpdate)
	OnNewsletterMuteChange(*events.NewsletterMuteChange)

	// AppState events
	OnContact(*events.Contact)
	OnPushName(*events.PushName)
	OnBusinessName(*events.BusinessName)
	OnPinChat(*events.Pin)
	OnMuteChat(*events.Mute)
	OnArchiveChat(*events.Archive)
	OnStarMessage(*events.Star)
	OnDeleteForMe(*events.DeleteForMe)
	OnMarkChatAsRead(*events.MarkChatAsRead)
	OnClearChat(*events.ClearChat)
	OnDeleteChat(*events.DeleteChat)
	OnHistorySync(*events.HistorySync)
	OnAppStateSyncComplete(*events.AppStateSyncComplete)

	// Label events
	OnLabelEdit(*events.LabelEdit)
	OnLabelAssociationChat(*events.LabelAssociationChat)
	OnLabelAssociationMessage(*events.LabelAssociationMessage)

	// Privacy events
	OnBlocklist(*events.Blocklist)
	OnPrivacySettings(*events.PrivacySettings)

	// Call events
	OnCallOffer(*events.CallOffer)
	OnCallAccept(*events.CallAccept)
	OnCallTerminate(*events.CallTerminate)

	// Identity events
	OnIdentityChange(*events.IdentityChange)

	// Offline sync events
	OnOfflineSyncPreview(*events.OfflineSyncPreview)
	OnOfflineSyncCompleted(*events.OfflineSyncCompleted)
}

// BaseHandler provides default no-op implementations for all Handler methods.
// Embed this in your handler to only implement the methods you need.
type BaseHandler struct{}

// Connection events
func (h *BaseHandler) OnConnected(*events.Connected)                 {}
func (h *BaseHandler) OnDisconnected(*events.Disconnected)           {}
func (h *BaseHandler) OnLoggedOut(*events.LoggedOut)                 {}
func (h *BaseHandler) OnStreamReplaced(*events.StreamReplaced)       {}
func (h *BaseHandler) OnStreamError(*events.StreamError)             {}
func (h *BaseHandler) OnConnectFailure(*events.ConnectFailure)       {}
func (h *BaseHandler) OnClientOutdated(*events.ClientOutdated)       {}
func (h *BaseHandler) OnTemporaryBan(*events.TemporaryBan)           {}
func (h *BaseHandler) OnKeepAliveTimeout(*events.KeepAliveTimeout)   {}
func (h *BaseHandler) OnKeepAliveRestored(*events.KeepAliveRestored) {}

// Message events
func (h *BaseHandler) OnMessage(*events.Message)                           {}
func (h *BaseHandler) OnReceipt(*events.Receipt)                           {}
func (h *BaseHandler) OnUndecryptableMessage(*events.UndecryptableMessage) {}
func (h *BaseHandler) OnMediaRetry(*events.MediaRetry)                     {}

// Presence events
func (h *BaseHandler) OnPresence(*events.Presence)         {}
func (h *BaseHandler) OnChatPresence(*events.ChatPresence) {}

// Group events
func (h *BaseHandler) OnGroupInfo(*events.GroupInfo)     {}
func (h *BaseHandler) OnJoinedGroup(*events.JoinedGroup) {}
func (h *BaseHandler) OnPicture(*events.Picture)         {}

// Newsletter events
func (h *BaseHandler) OnNewsletterJoin(*events.NewsletterJoin)             {}
func (h *BaseHandler) OnNewsletterLeave(*events.NewsletterLeave)           {}
func (h *BaseHandler) OnNewsletterLiveUpdate(*events.NewsletterLiveUpdate) {}
func (h *BaseHandler) OnNewsletterMuteChange(*events.NewsletterMuteChange) {}

// AppState events
func (h *BaseHandler) OnContact(*events.Contact)                           {}
func (h *BaseHandler) OnPushName(*events.PushName)                         {}
func (h *BaseHandler) OnBusinessName(*events.BusinessName)                 {}
func (h *BaseHandler) OnPinChat(*events.Pin)                               {}
func (h *BaseHandler) OnMuteChat(*events.Mute)                             {}
func (h *BaseHandler) OnArchiveChat(*events.Archive)                       {}
func (h *BaseHandler) OnStarMessage(*events.Star)                          {}
func (h *BaseHandler) OnDeleteForMe(*events.DeleteForMe)                   {}
func (h *BaseHandler) OnMarkChatAsRead(*events.MarkChatAsRead)             {}
func (h *BaseHandler) OnClearChat(*events.ClearChat)                       {}
func (h *BaseHandler) OnDeleteChat(*events.DeleteChat)                     {}
func (h *BaseHandler) OnHistorySync(*events.HistorySync)                   {}
func (h *BaseHandler) OnAppStateSyncComplete(*events.AppStateSyncComplete) {}

// Label events
func (h *BaseHandler) OnLabelEdit(*events.LabelEdit)                             {}
func (h *BaseHandler) OnLabelAssociationChat(*events.LabelAssociationChat)       {}
func (h *BaseHandler) OnLabelAssociationMessage(*events.LabelAssociationMessage) {}

// Privacy events
func (h *BaseHandler) OnBlocklist(*events.Blocklist)             {}
func (h *BaseHandler) OnPrivacySettings(*events.PrivacySettings) {}

// Call events
func (h *BaseHandler) OnCallOffer(*events.CallOffer)         {}
func (h *BaseHandler) OnCallAccept(*events.CallAccept)       {}
func (h *BaseHandler) OnCallTerminate(*events.CallTerminate) {}

// Identity events
func (h *BaseHandler) OnIdentityChange(*events.IdentityChange) {}

// Offline sync events
func (h *BaseHandler) OnOfflineSyncPreview(*events.OfflineSyncPreview)     {}
func (h *BaseHandler) OnOfflineSyncCompleted(*events.OfflineSyncCompleted) {}

// MessageInfo extracts common message info for convenience.
type MessageInfo struct {
	ID        string
	ChatJID   types.JID
	SenderJID types.JID
	FromMe    bool
	Timestamp int64
	IsGroup   bool
}

// ExtractMessageInfo extracts MessageInfo from an events.Message.
func ExtractMessageInfo(msg *events.Message) MessageInfo {
	return MessageInfo{
		ID:        msg.Info.ID,
		ChatJID:   msg.Info.Chat,
		SenderJID: msg.Info.Sender,
		FromMe:    msg.Info.IsFromMe,
		Timestamp: msg.Info.Timestamp.Unix(),
		IsGroup:   msg.Info.IsGroup,
	}
}
