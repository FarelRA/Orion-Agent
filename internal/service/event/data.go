package event

import (
	"context"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"

	"orion-agent/internal/data/extract"
	"orion-agent/internal/data/store"
	"orion-agent/internal/utils"
)

// DataHandler persists all event data to the database.
// All JIDs are normalized to LID form before saving.
// This handler captures ALL available information from whatsmeow events.
type DataHandler struct {
	BaseHandler
	ctx         context.Context
	log         waLog.Logger
	utils       *utils.Utils
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

// NewDataHandler creates a new DataHandler.
func NewDataHandler(
	ctx context.Context,
	log waLog.Logger,
	utils *utils.Utils,
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
) *DataHandler {
	return &DataHandler{
		ctx:         ctx,
		log:         log.Sub("DataHandler"),
		utils:       utils,
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

// ===========================================================================
// MESSAGE EVENTS
// ===========================================================================

// OnMessage saves incoming messages to the database.
func (h *DataHandler) OnMessage(evt *events.Message) {
	// Skip special message types that have their own handlers
	if evt.Message.GetReactionMessage() != nil {
		h.handleReaction(evt)
		return
	}
	if pm := evt.Message.GetProtocolMessage(); pm != nil {
		h.handleProtocolMessage(evt)
		return
	}
	if evt.Message.GetPollUpdateMessage() != nil {
		h.handlePollUpdate(evt)
		return
	}
	if evt.Message.GetPinInChatMessage() != nil {
		h.handlePinMessage(evt)
		return
	}
	if evt.Message.GetKeepInChatMessage() != nil {
		h.handleKeepMessage(evt)
		return
	}

	msg := extract.MessageFromEvent(evt)

	// Normalize all JIDs in the message
	msg.ChatJID = h.utils.NormalizeJID(h.ctx, msg.ChatJID)
	msg.SenderLID = h.utils.NormalizeJID(h.ctx, msg.SenderLID)
	msg.QuotedSenderLID = h.utils.NormalizeJID(h.ctx, msg.QuotedSenderLID)
	msg.BroadcastListJID = h.utils.NormalizeJID(h.ctx, msg.BroadcastListJID)

	// Normalize mentioned JIDs
	for i := range msg.MentionedJIDs {
		msg.MentionedJIDs[i] = h.utils.NormalizeJID(h.ctx, msg.MentionedJIDs[i])
	}
	for i := range msg.GroupMentions {
		msg.GroupMentions[i].GroupJID = h.utils.NormalizeJID(h.ctx, msg.GroupMentions[i].GroupJID)
	}

	// Normalize chat JID for operations
	chatJID := h.utils.NormalizeJID(h.ctx, evt.Info.Chat)
	senderJID := h.utils.NormalizeJID(h.ctx, evt.Info.Sender)

	// Ensure chat exists
	chatType := store.ChatTypeUser
	if evt.Info.IsGroup {
		chatType = store.ChatTypeGroup
	} else if chatJID.Server == types.NewsletterServer {
		chatType = store.ChatTypeNewsletter
	} else if chatJID.Server == types.BroadcastServer {
		chatType = store.ChatTypeBroadcast
	}
	if err := h.chats.EnsureExists(chatJID, chatType); err != nil {
		h.log.Errorf("Failed to ensure chat exists: %v", err)
	}

	// Save message
	if err := h.messages.Put(msg); err != nil {
		h.log.Errorf("Failed to save message: %v", err)
	} else {
		h.log.Debugf("Saved message %s from %s (type: %s)", msg.ID, msg.SenderLID, msg.MessageType)
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

	// Handle poll creation message
	if msg.MessageType == "poll" || msg.MessageType == "poll_v2" || msg.MessageType == "poll_v3" {
		h.savePollCreation(msg, chatJID, senderJID)
	}
}

// handleReaction handles reaction messages.
func (h *DataHandler) handleReaction(evt *events.Message) {
	rm := evt.Message.GetReactionMessage()
	if rm == nil {
		return
	}

	// Parse target message
	targetKey := rm.GetKey()
	targetMsgID := targetKey.GetID()
	targetChat, _ := types.ParseJID(targetKey.GetRemoteJID())

	// Normalize JIDs
	targetChat = h.utils.NormalizeJID(h.ctx, targetChat)
	senderLID := h.utils.NormalizeJID(h.ctx, evt.Info.Sender)

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

// handleProtocolMessage handles protocol messages (edits, revokes, etc.).
func (h *DataHandler) handleProtocolMessage(evt *events.Message) {
	pm := evt.Message.GetProtocolMessage()
	if pm == nil {
		return
	}

	targetKey := pm.GetKey()
	targetMsgID := targetKey.GetID()
	targetChat := h.utils.NormalizeJID(h.ctx, evt.Info.Chat)

	switch pm.GetType() {
	case waE2E.ProtocolMessage_MESSAGE_EDIT:
		editedMsg := pm.GetEditedMessage()
		newContent := ""
		if txt := editedMsg.GetConversation(); txt != "" {
			newContent = txt
		} else if ext := editedMsg.GetExtendedTextMessage(); ext != nil {
			newContent = ext.GetText()
		}
		if err := h.messages.MarkEdited(targetMsgID, targetChat, newContent, evt.Info.Timestamp); err != nil {
			h.log.Errorf("Failed to mark message as edited: %v", err)
		}

	case waE2E.ProtocolMessage_REVOKE:
		if err := h.messages.SetRevoked(targetMsgID, targetChat); err != nil {
			h.log.Errorf("Failed to mark message as revoked: %v", err)
		}

	case waE2E.ProtocolMessage_EPHEMERAL_SETTING:
		duration := pm.GetEphemeralExpiration()
		if err := h.chats.SetEphemeral(targetChat, duration, evt.Info.Timestamp); err != nil {
			h.log.Errorf("Failed to update ephemeral setting: %v", err)
		}
	}
}

// handlePollUpdate handles poll vote updates.
func (h *DataHandler) handlePollUpdate(evt *events.Message) {
	vote := extract.PollUpdateFromEvent(evt)
	if vote == nil {
		return
	}

	// Normalize JIDs
	vote.ChatJID = h.utils.NormalizeJID(h.ctx, vote.ChatJID)
	vote.VoterLID = h.utils.NormalizeJID(h.ctx, vote.VoterLID)

	if err := h.polls.SaveVote(vote); err != nil {
		h.log.Errorf("Failed to save poll vote: %v", err)
	}
}

// savePollCreation saves poll creation to the polls table.
func (h *DataHandler) savePollCreation(msg *store.Message, chatJID, creatorLID types.JID) {
	poll := &store.Poll{
		MessageID:     msg.ID,
		ChatJID:       chatJID,
		CreatorLID:    creatorLID,
		Question:      msg.PollName,
		Options:       msg.PollOptions,
		IsMultiSelect: msg.PollSelectMax != 1,
		SelectMax:     msg.PollSelectMax,
		EncryptionKey: msg.PollEncryptionKey,
		CreatedAt:     msg.Timestamp,
	}
	if err := h.polls.Put(poll); err != nil {
		h.log.Errorf("Failed to save poll: %v", err)
	}
}

// handlePinMessage handles pin/unpin messages.
func (h *DataHandler) handlePinMessage(evt *events.Message) {
	pin := evt.Message.GetPinInChatMessage()
	if pin == nil {
		return
	}

	// Get target message info
	key := pin.GetKey()
	targetMsgID := key.GetID()
	targetChat, _ := types.ParseJID(key.GetRemoteJID())
	targetChat = h.utils.NormalizeJID(h.ctx, targetChat)

	// Determine if pin or unpin
	isPinned := pin.GetType() == waE2E.PinInChatMessage_PIN_FOR_ALL
	pinTime := evt.Info.Timestamp

	if err := h.messages.SetPinned(targetMsgID, targetChat, isPinned, pinTime); err != nil {
		h.log.Errorf("Failed to update pin status for message %s: %v", targetMsgID, err)
	} else {
		action := "pinned"
		if !isPinned {
			action = "unpinned"
		}
		h.log.Debugf("Message %s %s in %s", targetMsgID, action, targetChat)
	}
}

// handleKeepMessage handles star/unstar (keep in chat) messages.
func (h *DataHandler) handleKeepMessage(evt *events.Message) {
	keep := evt.Message.GetKeepInChatMessage()
	if keep == nil {
		return
	}

	// Get target message info
	key := keep.GetKey()
	targetMsgID := key.GetID()
	targetChat, _ := types.ParseJID(key.GetRemoteJID())
	targetChat = h.utils.NormalizeJID(h.ctx, targetChat)

	// Determine if star or unstar
	isStarred := keep.GetKeepType() == waE2E.KeepType_KEEP_FOR_ALL

	if err := h.messages.SetStarred(targetMsgID, targetChat, isStarred); err != nil {
		h.log.Errorf("Failed to update star status for message %s: %v", targetMsgID, err)
	} else {
		action := "starred"
		if !isStarred {
			action = "unstarred"
		}
		h.log.Debugf("Message %s %s in %s", targetMsgID, action, targetChat)
	}
}

// OnReceipt saves message receipts to the database.
func (h *DataHandler) OnReceipt(evt *events.Receipt) {
	receipts := extract.ReceiptFromEvent(evt)

	// Normalize all JIDs in receipts
	for i := range receipts {
		receipts[i].ChatJID = h.utils.NormalizeJID(h.ctx, receipts[i].ChatJID)
		receipts[i].RecipientLID = h.utils.NormalizeJID(h.ctx, receipts[i].RecipientLID)
	}

	if err := h.receipts.PutMany(receipts); err != nil {
		h.log.Errorf("Failed to save receipts: %v", err)
	}
}

// OnUndecryptableMessage logs undecryptable messages.
func (h *DataHandler) OnUndecryptableMessage(evt *events.UndecryptableMessage) {
	h.log.Warnf("Undecryptable message from %s in %s: %v", evt.Info.Sender, evt.Info.Chat, evt.DecryptFailMode)
}

// ===========================================================================
// CONTACT EVENTS
// ===========================================================================

// OnContact saves full contact information from app state sync.
func (h *DataHandler) OnContact(evt *events.Contact) {
	contact := extract.ContactFromEvent(evt)
	contact.LID = h.utils.NormalizeJID(h.ctx, contact.LID)

	if err := h.contacts.Put(contact); err != nil {
		h.log.Errorf("Failed to save contact: %v", err)
	}
}

// OnPushName updates contact push names.
func (h *DataHandler) OnPushName(evt *events.PushName) {
	contact := extract.ContactFromPushName(evt)
	contact.LID = h.utils.NormalizeJID(h.ctx, contact.LID)

	if err := h.contacts.Put(contact); err != nil {
		h.log.Errorf("Failed to update push name: %v", err)
	}
}

// OnBusinessName updates contact business names.
func (h *DataHandler) OnBusinessName(evt *events.BusinessName) {
	contact := extract.ContactFromBusinessName(evt)
	contact.LID = h.utils.NormalizeJID(h.ctx, contact.LID)

	if err := h.contacts.Put(contact); err != nil {
		h.log.Errorf("Failed to update business name: %v", err)
	}
}

// OnPresence updates contact presence.
func (h *DataHandler) OnPresence(evt *events.Presence) {
	jid := h.utils.NormalizeJID(h.ctx, evt.From)
	if err := h.contacts.UpdatePresence(jid, !evt.Unavailable, evt.LastSeen); err != nil {
		h.log.Errorf("Failed to update presence: %v", err)
	}
}

// OnChatPresence handles typing/recording indicators.
func (h *DataHandler) OnChatPresence(evt *events.ChatPresence) {
	// Chat presence (typing, recording) is ephemeral - we can log but not persist
	h.log.Debugf("Chat presence from %s in %s: %s %s", evt.Sender, evt.Chat, evt.State, evt.Media)
}

// OnPicture updates profile picture info.
func (h *DataHandler) OnPicture(evt *events.Picture) {
	jid := h.utils.NormalizeJID(h.ctx, evt.JID)

	switch evt.JID.Server {
	case types.DefaultUserServer, types.HiddenUserServer:
		if err := h.contacts.UpdateProfilePic(jid, evt.PictureID, ""); err != nil {
			h.log.Errorf("Failed to update contact picture: %v", err)
		}
	case types.GroupServer:
		if err := h.groups.UpdateProfilePic(jid, evt.PictureID, ""); err != nil {
			h.log.Errorf("Failed to update group picture: %v", err)
		}
	case types.NewsletterServer:
		if err := h.newsletters.UpdateProfilePic(jid, evt.PictureID, ""); err != nil {
			h.log.Errorf("Failed to update newsletter picture: %v", err)
		}
	}
}

// ===========================================================================
// GROUP EVENTS
// ===========================================================================

// OnGroupInfo updates group information.
func (h *DataHandler) OnGroupInfo(evt *events.GroupInfo) {
	group := extract.GroupFromEvent(evt)
	group.JID = h.utils.NormalizeJID(h.ctx, group.JID)
	group.NameSetByLID = h.utils.NormalizeJID(h.ctx, group.NameSetByLID)
	group.TopicSetByLID = h.utils.NormalizeJID(h.ctx, group.TopicSetByLID)
	group.OwnerLID = h.utils.NormalizeJID(h.ctx, group.OwnerLID)
	group.CreatedByLID = h.utils.NormalizeJID(h.ctx, group.CreatedByLID)

	if err := h.groups.Put(group); err != nil {
		h.log.Errorf("Failed to update group info: %v", err)
	}

	groupJID := h.utils.NormalizeJID(h.ctx, evt.JID)

	// Handle participant changes
	for _, jid := range evt.Join {
		normalizedJID := h.utils.NormalizeJID(h.ctx, jid)
		if err := h.groups.PutParticipant(&store.GroupParticipant{
			GroupJID:  groupJID,
			MemberLID: normalizedJID,
		}); err != nil {
			h.log.Errorf("Failed to add participant: %v", err)
		}
	}
	for _, jid := range evt.Leave {
		normalizedJID := h.utils.NormalizeJID(h.ctx, jid)
		if err := h.groups.RemoveParticipant(groupJID, normalizedJID); err != nil {
			h.log.Errorf("Failed to remove participant: %v", err)
		}
	}
	for _, jid := range evt.Promote {
		normalizedJID := h.utils.NormalizeJID(h.ctx, jid)
		if err := h.groups.PutParticipant(&store.GroupParticipant{
			GroupJID:  groupJID,
			MemberLID: normalizedJID,
			IsAdmin:   true,
		}); err != nil {
			h.log.Errorf("Failed to promote participant: %v", err)
		}
	}
	for _, jid := range evt.Demote {
		normalizedJID := h.utils.NormalizeJID(h.ctx, jid)
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
	group.JID = h.utils.NormalizeJID(h.ctx, group.JID)
	group.OwnerLID = h.utils.NormalizeJID(h.ctx, group.OwnerLID)
	group.NameSetByLID = h.utils.NormalizeJID(h.ctx, group.NameSetByLID)
	group.TopicSetByLID = h.utils.NormalizeJID(h.ctx, group.TopicSetByLID)

	// Normalize participant JIDs
	for i := range participants {
		participants[i].GroupJID = h.utils.NormalizeJID(h.ctx, participants[i].GroupJID)
		participants[i].MemberLID = h.utils.NormalizeJID(h.ctx, participants[i].MemberLID)
		participants[i].AddedByLID = h.utils.NormalizeJID(h.ctx, participants[i].AddedByLID)
	}

	if err := h.groups.Put(group); err != nil {
		h.log.Errorf("Failed to save group: %v", err)
	}

	// Clear and repopulate participants
	groupJID := h.utils.NormalizeJID(h.ctx, evt.JID)
	h.groups.ClearParticipants(groupJID)
	if err := h.groups.PutParticipants(participants); err != nil {
		h.log.Errorf("Failed to save participants: %v", err)
	}

	// Ensure chat exists
	h.chats.EnsureExists(groupJID, store.ChatTypeGroup)
}

// ===========================================================================
// NEWSLETTER EVENTS
// ===========================================================================

// OnNewsletterJoin saves newsletter data when joining.
func (h *DataHandler) OnNewsletterJoin(evt *events.NewsletterJoin) {
	newsletter := extract.NewsletterFromJoinEvent(evt)
	if newsletter == nil {
		return
	}

	// Newsletter JIDs don't need normalization
	if err := h.newsletters.Put(newsletter); err != nil {
		h.log.Errorf("Failed to save newsletter: %v", err)
	}

	// Ensure chat exists
	h.chats.EnsureExists(newsletter.JID, store.ChatTypeNewsletter)
	h.log.Infof("Joined newsletter: %s (%s)", newsletter.Name, newsletter.JID)
}

// OnNewsletterLeave handles leaving a newsletter.
func (h *DataHandler) OnNewsletterLeave(evt *events.NewsletterLeave) {
	if err := h.newsletters.SetRole(evt.ID, "left"); err != nil {
		h.log.Errorf("Failed to mark newsletter as left: %v", err)
	}
	h.log.Infof("Left newsletter: %s", evt.ID)
}

// OnNewsletterMuteChange updates newsletter mute status.
func (h *DataHandler) OnNewsletterMuteChange(evt *events.NewsletterMuteChange) {
	muted := evt.Mute == types.NewsletterMuteOn
	if err := h.newsletters.SetMuted(evt.ID, muted); err != nil {
		h.log.Errorf("Failed to update newsletter mute: %v", err)
	}
}

// OnNewsletterLiveUpdate handles live newsletter updates.
func (h *DataHandler) OnNewsletterLiveUpdate(evt *events.NewsletterLiveUpdate) {
	for _, msg := range evt.Messages {
		// Process each message in the newsletter update
		h.log.Debugf("Newsletter %s update: message %s", evt.JID, msg.MessageServerID)
	}
}

// ===========================================================================
// CHAT STATE EVENTS
// ===========================================================================

// OnPinChat updates chat pin status.
func (h *DataHandler) OnPinChat(evt *events.Pin) {
	jid, pinned, ts := extract.ChatStateFromPin(evt)
	jid = h.utils.NormalizeJID(h.ctx, jid)
	if err := h.chats.SetPinned(jid, pinned, ts); err != nil {
		h.log.Errorf("Failed to update pin status: %v", err)
	}
}

// OnMuteChat updates chat mute status.
func (h *DataHandler) OnMuteChat(evt *events.Mute) {
	jid, mutedUntil := extract.ChatStateFromMute(evt)
	jid = h.utils.NormalizeJID(h.ctx, jid)
	if err := h.chats.SetMuted(jid, mutedUntil); err != nil {
		h.log.Errorf("Failed to update mute status: %v", err)
	}
}

// OnArchiveChat updates chat archive status.
func (h *DataHandler) OnArchiveChat(evt *events.Archive) {
	jid, archived := extract.ChatStateFromArchive(evt)
	jid = h.utils.NormalizeJID(h.ctx, jid)
	if err := h.chats.SetArchived(jid, archived); err != nil {
		h.log.Errorf("Failed to update archive status: %v", err)
	}
}

// OnMarkChatAsRead marks chat as read.
func (h *DataHandler) OnMarkChatAsRead(evt *events.MarkChatAsRead) {
	jid := h.utils.NormalizeJID(h.ctx, evt.JID)
	if err := h.chats.MarkRead(jid); err != nil {
		h.log.Errorf("Failed to mark chat as read: %v", err)
	}
}

// OnStarMessage updates message starred status.
func (h *DataHandler) OnStarMessage(evt *events.Star) {
	chatJID := h.utils.NormalizeJID(h.ctx, evt.ChatJID)
	starred := evt.Action.GetStarred()
	if err := h.messages.SetStarred(evt.MessageID, chatJID, starred); err != nil {
		h.log.Errorf("Failed to update star status: %v", err)
	}
}

// OnDeleteForMe marks message as deleted.
func (h *DataHandler) OnDeleteForMe(evt *events.DeleteForMe) {
	chatJID := h.utils.NormalizeJID(h.ctx, evt.ChatJID)
	if err := h.messages.Delete(evt.MessageID, chatJID); err != nil {
		h.log.Errorf("Failed to delete message: %v", err)
	}
}

// OnClearChat clears all messages from a chat.
func (h *DataHandler) OnClearChat(evt *events.ClearChat) {
	jid := h.utils.NormalizeJID(h.ctx, evt.JID)
	if err := h.chats.Clear(jid); err != nil {
		h.log.Errorf("Failed to clear chat: %v", err)
	}
}

// OnDeleteChat deletes a chat entirely.
func (h *DataHandler) OnDeleteChat(evt *events.DeleteChat) {
	jid := h.utils.NormalizeJID(h.ctx, evt.JID)
	if err := h.chats.Delete(jid); err != nil {
		h.log.Errorf("Failed to delete chat: %v", err)
	}
}

// ===========================================================================
// LABEL EVENTS
// ===========================================================================

// OnLabelEdit handles label creation/update/deletion.
func (h *DataHandler) OnLabelEdit(evt *events.LabelEdit) {
	if h.labels == nil {
		return
	}

	label := extract.LabelFromEvent(evt)
	if err := h.labels.Put(label); err != nil {
		h.log.Errorf("Failed to save label: %v", err)
	}
}

// OnLabelAssociationChat handles label assignment to chats.
func (h *DataHandler) OnLabelAssociationChat(evt *events.LabelAssociationChat) {
	if h.labels == nil {
		return
	}

	jid := h.utils.NormalizeJID(h.ctx, evt.JID)
	if evt.Action.GetLabeled() {
		if err := h.labels.AssociateChat(evt.LabelID, jid, evt.Timestamp); err != nil {
			h.log.Errorf("Failed to associate label with chat: %v", err)
		}
	} else {
		if err := h.labels.RemoveChatAssociation(evt.LabelID, jid); err != nil {
			h.log.Errorf("Failed to remove label from chat: %v", err)
		}
	}
}

// OnLabelAssociationMessage handles label assignment to messages.
func (h *DataHandler) OnLabelAssociationMessage(evt *events.LabelAssociationMessage) {
	if h.labels == nil {
		return
	}

	jid := h.utils.NormalizeJID(h.ctx, evt.JID)
	if evt.Action.GetLabeled() {
		if err := h.labels.AssociateMessage(evt.LabelID, jid, evt.MessageID, evt.Timestamp); err != nil {
			h.log.Errorf("Failed to associate label with message: %v", err)
		}
	} else {
		if err := h.labels.RemoveMessageAssociation(evt.LabelID, jid, evt.MessageID); err != nil {
			h.log.Errorf("Failed to remove label from message: %v", err)
		}
	}
}

// ===========================================================================
// PRIVACY EVENTS
// ===========================================================================

// OnBlocklist handles blocklist updates.
func (h *DataHandler) OnBlocklist(evt *events.Blocklist) {
	if h.blocklist == nil {
		return
	}

	blocked, unblocked := extract.BlocklistFromEvent(evt)

	for _, jid := range blocked {
		normalizedJID := h.utils.NormalizeJID(h.ctx, jid)
		if err := h.blocklist.Block(normalizedJID); err != nil {
			h.log.Errorf("Failed to block JID: %v", err)
		}
	}

	for _, jid := range unblocked {
		normalizedJID := h.utils.NormalizeJID(h.ctx, jid)
		if err := h.blocklist.Unblock(normalizedJID); err != nil {
			h.log.Errorf("Failed to unblock JID: %v", err)
		}
	}

	h.log.Infof("Blocklist updated: %d blocked, %d unblocked", len(blocked), len(unblocked))
}

// OnPrivacySettings handles privacy settings changes.
func (h *DataHandler) OnPrivacySettings(evt *events.PrivacySettings) {
	if h.privacy == nil {
		return
	}

	settings := extract.PrivacySettingsFromEvent(evt)
	if err := h.privacy.Put(settings); err != nil {
		h.log.Errorf("Failed to save privacy settings: %v", err)
	}
}

// ===========================================================================
// CALL EVENTS
// ===========================================================================

// OnCallOffer handles incoming call offers.
func (h *DataHandler) OnCallOffer(evt *events.CallOffer) {
	call := extract.CallFromOffer(evt)
	call.CallerLID = h.utils.NormalizeJID(h.ctx, call.CallerLID)
	call.GroupJID = h.utils.NormalizeJID(h.ctx, call.GroupJID)

	if err := h.calls.Put(call); err != nil {
		h.log.Errorf("Failed to save call offer: %v", err)
	}
}

// OnCallAccept handles call acceptance.
func (h *DataHandler) OnCallAccept(evt *events.CallAccept) {
	if err := h.calls.UpdateOutcome(evt.CallID, "accepted", 0); err != nil {
		h.log.Errorf("Failed to update call accept: %v", err)
	}
}

// OnCallTerminate handles call termination.
func (h *DataHandler) OnCallTerminate(evt *events.CallTerminate) {
	callID, outcome := extract.CallFromTerminate(evt)
	if err := h.calls.UpdateOutcome(callID, outcome, 0); err != nil {
		h.log.Errorf("Failed to update call outcome: %v", err)
	}
}

// ===========================================================================
// IDENTITY EVENTS
// ===========================================================================

// OnIdentityChange handles identity key changes.
func (h *DataHandler) OnIdentityChange(evt *events.IdentityChange) {
	jid := h.utils.NormalizeJID(h.ctx, evt.JID)
	h.log.Infof("Identity changed for %s (implicit: %v)", jid, evt.Implicit)
	// Identity changes are logged for security awareness
	// Could be extended to store in a security_events table
}

// ===========================================================================
// HISTORY SYNC
// ===========================================================================

// OnHistorySync processes history sync data.
func (h *DataHandler) OnHistorySync(evt *events.HistorySync) {
	data := extract.FromHistorySync(evt)

	// Save JID mappings
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
		contact.LID = h.utils.NormalizeJID(h.ctx, contact.LID)
		if err := h.contacts.Put(contact); err != nil {
			h.log.Errorf("Failed to save contact: %v", err)
		}
	}

	// Save chats - normalize JIDs
	for _, chat := range data.Chats {
		chat.JID = h.utils.NormalizeJID(h.ctx, chat.JID)
		if err := h.chats.Put(chat); err != nil {
			h.log.Errorf("Failed to save chat: %v", err)
		}
	}

	// Save groups - normalize JIDs
	for _, group := range data.Groups {
		group.JID = h.utils.NormalizeJID(h.ctx, group.JID)
		group.OwnerLID = h.utils.NormalizeJID(h.ctx, group.OwnerLID)
		if err := h.groups.Put(group); err != nil {
			h.log.Errorf("Failed to save group: %v", err)
		}
	}

	// Save participants - normalize JIDs
	for i := range data.Participants {
		data.Participants[i].GroupJID = h.utils.NormalizeJID(h.ctx, data.Participants[i].GroupJID)
		data.Participants[i].MemberLID = h.utils.NormalizeJID(h.ctx, data.Participants[i].MemberLID)
		if err := h.groups.PutParticipant(&data.Participants[i]); err != nil {
			h.log.Errorf("Failed to save participant: %v", err)
		}
	}

	// Save messages - normalize JIDs, infer sender if missing
	savedMsgs := 0
	for _, msg := range data.Messages {
		msg.ChatJID = h.utils.NormalizeJID(h.ctx, msg.ChatJID)
		msg.SenderLID = h.utils.NormalizeJID(h.ctx, msg.SenderLID)
		msg.QuotedSenderLID = h.utils.NormalizeJID(h.ctx, msg.QuotedSenderLID)

		// Infer sender if missing
		if msg.SenderLID.IsEmpty() {
			if msg.FromMe {
				msg.SenderLID = h.utils.OwnJID()
			} else if msg.ChatJID.Server == "s.whatsapp.net" || msg.ChatJID.Server == "lid" {
				// DM chat - sender is the chat JID
				msg.SenderLID = msg.ChatJID
			} else {
				// Group without sender - skip
				continue
			}
		}

		if err := h.messages.Put(msg); err != nil {
			h.log.Errorf("Failed to save message: %v", err)
		} else {
			savedMsgs++
		}
	}

	h.log.Infof("History sync: saved %d chats, %d groups, %d contacts, %d messages",
		len(data.Chats), len(data.Groups), len(data.Contacts), savedMsgs)
}

// OnAppStateSyncComplete handles app state sync completion.
func (h *DataHandler) OnAppStateSyncComplete(evt *events.AppStateSyncComplete) {
	h.log.Infof("App state sync complete: %s", evt.Name)
}

// OnOfflineSyncPreview handles offline sync preview.
func (h *DataHandler) OnOfflineSyncPreview(evt *events.OfflineSyncPreview) {
	h.log.Infof("Offline sync preview: %d total events, %d messages", evt.Total, evt.Messages)
}

// OnOfflineSyncCompleted handles offline sync completion.
func (h *DataHandler) OnOfflineSyncCompleted(evt *events.OfflineSyncCompleted) {
	h.log.Infof("Offline sync completed: %d events processed", evt.Count)
}
