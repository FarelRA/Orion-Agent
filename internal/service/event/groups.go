package event

import (
	"go.mau.fi/whatsmeow/types/events"

	"orion-agent/internal/data/extract"
	"orion-agent/internal/data/store"
)

// OnGroupInfo updates group information.
func (h *EventService) OnGroupInfo(evt *events.GroupInfo) {
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
func (h *EventService) OnJoinedGroup(evt *events.JoinedGroup) {
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
