package send

import (
	"context"
	"fmt"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"

	"orion-agent/internal/data/store"
	"orion-agent/internal/utils"
)

// SendService provides a high-level API for sending messages via WhatsApp.
type SendService struct {
	client   *whatsmeow.Client
	utils    *utils.Utils
	messages *store.MessageStore
	log      waLog.Logger
}

// NewSendService creates a new SendService.
func NewSendService(client *whatsmeow.Client, utils *utils.Utils, messages *store.MessageStore, log waLog.Logger) *SendService {
	return &SendService{
		client:   client,
		utils:    utils,
		messages: messages,
		log:      log.Sub("SendService"),
	}
}

// SetClient updates the whatsmeow client (for delayed initialization).
func (s *SendService) SetClient(client *whatsmeow.Client) {
	s.client = client
}

// Client returns the underlying whatsmeow client.
func (s *SendService) Client() *whatsmeow.Client {
	return s.client
}

// Send sends content to a recipient.
func (s *SendService) Send(ctx context.Context, to types.JID, content Content, opts ...SendOption) (*SendResult, error) {
	if s.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	// Upload media if needed (before building message)
	if content.MediaType() != "" {
		if uploader, ok := content.(MediaUploader); ok && !uploader.IsUploaded() {
			if err := uploader.Upload(ctx, s.client); err != nil {
				return nil, fmt.Errorf("failed to upload media: %w", err)
			}
		}
	}

	// Build the message
	msg, err := content.ToMessage()
	if err != nil {
		return nil, fmt.Errorf("failed to build message: %w", err)
	}

	// Apply options
	cfg := applyOptions(opts)
	extra := cfg.toSendRequestExtra()

	// Send
	resp, err := s.client.SendMessage(ctx, to, msg, extra)
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	result := &SendResult{
		MessageID: resp.ID,
		ServerID:  resp.ServerID,
		Timestamp: resp.Timestamp,
		Recipient: to,
		Sender:    resp.Sender,
		DebugInfo: resp.DebugTimings,
	}

	// Save sent message to database
	s.saveSentMessage(result, content)

	return result, nil
}

// saveSentMessage saves a sent message to the database.
func (s *SendService) saveSentMessage(result *SendResult, content Content) {
	if s.messages == nil {
		return
	}

	ownJID := s.utils.OwnJID()

	msg := &store.Message{
		ID:          result.MessageID,
		ChatJID:     result.Recipient,
		SenderLID:   ownJID,
		FromMe:      true,
		Timestamp:   result.Timestamp,
		ServerID:    int(result.ServerID),
		MessageType: content.MessageType(),
		TextContent: content.TextContent(),
		Caption:     content.GetCaption(),
	}

	// Add mentioned JIDs
	if mentions := content.GetMentionedJIDs(); len(mentions) > 0 {
		msg.MentionedJIDs = mentions
	}

	// Add reply/forward context
	if ctx := content.GetContextInfo(); ctx != nil {
		if ctx.QuotedMessageID != "" {
			msg.QuotedMessageID = string(ctx.QuotedMessageID)
		}
		if !ctx.QuotedParticipant.IsEmpty() {
			msg.QuotedSenderLID = ctx.QuotedParticipant
		}
		msg.IsForwarded = ctx.IsForwarded
		msg.ForwardingScore = int(ctx.ForwardingScore)
		if ctx.Expiration > 0 {
			msg.IsEphemeral = true
		}
	}

	// Type-specific fields
	switch c := content.(type) {
	case *ImageContent:
		msg.Mimetype = c.MimeType
		msg.Width = int(c.Width)
		msg.Height = int(c.Height)
		msg.IsViewOnce = c.ViewOnce
		if c.uploaded != nil {
			msg.MediaURL = c.uploaded.URL
			msg.MediaDirectPath = c.uploaded.DirectPath
			msg.MediaKey = c.uploaded.MediaKey
			msg.FileSHA256 = c.uploaded.FileSHA256
			msg.FileEncSHA256 = c.uploaded.FileEncSHA256
			msg.FileLength = int64(c.uploaded.FileLength)
		}

	case *VideoContent:
		msg.Mimetype = c.MimeType
		msg.Width = int(c.Width)
		msg.Height = int(c.Height)
		msg.DurationSeconds = int(c.DurationSeconds)
		msg.IsViewOnce = c.ViewOnce
		msg.IsGIF = c.GifPlayback
		if c.uploaded != nil {
			msg.MediaURL = c.uploaded.URL
			msg.MediaDirectPath = c.uploaded.DirectPath
			msg.MediaKey = c.uploaded.MediaKey
			msg.FileSHA256 = c.uploaded.FileSHA256
			msg.FileEncSHA256 = c.uploaded.FileEncSHA256
			msg.FileLength = int64(c.uploaded.FileLength)
		}

	case *AudioContent:
		msg.Mimetype = c.MimeType
		msg.DurationSeconds = int(c.DurationSeconds)
		msg.IsPTT = c.IsPTT
		msg.Waveform = c.Waveform
		if c.uploaded != nil {
			msg.MediaURL = c.uploaded.URL
			msg.MediaDirectPath = c.uploaded.DirectPath
			msg.MediaKey = c.uploaded.MediaKey
			msg.FileSHA256 = c.uploaded.FileSHA256
			msg.FileEncSHA256 = c.uploaded.FileEncSHA256
			msg.FileLength = int64(c.uploaded.FileLength)
		}

	case *DocumentContent:
		msg.Mimetype = c.MimeType
		msg.DisplayName = c.Filename
		if c.uploaded != nil {
			msg.MediaURL = c.uploaded.URL
			msg.MediaDirectPath = c.uploaded.DirectPath
			msg.MediaKey = c.uploaded.MediaKey
			msg.FileSHA256 = c.uploaded.FileSHA256
			msg.FileEncSHA256 = c.uploaded.FileEncSHA256
			msg.FileLength = int64(c.uploaded.FileLength)
		}

	case *StickerContent:
		msg.Mimetype = c.MimeType
		msg.Width = int(c.Width)
		msg.Height = int(c.Height)
		msg.IsAnimated = c.IsAnimated
		if c.uploaded != nil {
			msg.MediaURL = c.uploaded.URL
			msg.MediaDirectPath = c.uploaded.DirectPath
			msg.MediaKey = c.uploaded.MediaKey
			msg.FileSHA256 = c.uploaded.FileSHA256
			msg.FileEncSHA256 = c.uploaded.FileEncSHA256
			msg.FileLength = int64(c.uploaded.FileLength)
		}

	case *LocationContent:
		msg.Latitude = c.Latitude
		msg.Longitude = c.Longitude
		msg.LocationName = c.Name
		msg.LocationAddress = c.Address
		msg.LocationURL = c.URL

	case *LiveLocationContent:
		msg.Latitude = c.Latitude
		msg.Longitude = c.Longitude
		msg.IsLiveLocation = true
		msg.AccuracyMeters = int(c.AccuracyInMeters)
		msg.SpeedMPS = float64(c.SpeedInMps)
		msg.DegreesClockwise = int(c.Heading)
		msg.LiveLocationSeq = int(c.SequenceNumber)

	case *ContactContent:
		msg.VCards = []string{c.VCard}
		msg.DisplayName = c.DisplayName

	case *ContactsArrayContent:
		msg.DisplayName = c.DisplayName
		for _, contact := range c.Contacts {
			msg.VCards = append(msg.VCards, contact.VCard)
		}

	case *PollContent:
		msg.PollName = c.Question
		msg.PollOptions = c.Options
		msg.PollSelectMax = c.SelectableCount
		msg.PollEncryptionKey = c.encKey

	case *ExtendedTextContent:
		msg.PreviewTitle = c.Title
		msg.PreviewDescription = c.Description
		msg.PreviewURL = c.CanonicalURL
		msg.PreviewMatchedText = c.MatchedText
		msg.Thumbnail = c.ThumbnailJPEG

	case *GroupInviteContent:
		msg.InviteGroupJID = c.GroupJID
		msg.InviteCode = c.InviteCode
		msg.InviteExpiration = c.Expiration
		msg.DisplayName = c.GroupName
		msg.Thumbnail = c.ThumbnailJPEG

	case *EventContent:
		msg.EventName = c.Name
		msg.EventDescription = c.Description
		msg.EventStartTime = c.StartTime.Unix()
		msg.EventEndTime = c.EndTime.Unix()
		msg.EventJoinLink = c.JoinLink
		msg.EventIsCanceled = c.IsCanceled
		if c.Location != nil {
			msg.Latitude = c.Location.Latitude
			msg.Longitude = c.Location.Longitude
			msg.LocationName = c.Location.Name
			msg.LocationAddress = c.Location.Address
		}
	}

	if err := s.messages.Put(msg); err != nil {
		s.log.Warnf("Failed to save sent message %s: %v", result.MessageID, err)
	}
}

// SendToGroup sends content to a group.
func (s *SendService) SendToGroup(ctx context.Context, groupJID types.JID, content Content, opts ...SendOption) (*SendResult, error) {
	if groupJID.Server != types.GroupServer {
		return nil, fmt.Errorf("not a group JID: %s", groupJID)
	}
	return s.Send(ctx, groupJID, content, opts...)
}

// SendToNewsletter sends content to a newsletter/channel.
func (s *SendService) SendToNewsletter(ctx context.Context, newsletterJID types.JID, content Content, opts ...SendOption) (*SendResult, error) {
	if newsletterJID.Server != types.NewsletterServer {
		return nil, fmt.Errorf("not a newsletter JID: %s", newsletterJID)
	}
	return s.Send(ctx, newsletterJID, content, opts...)
}

// SendMany sends content to multiple recipients.
func (s *SendService) SendMany(ctx context.Context, recipients []types.JID, content Content, opts ...SendOption) ([]*SendResult, error) {
	if len(recipients) == 0 {
		return nil, fmt.Errorf("no recipients provided")
	}

	// For media, upload once then reuse
	if content.MediaType() != "" {
		if uploader, ok := content.(MediaUploader); ok && !uploader.IsUploaded() {
			if err := uploader.Upload(ctx, s.client); err != nil {
				return nil, fmt.Errorf("failed to upload media: %w", err)
			}
		}
	}

	results := make([]*SendResult, 0, len(recipients))
	var errors []error

	for _, to := range recipients {
		result, err := s.Send(ctx, to, content, opts...)
		if err != nil {
			s.log.Warnf("Failed to send to %s: %v", to, err)
			errors = append(errors, fmt.Errorf("%s: %w", to, err))
			continue
		}
		results = append(results, result)
	}

	if len(results) == 0 && len(errors) > 0 {
		return nil, fmt.Errorf("all sends failed, first error: %w", errors[0])
	}

	return results, nil
}

// SendManyCollectErrors sends to multiple recipients and returns all errors.
type SendManyResult struct {
	Successful []*SendResult
	Failed     map[types.JID]error
}

// SendManyWithErrors sends content to multiple recipients and collects all errors.
func (s *SendService) SendManyWithErrors(ctx context.Context, recipients []types.JID, content Content, opts ...SendOption) *SendManyResult {
	result := &SendManyResult{
		Successful: make([]*SendResult, 0),
		Failed:     make(map[types.JID]error),
	}

	if len(recipients) == 0 {
		return result
	}

	// For media, upload once then reuse
	if content.MediaType() != "" {
		if uploader, ok := content.(MediaUploader); ok && !uploader.IsUploaded() {
			if err := uploader.Upload(ctx, s.client); err != nil {
				for _, to := range recipients {
					result.Failed[to] = fmt.Errorf("upload failed: %w", err)
				}
				return result
			}
		}
	}

	for _, to := range recipients {
		sendResult, err := s.Send(ctx, to, content, opts...)
		if err != nil {
			result.Failed[to] = err
		} else {
			result.Successful = append(result.Successful, sendResult)
		}
	}

	return result
}

// MediaUploader is implemented by content types that need to upload media.
type MediaUploader interface {
	Upload(ctx context.Context, client *whatsmeow.Client) error
	IsUploaded() bool
}

// Broadcast sends content to a broadcast list.
func (s *SendService) Broadcast(ctx context.Context, recipients []types.JID, content Content, opts ...SendOption) ([]*SendResult, error) {
	if len(recipients) == 0 {
		return nil, fmt.Errorf("no recipients provided")
	}

	// For media, upload once then reuse
	if content.MediaType() != "" {
		if uploader, ok := content.(MediaUploader); ok && !uploader.IsUploaded() {
			if err := uploader.Upload(ctx, s.client); err != nil {
				return nil, fmt.Errorf("failed to upload media: %w", err)
			}
		}
	}

	results := make([]*SendResult, 0, len(recipients))
	for _, to := range recipients {
		result, err := s.Send(ctx, to, content, opts...)
		if err != nil {
			s.log.Warnf("Broadcast to %s failed: %v", to, err)
			continue
		}
		results = append(results, result)
	}

	return results, nil
}
