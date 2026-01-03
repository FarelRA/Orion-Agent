package event

import (
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"orion-agent/internal/data/extract"
)

// OnContact saves full contact information from app state sync.
func (h *EventService) OnContact(evt *events.Contact) {
	contact := extract.ContactFromEvent(evt)
	contact.LID = h.utils.NormalizeJID(h.ctx, contact.LID)

	if err := h.contacts.Put(contact); err != nil {
		h.log.Errorf("Failed to save contact: %v", err)
	}
}

// OnPushName updates contact push names.
func (h *EventService) OnPushName(evt *events.PushName) {
	contact := extract.ContactFromPushName(evt)
	contact.LID = h.utils.NormalizeJID(h.ctx, contact.LID)

	if err := h.contacts.Put(contact); err != nil {
		h.log.Errorf("Failed to update push name: %v", err)
	}
}

// OnBusinessName updates contact business names.
func (h *EventService) OnBusinessName(evt *events.BusinessName) {
	contact := extract.ContactFromBusinessName(evt)
	contact.LID = h.utils.NormalizeJID(h.ctx, contact.LID)

	if err := h.contacts.Put(contact); err != nil {
		h.log.Errorf("Failed to update business name: %v", err)
	}
}

// OnPresence updates contact presence.
func (h *EventService) OnPresence(evt *events.Presence) {
	jid := h.utils.NormalizeJID(h.ctx, evt.From)
	if err := h.contacts.UpdatePresence(jid, !evt.Unavailable, evt.LastSeen); err != nil {
		h.log.Errorf("Failed to update presence: %v", err)
	}
}

// OnChatPresence handles typing/recording indicators.
func (h *EventService) OnChatPresence(evt *events.ChatPresence) {
	// Chat presence (typing, recording) is ephemeral - we can log but not persist
	h.log.Debugf("Chat presence from %s in %s: %s %s", evt.Sender, evt.Chat, evt.State, evt.Media)
}

// OnPicture updates profile picture info.
func (h *EventService) OnPicture(evt *events.Picture) {
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
