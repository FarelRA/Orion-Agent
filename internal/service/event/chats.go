package event

import (
	"go.mau.fi/whatsmeow/types/events"

	"orion-agent/internal/data/extract"
)

// OnPinChat updates chat pin status.
func (h *EventService) OnPinChat(evt *events.Pin) {
	jid, pinned, ts := extract.ChatStateFromPin(evt)
	jid = h.utils.NormalizeJID(h.ctx, jid)
	if err := h.chats.SetPinned(jid, pinned, ts); err != nil {
		h.log.Errorf("Failed to update pin status: %v", err)
	}
}

// OnMuteChat updates chat mute status.
func (h *EventService) OnMuteChat(evt *events.Mute) {
	jid, mutedUntil := extract.ChatStateFromMute(evt)
	jid = h.utils.NormalizeJID(h.ctx, jid)
	if err := h.chats.SetMuted(jid, mutedUntil); err != nil {
		h.log.Errorf("Failed to update mute status: %v", err)
	}
}

// OnArchiveChat updates chat archive status.
func (h *EventService) OnArchiveChat(evt *events.Archive) {
	jid, archived := extract.ChatStateFromArchive(evt)
	jid = h.utils.NormalizeJID(h.ctx, jid)
	if err := h.chats.SetArchived(jid, archived); err != nil {
		h.log.Errorf("Failed to update archive status: %v", err)
	}
}

// OnMarkChatAsRead marks chat as read.
func (h *EventService) OnMarkChatAsRead(evt *events.MarkChatAsRead) {
	jid := h.utils.NormalizeJID(h.ctx, evt.JID)
	if err := h.chats.MarkRead(jid); err != nil {
		h.log.Errorf("Failed to mark chat as read: %v", err)
	}
}

// OnStarMessage updates message starred status.
func (h *EventService) OnStarMessage(evt *events.Star) {
	chatJID := h.utils.NormalizeJID(h.ctx, evt.ChatJID)
	starred := evt.Action.GetStarred()
	if err := h.messages.SetStarred(evt.MessageID, chatJID, starred); err != nil {
		h.log.Errorf("Failed to update star status: %v", err)
	}
}

// OnDeleteForMe marks message as deleted.
func (h *EventService) OnDeleteForMe(evt *events.DeleteForMe) {
	chatJID := h.utils.NormalizeJID(h.ctx, evt.ChatJID)
	if err := h.messages.Delete(evt.MessageID, chatJID); err != nil {
		h.log.Errorf("Failed to delete message: %v", err)
	}
}

// OnClearChat clears all messages from a chat.
func (h *EventService) OnClearChat(evt *events.ClearChat) {
	jid := h.utils.NormalizeJID(h.ctx, evt.JID)
	if err := h.chats.Clear(jid); err != nil {
		h.log.Errorf("Failed to clear chat: %v", err)
	}
}

// OnDeleteChat deletes a chat entirely.
func (h *EventService) OnDeleteChat(evt *events.DeleteChat) {
	jid := h.utils.NormalizeJID(h.ctx, evt.JID)
	if err := h.chats.Delete(jid); err != nil {
		h.log.Errorf("Failed to delete chat: %v", err)
	}
}
