package extract

import (
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"orion-agent/internal/store"
)

// MessageFromEvent extracts a store.Message from events.Message.
// This captures ALL available fields from the whatsmeow event.
func MessageFromEvent(evt *events.Message) *store.Message {
	msg := &store.Message{
		// Identity fields
		ID:        evt.Info.ID,
		ChatJID:   evt.Info.Chat,
		SenderLID: evt.Info.Sender,
		FromMe:    evt.Info.IsFromMe,
		Timestamp: evt.Info.Timestamp,
		ServerID:  evt.Info.ServerID, // Capture server ID
		PushName:  evt.Info.PushName,

		// Message classification
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

	// Extract text content from all text message types
	extractTextContent(evt.Message, msg)

	// Extract protocol message type
	if pm := evt.Message.GetProtocolMessage(); pm != nil {
		msg.ProtocolType = int(pm.GetType())
	}

	// Extract media from all media message types
	extractMedia(evt.Message, msg)

	// Extract context (quotes, mentions, forwarding)
	extractContext(evt.Message, msg)

	// Extract location from location messages
	extractLocation(evt.Message, msg)

	// Extract contact card from contact messages
	extractContactCard(evt.Message, msg)

	// Extract poll from poll messages
	extractPoll(evt.Message, msg)

	// Extract event message data
	extractEventMessage(evt.Message, msg)

	// Extract interactive message data (buttons, lists, etc.)
	extractInteractive(evt.Message, msg)

	return msg
}

// extractTextContent extracts text from various message types.
func extractTextContent(msg *waE2E.Message, m *store.Message) {
	if msg == nil {
		return
	}

	// Simple conversation text
	if txt := msg.GetConversation(); txt != "" {
		m.TextContent = txt
		return
	}

	// Extended text message
	if ext := msg.GetExtendedTextMessage(); ext != nil {
		m.TextContent = ext.GetText()
		return
	}

	// Buttons response
	if br := msg.GetButtonsResponseMessage(); br != nil {
		m.TextContent = br.GetSelectedDisplayText()
		return
	}

	// List response
	if lr := msg.GetListResponseMessage(); lr != nil {
		m.TextContent = lr.GetTitle()
		return
	}

	// Template button reply
	if tbr := msg.GetTemplateButtonReplyMessage(); tbr != nil {
		m.TextContent = tbr.GetSelectedDisplayText()
		return
	}
}

// determineMessageType determines the message type from the protobuf.
// This handles ALL known message types comprehensively.
func determineMessageType(msg *waE2E.Message) string {
	if msg == nil {
		return "unknown"
	}

	switch {
	// Text messages
	case msg.GetConversation() != "":
		return "text"
	case msg.GetExtendedTextMessage() != nil:
		return "extended_text"

	// Media messages
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
	case msg.GetPtvMessage() != nil:
		return "ptv" // Push-to-talk video (video note)

	// Location messages
	case msg.GetLocationMessage() != nil:
		return "location"
	case msg.GetLiveLocationMessage() != nil:
		return "live_location"

	// Contact messages
	case msg.GetContactMessage() != nil:
		return "contact"
	case msg.GetContactsArrayMessage() != nil:
		return "contacts_array"

	// Poll messages
	case msg.GetPollCreationMessage() != nil:
		return "poll"
	case msg.GetPollCreationMessageV2() != nil:
		return "poll_v2"
	case msg.GetPollCreationMessageV3() != nil:
		return "poll_v3"
	case msg.GetPollUpdateMessage() != nil:
		return "poll_update"

	// Reaction
	case msg.GetReactionMessage() != nil:
		return "reaction"

	// Protocol messages
	case msg.GetProtocolMessage() != nil:
		return getProtocolType(msg.GetProtocolMessage())

	// Interactive messages
	case msg.GetButtonsMessage() != nil:
		return "buttons"
	case msg.GetButtonsResponseMessage() != nil:
		return "buttons_response"
	case msg.GetListMessage() != nil:
		return "list"
	case msg.GetListResponseMessage() != nil:
		return "list_response"
	case msg.GetTemplateMessage() != nil:
		return "template"
	case msg.GetTemplateButtonReplyMessage() != nil:
		return "template_reply"
	case msg.GetInteractiveMessage() != nil:
		return "interactive"
	case msg.GetInteractiveResponseMessage() != nil:
		return "interactive_response"

	// View once wrappers
	case msg.GetViewOnceMessage() != nil:
		return "view_once"
	case msg.GetViewOnceMessageV2() != nil:
		return "view_once_v2"
	case msg.GetViewOnceMessageV2Extension() != nil:
		return "view_once_v2_ext"

	// Business/Commerce messages
	case msg.GetOrderMessage() != nil:
		return "order"
	case msg.GetProductMessage() != nil:
		return "product"
	// CatalogSnapshot not available in this version
	case msg.GetInvoiceMessage() != nil:
		return "invoice"
	case msg.GetRequestPaymentMessage() != nil:
		return "payment_request"
	case msg.GetDeclinePaymentRequestMessage() != nil:
		return "payment_declined"
	case msg.GetCancelPaymentRequestMessage() != nil:
		return "payment_cancelled"
	case msg.GetPaymentInviteMessage() != nil:
		return "payment_invite"
	case msg.GetSendPaymentMessage() != nil:
		return "payment_sent"

	// Group/Channel messages
	case msg.GetGroupInviteMessage() != nil:
		return "group_invite"
	case msg.GetNewsletterAdminInviteMessage() != nil:
		return "newsletter_admin_invite"

	// Event messages
	case msg.GetEventMessage() != nil:
		return "event"
	// EventInviteMessage not available in this version

	// Comment messages
	case msg.GetCommentMessage() != nil:
		return "comment"

	// Keep/Pin messages
	case msg.GetKeepInChatMessage() != nil:
		return "keep_in_chat"
	case msg.GetPinInChatMessage() != nil:
		return "pin_in_chat"

	// Call messages
	case msg.GetCall() != nil:
		return "call"
	case msg.GetBcallMessage() != nil:
		return "bcall"

	// Status/Story
	case msg.GetHighlyStructuredMessage() != nil:
		return "highly_structured"

	// Encrypted reactions
	case msg.GetEncReactionMessage() != nil:
		return "encrypted_reaction"

	// Bot messages
	case msg.GetBotInvokeMessage() != nil:
		return "bot_invoke"

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
		return "message_edit"
	case waE2E.ProtocolMessage_APP_STATE_SYNC_KEY_SHARE:
		return "app_state_key_share"
	case waE2E.ProtocolMessage_HISTORY_SYNC_NOTIFICATION:
		return "history_sync_notification"
	case waE2E.ProtocolMessage_INITIAL_SECURITY_NOTIFICATION_SETTING_SYNC:
		return "security_notification_sync"
	case waE2E.ProtocolMessage_APP_STATE_FATAL_EXCEPTION_NOTIFICATION:
		return "app_state_fatal"
	// MESSAGE_REQUEST_RESPONSE_MESSAGE not available in this version
	case waE2E.ProtocolMessage_SHARE_PHONE_NUMBER:
		return "share_phone_number"
	case waE2E.ProtocolMessage_PEER_DATA_OPERATION_REQUEST_MESSAGE:
		return "peer_data_request"
	case waE2E.ProtocolMessage_PEER_DATA_OPERATION_REQUEST_RESPONSE_MESSAGE:
		return "peer_data_response"
	case waE2E.ProtocolMessage_REQUEST_WELCOME_MESSAGE:
		return "request_welcome"
	case waE2E.ProtocolMessage_BOT_FEEDBACK_MESSAGE:
		return "bot_feedback"
	case waE2E.ProtocolMessage_MEDIA_NOTIFY_MESSAGE:
		return "media_notify"
	default:
		return "protocol"
	}
}

// extractMedia extracts media fields from message.
func extractMedia(msg *waE2E.Message, m *store.Message) {
	if img := msg.GetImageMessage(); img != nil {
		extractImageMedia(img, m)
		return
	}

	if vid := msg.GetVideoMessage(); vid != nil {
		extractVideoMedia(vid, m)
		return
	}

	// PTV (Push-to-talk video) uses VideoMessage structure
	if ptv := msg.GetPtvMessage(); ptv != nil {
		extractVideoMedia(ptv, m)
		m.MessageType = "ptv"
		return
	}

	if aud := msg.GetAudioMessage(); aud != nil {
		extractAudioMedia(aud, m)
		return
	}

	if doc := msg.GetDocumentMessage(); doc != nil {
		extractDocumentMedia(doc, m)
		return
	}

	if stk := msg.GetStickerMessage(); stk != nil {
		extractStickerMedia(stk, m)
		return
	}

	// Handle view once wrappers
	if vo := msg.GetViewOnceMessage(); vo != nil {
		extractMedia(vo.GetMessage(), m)
		m.IsViewOnce = true
		return
	}
	if vo2 := msg.GetViewOnceMessageV2(); vo2 != nil {
		extractMedia(vo2.GetMessage(), m)
		m.IsViewOnce = true
		return
	}
	if vo2ext := msg.GetViewOnceMessageV2Extension(); vo2ext != nil {
		extractMedia(vo2ext.GetMessage(), m)
		m.IsViewOnce = true
		return
	}
}

func extractImageMedia(img *waE2E.ImageMessage, m *store.Message) {
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
}

func extractVideoMedia(vid *waE2E.VideoMessage, m *store.Message) {
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
}

func extractAudioMedia(aud *waE2E.AudioMessage, m *store.Message) {
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
}

func extractDocumentMedia(doc *waE2E.DocumentMessage, m *store.Message) {
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
}

func extractStickerMedia(stk *waE2E.StickerMessage, m *store.Message) {
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

	// Ephemeral duration from context
	if ctx.GetExpiration() > 0 {
		m.IsEphemeral = true
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
	if poll := msg.GetPollCreationMessageV3(); poll != nil {
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

	// Event
	if event := msg.GetEventMessage(); event != nil {
		return event.GetName()
	}

	return ""
}

// getContextInfo extracts ContextInfo from any message type that can contain it.
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
	if ptv := msg.GetPtvMessage(); ptv != nil {
		return ptv.GetContextInfo()
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
	if live := msg.GetLiveLocationMessage(); live != nil {
		return live.GetContextInfo()
	}
	if contact := msg.GetContactMessage(); contact != nil {
		return contact.GetContextInfo()
	}
	if contacts := msg.GetContactsArrayMessage(); contacts != nil {
		return contacts.GetContextInfo()
	}
	if poll := msg.GetPollCreationMessage(); poll != nil {
		return poll.GetContextInfo()
	}
	if poll := msg.GetPollCreationMessageV2(); poll != nil {
		return poll.GetContextInfo()
	}
	if poll := msg.GetPollCreationMessageV3(); poll != nil {
		return poll.GetContextInfo()
	}
	if event := msg.GetEventMessage(); event != nil {
		return event.GetContextInfo()
	}
	if groupInvite := msg.GetGroupInviteMessage(); groupInvite != nil {
		return groupInvite.GetContextInfo()
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
		m.Thumbnail = loc.GetJPEGThumbnail()
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
		m.Thumbnail = live.GetJPEGThumbnail()
		return
	}
}

// extractContactCard extracts contact card from message.
func extractContactCard(msg *waE2E.Message, m *store.Message) {
	if contact := msg.GetContactMessage(); contact != nil {
		m.DisplayName = contact.GetDisplayName()
		m.VCard = contact.GetVcard()
		return
	}

	// For contacts array, we get the display name from the array
	if contacts := msg.GetContactsArrayMessage(); contacts != nil {
		m.DisplayName = contacts.GetDisplayName()
		// The VCard field will contain all vcards concatenated
		for _, c := range contacts.GetContacts() {
			if m.VCard != "" {
				m.VCard += "\n---\n"
			}
			m.VCard += c.GetVcard()
		}
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

// extractEventMessage extracts event message details.
func extractEventMessage(msg *waE2E.Message, m *store.Message) {
	event := msg.GetEventMessage()
	if event == nil {
		return
	}

	// Store event details in text content as structured data
	m.TextContent = event.GetName()
	m.Caption = event.GetDescription()

	// Extract location if present
	if loc := event.GetLocation(); loc != nil {
		m.Latitude = loc.GetDegreesLatitude()
		m.Longitude = loc.GetDegreesLongitude()
		m.LocationName = loc.GetName()
		m.LocationAddress = loc.GetAddress()
	}
}

// extractInteractive extracts interactive message components.
func extractInteractive(msg *waE2E.Message, m *store.Message) {
	// Buttons message
	if buttons := msg.GetButtonsMessage(); buttons != nil {
		m.TextContent = buttons.GetContentText()
		m.Caption = buttons.GetFooterText()
		return
	}

	// List message
	if list := msg.GetListMessage(); list != nil {
		m.TextContent = list.GetTitle()
		m.Caption = list.GetDescription()
		return
	}

	// Interactive message (newer format)
	if interactive := msg.GetInteractiveMessage(); interactive != nil {
		if body := interactive.GetBody(); body != nil {
			m.TextContent = body.GetText()
		}
		if footer := interactive.GetFooter(); footer != nil {
			m.Caption = footer.GetText()
		}
		return
	}
}
