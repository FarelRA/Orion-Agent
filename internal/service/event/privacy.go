package event

import (
	"go.mau.fi/whatsmeow/types/events"

	"orion-agent/internal/data/extract"
)

// OnBlocklist handles blocklist updates.
func (h *EventService) OnBlocklist(evt *events.Blocklist) {
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
func (h *EventService) OnPrivacySettings(evt *events.PrivacySettings) {
	if h.privacy == nil {
		return
	}

	settings := extract.PrivacySettingsFromEvent(evt)
	if err := h.privacy.Put(settings); err != nil {
		h.log.Errorf("Failed to save privacy settings: %v", err)
	}
}
