package event

import (
	"go.mau.fi/whatsmeow/types/events"

	"orion-agent/internal/data/extract"
)

// OnLabelEdit handles label creation/update/deletion.
func (h *EventService) OnLabelEdit(evt *events.LabelEdit) {
	if h.labels == nil {
		return
	}

	label := extract.LabelFromEvent(evt)
	if err := h.labels.Put(label); err != nil {
		h.log.Errorf("Failed to save label: %v", err)
	}
}

// OnLabelAssociationChat handles label assignment to chats.
func (h *EventService) OnLabelAssociationChat(evt *events.LabelAssociationChat) {
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
func (h *EventService) OnLabelAssociationMessage(evt *events.LabelAssociationMessage) {
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
