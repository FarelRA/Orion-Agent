package handler

import (
	"context"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"

	"orion-agent/internal/event"
	"orion-agent/internal/extract"
	"orion-agent/internal/service"
	"orion-agent/internal/store"
)

// DataHandler persists all event data to the database.
// All JIDs are normalized to LID form before saving.
type DataHandler struct {
	event.BaseHandler
	ctx         context.Context
	log         waLog.Logger
	jidService  *service.JIDService
	messages    *store.MessageStore
	contacts    *store.ContactStore
	chats       *store.ChatStore
	groups      *store.GroupStore
	newsletters *store.NewsletterStore
	receipts    *store.ReceiptStore
	reactions   *store.ReactionStore
	calls       *store.CallStore
	polls       *store.PollStore
}

// NewDataHandler creates a new DataHandler.
func NewDataHandler(
	ctx context.Context,
	log waLog.Logger,
	jidService *service.JIDService,
	messages *store.MessageStore,
	contacts *store.ContactStore,
	chats *store.ChatStore,
	groups *store.GroupStore,
	newsletters *store.NewsletterStore,
	receipts *store.ReceiptStore,
	reactions *store.ReactionStore,
	calls *store.CallStore,
	polls *store.PollStore,
) *DataHandler {
	return &DataHandler{
		ctx:         ctx,
		log:         log.Sub("DataHandler"),
		jidService:  jidService,
		messages:    messages,
		contacts:    contacts,
		chats:       chats,
		groups:      groups,
		newsletters: newsletters,
		receipts:    receipts,
		reactions:   reactions,
		calls:       calls,
		polls:       polls,
	}
}

// OnMessage saves incoming messages to the database.
func (h *DataHandler) OnMessage(evt *events.Message) {
	msg := extract.MessageFromEvent(evt)

	// Normalize all JIDs in the message
	msg.ChatJID = h.jidService.NormalizeJID(h.ctx, msg.ChatJID)
	msg.SenderLID = h.jidService.NormalizeJID(h.ctx, msg.SenderLID)
	msg.QuotedSenderLID = h.jidService.NormalizeJID(h.ctx, msg.QuotedSenderLID)
	msg.BroadcastListJID = h.jidService.NormalizeJID(h.ctx, msg.BroadcastListJID)

	// Normalize chat JID for operations
	chatJID := h.jidService.NormalizeJID(h.ctx, evt.Info.Chat)
	senderJID := h.jidService.NormalizeJID(h.ctx, evt.Info.Sender)

	// Ensure chat exists
	chatType := store.ChatTypeUser
	if evt.Info.IsGroup {
		chatType = store.ChatTypeGroup
	}
	if err := h.chats.EnsureExists(chatJID, chatType); err != nil {
		h.log.Errorf("Failed to ensure chat exists: %v", err)
	}

	// Save message
	if err := h.messages.Put(msg); err != nil {
		h.log.Errorf("Failed to save message: %v", err)
	} else {
		h.log.Debugf("Saved message %s from %s", msg.ID, msg.SenderLID)
	}

	// Update chat last message
	if err := h.chats.UpdateLastMessage(chatJID, msg.ID, msg.Timestamp); err != nil {
		h.log.Errorf("Failed to update chat last message: %v", err)
	}

	// Update contact push name if available
	if evt.Info.PushName != "" && !senderJID.IsEmpty() {
		if err := h.contacts.UpdatePushName(senderJID, evt.Info.PushName); err != nil {
			h.log.Errorf("Failed to update push name: %v", err)
		}
	}
}

// OnReceipt saves message receipts to the database.
func (h *DataHandler) OnReceipt(evt *events.Receipt) {
	receipts := extract.ReceiptFromEvent(evt)

	// Normalize all JIDs in receipts
	for i := range receipts {
		receipts[i].ChatJID = h.jidService.NormalizeJID(h.ctx, receipts[i].ChatJID)
		receipts[i].RecipientLID = h.jidService.NormalizeJID(h.ctx, receipts[i].RecipientLID)
	}

	if err := h.receipts.PutMany(receipts); err != nil {
		h.log.Errorf("Failed to save receipts: %v", err)
	}
}

// OnPushName updates contact push names.
func (h *DataHandler) OnPushName(evt *events.PushName) {
	contact := extract.ContactFromPushName(evt)
	contact.LID = h.jidService.NormalizeJID(h.ctx, contact.LID)

	if err := h.contacts.Put(contact); err != nil {
		h.log.Errorf("Failed to update push name: %v", err)
	}
}

// OnBusinessName updates contact business names.
func (h *DataHandler) OnBusinessName(evt *events.BusinessName) {
	contact := extract.ContactFromBusinessName(evt)
	contact.LID = h.jidService.NormalizeJID(h.ctx, contact.LID)

	if err := h.contacts.Put(contact); err != nil {
		h.log.Errorf("Failed to update business name: %v", err)
	}
}

// OnGroupInfo updates group information.
func (h *DataHandler) OnGroupInfo(evt *events.GroupInfo) {
	group := extract.GroupFromEvent(evt)
	group.JID = h.jidService.NormalizeJID(h.ctx, group.JID)
	group.NameSetByLID = h.jidService.NormalizeJID(h.ctx, group.NameSetByLID)
	group.TopicSetByLID = h.jidService.NormalizeJID(h.ctx, group.TopicSetByLID)
	group.OwnerLID = h.jidService.NormalizeJID(h.ctx, group.OwnerLID)
	group.CreatedByLID = h.jidService.NormalizeJID(h.ctx, group.CreatedByLID)

	if err := h.groups.Put(group); err != nil {
		h.log.Errorf("Failed to update group info: %v", err)
	}

	groupJID := h.jidService.NormalizeJID(h.ctx, evt.JID)

	// Handle participant changes - normalize all JIDs
	for _, jid := range evt.Join {
		normalizedJID := h.jidService.NormalizeJID(h.ctx, jid)
		if err := h.groups.PutParticipant(&store.GroupParticipant{
			GroupJID:  groupJID,
			MemberLID: normalizedJID,
		}); err != nil {
			h.log.Errorf("Failed to add participant: %v", err)
		}
	}
	for _, jid := range evt.Leave {
		normalizedJID := h.jidService.NormalizeJID(h.ctx, jid)
		if err := h.groups.RemoveParticipant(groupJID, normalizedJID); err != nil {
			h.log.Errorf("Failed to remove participant: %v", err)
		}
	}
	for _, jid := range evt.Promote {
		normalizedJID := h.jidService.NormalizeJID(h.ctx, jid)
		if err := h.groups.PutParticipant(&store.GroupParticipant{
			GroupJID:  groupJID,
			MemberLID: normalizedJID,
			IsAdmin:   true,
		}); err != nil {
			h.log.Errorf("Failed to promote participant: %v", err)
		}
	}
	for _, jid := range evt.Demote {
		normalizedJID := h.jidService.NormalizeJID(h.ctx, jid)
		if err := h.groups.PutParticipant(&store.GroupParticipant{
			GroupJID:  groupJID,
			MemberLID: normalizedJID,
			IsAdmin:   false,
		}); err != nil {
			h.log.Errorf("Failed to demote participant: %v", err)
		}
	}
}

// OnJoinedGroup saves full group info when joining.
func (h *DataHandler) OnJoinedGroup(evt *events.JoinedGroup) {
	group, participants := extract.GroupFromJoinedEvent(evt)

	// Normalize group JIDs
	group.JID = h.jidService.NormalizeJID(h.ctx, group.JID)
	group.OwnerLID = h.jidService.NormalizeJID(h.ctx, group.OwnerLID)
	group.NameSetByLID = h.jidService.NormalizeJID(h.ctx, group.NameSetByLID)
	group.TopicSetByLID = h.jidService.NormalizeJID(h.ctx, group.TopicSetByLID)

	// Normalize participant JIDs
	for i := range participants {
		participants[i].GroupJID = h.jidService.NormalizeJID(h.ctx, participants[i].GroupJID)
		participants[i].MemberLID = h.jidService.NormalizeJID(h.ctx, participants[i].MemberLID)
		participants[i].AddedByLID = h.jidService.NormalizeJID(h.ctx, participants[i].AddedByLID)
	}

	if err := h.groups.Put(group); err != nil {
		h.log.Errorf("Failed to save group: %v", err)
	}

	// Clear and repopulate participants
	groupJID := h.jidService.NormalizeJID(h.ctx, evt.JID)
	h.groups.ClearParticipants(groupJID)
	if err := h.groups.PutParticipants(participants); err != nil {
		h.log.Errorf("Failed to save participants: %v", err)
	}

	// Ensure chat exists
	h.chats.EnsureExists(groupJID, store.ChatTypeGroup)
}

// OnPinChat updates chat pin status.
func (h *DataHandler) OnPinChat(evt *events.Pin) {
	jid, pinned, ts := extract.ChatStateFromPin(evt)
	jid = h.jidService.NormalizeJID(h.ctx, jid)
	if err := h.chats.SetPinned(jid, pinned, ts); err != nil {
		h.log.Errorf("Failed to update pin status: %v", err)
	}
}

// OnMuteChat updates chat mute status.
func (h *DataHandler) OnMuteChat(evt *events.Mute) {
	jid, mutedUntil := extract.ChatStateFromMute(evt)
	jid = h.jidService.NormalizeJID(h.ctx, jid)
	if err := h.chats.SetMuted(jid, mutedUntil); err != nil {
		h.log.Errorf("Failed to update mute status: %v", err)
	}
}

// OnArchiveChat updates chat archive status.
func (h *DataHandler) OnArchiveChat(evt *events.Archive) {
	jid, archived := extract.ChatStateFromArchive(evt)
	jid = h.jidService.NormalizeJID(h.ctx, jid)
	if err := h.chats.SetArchived(jid, archived); err != nil {
		h.log.Errorf("Failed to update archive status: %v", err)
	}
}

// OnMarkChatAsRead marks chat as read.
func (h *DataHandler) OnMarkChatAsRead(evt *events.MarkChatAsRead) {
	jid := h.jidService.NormalizeJID(h.ctx, evt.JID)
	if err := h.chats.MarkRead(jid); err != nil {
		h.log.Errorf("Failed to mark chat as read: %v", err)
	}
}

// OnStarMessage updates message starred status.
func (h *DataHandler) OnStarMessage(evt *events.Star) {
	chatJID := h.jidService.NormalizeJID(h.ctx, evt.ChatJID)
	starred := evt.Action.GetStarred()
	if err := h.messages.SetStarred(evt.MessageID, chatJID, starred); err != nil {
		h.log.Errorf("Failed to update star status: %v", err)
	}
}

// OnDeleteForMe marks message as deleted.
func (h *DataHandler) OnDeleteForMe(evt *events.DeleteForMe) {
	chatJID := h.jidService.NormalizeJID(h.ctx, evt.ChatJID)
	if err := h.messages.Delete(evt.MessageID, chatJID); err != nil {
		h.log.Errorf("Failed to delete message: %v", err)
	}
}

// OnClearChat clears all messages from a chat.
func (h *DataHandler) OnClearChat(evt *events.ClearChat) {
	jid := h.jidService.NormalizeJID(h.ctx, evt.JID)
	if err := h.chats.Clear(jid); err != nil {
		h.log.Errorf("Failed to clear chat: %v", err)
	}
}

// OnDeleteChat deletes a chat entirely.
func (h *DataHandler) OnDeleteChat(evt *events.DeleteChat) {
	jid := h.jidService.NormalizeJID(h.ctx, evt.JID)
	if err := h.chats.Delete(jid); err != nil {
		h.log.Errorf("Failed to delete chat: %v", err)
	}
}

// OnPresence updates contact presence.
func (h *DataHandler) OnPresence(evt *events.Presence) {
	jid := h.jidService.NormalizeJID(h.ctx, evt.From)
	if err := h.contacts.UpdatePresence(jid, !evt.Unavailable, evt.LastSeen); err != nil {
		h.log.Errorf("Failed to update presence: %v", err)
	}
}

// OnPicture updates profile picture info.
func (h *DataHandler) OnPicture(evt *events.Picture) {
	jid := h.jidService.NormalizeJID(h.ctx, evt.JID)

	switch evt.JID.Server {
	case types.DefaultUserServer, types.HiddenUserServer:
		// Contact picture
		if err := h.contacts.UpdateProfilePic(jid, evt.PictureID, ""); err != nil {
			h.log.Errorf("Failed to update contact picture: %v", err)
		}
	case types.GroupServer:
		// Group picture
		if err := h.groups.UpdateProfilePic(jid, evt.PictureID, ""); err != nil {
			h.log.Errorf("Failed to update group picture: %v", err)
		}
	case types.NewsletterServer:
		// Newsletter picture
		if err := h.newsletters.UpdateProfilePic(jid, evt.PictureID, ""); err != nil {
			h.log.Errorf("Failed to update newsletter picture: %v", err)
		}
	}
}

// OnHistorySync processes history sync data.
func (h *DataHandler) OnHistorySync(evt *events.HistorySync) {
	data := extract.FromHistorySync(evt)

	// Save JID mappings (using ContactStore.PutJIDMappings)
	// Note: JID mappings are already LID/PN pairs, no normalization needed
	if len(data.JIDMappings) > 0 {
		mappings := make([]store.JIDMapping, len(data.JIDMappings))
		for i, m := range data.JIDMappings {
			mappings[i] = store.JIDMapping{LID: m.LID, PN: m.PN}
		}
		if err := h.contacts.PutJIDMappings(mappings); err != nil {
			h.log.Errorf("Failed to save JID mappings: %v", err)
		}
	}

	// Save contacts - normalize LIDs
	for _, contact := range data.Contacts {
		contact.LID = h.jidService.NormalizeJID(h.ctx, contact.LID)
		if err := h.contacts.Put(contact); err != nil {
			h.log.Errorf("Failed to save contact: %v", err)
		}
	}

	// Save chats - normalize JIDs
	for _, chat := range data.Chats {
		chat.JID = h.jidService.NormalizeJID(h.ctx, chat.JID)
		if err := h.chats.Put(chat); err != nil {
			h.log.Errorf("Failed to save chat: %v", err)
		}
	}

	// Save groups - normalize JIDs
	for _, group := range data.Groups {
		group.JID = h.jidService.NormalizeJID(h.ctx, group.JID)
		group.OwnerLID = h.jidService.NormalizeJID(h.ctx, group.OwnerLID)
		if err := h.groups.Put(group); err != nil {
			h.log.Errorf("Failed to save group: %v", err)
		}
	}

	// Save participants - normalize JIDs
	for i := range data.Participants {
		data.Participants[i].GroupJID = h.jidService.NormalizeJID(h.ctx, data.Participants[i].GroupJID)
		data.Participants[i].MemberLID = h.jidService.NormalizeJID(h.ctx, data.Participants[i].MemberLID)
		if err := h.groups.PutParticipant(&data.Participants[i]); err != nil {
			h.log.Errorf("Failed to save participant: %v", err)
		}
	}

	// Save messages - normalize JIDs
	for _, msg := range data.Messages {
		msg.ChatJID = h.jidService.NormalizeJID(h.ctx, msg.ChatJID)
		msg.SenderLID = h.jidService.NormalizeJID(h.ctx, msg.SenderLID)
		msg.QuotedSenderLID = h.jidService.NormalizeJID(h.ctx, msg.QuotedSenderLID)
		if err := h.messages.Put(msg); err != nil {
			h.log.Errorf("Failed to save message: %v", err)
		}
	}

	h.log.Infof("History sync: saved %d chats, %d groups, %d contacts, %d messages",
		len(data.Chats), len(data.Groups), len(data.Contacts), len(data.Messages))
}

// OnReaction handles message reactions.
func (h *DataHandler) OnReaction(evt *events.Message) {
	rm := evt.Message.GetReactionMessage()
	if rm == nil {
		return
	}

	// Parse target message
	targetKey := rm.GetKey()
	targetMsgID := targetKey.GetID()
	targetChat, _ := types.ParseJID(targetKey.GetRemoteJID())

	// Normalize JIDs
	targetChat = h.jidService.NormalizeJID(h.ctx, targetChat)
	senderLID := h.jidService.NormalizeJID(h.ctx, evt.Info.Sender)

	if rm.GetText() == "" {
		// Reaction removal
		if err := h.reactions.Delete(targetMsgID, targetChat, senderLID); err != nil {
			h.log.Errorf("Failed to delete reaction: %v", err)
		}
	} else {
		// Reaction add/update
		if err := h.reactions.Put(&store.Reaction{
			MessageID: targetMsgID,
			ChatJID:   targetChat,
			SenderLID: senderLID,
			Emoji:     rm.GetText(),
			Timestamp: evt.Info.Timestamp,
		}); err != nil {
			h.log.Errorf("Failed to save reaction: %v", err)
		}
	}
}

// OnCallOffer handles incoming call offers.
func (h *DataHandler) OnCallOffer(evt *events.CallOffer) {
	callerLID := h.jidService.NormalizeJID(h.ctx, evt.CallCreator)
	groupJID := h.jidService.NormalizeJID(h.ctx, evt.GroupJID)

	isGroup := !evt.GroupJID.IsEmpty()

	if err := h.calls.Put(&store.Call{
		CallID:    evt.CallID,
		CallerLID: callerLID,
		GroupJID:  groupJID,
		IsGroup:   isGroup,
		Timestamp: evt.Timestamp,
		Outcome:   "pending",
	}); err != nil {
		h.log.Errorf("Failed to save call offer: %v", err)
	}
}

// OnCallTerminate handles call termination.
func (h *DataHandler) OnCallTerminate(evt *events.CallTerminate) {
	outcome := "ended"
	if evt.Reason == "timeout" {
		outcome = "missed"
	} else if evt.Reason == "busy" {
		outcome = "busy"
	} else if evt.Reason == "reject" {
		outcome = "rejected"
	}

	if err := h.calls.UpdateOutcome(evt.CallID, outcome, 0); err != nil {
		h.log.Errorf("Failed to update call outcome: %v", err)
	}
}

// OnMessageEdit handles message edits.
func (h *DataHandler) OnMessageEdit(evt *events.Message) {
	pm := evt.Message.GetProtocolMessage()
	if pm == nil || pm.GetType() != waE2E.ProtocolMessage_MESSAGE_EDIT {
		return
	}

	editedMsg := pm.GetEditedMessage()
	targetKey := pm.GetKey()
	targetMsgID := targetKey.GetID()
	targetChat := h.jidService.NormalizeJID(h.ctx, evt.Info.Chat)

	// Extract new content
	newContent := ""
	if txt := editedMsg.GetConversation(); txt != "" {
		newContent = txt
	} else if ext := editedMsg.GetExtendedTextMessage(); ext != nil {
		newContent = ext.GetText()
	}

	if err := h.messages.MarkEdited(targetMsgID, targetChat, newContent, evt.Info.Timestamp); err != nil {
		h.log.Errorf("Failed to mark message as edited: %v", err)
	}
}

// OnMessageRevoke handles message revocation (delete for everyone).
func (h *DataHandler) OnMessageRevoke(evt *events.Message) {
	pm := evt.Message.GetProtocolMessage()
	if pm == nil || pm.GetType() != waE2E.ProtocolMessage_REVOKE {
		return
	}

	targetKey := pm.GetKey()
	targetMsgID := targetKey.GetID()
	targetChat := h.jidService.NormalizeJID(h.ctx, evt.Info.Chat)

	if err := h.messages.SetRevoked(targetMsgID, targetChat); err != nil {
		h.log.Errorf("Failed to mark message as revoked: %v", err)
	}
}
