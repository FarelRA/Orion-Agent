package extract

import (
	"time"

	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"orion-agent/internal/data/store"
)

// HistorySyncData contains all extracted data from a history sync event.
type HistorySyncData struct {
	Chats            []*store.Chat
	Messages         []*store.Message
	Contacts         []*store.Contact
	Groups           []*store.Group
	Participants     []store.GroupParticipant
	PastParticipants []store.PastParticipant
	JIDMappings      []JIDMapping
}

// JIDMapping represents a PN to LID mapping.
type JIDMapping struct {
	PN  types.JID
	LID types.JID
}

// FromHistorySync extracts all data from a history sync event.
func FromHistorySync(evt *events.HistorySync) *HistorySyncData {
	data := &HistorySyncData{}
	hs := evt.Data

	// Extract JID mappings
	if mappings := hs.GetPhoneNumberToLidMappings(); len(mappings) > 0 {
		for _, m := range mappings {
			pnJID, _ := types.ParseJID(m.GetPnJID())
			lidJID, _ := types.ParseJID(m.GetLidJID())
			if !pnJID.IsEmpty() && !lidJID.IsEmpty() {
				data.JIDMappings = append(data.JIDMappings, JIDMapping{PN: pnJID, LID: lidJID})
			}
		}
	}

	// Extract contacts from pushnames
	if pushnames := hs.GetPushnames(); len(pushnames) > 0 {
		for _, pn := range pushnames {
			if pn.GetID() != "" {
				jid, _ := types.ParseJID(pn.GetID())
				data.Contacts = append(data.Contacts, &store.Contact{
					LID:       jid,
					PushName:  pn.GetPushname(),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				})
			}
		}
	}

	// Extract conversations (chats/groups with messages)
	if convs := hs.GetConversations(); len(convs) > 0 {
		for _, conv := range convs {
			chat, group, participants, pastParticipants, messages := extractConversation(conv)
			if chat != nil {
				data.Chats = append(data.Chats, chat)
			}
			if group != nil {
				data.Groups = append(data.Groups, group)
			}
			data.Participants = append(data.Participants, participants...)
			data.PastParticipants = append(data.PastParticipants, pastParticipants...)
			data.Messages = append(data.Messages, messages...)
		}
	}

	return data
}

func extractConversation(conv *waHistorySync.Conversation) (
	*store.Chat, *store.Group, []store.GroupParticipant, []store.PastParticipant, []*store.Message,
) {
	jid, _ := types.ParseJID(conv.GetID())
	if jid.IsEmpty() {
		return nil, nil, nil, nil, nil
	}

	// Create base chat
	chat := &store.Chat{
		JID:                  jid,
		Name:                 conv.GetDisplayName(),
		UnreadCount:          int(conv.GetUnreadCount()),
		UnreadMentionCount:   int(conv.GetUnreadMentionCount()),
		IsArchived:           conv.GetArchived(),
		IsPinned:             conv.GetPinned() > 0,
		MarkedAsUnread:       conv.GetMarkedAsUnread(),
		EphemeralDuration:    conv.GetEphemeralExpiration(),
		EndOfHistoryTransfer: conv.GetEndOfHistoryTransfer(),
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	// Set timestamps
	if ts := conv.GetConversationTimestamp(); ts > 0 {
		chat.ConversationTimestamp = time.Unix(int64(ts), 0)
	}
	if ts := conv.GetPinned(); ts > 0 {
		chat.PinTimestamp = time.Unix(int64(ts), 0)
	}
	if ts := conv.GetMuteEndTime(); ts > 0 {
		chat.MutedUntil = time.Unix(int64(ts), 0)
	}
	if ts := conv.GetEphemeralSettingTimestamp(); ts > 0 {
		chat.EphemeralSettingTimestamp = time.Unix(ts, 0)
	}
	if ts := conv.GetLastMsgTimestamp(); ts > 0 {
		chat.LastMessageAt = time.Unix(int64(ts), 0)
	}

	// Determine chat type
	chat.ChatType = determineChatType(jid)

	// Extract group if applicable
	var group *store.Group
	var participants []store.GroupParticipant
	var pastParticipants []store.PastParticipant

	if jid.Server == types.GroupServer {
		group = &store.Group{
			JID:               jid,
			Name:              conv.GetName(),
			Topic:             conv.GetDescription(),
			EphemeralDuration: conv.GetEphemeralExpiration(),
			IsLocked:          conv.GetLocked(),
			IsCommunity:       conv.GetIsParentGroup(),
			IsParentGroup:     conv.GetIsParentGroup(),
			IsDefaultSubgroup: conv.GetIsDefaultSubgroup(),
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}

		if ts := conv.GetCreatedAt(); ts > 0 {
			group.CreatedAtWA = time.Unix(int64(ts), 0)
		}
		if creator := conv.GetCreatedBy(); creator != "" {
			group.CreatedByLID, _ = types.ParseJID(creator)
		}
		if parent := conv.GetParentGroupID(); parent != "" {
			group.ParentGroupJID, _ = types.ParseJID(parent)
		}

		// Extract participants
		for _, p := range conv.GetParticipant() {
			memberJID, _ := types.ParseJID(p.GetUserJID())
			if !memberJID.IsEmpty() {
				rank := p.GetRank()
				participants = append(participants, store.GroupParticipant{
					GroupJID:     jid,
					MemberLID:    memberJID,
					IsAdmin:      rank == waHistorySync.GroupParticipant_ADMIN || rank == waHistorySync.GroupParticipant_SUPERADMIN,
					IsSuperAdmin: rank == waHistorySync.GroupParticipant_SUPERADMIN,
				})
			}
		}
	}

	// Extract messages
	var messages []*store.Message
	for _, histMsg := range conv.GetMessages() {
		if msg := extractHistoryMessage(histMsg, jid); msg != nil {
			messages = append(messages, msg)
		}
	}

	return chat, group, participants, pastParticipants, messages
}

func extractHistoryMessage(histMsg *waHistorySync.HistorySyncMsg, chatJID types.JID) *store.Message {
	webMsg := histMsg.GetMessage()
	if webMsg == nil {
		return nil
	}

	msg := &store.Message{
		ID:          webMsg.GetKey().GetID(),
		ChatJID:     chatJID,
		FromMe:      webMsg.GetKey().GetFromMe(),
		MessageType: "unknown",
		CreatedAt:   time.Now(),
	}

	// Timestamp
	if ts := webMsg.GetMessageTimestamp(); ts > 0 {
		msg.Timestamp = time.Unix(int64(ts), 0)
	}

	// Sender
	if sender := webMsg.GetParticipant(); sender != "" {
		msg.SenderLID, _ = types.ParseJID(sender)
	} else if webMsg.GetKey().GetFromMe() {
		// FromMe, sender is self
	} else if chatJID.Server != types.GroupServer {
		// DM, sender is the chat
		msg.SenderLID = chatJID
	}

	// Push name
	msg.PushName = webMsg.GetPushName()

	// Message content
	if protoMsg := webMsg.GetMessage(); protoMsg != nil {
		msg.MessageType = determineMessageType(protoMsg)

		// Text content
		if txt := protoMsg.GetConversation(); txt != "" {
			msg.TextContent = txt
		} else if ext := protoMsg.GetExtendedTextMessage(); ext != nil {
			msg.TextContent = ext.GetText()
		}

		// Extract media
		extractMedia(protoMsg, msg)

		// Extract context
		extractContext(protoMsg, msg)

		// Extract location
		extractLocation(protoMsg, msg)

		// Extract contact
		extractContactCard(protoMsg, msg)

		// Extract poll
		extractPoll(protoMsg, msg)
	}

	// Status flags
	msg.IsStarred = webMsg.GetStarred()

	return msg
}

func determineChatType(jid types.JID) store.ChatType {
	switch jid.Server {
	case types.GroupServer:
		return store.ChatTypeGroup
	case types.BroadcastServer:
		if jid.User == "status" {
			return store.ChatTypeStatus
		}
		return store.ChatTypeBroadcast
	case types.NewsletterServer:
		return store.ChatTypeNewsletter
	default:
		return store.ChatTypeUser
	}
}
