package send

import (
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"

	"orion-agent/internal/utils/media"
)

// TextContent represents a simple text message.
type TextContent struct {
	Text        string
	ContextInfo *ContextInfo
}

// Text creates a simple text message.
func Text(text string) *TextContent {
	return &TextContent{Text: text}
}

// WithContext adds context info to the text message.
func (t *TextContent) WithContext(ctx *ContextInfo) *TextContent {
	t.ContextInfo = ctx
	return t
}

// ToMessage implements Content.
func (t *TextContent) ToMessage() (*waE2E.Message, error) {
	msg := &waE2E.Message{
		Conversation: proto.String(t.Text),
	}

	// If we have context (reply, mentions, etc.), use ExtendedTextMessage
	if t.ContextInfo != nil && t.ContextInfo.Build() != nil {
		msg.Conversation = nil
		msg.ExtendedTextMessage = &waE2E.ExtendedTextMessage{
			Text:        proto.String(t.Text),
			ContextInfo: t.ContextInfo.Build(),
		}
	}

	return msg, nil
}

// MediaType implements Content.
func (t *TextContent) MediaType() media.Type {
	return "" // Not a media message
}

// ExtendedTextContent represents a text message with link preview, mentions, etc.
type ExtendedTextContent struct {
	Text          string
	Title         string
	Description   string
	CanonicalURL  string
	MatchedText   string
	PreviewType   waE2E.ExtendedTextMessage_PreviewType
	ThumbnailJPEG []byte
	MentionedJIDs []types.JID
	ContextInfo   *ContextInfo
}

// ExtendedText creates an extended text message.
func ExtendedText(text string) *ExtendedTextContent {
	return &ExtendedTextContent{Text: text}
}

// TextWithPreview creates a text message with link preview.
func TextWithPreview(text, url, title, description string) *ExtendedTextContent {
	return &ExtendedTextContent{
		Text:         text,
		CanonicalURL: url,
		MatchedText:  url,
		Title:        title,
		Description:  description,
		PreviewType:  waE2E.ExtendedTextMessage_VIDEO,
	}
}

// TextWithMentions creates a text message with mentions.
func TextWithMentions(text string, mentions ...types.JID) *ExtendedTextContent {
	return &ExtendedTextContent{
		Text:          text,
		MentionedJIDs: mentions,
	}
}

// WithTitle sets the link preview title.
func (e *ExtendedTextContent) WithTitle(title string) *ExtendedTextContent {
	e.Title = title
	return e
}

// WithDescription sets the link preview description.
func (e *ExtendedTextContent) WithDescription(desc string) *ExtendedTextContent {
	e.Description = desc
	return e
}

// WithURL sets the canonical URL for link preview.
func (e *ExtendedTextContent) WithURL(url string) *ExtendedTextContent {
	e.CanonicalURL = url
	e.MatchedText = url
	return e
}

// WithThumbnail sets the link preview thumbnail.
func (e *ExtendedTextContent) WithThumbnail(jpeg []byte) *ExtendedTextContent {
	e.ThumbnailJPEG = jpeg
	return e
}

// WithMentions adds mentioned JIDs.
func (e *ExtendedTextContent) WithMentions(jids ...types.JID) *ExtendedTextContent {
	e.MentionedJIDs = append(e.MentionedJIDs, jids...)
	return e
}

// WithContext adds context info.
func (e *ExtendedTextContent) WithContext(ctx *ContextInfo) *ExtendedTextContent {
	e.ContextInfo = ctx
	return e
}

// ToMessage implements Content.
func (e *ExtendedTextContent) ToMessage() (*waE2E.Message, error) {
	ext := &waE2E.ExtendedTextMessage{
		Text: proto.String(e.Text),
	}

	// Link preview
	if e.CanonicalURL != "" {
		ext.MatchedText = proto.String(e.MatchedText)
		ext.Title = proto.String(e.Title)
		ext.Description = proto.String(e.Description)
		ext.PreviewType = &e.PreviewType
	}

	if len(e.ThumbnailJPEG) > 0 {
		ext.JPEGThumbnail = e.ThumbnailJPEG
	}

	// Build context
	var ctxInfo *waE2E.ContextInfo
	if e.ContextInfo != nil {
		ctxInfo = e.ContextInfo.Build()
	}

	// Add mentions to context
	if len(e.MentionedJIDs) > 0 {
		if ctxInfo == nil {
			ctxInfo = &waE2E.ContextInfo{}
		}
		mentions := make([]string, len(e.MentionedJIDs))
		for i, jid := range e.MentionedJIDs {
			mentions[i] = jid.String()
		}
		ctxInfo.MentionedJID = mentions
	}

	if ctxInfo != nil {
		ext.ContextInfo = ctxInfo
	}

	return &waE2E.Message{
		ExtendedTextMessage: ext,
	}, nil
}

// MediaType implements Content.
func (e *ExtendedTextContent) MediaType() media.Type {
	return "" // Not a media message
}
