package send

import (
	"context"
	"fmt"
	"time"

	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
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

	return &SendResult{
		MessageID: resp.ID,
		ServerID:  resp.ServerID,
		Timestamp: resp.Timestamp,
		Recipient: to,
		Sender:    resp.Sender,
		DebugInfo: resp.DebugTimings,
	}, nil
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

	return &SendResult{
		MessageID: resp.ID,
		ServerID:  resp.ServerID,
		Timestamp: resp.Timestamp,
		Recipient: chat,
		Sender:    resp.Sender,
		DebugInfo: resp.DebugTimings,
	}, nil
}
