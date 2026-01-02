package send

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"

	"orion-agent/internal/utils/media"
)

// PollContent represents a poll creation message.
type PollContent struct {
	Question        string
	Options         []string
	SelectableCount int
	ContextInfo     *ContextInfo

	encKey []byte
}

// Poll creates a single-select poll.
func Poll(question string, options []string) *PollContent {
	return &PollContent{
		Question:        question,
		Options:         options,
		SelectableCount: 1,
	}
}

// PollMultiSelect creates a multi-select poll.
func PollMultiSelect(question string, options []string, maxSelect int) *PollContent {
	return &PollContent{
		Question:        question,
		Options:         options,
		SelectableCount: maxSelect,
	}
}

// WithContext adds context info.
func (p *PollContent) WithContext(ctx *ContextInfo) *PollContent {
	p.ContextInfo = ctx
	return p
}

// ToMessage implements Content.
func (p *PollContent) ToMessage() (*waE2E.Message, error) {
	if p.encKey == nil {
		p.encKey = make([]byte, 32)
		if _, err := rand.Read(p.encKey); err != nil {
			return nil, fmt.Errorf("failed to generate encryption key: %w", err)
		}
	}

	options := make([]*waE2E.PollCreationMessage_Option, len(p.Options))
	for i, opt := range p.Options {
		options[i] = &waE2E.PollCreationMessage_Option{
			OptionName: proto.String(opt),
		}
	}

	poll := &waE2E.PollCreationMessage{
		EncKey:                 p.encKey,
		Name:                   proto.String(p.Question),
		Options:                options,
		SelectableOptionsCount: proto.Uint32(uint32(p.SelectableCount)),
	}

	if p.ContextInfo != nil {
		poll.ContextInfo = p.ContextInfo.Build()
	}

	return &waE2E.Message{PollCreationMessage: poll}, nil
}

// MediaType implements Content.
func (p *PollContent) MediaType() media.Type {
	return ""
}

// EncryptionKey returns the poll encryption key (needed for voting).
func (p *PollContent) EncryptionKey() []byte {
	return p.encKey
}

// SetEncryptionKey sets the poll encryption key (for pre-generated keys).
func (p *PollContent) SetEncryptionKey(key []byte) *PollContent {
	p.encKey = key
	return p
}

// SendPollVote sends a vote to a poll using whatsmeow's built-in method.
// Note: pollInfo must contain the original poll message info with the encrypted poll data.
func (s *SendService) SendPollVote(ctx context.Context, pollInfo *types.MessageInfo, selectedOptions []string) (*SendResult, error) {
	if s.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	// Build the vote message
	voteMsg, err := s.client.BuildPollVote(ctx, pollInfo, selectedOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to build poll vote: %w", err)
	}

	resp, err := s.client.SendMessage(ctx, pollInfo.Chat, voteMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to send poll vote: %w", err)
	}

	return &SendResult{
		MessageID: resp.ID,
		ServerID:  resp.ServerID,
		Timestamp: resp.Timestamp,
		Recipient: pollInfo.Chat,
		Sender:    resp.Sender,
		DebugInfo: resp.DebugTimings,
	}, nil
}

// GroupInviteContent represents a group invite message.
type GroupInviteContent struct {
	GroupJID      types.JID
	GroupName     string
	InviteCode    string
	Expiration    int64
	Caption       string
	ThumbnailJPEG []byte
	ContextInfo   *ContextInfo
}

// GroupInvite creates a group invite message.
func GroupInvite(groupJID types.JID, inviteCode, groupName string) *GroupInviteContent {
	return &GroupInviteContent{
		GroupJID:   groupJID,
		GroupName:  groupName,
		InviteCode: inviteCode,
	}
}

// WithExpiration sets the invite expiration time.
func (g *GroupInviteContent) WithExpiration(exp time.Time) *GroupInviteContent {
	g.Expiration = exp.Unix()
	return g
}

// WithCaption adds a caption.
func (g *GroupInviteContent) WithCaption(caption string) *GroupInviteContent {
	g.Caption = caption
	return g
}

// WithThumbnail sets the group thumbnail.
func (g *GroupInviteContent) WithThumbnail(jpeg []byte) *GroupInviteContent {
	g.ThumbnailJPEG = jpeg
	return g
}

// WithContext adds context info.
func (g *GroupInviteContent) WithContext(ctx *ContextInfo) *GroupInviteContent {
	g.ContextInfo = ctx
	return g
}

// ToMessage implements Content.
func (g *GroupInviteContent) ToMessage() (*waE2E.Message, error) {
	invite := &waE2E.GroupInviteMessage{
		GroupJID:   proto.String(g.GroupJID.String()),
		GroupName:  proto.String(g.GroupName),
		InviteCode: proto.String(g.InviteCode),
	}

	if g.Expiration > 0 {
		invite.InviteExpiration = proto.Int64(g.Expiration)
	}
	if g.Caption != "" {
		invite.Caption = proto.String(g.Caption)
	}
	if len(g.ThumbnailJPEG) > 0 {
		invite.JPEGThumbnail = g.ThumbnailJPEG
	}
	if g.ContextInfo != nil {
		invite.ContextInfo = g.ContextInfo.Build()
	}

	return &waE2E.Message{GroupInviteMessage: invite}, nil
}

// MediaType implements Content.
func (g *GroupInviteContent) MediaType() media.Type {
	return ""
}

// BuildGroupInvite builds a group invite from group info.
func (s *SendService) BuildGroupInvite(ctx context.Context, groupJID types.JID) (*GroupInviteContent, error) {
	if s.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	// Get group info
	groupInfo, err := s.client.GetGroupInfo(ctx, groupJID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group info: %w", err)
	}

	// Get invite link
	inviteCode, err := s.client.GetGroupInviteLink(ctx, groupJID, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get invite link: %w", err)
	}

	return GroupInvite(groupJID, inviteCode, groupInfo.Name), nil
}

// EventContent represents an event message.
type EventContent struct {
	Name               string
	Description        string
	Location           *LocationContent
	StartTime          time.Time
	EndTime            time.Time
	JoinLink           string
	ExtraGuestsAllowed bool
	IsCanceled         bool
	ContextInfo        *ContextInfo
}

// Event creates an event message.
func Event(name string, startTime, endTime time.Time) *EventContent {
	return &EventContent{
		Name:      name,
		StartTime: startTime,
		EndTime:   endTime,
	}
}

// WithDescription adds a description.
func (e *EventContent) WithDescription(desc string) *EventContent {
	e.Description = desc
	return e
}

// WithLocation adds a location.
func (e *EventContent) WithLocation(loc *LocationContent) *EventContent {
	e.Location = loc
	return e
}

// WithJoinLink adds a join link.
func (e *EventContent) WithJoinLink(link string) *EventContent {
	e.JoinLink = link
	return e
}

// AllowExtraGuests allows invitees to bring guests.
func (e *EventContent) AllowExtraGuests() *EventContent {
	e.ExtraGuestsAllowed = true
	return e
}

// AsCanceled marks the event as canceled.
func (e *EventContent) AsCanceled() *EventContent {
	e.IsCanceled = true
	return e
}

// WithContext adds context info.
func (e *EventContent) WithContext(ctx *ContextInfo) *EventContent {
	e.ContextInfo = ctx
	return e
}

// ToMessage implements Content.
func (e *EventContent) ToMessage() (*waE2E.Message, error) {
	event := &waE2E.EventMessage{
		Name:               proto.String(e.Name),
		StartTime:          proto.Int64(e.StartTime.Unix()),
		EndTime:            proto.Int64(e.EndTime.Unix()),
		ExtraGuestsAllowed: proto.Bool(e.ExtraGuestsAllowed),
		IsCanceled:         proto.Bool(e.IsCanceled),
	}

	if e.Description != "" {
		event.Description = proto.String(e.Description)
	}
	if e.JoinLink != "" {
		event.JoinLink = proto.String(e.JoinLink)
	}
	if e.Location != nil {
		locMsg, err := e.Location.ToMessage()
		if err == nil && locMsg.LocationMessage != nil {
			event.Location = locMsg.LocationMessage
		}
	}
	if e.ContextInfo != nil {
		event.ContextInfo = e.ContextInfo.Build()
	}

	return &waE2E.Message{EventMessage: event}, nil
}

// MediaType implements Content.
func (e *EventContent) MediaType() media.Type {
	return ""
}

// GetClient returns the underlying whatsmeow client for advanced operations.
func (s *SendService) GetClient() *whatsmeow.Client {
	return s.client
}
