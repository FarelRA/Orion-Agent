package event

import (
	"go.mau.fi/whatsmeow/types/events"

	"orion-agent/internal/data/extract"
)

// OnCallOffer handles incoming call offers.
func (h *EventService) OnCallOffer(evt *events.CallOffer) {
	call := extract.CallFromOffer(evt)
	call.CallerLID = h.utils.NormalizeJID(h.ctx, call.CallerLID)
	call.GroupJID = h.utils.NormalizeJID(h.ctx, call.GroupJID)

	if err := h.calls.Put(call); err != nil {
		h.log.Errorf("Failed to save call offer: %v", err)
	}
}

// OnCallAccept handles call acceptance.
func (h *EventService) OnCallAccept(evt *events.CallAccept) {
	if err := h.calls.UpdateOutcome(evt.CallID, "accepted", 0); err != nil {
		h.log.Errorf("Failed to update call accept: %v", err)
	}
}

// OnCallTerminate handles call termination.
func (h *EventService) OnCallTerminate(evt *events.CallTerminate) {
	callID, outcome := extract.CallFromTerminate(evt)
	if err := h.calls.UpdateOutcome(callID, outcome, 0); err != nil {
		h.log.Errorf("Failed to update call outcome: %v", err)
	}
}
