package send

import (
	"context"
	"fmt"
	"time"

	"go.mau.fi/whatsmeow/types"
)

// SetAvailable marks the user as online.
func (s *SendService) SetAvailable(ctx context.Context) error {
	if s.client == nil {
		return fmt.Errorf("client not initialized")
	}
	return s.client.SendPresence(ctx, types.PresenceAvailable)
}

// SetUnavailable marks the user as offline.
func (s *SendService) SetUnavailable(ctx context.Context) error {
	if s.client == nil {
		return fmt.Errorf("client not initialized")
	}
	return s.client.SendPresence(ctx, types.PresenceUnavailable)
}

// StartTyping shows the typing indicator in a chat.
func (s *SendService) StartTyping(ctx context.Context, chat types.JID) error {
	if s.client == nil {
		return fmt.Errorf("client not initialized")
	}
	return s.client.SendChatPresence(ctx, chat, types.ChatPresenceComposing, types.ChatPresenceMediaText)
}

// StopTyping clears the typing indicator in a chat.
func (s *SendService) StopTyping(ctx context.Context, chat types.JID) error {
	if s.client == nil {
		return fmt.Errorf("client not initialized")
	}
	return s.client.SendChatPresence(ctx, chat, types.ChatPresencePaused, types.ChatPresenceMediaText)
}

// StartRecording shows the recording indicator in a chat.
func (s *SendService) StartRecording(ctx context.Context, chat types.JID) error {
	if s.client == nil {
		return fmt.Errorf("client not initialized")
	}
	return s.client.SendChatPresence(ctx, chat, types.ChatPresenceComposing, types.ChatPresenceMediaAudio)
}

// StopRecording clears the recording indicator in a chat.
func (s *SendService) StopRecording(ctx context.Context, chat types.JID) error {
	if s.client == nil {
		return fmt.Errorf("client not initialized")
	}
	return s.client.SendChatPresence(ctx, chat, types.ChatPresencePaused, types.ChatPresenceMediaAudio)
}

// MarkRead marks messages as read.
func (s *SendService) MarkRead(ctx context.Context, chat types.JID, sender types.JID, messageIDs ...types.MessageID) error {
	if s.client == nil {
		return fmt.Errorf("client not initialized")
	}
	if len(messageIDs) == 0 {
		return nil
	}
	return s.client.MarkRead(ctx, messageIDs, time.Now(), chat, sender)
}

// MarkReadSingle is a convenience method for marking a single message as read.
func (s *SendService) MarkReadSingle(ctx context.Context, chat types.JID, sender types.JID, messageID types.MessageID) error {
	return s.MarkRead(ctx, chat, sender, messageID)
}

// SubscribePresence subscribes to presence updates for a user.
func (s *SendService) SubscribePresence(ctx context.Context, jid types.JID) error {
	if s.client == nil {
		return fmt.Errorf("client not initialized")
	}
	return s.client.SubscribePresence(ctx, jid)
}
