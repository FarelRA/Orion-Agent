package send

import (
	"context"
	"fmt"
	"time"

	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"

	"orion-agent/internal/data/store"
)

// Edit edits a previously sent message.
func (s *SendService) Edit(ctx context.Context, chat types.JID, msgID types.MessageID, newText string) (*SendResult, error) {
	if s.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	editMsg := &waE2E.Message{
		ProtocolMessage: &waE2E.ProtocolMessage{
			Key: &waCommon.MessageKey{
				RemoteJID: proto.String(chat.String()),
				FromMe:    proto.Bool(true),
				ID:        proto.String(string(msgID)),
			},
			Type: waE2E.ProtocolMessage_MESSAGE_EDIT.Enum(),
			EditedMessage: &waE2E.Message{
				Conversation: proto.String(newText),
			},
		},
	}

	resp, err := s.client.SendMessage(ctx, chat, editMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to edit message: %w", err)
	}

	// Save edit to database
	if s.messages != nil {
		if err := s.messages.MarkEdited(string(msgID), chat, newText, resp.Timestamp); err != nil {
			s.log.Warnf("Failed to save edit for message %s: %v", msgID, err)
		}
	}

	return &SendResult{
		MessageID: resp.ID,
		ServerID:  resp.ServerID,
		Timestamp: resp.Timestamp,
		Recipient: chat,
		Sender:    resp.Sender,
		DebugInfo: resp.DebugTimings,
	}, nil
}

// EditExtended edits a message with extended text (preserving context).
func (s *SendService) EditExtended(ctx context.Context, chat types.JID, msgID types.MessageID, newText string, ctxInfo *ContextInfo) (*SendResult, error) {
	if s.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	editedMsg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String(newText),
		},
	}

	if ctxInfo != nil {
		editedMsg.ExtendedTextMessage.ContextInfo = ctxInfo.Build()
	}

	editMsg := &waE2E.Message{
		ProtocolMessage: &waE2E.ProtocolMessage{
			Key: &waCommon.MessageKey{
				RemoteJID: proto.String(chat.String()),
				FromMe:    proto.Bool(true),
				ID:        proto.String(string(msgID)),
			},
			Type:          waE2E.ProtocolMessage_MESSAGE_EDIT.Enum(),
			EditedMessage: editedMsg,
		},
	}

	resp, err := s.client.SendMessage(ctx, chat, editMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to edit message: %w", err)
	}

	// Save edit to database
	if s.messages != nil {
		if err := s.messages.MarkEdited(string(msgID), chat, newText, resp.Timestamp); err != nil {
			s.log.Warnf("Failed to save edit for message %s: %v", msgID, err)
		}
	}

	return &SendResult{
		MessageID: resp.ID,
		ServerID:  resp.ServerID,
		Timestamp: resp.Timestamp,
		Recipient: chat,
		Sender:    resp.Sender,
		DebugInfo: resp.DebugTimings,
	}, nil
}

// Revoke deletes a message for everyone.
func (s *SendService) Revoke(ctx context.Context, chat types.JID, sender types.JID, msgID types.MessageID, fromMe bool) (*SendResult, error) {
	if s.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	key := &waCommon.MessageKey{
		RemoteJID: proto.String(chat.String()),
		FromMe:    proto.Bool(fromMe),
		ID:        proto.String(string(msgID)),
	}

	// For group messages from others, set participant
	if !fromMe && chat.Server == types.GroupServer && !sender.IsEmpty() {
		key.Participant = proto.String(sender.String())
	}

	revokeMsg := &waE2E.Message{
		ProtocolMessage: &waE2E.ProtocolMessage{
			Key:  key,
			Type: waE2E.ProtocolMessage_REVOKE.Enum(),
		},
	}

	resp, err := s.client.SendMessage(ctx, chat, revokeMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to revoke message: %w", err)
	}

	// Save revoke to database
	if s.messages != nil {
		if err := s.messages.SetRevoked(string(msgID), chat); err != nil {
			s.log.Warnf("Failed to save revoke for message %s: %v", msgID, err)
		}
	}

	return &SendResult{
		MessageID: resp.ID,
		ServerID:  resp.ServerID,
		Timestamp: resp.Timestamp,
		Recipient: chat,
		Sender:    resp.Sender,
		DebugInfo: resp.DebugTimings,
	}, nil
}

// RevokeOwn is a convenience method for revoking your own message.
func (s *SendService) RevokeOwn(ctx context.Context, chat types.JID, msgID types.MessageID) (*SendResult, error) {
	return s.Revoke(ctx, chat, types.JID{}, msgID, true)
}

// React sends a reaction to a message.
func (s *SendService) React(ctx context.Context, chat types.JID, targetMsgID types.MessageID, targetSender types.JID, emoji string) (*SendResult, error) {
	if s.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	key := &waCommon.MessageKey{
		RemoteJID: proto.String(chat.String()),
		ID:        proto.String(string(targetMsgID)),
	}

	// Determine FromMe based on sender comparison with our JID
	fromMe := false
	if s.client.Store.ID != nil && !targetSender.IsEmpty() {
		fromMe = targetSender.User == s.client.Store.ID.User
	}
	key.FromMe = proto.Bool(fromMe)

	// Set participant for group messages from others
	if !fromMe && chat.Server == types.GroupServer && !targetSender.IsEmpty() {
		key.Participant = proto.String(targetSender.String())
	}

	reactionMsg := &waE2E.Message{
		ReactionMessage: &waE2E.ReactionMessage{
			Key:               key,
			Text:              proto.String(emoji),
			SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
		},
	}

	resp, err := s.client.SendMessage(ctx, chat, reactionMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to send reaction: %w", err)
	}

	// Save reaction to database
	if s.reactions != nil {
		ownJID := s.utils.OwnJID()
		if emoji == "" {
			// Remove reaction
			if err := s.reactions.Delete(string(targetMsgID), chat, ownJID); err != nil {
				s.log.Warnf("Failed to delete reaction for message %s: %v", targetMsgID, err)
			}
		} else {
			// Add reaction
			if err := s.reactions.Put(&store.Reaction{
				MessageID: string(targetMsgID),
				ChatJID:   chat,
				SenderLID: ownJID,
				Emoji:     emoji,
				Timestamp: resp.Timestamp,
			}); err != nil {
				s.log.Warnf("Failed to save reaction for message %s: %v", targetMsgID, err)
			}
		}
	}

	return &SendResult{
		MessageID: resp.ID,
		ServerID:  resp.ServerID,
		Timestamp: resp.Timestamp,
		Recipient: chat,
		Sender:    resp.Sender,
		DebugInfo: resp.DebugTimings,
	}, nil
}

// RemoveReaction removes a reaction from a message (sends empty emoji).
func (s *SendService) RemoveReaction(ctx context.Context, chat types.JID, targetMsgID types.MessageID, targetSender types.JID) (*SendResult, error) {
	return s.React(ctx, chat, targetMsgID, targetSender, "")
}

// Reply sends a reply to a message.
func (s *SendService) Reply(ctx context.Context, chat types.JID, replyToID types.MessageID, replyToSender types.JID, content Content, opts ...SendOption) (*SendResult, error) {
	// Add reply context to the content
	replyCtx := ReplyContext(replyToID, replyToSender, nil)

	// Apply context based on content type
	switch c := content.(type) {
	case *TextContent:
		c.ContextInfo = replyCtx
	case *ExtendedTextContent:
		c.ContextInfo = replyCtx
	case *ImageContent:
		c.ContextInfo = replyCtx
	case *VideoContent:
		c.ContextInfo = replyCtx
	case *AudioContent:
		c.ContextInfo = replyCtx
	case *DocumentContent:
		c.ContextInfo = replyCtx
	case *StickerContent:
		c.ContextInfo = replyCtx
	case *LocationContent:
		c.ContextInfo = replyCtx
	case *LiveLocationContent:
		c.ContextInfo = replyCtx
	case *ContactContent:
		c.ContextInfo = replyCtx
	case *ContactsArrayContent:
		c.ContextInfo = replyCtx
	case *PollContent:
		c.ContextInfo = replyCtx
	case *GroupInviteContent:
		c.ContextInfo = replyCtx
	case *EventContent:
		c.ContextInfo = replyCtx
	}

	return s.Send(ctx, chat, content, opts...)
}

// Forward forwards a message to another chat with proper forwarding context.
func (s *SendService) Forward(ctx context.Context, to types.JID, originalMsg *waE2E.Message, opts ...SendOption) (*SendResult, error) {
	if s.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}
	if originalMsg == nil {
		return nil, fmt.Errorf("original message is nil")
	}

	// Deep clone the message using protobuf
	clonedMsg := proto.Clone(originalMsg).(*waE2E.Message)

	// Add forwarding context to the cloned message
	forwardCtx := &waE2E.ContextInfo{
		IsForwarded:     proto.Bool(true),
		ForwardingScore: proto.Uint32(1),
	}

	// Apply forward context to the appropriate message type
	applyForwardContext(clonedMsg, forwardCtx)

	cfg := applyOptions(opts)
	extra := cfg.toSendRequestExtra()

	resp, err := s.client.SendMessage(ctx, to, clonedMsg, extra)
	if err != nil {
		return nil, fmt.Errorf("failed to forward message: %w", err)
	}

	result := &SendResult{
		MessageID: resp.ID,
		ServerID:  resp.ServerID,
		Timestamp: resp.Timestamp,
		Recipient: to,
		Sender:    resp.Sender,
		DebugInfo: resp.DebugTimings,
	}

	// Save forwarded message to database
	s.saveForwardedMessage(result, clonedMsg)

	return result, nil
}

// saveForwardedMessage saves a forwarded message to the database.
func (s *SendService) saveForwardedMessage(result *SendResult, msg *waE2E.Message) {
	if s.messages == nil {
		return
	}

	ownJID := s.utils.OwnJID()

	storeMsg := &store.Message{
		ID:          string(result.MessageID),
		ChatJID:     result.Recipient,
		SenderLID:   ownJID,
		FromMe:      true,
		Timestamp:   result.Timestamp,
		ServerID:    int(result.ServerID),
		IsForwarded: true,
	}

	// Extract all available data based on message type
	switch {
	case msg.ExtendedTextMessage != nil:
		ext := msg.ExtendedTextMessage
		storeMsg.MessageType = "text"
		storeMsg.TextContent = ext.GetText()
		storeMsg.PreviewTitle = ext.GetTitle()
		storeMsg.PreviewDescription = ext.GetDescription()
		storeMsg.PreviewURL = ext.GetMatchedText() // URL is in MatchedText
		storeMsg.PreviewMatchedText = ext.GetMatchedText()
		if ctx := ext.ContextInfo; ctx != nil {
			storeMsg.ForwardingScore = int(ctx.GetForwardingScore())
			storeMsg.MentionedJIDs = parseJIDStrings(ctx.MentionedJID)
		}

	case msg.ImageMessage != nil:
		img := msg.ImageMessage
		storeMsg.MessageType = "image"
		storeMsg.Caption = img.GetCaption()
		storeMsg.Mimetype = img.GetMimetype()
		storeMsg.MediaURL = img.GetURL()
		storeMsg.MediaDirectPath = img.GetDirectPath()
		storeMsg.MediaKey = img.GetMediaKey()
		storeMsg.FileSHA256 = img.GetFileSHA256()
		storeMsg.FileEncSHA256 = img.GetFileEncSHA256()
		storeMsg.FileLength = int64(img.GetFileLength())
		storeMsg.Width = int(img.GetWidth())
		storeMsg.Height = int(img.GetHeight())
		storeMsg.IsViewOnce = img.GetViewOnce()
		if ctx := img.ContextInfo; ctx != nil {
			storeMsg.ForwardingScore = int(ctx.GetForwardingScore())
		}

	case msg.VideoMessage != nil:
		vid := msg.VideoMessage
		storeMsg.MessageType = "video"
		storeMsg.Caption = vid.GetCaption()
		storeMsg.Mimetype = vid.GetMimetype()
		storeMsg.MediaURL = vid.GetURL()
		storeMsg.MediaDirectPath = vid.GetDirectPath()
		storeMsg.MediaKey = vid.GetMediaKey()
		storeMsg.FileSHA256 = vid.GetFileSHA256()
		storeMsg.FileEncSHA256 = vid.GetFileEncSHA256()
		storeMsg.FileLength = int64(vid.GetFileLength())
		storeMsg.Width = int(vid.GetWidth())
		storeMsg.Height = int(vid.GetHeight())
		storeMsg.DurationSeconds = int(vid.GetSeconds())
		storeMsg.IsGIF = vid.GetGifPlayback()
		storeMsg.IsViewOnce = vid.GetViewOnce()
		if ctx := vid.ContextInfo; ctx != nil {
			storeMsg.ForwardingScore = int(ctx.GetForwardingScore())
		}

	case msg.AudioMessage != nil:
		aud := msg.AudioMessage
		storeMsg.MessageType = "audio"
		storeMsg.Mimetype = aud.GetMimetype()
		storeMsg.MediaURL = aud.GetURL()
		storeMsg.MediaDirectPath = aud.GetDirectPath()
		storeMsg.MediaKey = aud.GetMediaKey()
		storeMsg.FileSHA256 = aud.GetFileSHA256()
		storeMsg.FileEncSHA256 = aud.GetFileEncSHA256()
		storeMsg.FileLength = int64(aud.GetFileLength())
		storeMsg.DurationSeconds = int(aud.GetSeconds())
		storeMsg.IsPTT = aud.GetPTT()
		if ctx := aud.ContextInfo; ctx != nil {
			storeMsg.ForwardingScore = int(ctx.GetForwardingScore())
		}

	case msg.DocumentMessage != nil:
		doc := msg.DocumentMessage
		storeMsg.MessageType = "document"
		storeMsg.Caption = doc.GetCaption()
		storeMsg.Mimetype = doc.GetMimetype()
		storeMsg.DisplayName = doc.GetFileName()
		storeMsg.MediaURL = doc.GetURL()
		storeMsg.MediaDirectPath = doc.GetDirectPath()
		storeMsg.MediaKey = doc.GetMediaKey()
		storeMsg.FileSHA256 = doc.GetFileSHA256()
		storeMsg.FileEncSHA256 = doc.GetFileEncSHA256()
		storeMsg.FileLength = int64(doc.GetFileLength())
		if ctx := doc.ContextInfo; ctx != nil {
			storeMsg.ForwardingScore = int(ctx.GetForwardingScore())
		}

	case msg.StickerMessage != nil:
		stk := msg.StickerMessage
		storeMsg.MessageType = "sticker"
		storeMsg.Mimetype = stk.GetMimetype()
		storeMsg.MediaURL = stk.GetURL()
		storeMsg.MediaDirectPath = stk.GetDirectPath()
		storeMsg.MediaKey = stk.GetMediaKey()
		storeMsg.FileSHA256 = stk.GetFileSHA256()
		storeMsg.FileEncSHA256 = stk.GetFileEncSHA256()
		storeMsg.FileLength = int64(stk.GetFileLength())
		storeMsg.Width = int(stk.GetWidth())
		storeMsg.Height = int(stk.GetHeight())
		storeMsg.IsAnimated = stk.GetIsAnimated()
		if ctx := stk.ContextInfo; ctx != nil {
			storeMsg.ForwardingScore = int(ctx.GetForwardingScore())
		}

	case msg.LocationMessage != nil:
		loc := msg.LocationMessage
		storeMsg.MessageType = "location"
		storeMsg.Latitude = loc.GetDegreesLatitude()
		storeMsg.Longitude = loc.GetDegreesLongitude()
		storeMsg.LocationName = loc.GetName()
		storeMsg.LocationAddress = loc.GetAddress()
		storeMsg.LocationURL = loc.GetURL()
		storeMsg.TextContent = loc.GetComment()
		if ctx := loc.ContextInfo; ctx != nil {
			storeMsg.ForwardingScore = int(ctx.GetForwardingScore())
		}

	case msg.LiveLocationMessage != nil:
		loc := msg.LiveLocationMessage
		storeMsg.MessageType = "live_location"
		storeMsg.Latitude = loc.GetDegreesLatitude()
		storeMsg.Longitude = loc.GetDegreesLongitude()
		storeMsg.Caption = loc.GetCaption()
		storeMsg.IsLiveLocation = true
		storeMsg.AccuracyMeters = int(loc.GetAccuracyInMeters())
		storeMsg.SpeedMPS = float64(loc.GetSpeedInMps())
		storeMsg.DegreesClockwise = int(loc.GetDegreesClockwiseFromMagneticNorth())
		storeMsg.LiveLocationSeq = int(loc.GetSequenceNumber())
		if ctx := loc.ContextInfo; ctx != nil {
			storeMsg.ForwardingScore = int(ctx.GetForwardingScore())
		}

	case msg.ContactMessage != nil:
		con := msg.ContactMessage
		storeMsg.MessageType = "contact"
		storeMsg.DisplayName = con.GetDisplayName()
		storeMsg.VCards = []string{con.GetVcard()}
		if ctx := con.ContextInfo; ctx != nil {
			storeMsg.ForwardingScore = int(ctx.GetForwardingScore())
		}

	case msg.ContactsArrayMessage != nil:
		arr := msg.ContactsArrayMessage
		storeMsg.MessageType = "contacts"
		storeMsg.DisplayName = arr.GetDisplayName()
		for _, c := range arr.Contacts {
			storeMsg.VCards = append(storeMsg.VCards, c.GetVcard())
		}
		if ctx := arr.ContextInfo; ctx != nil {
			storeMsg.ForwardingScore = int(ctx.GetForwardingScore())
		}

	default:
		storeMsg.MessageType = "unknown"
		storeMsg.ForwardingScore = 1
	}

	if err := s.messages.Put(storeMsg); err != nil {
		s.log.Warnf("Failed to save forwarded message %s: %v", result.MessageID, err)
	}
}

// parseJIDStrings converts a slice of JID strings to types.JID
func parseJIDStrings(jidStrs []string) []types.JID {
	if len(jidStrs) == 0 {
		return nil
	}
	result := make([]types.JID, 0, len(jidStrs))
	for _, s := range jidStrs {
		if jid, err := types.ParseJID(s); err == nil {
			result = append(result, jid)
		}
	}
	return result
}

// applyForwardContext applies forwarding context to the message.
func applyForwardContext(msg *waE2E.Message, ctx *waE2E.ContextInfo) {
	switch {
	case msg.Conversation != nil:
		// Convert to extended text to hold context
		msg.ExtendedTextMessage = &waE2E.ExtendedTextMessage{
			Text:        msg.Conversation,
			ContextInfo: ctx,
		}
		msg.Conversation = nil
	case msg.ExtendedTextMessage != nil:
		mergeForwardContext(msg.ExtendedTextMessage.ContextInfo, ctx)
		if msg.ExtendedTextMessage.ContextInfo == nil {
			msg.ExtendedTextMessage.ContextInfo = ctx
		}
	case msg.ImageMessage != nil:
		mergeForwardContext(msg.ImageMessage.ContextInfo, ctx)
		if msg.ImageMessage.ContextInfo == nil {
			msg.ImageMessage.ContextInfo = ctx
		}
	case msg.VideoMessage != nil:
		mergeForwardContext(msg.VideoMessage.ContextInfo, ctx)
		if msg.VideoMessage.ContextInfo == nil {
			msg.VideoMessage.ContextInfo = ctx
		}
	case msg.AudioMessage != nil:
		mergeForwardContext(msg.AudioMessage.ContextInfo, ctx)
		if msg.AudioMessage.ContextInfo == nil {
			msg.AudioMessage.ContextInfo = ctx
		}
	case msg.DocumentMessage != nil:
		mergeForwardContext(msg.DocumentMessage.ContextInfo, ctx)
		if msg.DocumentMessage.ContextInfo == nil {
			msg.DocumentMessage.ContextInfo = ctx
		}
	case msg.StickerMessage != nil:
		mergeForwardContext(msg.StickerMessage.ContextInfo, ctx)
		if msg.StickerMessage.ContextInfo == nil {
			msg.StickerMessage.ContextInfo = ctx
		}
	case msg.LocationMessage != nil:
		mergeForwardContext(msg.LocationMessage.ContextInfo, ctx)
		if msg.LocationMessage.ContextInfo == nil {
			msg.LocationMessage.ContextInfo = ctx
		}
	case msg.LiveLocationMessage != nil:
		mergeForwardContext(msg.LiveLocationMessage.ContextInfo, ctx)
		if msg.LiveLocationMessage.ContextInfo == nil {
			msg.LiveLocationMessage.ContextInfo = ctx
		}
	case msg.ContactMessage != nil:
		mergeForwardContext(msg.ContactMessage.ContextInfo, ctx)
		if msg.ContactMessage.ContextInfo == nil {
			msg.ContactMessage.ContextInfo = ctx
		}
	case msg.ContactsArrayMessage != nil:
		mergeForwardContext(msg.ContactsArrayMessage.ContextInfo, ctx)
		if msg.ContactsArrayMessage.ContextInfo == nil {
			msg.ContactsArrayMessage.ContextInfo = ctx
		}
	}
}

// mergeForwardContext merges forward context into existing context.
func mergeForwardContext(existing *waE2E.ContextInfo, forward *waE2E.ContextInfo) {
	if existing == nil {
		return
	}
	existing.IsForwarded = forward.IsForwarded
	// Increment forwarding score if already forwarded
	if existing.ForwardingScore != nil {
		existing.ForwardingScore = proto.Uint32(*existing.ForwardingScore + 1)
	} else {
		existing.ForwardingScore = forward.ForwardingScore
	}
}

// Pin pins a message in a chat.
func (s *SendService) Pin(ctx context.Context, chat types.JID, msgID types.MessageID, sender types.JID, fromMe bool) (*SendResult, error) {
	if s.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	key := &waCommon.MessageKey{
		RemoteJID: proto.String(chat.String()),
		FromMe:    proto.Bool(fromMe),
		ID:        proto.String(string(msgID)),
	}

	if !fromMe && chat.Server == types.GroupServer && !sender.IsEmpty() {
		key.Participant = proto.String(sender.String())
	}

	now := time.Now().UnixMilli()
	pinMsg := &waE2E.Message{
		PinInChatMessage: &waE2E.PinInChatMessage{
			Key:               key,
			Type:              waE2E.PinInChatMessage_PIN_FOR_ALL.Enum(),
			SenderTimestampMS: proto.Int64(now),
		},
	}

	resp, err := s.client.SendMessage(ctx, chat, pinMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to pin message: %w", err)
	}

	// Save pin to database
	if s.messages != nil {
		if err := s.messages.SetPinned(string(msgID), chat, true, resp.Timestamp); err != nil {
			s.log.Warnf("Failed to save pin for message %s: %v", msgID, err)
		}
	}

	return &SendResult{
		MessageID: resp.ID,
		ServerID:  resp.ServerID,
		Timestamp: resp.Timestamp,
		Recipient: chat,
		Sender:    resp.Sender,
		DebugInfo: resp.DebugTimings,
	}, nil
}

// PinOwn is a convenience method for pinning your own message.
func (s *SendService) PinOwn(ctx context.Context, chat types.JID, msgID types.MessageID) (*SendResult, error) {
	return s.Pin(ctx, chat, msgID, types.JID{}, true)
}

// Unpin unpins a message in a chat.
func (s *SendService) Unpin(ctx context.Context, chat types.JID, msgID types.MessageID, sender types.JID, fromMe bool) (*SendResult, error) {
	if s.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	key := &waCommon.MessageKey{
		RemoteJID: proto.String(chat.String()),
		FromMe:    proto.Bool(fromMe),
		ID:        proto.String(string(msgID)),
	}

	if !fromMe && chat.Server == types.GroupServer && !sender.IsEmpty() {
		key.Participant = proto.String(sender.String())
	}

	now := time.Now().UnixMilli()
	unpinMsg := &waE2E.Message{
		PinInChatMessage: &waE2E.PinInChatMessage{
			Key:               key,
			Type:              waE2E.PinInChatMessage_UNPIN_FOR_ALL.Enum(),
			SenderTimestampMS: proto.Int64(now),
		},
	}

	resp, err := s.client.SendMessage(ctx, chat, unpinMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to unpin message: %w", err)
	}

	// Save unpin to database
	if s.messages != nil {
		if err := s.messages.SetPinned(string(msgID), chat, false, time.Time{}); err != nil {
			s.log.Warnf("Failed to save unpin for message %s: %v", msgID, err)
		}
	}

	return &SendResult{
		MessageID: resp.ID,
		ServerID:  resp.ServerID,
		Timestamp: resp.Timestamp,
		Recipient: chat,
		Sender:    resp.Sender,
		DebugInfo: resp.DebugTimings,
	}, nil
}

// UnpinOwn is a convenience method for unpinning your own message.
func (s *SendService) UnpinOwn(ctx context.Context, chat types.JID, msgID types.MessageID) (*SendResult, error) {
	return s.Unpin(ctx, chat, msgID, types.JID{}, true)
}

// Star stars a message (keep in chat).
func (s *SendService) Star(ctx context.Context, chat types.JID, msgID types.MessageID, sender types.JID, fromMe bool) (*SendResult, error) {
	if s.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	key := &waCommon.MessageKey{
		RemoteJID: proto.String(chat.String()),
		FromMe:    proto.Bool(fromMe),
		ID:        proto.String(string(msgID)),
	}

	if !fromMe && chat.Server == types.GroupServer && !sender.IsEmpty() {
		key.Participant = proto.String(sender.String())
	}

	now := time.Now().UnixMilli()
	keepType := waE2E.KeepType_KEEP_FOR_ALL
	keepMsg := &waE2E.Message{
		KeepInChatMessage: &waE2E.KeepInChatMessage{
			Key:         key,
			KeepType:    &keepType,
			TimestampMS: proto.Int64(now),
		},
	}

	resp, err := s.client.SendMessage(ctx, chat, keepMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to star message: %w", err)
	}

	// Save star to database
	if s.messages != nil {
		if err := s.messages.SetStarred(string(msgID), chat, true); err != nil {
			s.log.Warnf("Failed to save star for message %s: %v", msgID, err)
		}
	}

	return &SendResult{
		MessageID: resp.ID,
		ServerID:  resp.ServerID,
		Timestamp: resp.Timestamp,
		Recipient: chat,
		Sender:    resp.Sender,
		DebugInfo: resp.DebugTimings,
	}, nil
}

// Unstar removes star from a message.
func (s *SendService) Unstar(ctx context.Context, chat types.JID, msgID types.MessageID, sender types.JID, fromMe bool) (*SendResult, error) {
	if s.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	key := &waCommon.MessageKey{
		RemoteJID: proto.String(chat.String()),
		FromMe:    proto.Bool(fromMe),
		ID:        proto.String(string(msgID)),
	}

	if !fromMe && chat.Server == types.GroupServer && !sender.IsEmpty() {
		key.Participant = proto.String(sender.String())
	}

	now := time.Now().UnixMilli()
	keepType := waE2E.KeepType_UNDO_KEEP_FOR_ALL
	keepMsg := &waE2E.Message{
		KeepInChatMessage: &waE2E.KeepInChatMessage{
			Key:         key,
			KeepType:    &keepType,
			TimestampMS: proto.Int64(now),
		},
	}

	resp, err := s.client.SendMessage(ctx, chat, keepMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to unstar message: %w", err)
	}

	// Save unstar to database
	if s.messages != nil {
		if err := s.messages.SetStarred(string(msgID), chat, false); err != nil {
			s.log.Warnf("Failed to save unstar for message %s: %v", msgID, err)
		}
	}

	return &SendResult{
		MessageID: resp.ID,
		ServerID:  resp.ServerID,
		Timestamp: resp.Timestamp,
		Recipient: chat,
		Sender:    resp.Sender,
		DebugInfo: resp.DebugTimings,
	}, nil
}
