package event

import (
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"orion-agent/internal/data/extract"
	"orion-agent/internal/data/store"
)

// OnNewsletterJoin saves newsletter data when joining.
func (h *EventService) OnNewsletterJoin(evt *events.NewsletterJoin) {
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
func (h *EventService) OnNewsletterLeave(evt *events.NewsletterLeave) {
	if err := h.newsletters.SetRole(evt.ID, "left"); err != nil {
		h.log.Errorf("Failed to mark newsletter as left: %v", err)
	}
	h.log.Infof("Left newsletter: %s", evt.ID)
}

// OnNewsletterMuteChange updates newsletter mute status.
func (h *EventService) OnNewsletterMuteChange(evt *events.NewsletterMuteChange) {
	muted := evt.Mute == types.NewsletterMuteOn
	if err := h.newsletters.SetMuted(evt.ID, muted); err != nil {
		h.log.Errorf("Failed to update newsletter mute: %v", err)
	}
}

// OnNewsletterLiveUpdate handles live newsletter updates.
func (h *EventService) OnNewsletterLiveUpdate(evt *events.NewsletterLiveUpdate) {
	for _, msg := range evt.Messages {
		// Process each message in the newsletter update
		h.log.Debugf("Newsletter %s update: message %s", evt.JID, msg.MessageServerID)
	}
}
