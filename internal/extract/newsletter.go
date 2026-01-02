package extract

import (
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"orion-agent/internal/store"
)

// NewsletterFromJoinEvent extracts newsletter data from a NewsletterJoin event.
func NewsletterFromJoinEvent(evt *events.NewsletterJoin) *store.Newsletter {
	return NewsletterFromMetadata(&evt.NewsletterMetadata)
}

// NewsletterFromMetadata extracts newsletter data from NewsletterMetadata.
func NewsletterFromMetadata(meta *types.NewsletterMetadata) *store.Newsletter {
	if meta == nil {
		return nil
	}

	n := &store.Newsletter{
		JID:               meta.ID,
		Name:              meta.ThreadMeta.Name.Text,
		Description:       meta.ThreadMeta.Description.Text,
		SubscriberCount:   int64(meta.ThreadMeta.SubscriberCount),
		VerificationState: string(meta.ThreadMeta.VerificationState),
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	// Extract invite code if available
	if meta.ThreadMeta.InviteCode != "" {
		n.InviteCode = meta.ThreadMeta.InviteCode
		n.InviteLink = "https://whatsapp.com/channel/" + meta.ThreadMeta.InviteCode
	}

	// Extract picture info
	if meta.ThreadMeta.Picture != nil {
		n.PictureID = meta.ThreadMeta.Picture.ID
		n.PictureURL = meta.ThreadMeta.Picture.URL
	}
	n.PreviewURL = meta.ThreadMeta.Preview.URL

	// Extract viewer metadata (user's relationship with the newsletter)
	if meta.ViewerMeta != nil {
		n.Role = string(meta.ViewerMeta.Role)
		n.Muted = meta.ViewerMeta.Mute == types.NewsletterMuteOn
	}

	// Extract creation timestamp
	if !meta.ThreadMeta.CreationTime.IsZero() {
		n.CreatedAt = meta.ThreadMeta.CreationTime.Time
	}

	return n
}

// ContactFromEvent extracts full contact data from events.Contact.
// The Contact event contains a ContactAction with the actual data.
func ContactFromEvent(evt *events.Contact) *store.Contact {
	contact := &store.Contact{
		LID:       evt.JID,
		CreatedAt: time.Now(),
		UpdatedAt: evt.Timestamp,
	}

	// Extract data from the ContactAction
	if evt.Action != nil {
		contact.FullName = evt.Action.GetFullName()
		contact.FirstName = evt.Action.GetFirstName()
	}

	return contact
}

// BlocklistFromEvent extracts blocked JIDs from events.Blocklist.
func BlocklistFromEvent(evt *events.Blocklist) (blocked []types.JID, unblocked []types.JID) {
	for _, change := range evt.Changes {
		if change.Action == events.BlocklistChangeActionBlock {
			blocked = append(blocked, change.JID)
		} else if change.Action == events.BlocklistChangeActionUnblock {
			unblocked = append(unblocked, change.JID)
		}
	}
	return
}

// PrivacySettingsFromEvent extracts privacy settings from events.PrivacySettings.
func PrivacySettingsFromEvent(evt *events.PrivacySettings) *store.PrivacySettings {
	settings := &store.PrivacySettings{
		UpdatedAt: time.Now(),
	}

	// The NewSettings is a types.PrivacySettings struct, not a slice
	settings.GroupAdd = string(evt.NewSettings.GroupAdd)
	settings.LastSeen = string(evt.NewSettings.LastSeen)
	settings.Status = string(evt.NewSettings.Status)
	settings.Profile = string(evt.NewSettings.Profile)
	settings.ReadReceipts = string(evt.NewSettings.ReadReceipts)
	settings.Online = string(evt.NewSettings.Online)
	settings.CallAdd = string(evt.NewSettings.CallAdd)

	return settings
}

// LabelFromEvent extracts label data from events.LabelEdit.
func LabelFromEvent(evt *events.LabelEdit) *store.Label {
	label := &store.Label{
		ID:        evt.LabelID,
		CreatedAt: evt.Timestamp,
		UpdatedAt: evt.Timestamp,
	}

	// Extract data from the LabelEditAction
	if evt.Action != nil {
		label.Name = evt.Action.GetName()
		label.Color = int(evt.Action.GetColor())
		label.PredefinedID = int(evt.Action.GetPredefinedID())
		label.Deleted = evt.Action.GetDeleted()
	}

	return label
}

// LabelAssociationFromChatEvent extracts label association from events.LabelAssociationChat.
func LabelAssociationFromChatEvent(evt *events.LabelAssociationChat) *store.LabelAssociation {
	return &store.LabelAssociation{
		LabelID:    evt.LabelID,
		TargetType: "chat",
		TargetJID:  evt.JID,
		Timestamp:  evt.Timestamp,
	}
}

// LabelAssociationFromMessageEvent extracts label association from events.LabelAssociationMessage.
func LabelAssociationFromMessageEvent(evt *events.LabelAssociationMessage) *store.LabelAssociation {
	return &store.LabelAssociation{
		LabelID:    evt.LabelID,
		TargetType: "message",
		TargetJID:  evt.JID,
		MessageID:  evt.MessageID,
		Timestamp:  evt.Timestamp,
	}
}

// CallFromOffer extracts call data from events.CallOffer.
func CallFromOffer(evt *events.CallOffer) *store.Call {
	call := &store.Call{
		CallID:    evt.CallID,
		CallerLID: evt.CallCreator,
		GroupJID:  evt.GroupJID,
		IsGroup:   !evt.GroupJID.IsEmpty(),
		Timestamp: evt.Timestamp,
		Outcome:   "pending",
	}

	// Video call detection based on Data is not available in all versions
	// The IsVideo field may need to come from other sources

	return call
}

// CallFromAccept extracts call accept info from events.CallAccept.
func CallFromAccept(evt *events.CallAccept) (callID string, accepted bool) {
	return evt.CallID, true
}

// CallFromTerminate extracts termination info from events.CallTerminate.
func CallFromTerminate(evt *events.CallTerminate) (callID string, outcome string) {
	callID = evt.CallID
	switch evt.Reason {
	case "timeout":
		outcome = "missed"
	case "busy":
		outcome = "busy"
	case "reject":
		outcome = "rejected"
	case "":
		outcome = "ended"
	default:
		outcome = evt.Reason
	}
	return
}

// PollUpdateFromEvent extracts poll vote data from a poll update message.
func PollUpdateFromEvent(evt *events.Message) *store.PollVote {
	pollUpdate := evt.Message.GetPollUpdateMessage()
	if pollUpdate == nil {
		return nil
	}

	// The poll update message references the original poll
	pollKey := pollUpdate.GetPollCreationMessageKey()
	if pollKey == nil {
		return nil
	}

	targetChat, _ := types.ParseJID(pollKey.GetRemoteJID())

	return &store.PollVote{
		MessageID: pollKey.GetID(),
		ChatJID:   targetChat,
		VoterLID:  evt.Info.Sender,
		Timestamp: evt.Info.Timestamp,
		// Selected options are encrypted, need to decrypt with poll key
	}
}
