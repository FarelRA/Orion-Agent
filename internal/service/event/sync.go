package event

import (
	"go.mau.fi/whatsmeow/types/events"

	"orion-agent/internal/data/extract"
	"orion-agent/internal/data/store"
)

// OnIdentityChange handles identity key changes.
func (h *EventService) OnIdentityChange(evt *events.IdentityChange) {
	jid := h.utils.NormalizeJID(h.ctx, evt.JID)
	h.log.Infof("Identity changed for %s (implicit: %v)", jid, evt.Implicit)
	// Identity changes are logged for security awareness
	// Could be extended to store in a security_events table
}

// OnHistorySync processes history sync data.
func (h *EventService) OnHistorySync(evt *events.HistorySync) {
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
func (h *EventService) OnAppStateSyncComplete(evt *events.AppStateSyncComplete) {
	h.log.Infof("App state sync complete: %s", evt.Name)
}

// OnOfflineSyncPreview handles offline sync preview.
func (h *EventService) OnOfflineSyncPreview(evt *events.OfflineSyncPreview) {
	h.log.Infof("Offline sync preview: %d total events, %d messages", evt.Total, evt.Messages)
}

// OnOfflineSyncCompleted handles offline sync completion.
func (h *EventService) OnOfflineSyncCompleted(evt *events.OfflineSyncCompleted) {
	h.log.Infof("Offline sync completed: %d events processed", evt.Count)
}
