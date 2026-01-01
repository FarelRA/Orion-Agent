package extract

import (
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"orion-agent/internal/store"
)

// GroupFromEvent extracts a store.Group from events.GroupInfo.
// Note: events.GroupInfo contains partial updates, not full group info.
func GroupFromEvent(evt *events.GroupInfo) *store.Group {
	g := &store.Group{
		JID:       evt.JID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Name change
	if evt.Name != nil {
		g.Name = evt.Name.Name
		g.NameSetAt = evt.Name.NameSetAt
		g.NameSetByLID = evt.Name.NameSetBy
	}

	// Topic change
	if evt.Topic != nil {
		g.Topic = evt.Topic.Topic
		g.TopicID = evt.Topic.TopicID
		g.TopicSetAt = evt.Topic.TopicSetAt
		g.TopicSetByLID = evt.Topic.TopicSetBy
	}

	// Settings
	if evt.Announce != nil {
		g.IsAnnounce = evt.Announce.IsAnnounce
	}
	if evt.Locked != nil {
		g.IsLocked = evt.Locked.IsLocked
	}
	if evt.Ephemeral != nil {
		g.EphemeralDuration = evt.Ephemeral.DisappearingTimer
	}

	return g
}

// GroupFromJoinedEvent extracts a store.Group from events.JoinedGroup.
// JoinedGroup embeds types.GroupInfo which has the full group data.
func GroupFromJoinedEvent(evt *events.JoinedGroup) (*store.Group, []store.GroupParticipant) {
	// JoinedGroup embeds types.GroupInfo
	g := &store.Group{
		JID:               evt.JID,
		Name:              evt.Name,       // From embedded GroupName
		NameSetAt:         evt.NameSetAt,  // From embedded GroupName
		Topic:             evt.Topic,      // From embedded GroupTopic
		TopicID:           evt.TopicID,    // From embedded GroupTopic
		TopicSetAt:        evt.TopicSetAt, // From embedded GroupTopic
		TopicSetByLID:     evt.TopicSetBy, // From embedded GroupTopic
		OwnerLID:          evt.OwnerJID,
		IsAnnounce:        evt.IsAnnounce,        // From embedded GroupAnnounce
		IsLocked:          evt.IsLocked,          // From embedded GroupLocked
		IsIncognito:       evt.IsIncognito,       // From embedded GroupIncognito
		IsParentGroup:     evt.IsParent,          // From embedded GroupParent
		IsDefaultSubgroup: evt.IsDefaultSubGroup, // From embedded GroupIsDefaultSub
		LinkedParentJID:   evt.LinkedParentJID,   // From embedded GroupLinkedParent
		ParticipantCount:  len(evt.Participants),
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	if !evt.GroupCreated.IsZero() {
		g.CreatedAtWA = evt.GroupCreated
	}

	// MemberAddMode
	if evt.MemberAddMode != "" {
		g.MemberAddMode = string(evt.MemberAddMode)
	}

	// Extract participants
	participants := make([]store.GroupParticipant, 0, len(evt.Participants))
	for _, p := range evt.Participants {
		participants = append(participants, store.GroupParticipant{
			GroupJID:     evt.JID,
			MemberLID:    p.JID,
			IsAdmin:      p.IsAdmin || p.IsSuperAdmin,
			IsSuperAdmin: p.IsSuperAdmin,
			DisplayName:  p.DisplayName,
			ErrorCode:    int(p.Error),
		})
	}

	return g, participants
}

// ContactFromPushName creates a minimal contact from a push name event.
func ContactFromPushName(evt *events.PushName) *store.Contact {
	return &store.Contact{
		LID:       evt.JID,
		PushName:  evt.NewPushName,
		UpdatedAt: time.Now(),
		CreatedAt: time.Now(),
	}
}

// ContactFromBusinessName creates a minimal contact from a business name event.
func ContactFromBusinessName(evt *events.BusinessName) *store.Contact {
	return &store.Contact{
		LID:          evt.JID,
		BusinessName: evt.NewBusinessName,
		IsBusiness:   true,
		UpdatedAt:    time.Now(),
		CreatedAt:    time.Now(),
	}
}

// ReceiptFromEvent extracts receipt data from events.Receipt.
func ReceiptFromEvent(evt *events.Receipt) []store.Receipt {
	receipts := make([]store.Receipt, 0, len(evt.MessageIDs))

	for _, msgID := range evt.MessageIDs {
		receipts = append(receipts, store.Receipt{
			MessageID:    msgID,
			ChatJID:      evt.Chat,
			RecipientLID: evt.Sender,
			ReceiptType:  string(evt.Type),
			Timestamp:    evt.Timestamp,
		})
	}

	return receipts
}

// ChatStateFromPin extracts pin state from events.Pin.
func ChatStateFromPin(evt *events.Pin) (types.JID, bool, time.Time) {
	return evt.JID, evt.Action.GetPinned(), evt.Timestamp
}

// ChatStateFromMute extracts mute state from events.Mute.
func ChatStateFromMute(evt *events.Mute) (types.JID, time.Time) {
	if evt.Action.GetMuted() {
		muteEnd := time.Unix(evt.Action.GetMuteEndTimestamp(), 0)
		return evt.JID, muteEnd
	}
	return evt.JID, time.Time{}
}

// ChatStateFromArchive extracts archive state from events.Archive.
func ChatStateFromArchive(evt *events.Archive) (types.JID, bool) {
	return evt.JID, evt.Action.GetArchived()
}
