package send

import (
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// ContextInfo provides context for message replies, quotes, mentions, and forwards.
type ContextInfo struct {
	// Reply context
	QuotedMessageID   types.MessageID
	QuotedMessage     *waE2E.Message
	QuotedParticipant types.JID
	RemoteJID         types.JID // Chat where quoted message is from

	// Mentions
	MentionedJIDs []types.JID

	// Forwarding
	IsForwarded     bool
	ForwardingScore uint32

	// Ephemeral (disappearing messages)
	Expiration uint32
}

// NewContext creates a new empty ContextInfo.
func NewContext() *ContextInfo {
	return &ContextInfo{}
}

// ReplyTo sets up the context as a reply to a message.
func (c *ContextInfo) ReplyTo(msgID types.MessageID, sender types.JID, quotedMsg *waE2E.Message) *ContextInfo {
	c.QuotedMessageID = msgID
	c.QuotedParticipant = sender
	c.QuotedMessage = quotedMsg
	return c
}

// WithRemoteJID sets the remote JID for cross-chat replies.
func (c *ContextInfo) WithRemoteJID(jid types.JID) *ContextInfo {
	c.RemoteJID = jid
	return c
}

// WithMentions adds mentioned JIDs.
func (c *ContextInfo) WithMentions(jids ...types.JID) *ContextInfo {
	c.MentionedJIDs = append(c.MentionedJIDs, jids...)
	return c
}

// AsForward marks the message as forwarded.
func (c *ContextInfo) AsForward(score uint32) *ContextInfo {
	c.IsForwarded = true
	c.ForwardingScore = score
	return c
}

// WithExpiration sets ephemeral message expiration (in seconds).
func (c *ContextInfo) WithExpiration(seconds uint32) *ContextInfo {
	c.Expiration = seconds
	return c
}

// Build converts to a waE2E.ContextInfo protobuf.
func (c *ContextInfo) Build() *waE2E.ContextInfo {
	if c == nil {
		return nil
	}

	// Check if there's anything to build
	hasContent := c.QuotedMessageID != "" ||
		len(c.MentionedJIDs) > 0 ||
		c.IsForwarded ||
		c.Expiration > 0

	if !hasContent {
		return nil
	}

	ctx := &waE2E.ContextInfo{}

	// Reply context
	if c.QuotedMessageID != "" {
		stanzaID := string(c.QuotedMessageID)
		ctx.StanzaID = &stanzaID

		if !c.QuotedParticipant.IsEmpty() {
			participant := c.QuotedParticipant.String()
			ctx.Participant = &participant
		}

		if c.QuotedMessage != nil {
			ctx.QuotedMessage = c.QuotedMessage
		}

		if !c.RemoteJID.IsEmpty() {
			remoteJID := c.RemoteJID.String()
			ctx.RemoteJID = &remoteJID
		}
	}

	// Mentions
	if len(c.MentionedJIDs) > 0 {
		mentions := make([]string, len(c.MentionedJIDs))
		for i, jid := range c.MentionedJIDs {
			mentions[i] = jid.String()
		}
		ctx.MentionedJID = mentions
	}

	// Forwarding
	if c.IsForwarded {
		ctx.IsForwarded = proto.Bool(true)
		if c.ForwardingScore > 0 {
			ctx.ForwardingScore = proto.Uint32(c.ForwardingScore)
		}
	}

	// Ephemeral
	if c.Expiration > 0 {
		ctx.Expiration = proto.Uint32(c.Expiration)
	}

	return ctx
}

// ReplyContext is a convenience builder for reply context.
func ReplyContext(msgID types.MessageID, sender types.JID, quotedMsg *waE2E.Message) *ContextInfo {
	return NewContext().ReplyTo(msgID, sender, quotedMsg)
}

// ForwardContext is a convenience builder for forward context.
func ForwardContext(score uint32) *ContextInfo {
	return NewContext().AsForward(score)
}

// MentionContext is a convenience builder for mentions.
func MentionContext(jids ...types.JID) *ContextInfo {
	return NewContext().WithMentions(jids...)
}
