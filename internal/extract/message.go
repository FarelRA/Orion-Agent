package extract

import (
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"orion-agent/internal/store"
)

// MessageFromEvent extracts a store.Message from events.Message.
func MessageFromEvent(evt *events.Message) *store.Message {
	msg := &store.Message{
		ID:          evt.Info.ID,
		ChatJID:     evt.Info.Chat,
		SenderLID:   evt.Info.Sender,
		FromMe:      evt.Info.IsFromMe,
		Timestamp:   evt.Info.Timestamp,
		PushName:    evt.Info.PushName,
		MessageType: determineMessageType(evt.Message),
		IsEphemeral: evt.IsEphemeral,
		IsViewOnce:  evt.IsViewOnce || evt.IsViewOnceV2,
		CreatedAt:   time.Now(),
	}

	// Broadcast detection
	if evt.Info.Chat.Server == "broadcast" {
		msg.IsBroadcast = true
		msg.BroadcastListJID = evt.Info.Chat
	}

	// Extract text content
	if txt := evt.Message.GetConversation(); txt != "" {
		msg.TextContent = txt
	} else if ext := evt.Message.GetExtendedTextMessage(); ext != nil {
		msg.TextContent = ext.GetText()
	}

	// Extract protocol message type
	if pm := evt.Message.GetProtocolMessage(); pm != nil {
		msg.ProtocolType = int(pm.GetType())
	}

	// Extract media
	extractMedia(evt.Message, msg)

	// Extract context (quotes, mentions, forwarding)
	extractContext(evt.Message, msg)

	// Extract location
	extractLocation(evt.Message, msg)

	// Extract contact card
	extractContactCard(evt.Message, msg)

	// Extract poll
	extractPoll(evt.Message, msg)

	return msg
}

// determineMessageType determines the message type from the protobuf.
func determineMessageType(msg *waE2E.Message) string {
	if msg == nil {
		return "unknown"
	}

	switch {
	case msg.GetConversation() != "":
		return "text"
	case msg.GetExtendedTextMessage() != nil:
		return "text"
	case msg.GetImageMessage() != nil:
		return "image"
	case msg.GetVideoMessage() != nil:
		return "video"
	case msg.GetAudioMessage() != nil:
		return "audio"
	case msg.GetDocumentMessage() != nil:
		return "document"
	case msg.GetStickerMessage() != nil:
		return "sticker"
	case msg.GetLocationMessage() != nil:
		return "location"
	case msg.GetLiveLocationMessage() != nil:
		return "live_location"
	case msg.GetContactMessage() != nil:
		return "contact"
	case msg.GetContactsArrayMessage() != nil:
		return "contacts"
	case msg.GetPollCreationMessage() != nil || msg.GetPollCreationMessageV2() != nil || msg.GetPollCreationMessageV3() != nil:
		return "poll"
	case msg.GetPollUpdateMessage() != nil:
		return "poll_update"
	case msg.GetReactionMessage() != nil:
		return "reaction"
	case msg.GetProtocolMessage() != nil:
		return getProtocolType(msg.GetProtocolMessage())
	case msg.GetButtonsMessage() != nil:
		return "buttons"
	case msg.GetTemplateMessage() != nil:
		return "template"
	case msg.GetListMessage() != nil:
		return "list"
	case msg.GetViewOnceMessage() != nil:
		return "view_once"
	case msg.GetViewOnceMessageV2() != nil:
		return "view_once_v2"
	case msg.GetOrderMessage() != nil:
		return "order"
	case msg.GetGroupInviteMessage() != nil:
		return "group_invite"
	case msg.GetNewsletterAdminInviteMessage() != nil:
		return "newsletter_invite"
	default:
		return "unknown"
	}
}

func getProtocolType(pm *waE2E.ProtocolMessage) string {
	if pm == nil {
		return "protocol"
	}
	switch pm.GetType() {
	case waE2E.ProtocolMessage_REVOKE:
		return "revoke"
	case waE2E.ProtocolMessage_EPHEMERAL_SETTING:
		return "ephemeral_setting"
	case waE2E.ProtocolMessage_MESSAGE_EDIT:
		return "edit"
	case waE2E.ProtocolMessage_APP_STATE_SYNC_KEY_SHARE:
		return "app_state_key"
	case waE2E.ProtocolMessage_HISTORY_SYNC_NOTIFICATION:
		return "history_sync"
	case waE2E.ProtocolMessage_INITIAL_SECURITY_NOTIFICATION_SETTING_SYNC:
		return "security_notification"
	default:
		return "protocol"
	}
}

// extractMedia extracts media fields from message.
func extractMedia(msg *waE2E.Message, m *store.Message) {
	if img := msg.GetImageMessage(); img != nil {
		m.MediaURL = img.GetURL()
		m.MediaDirectPath = img.GetDirectPath()
		m.MediaKey = img.GetMediaKey()
		m.MediaKeyTimestamp = img.GetMediaKeyTimestamp()
		m.FileSHA256 = img.GetFileSHA256()
		m.FileEncSHA256 = img.GetFileEncSHA256()
		m.FileLength = int64(img.GetFileLength())
		m.Mimetype = img.GetMimetype()
		m.Width = int(img.GetWidth())
		m.Height = int(img.GetHeight())
		m.Thumbnail = img.GetJPEGThumbnail()
		m.Caption = img.GetCaption()
		m.ThumbnailDirectPath = img.GetThumbnailDirectPath()
		m.ThumbnailSHA256 = img.GetThumbnailSHA256()
		m.ThumbnailEncSHA256 = img.GetThumbnailEncSHA256()
		return
	}

	if vid := msg.GetVideoMessage(); vid != nil {
		m.MediaURL = vid.GetURL()
		m.MediaDirectPath = vid.GetDirectPath()
		m.MediaKey = vid.GetMediaKey()
		m.MediaKeyTimestamp = vid.GetMediaKeyTimestamp()
		m.FileSHA256 = vid.GetFileSHA256()
		m.FileEncSHA256 = vid.GetFileEncSHA256()
		m.FileLength = int64(vid.GetFileLength())
		m.Mimetype = vid.GetMimetype()
		m.Width = int(vid.GetWidth())
		m.Height = int(vid.GetHeight())
		m.DurationSeconds = int(vid.GetSeconds())
		m.Thumbnail = vid.GetJPEGThumbnail()
		m.Caption = vid.GetCaption()
		m.IsGIF = vid.GetGifPlayback()
		m.StreamingSidecar = vid.GetStreamingSidecar()
		m.ThumbnailDirectPath = vid.GetThumbnailDirectPath()
		m.ThumbnailSHA256 = vid.GetThumbnailSHA256()
		m.ThumbnailEncSHA256 = vid.GetThumbnailEncSHA256()
		return
	}

	if aud := msg.GetAudioMessage(); aud != nil {
		m.MediaURL = aud.GetURL()
		m.MediaDirectPath = aud.GetDirectPath()
		m.MediaKey = aud.GetMediaKey()
		m.MediaKeyTimestamp = aud.GetMediaKeyTimestamp()
		m.FileSHA256 = aud.GetFileSHA256()
		m.FileEncSHA256 = aud.GetFileEncSHA256()
		m.FileLength = int64(aud.GetFileLength())
		m.Mimetype = aud.GetMimetype()
		m.DurationSeconds = int(aud.GetSeconds())
		m.IsPTT = aud.GetPTT()
		m.Waveform = aud.GetWaveform()
		return
	}

	if doc := msg.GetDocumentMessage(); doc != nil {
		m.MediaURL = doc.GetURL()
		m.MediaDirectPath = doc.GetDirectPath()
		m.MediaKey = doc.GetMediaKey()
		m.MediaKeyTimestamp = doc.GetMediaKeyTimestamp()
		m.FileSHA256 = doc.GetFileSHA256()
		m.FileEncSHA256 = doc.GetFileEncSHA256()
		m.FileLength = int64(doc.GetFileLength())
		m.Mimetype = doc.GetMimetype()
		m.Thumbnail = doc.GetJPEGThumbnail()
		m.Caption = doc.GetCaption()
		m.DisplayName = doc.GetFileName()
		m.ThumbnailDirectPath = doc.GetThumbnailDirectPath()
		m.ThumbnailSHA256 = doc.GetThumbnailSHA256()
		m.ThumbnailEncSHA256 = doc.GetThumbnailEncSHA256()
		return
	}

	if stk := msg.GetStickerMessage(); stk != nil {
		m.MediaURL = stk.GetURL()
		m.MediaDirectPath = stk.GetDirectPath()
		m.MediaKey = stk.GetMediaKey()
		m.MediaKeyTimestamp = stk.GetMediaKeyTimestamp()
		m.FileSHA256 = stk.GetFileSHA256()
		m.FileEncSHA256 = stk.GetFileEncSHA256()
		m.FileLength = int64(stk.GetFileLength())
		m.Mimetype = stk.GetMimetype()
		m.Width = int(stk.GetWidth())
		m.Height = int(stk.GetHeight())
		m.IsAnimated = stk.GetIsAnimated()
		// StickerPackID and StickerPackName not available in waE2E.StickerMessage
		return
	}

	// Handle view once
	if vo := msg.GetViewOnceMessage(); vo != nil {
		extractMedia(vo.GetMessage(), m)
		return
	}
	if vo2 := msg.GetViewOnceMessageV2(); vo2 != nil {
		extractMedia(vo2.GetMessage(), m)
		return
	}
}

// extractContext extracts context info (quotes, mentions, forwarding).
func extractContext(msg *waE2E.Message, m *store.Message) {
	ctx := getContextInfo(msg)
	if ctx == nil {
		return
	}

	// Quoted message
	if ctx.GetStanzaID() != "" {
		m.QuotedMessageID = ctx.GetStanzaID()
		if ctx.GetParticipant() != "" {
			m.QuotedSenderLID, _ = types.ParseJID(ctx.GetParticipant())
		}

		// Extract quoted message content and type
		if quotedMsg := ctx.GetQuotedMessage(); quotedMsg != nil {
			m.QuotedMessageType = determineMessageType(quotedMsg)
			m.QuotedContent = extractQuotedContent(quotedMsg)
		}
	}

	// Mentioned JIDs
	if mentioned := ctx.GetMentionedJID(); len(mentioned) > 0 {
		m.MentionedJIDs = make([]types.JID, 0, len(mentioned))
		for _, jidStr := range mentioned {
			if jid, err := types.ParseJID(jidStr); err == nil {
				m.MentionedJIDs = append(m.MentionedJIDs, jid)
			}
		}
	}

	// Group mentions
	if groupMentions := ctx.GetGroupMentions(); len(groupMentions) > 0 {
		m.GroupMentions = make([]store.GroupMention, 0, len(groupMentions))
		for _, gm := range groupMentions {
			if gm.GetGroupJID() != "" {
				jid, _ := types.ParseJID(gm.GetGroupJID())
				m.GroupMentions = append(m.GroupMentions, store.GroupMention{
					GroupJID:     jid,
					GroupSubject: gm.GetGroupSubject(),
				})
			}
		}
	}

	// Forwarding
	if ctx.GetIsForwarded() {
		m.IsForwarded = true
		m.ForwardingScore = int(ctx.GetForwardingScore())
	}
}

// extractQuotedContent extracts text content from a quoted message.
func extractQuotedContent(msg *waE2E.Message) string {
	if msg == nil {
		return ""
	}

	// Text messages
	if text := msg.GetConversation(); text != "" {
		return text
	}
	if ext := msg.GetExtendedTextMessage(); ext != nil {
		return ext.GetText()
	}

	// Media with captions
	if img := msg.GetImageMessage(); img != nil {
		return img.GetCaption()
	}
	if vid := msg.GetVideoMessage(); vid != nil {
		return vid.GetCaption()
	}
	if doc := msg.GetDocumentMessage(); doc != nil {
		return doc.GetCaption()
	}

	// Poll
	if poll := msg.GetPollCreationMessage(); poll != nil {
		return poll.GetName()
	}
	if poll := msg.GetPollCreationMessageV2(); poll != nil {
		return poll.GetName()
	}

	// Location
	if loc := msg.GetLocationMessage(); loc != nil {
		return loc.GetName()
	}

	// Contact
	if contact := msg.GetContactMessage(); contact != nil {
		return contact.GetDisplayName()
	}

	return ""
}

// getContextInfo extracts ContextInfo from any message type.
func getContextInfo(msg *waE2E.Message) *waE2E.ContextInfo {
	if msg == nil {
		return nil
	}

	if ext := msg.GetExtendedTextMessage(); ext != nil {
		return ext.GetContextInfo()
	}
	if img := msg.GetImageMessage(); img != nil {
		return img.GetContextInfo()
	}
	if vid := msg.GetVideoMessage(); vid != nil {
		return vid.GetContextInfo()
	}
	if aud := msg.GetAudioMessage(); aud != nil {
		return aud.GetContextInfo()
	}
	if doc := msg.GetDocumentMessage(); doc != nil {
		return doc.GetContextInfo()
	}
	if stk := msg.GetStickerMessage(); stk != nil {
		return stk.GetContextInfo()
	}
	if loc := msg.GetLocationMessage(); loc != nil {
		return loc.GetContextInfo()
	}
	if contact := msg.GetContactMessage(); contact != nil {
		return contact.GetContextInfo()
	}

	return nil
}

// extractLocation extracts location from message.
func extractLocation(msg *waE2E.Message, m *store.Message) {
	if loc := msg.GetLocationMessage(); loc != nil {
		m.Latitude = loc.GetDegreesLatitude()
		m.Longitude = loc.GetDegreesLongitude()
		m.LocationName = loc.GetName()
		m.LocationAddress = loc.GetAddress()
		m.LocationURL = loc.GetURL()
		m.AccuracyMeters = int(loc.GetAccuracyInMeters())
		return
	}

	if live := msg.GetLiveLocationMessage(); live != nil {
		m.Latitude = live.GetDegreesLatitude()
		m.Longitude = live.GetDegreesLongitude()
		m.IsLiveLocation = true
		m.AccuracyMeters = int(live.GetAccuracyInMeters())
		m.SpeedMPS = float64(live.GetSpeedInMps())
		m.DegreesClockwise = int(live.GetDegreesClockwiseFromMagneticNorth())
		m.LiveLocationSeq = int(live.GetSequenceNumber())
		m.Caption = live.GetCaption()
		return
	}
}

// extractContactCard extracts contact card from message.
func extractContactCard(msg *waE2E.Message, m *store.Message) {
	if contact := msg.GetContactMessage(); contact != nil {
		m.DisplayName = contact.GetDisplayName()
		m.VCard = contact.GetVcard()
	}
}

// extractPoll extracts poll from message.
func extractPoll(msg *waE2E.Message, m *store.Message) {
	// Try all poll creation versions
	var poll *waE2E.PollCreationMessage
	if p := msg.GetPollCreationMessage(); p != nil {
		poll = p
	} else if p := msg.GetPollCreationMessageV2(); p != nil {
		poll = p
	} else if p := msg.GetPollCreationMessageV3(); p != nil {
		poll = p
	}

	if poll != nil {
		m.PollName = poll.GetName()
		options := poll.GetOptions()
		m.PollOptions = make([]string, len(options))
		for i, opt := range options {
			m.PollOptions[i] = opt.GetOptionName()
		}
		m.PollSelectMax = int(poll.GetSelectableOptionsCount())
		m.PollEncryptionKey = poll.GetEncKey()
	}
}
