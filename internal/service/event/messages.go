package event

import (
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"orion-agent/internal/data/extract"
	"orion-agent/internal/data/store"
)

// OnMessage saves incoming messages to the database.
func (h *EventService) OnMessage(evt *events.Message) {
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

	// Process through agent (side-by-side with save)
	if h.agent != nil {
		go h.agent.HandleMessage(h.ctx, msg)
	}

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

		// Queue media download
		if h.media != nil {
			h.media.QueueMessageMedia(msg)
		}
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
func (h *EventService) handleReaction(evt *events.Message) {
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
func (h *EventService) handleProtocolMessage(evt *events.Message) {
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
func (h *EventService) handlePollUpdate(evt *events.Message) {
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
func (h *EventService) savePollCreation(msg *store.Message, chatJID, creatorLID types.JID) {
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
func (h *EventService) handlePinMessage(evt *events.Message) {
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
func (h *EventService) handleKeepMessage(evt *events.Message) {
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
func (h *EventService) OnReceipt(evt *events.Receipt) {
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
func (h *EventService) OnUndecryptableMessage(evt *events.UndecryptableMessage) {
	h.log.Warnf("Undecryptable message from %s in %s: %v", evt.Info.Sender, evt.Info.Chat, evt.DecryptFailMode)
}
