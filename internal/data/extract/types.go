// Package types provides common type definitions for the extract package.
package extract

import (
	"go.mau.fi/whatsmeow/types"
)

// MediaInfo contains extracted media information.
type MediaInfo struct {
	Type      string // image, video, audio, document, sticker
	FileName  string
	MimeType  string
	FileSize  int64
	MediaKey  []byte
	URL       string
	Height    int
	Width     int
	Duration  int // seconds for audio/video
	PageCount int // for documents
	Thumbnail []byte
}

// QuoteInfo contains quoted message information.
type QuoteInfo struct {
	MessageID   string
	SenderJID   types.JID
	MessageType string
	Content     string
}

// MentionInfo contains mention information.
type MentionInfo struct {
	MentionedJIDs []types.JID
	GroupMentions []GroupMentionInfo
}

// GroupMentionInfo contains group mention information.
type GroupMentionInfo struct {
	GroupJID types.JID
	Subject  string
}

// ContextInfo contains message context information.
type ContextInfo struct {
	Quote         *QuoteInfo
	Mentions      *MentionInfo
	IsForwarded   bool
	ForwardedFrom types.JID
	ForwardScore  int
}

// LocationInfo contains location information.
type LocationInfo struct {
	Latitude         float64
	Longitude        float64
	Name             string
	Address          string
	URL              string
	AccuracyInMeters int
}

// PollInfo contains poll information.
type PollInfo struct {
	Name            string
	Options         []string
	SelectableCount int
}

// ContactCardInfo contains contact card information.
type ContactCardInfo struct {
	DisplayName string
	VCard       string
	Contacts    []string // Multiple contacts if ContactsArrayMessage
}
