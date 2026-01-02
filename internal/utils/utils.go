// Package utils provides utility functions used throughout the application.
// The Utils struct aggregates all utility functionality and should be passed
// to all components that need JID normalization, media handling, or retry logic.
package utils

import (
	"context"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// ContactGetter is an interface for getting contact JID mappings.
type ContactGetter interface {
	GetLIDForPN(pn types.JID) (types.JID, error)
	GetPNForLID(lid types.JID) (types.JID, error)
	UpdatePN(lid, pn types.JID) error
}

// Utils aggregates all utility functions for the application.
// Pass this to all components that need utilities.
type Utils struct {
	contacts ContactGetter
	client   *whatsmeow.Client
	cache    sync.Map // JID cache: pn.String() -> LID
}

// New creates a new Utils instance.
func New(contacts ContactGetter, client *whatsmeow.Client) *Utils {
	return &Utils{
		contacts: contacts,
		client:   client,
	}
}

// SetClient sets the WhatsApp client (for delayed initialization).
func (u *Utils) SetClient(client *whatsmeow.Client) {
	u.client = client
}

// ===========================================================================
// JID UTILITIES
// ===========================================================================

// NormalizeJID converts a PN to LID form. Returns as-is if not a PN.
func (u *Utils) NormalizeJID(ctx context.Context, jid types.JID) types.JID {
	return u.ToLID(ctx, jid)
}

// NormalizeJIDs converts multiple JIDs to LID form.
func (u *Utils) NormalizeJIDs(ctx context.Context, jids []types.JID) []types.JID {
	result := make([]types.JID, len(jids))
	for i, jid := range jids {
		result[i] = u.NormalizeJID(ctx, jid)
	}
	return result
}

// ToLID converts a PN to LID. Returns the LID if found, otherwise returns as-is.
func (u *Utils) ToLID(ctx context.Context, pn types.JID) types.JID {
	// If empty or not a PN, return as-is
	if pn.IsEmpty() || !u.IsPN(pn) {
		return pn
	}

	// Check cache first
	if cached, ok := u.cache.Load(pn.String()); ok {
		return cached.(types.JID)
	}

	// Check database
	if u.contacts != nil {
		lid, err := u.contacts.GetLIDForPN(pn)
		if err == nil && !lid.IsEmpty() {
			u.cache.Store(pn.String(), lid)
			return lid
		}
	}

	// Fetch from WhatsApp if client available
	if u.client != nil && u.client.Store != nil && u.client.Store.LIDs != nil {
		lid, err := u.client.Store.LIDs.GetLIDForPN(ctx, pn)
		if err == nil && !lid.IsEmpty() {
			u.cache.Store(pn.String(), lid)
			// Store the mapping
			if u.contacts != nil {
				u.contacts.UpdatePN(lid, pn)
			}
			return lid
		}
	}

	return pn
}

// ToPN converts a LID to PN. Returns as-is if not found.
func (u *Utils) ToPN(lid types.JID) types.JID {
	if lid.IsEmpty() || !u.IsLID(lid) {
		return lid
	}

	if u.contacts != nil {
		pn, err := u.contacts.GetPNForLID(lid)
		if err == nil && !pn.IsEmpty() {
			return pn
		}
	}

	return lid
}

// StoreMappingFromEvent stores a PN/LID mapping from event data.
func (u *Utils) StoreMappingFromEvent(pn, lid types.JID) {
	if pn.IsEmpty() || lid.IsEmpty() {
		return
	}
	if !u.IsPN(pn) || !u.IsLID(lid) {
		return
	}

	u.cache.Store(pn.String(), lid)
	if u.contacts != nil {
		u.contacts.UpdatePN(lid, pn)
	}
}

// IsLID returns true if this JID is a LID (local identifier).
func (u *Utils) IsLID(jid types.JID) bool {
	return jid.Server == types.HiddenUserServer
}

// IsPN returns true if this JID is a PN (phone number).
func (u *Utils) IsPN(jid types.JID) bool {
	return jid.Server == types.DefaultUserServer && jid.User != ""
}

// IsUser returns true if the JID is a user (not group/newsletter).
func (u *Utils) IsUser(jid types.JID) bool {
	return jid.Server == types.DefaultUserServer || jid.Server == types.HiddenUserServer
}

// IsGroup returns true if the JID is a group.
func (u *Utils) IsGroup(jid types.JID) bool {
	return jid.Server == types.GroupServer
}

// IsNewsletter returns true if the JID is a newsletter/channel.
func (u *Utils) IsNewsletter(jid types.JID) bool {
	return jid.Server == types.NewsletterServer
}

// IsBroadcast returns true if the JID is a broadcast list.
func (u *Utils) IsBroadcast(jid types.JID) bool {
	return jid.Server == types.BroadcastServer
}

// IsStatus returns true if the JID is a status update.
func (u *Utils) IsStatus(jid types.JID) bool {
	return jid.User == "status" && jid.Server == types.BroadcastServer
}

// ToUserJID strips device info and returns the base user JID.
func (u *Utils) ToUserJID(jid types.JID) types.JID {
	return types.JID{
		User:   jid.User,
		Server: jid.Server,
	}
}

// ParseJID parses a JID string into types.JID.
func (u *Utils) ParseJID(jidStr string) (types.JID, error) {
	return types.ParseJID(jidStr)
}

// FromPhone creates a user JID from a phone number.
func (u *Utils) FromPhone(phone string) types.JID {
	cleaned := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)

	return types.JID{
		User:   cleaned,
		Server: types.DefaultUserServer,
	}
}

// ===========================================================================
// MEDIA UTILITIES
// ===========================================================================

// MediaType represents a media type.
type MediaType string

// Type is an alias for MediaType (for backward compatibility).
type Type = MediaType

const (
	MediaTypeImage    MediaType = "image"
	MediaTypeVideo    MediaType = "video"
	MediaTypeAudio    MediaType = "audio"
	MediaTypeDocument MediaType = "document"
	MediaTypeSticker  MediaType = "sticker"
	MediaTypeUnknown  MediaType = "unknown"

	// Type aliases for backward compatibility with old media package
	TypeImage    = MediaTypeImage
	TypeVideo    = MediaTypeVideo
	TypeAudio    = MediaTypeAudio
	TypeDocument = MediaTypeDocument
	TypeSticker  = MediaTypeSticker
	TypeUnknown  = MediaTypeUnknown
)

// WhatsmeowType maps MediaType to whatsmeow.MediaType.
func (t MediaType) WhatsmeowType() whatsmeow.MediaType {
	switch t {
	case MediaTypeImage:
		return whatsmeow.MediaImage
	case MediaTypeVideo:
		return whatsmeow.MediaVideo
	case MediaTypeAudio:
		return whatsmeow.MediaAudio
	case MediaTypeDocument:
		return whatsmeow.MediaDocument
	case MediaTypeSticker:
		return whatsmeow.MediaImage
	default:
		return whatsmeow.MediaDocument
	}
}

// MediaTypeFromMime detects media type from MIME type.
func (u *Utils) MediaTypeFromMime(mime string) MediaType {
	mime = strings.ToLower(mime)
	switch {
	case strings.HasPrefix(mime, "image/webp"):
		return MediaTypeSticker
	case strings.HasPrefix(mime, "image/"):
		return MediaTypeImage
	case strings.HasPrefix(mime, "video/"):
		return MediaTypeVideo
	case strings.HasPrefix(mime, "audio/"):
		return MediaTypeAudio
	default:
		return MediaTypeDocument
	}
}

// MediaTypeFromExtension detects media type from file extension.
func (u *Utils) MediaTypeFromExtension(filename string) MediaType {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff":
		return MediaTypeImage
	case ".webp":
		return MediaTypeSticker
	case ".mp4", ".mov", ".avi", ".mkv", ".webm", ".3gp":
		return MediaTypeVideo
	case ".mp3", ".ogg", ".wav", ".m4a", ".aac", ".opus", ".flac":
		return MediaTypeAudio
	default:
		return MediaTypeDocument
	}
}

// ===========================================================================
// RETRY UTILITIES
// ===========================================================================

// RetryConfig holds retry configuration.
type RetryConfig struct {
	MaxAttempts int
	InitialWait time.Duration
	MaxWait     time.Duration
	Multiplier  float64
}

// DefaultRetryConfig returns sensible retry defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		InitialWait: 100 * time.Millisecond,
		MaxWait:     10 * time.Second,
		Multiplier:  2.0,
	}
}

// Retry executes fn with retry logic.
func (u *Utils) Retry(ctx context.Context, maxAttempts int, fn func() error) error {
	cfg := DefaultRetryConfig()
	cfg.MaxAttempts = maxAttempts

	var err error
	wait := cfg.InitialWait

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		err = fn()
		if err == nil {
			return nil
		}

		if attempt == cfg.MaxAttempts {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}

		wait = time.Duration(float64(wait) * cfg.Multiplier)
		if wait > cfg.MaxWait {
			wait = cfg.MaxWait
		}
	}

	return err
}

// RetryWithBackoff executes with custom backoff calculation.
func (u *Utils) RetryWithBackoff(ctx context.Context, maxAttempts int, backoffFn func(attempt int) time.Duration, fn func() error) error {
	var err error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err = fn()
		if err == nil {
			return nil
		}

		if attempt == maxAttempts {
			break
		}

		wait := backoffFn(attempt)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}

	return err
}

// ExponentialBackoff returns a backoff function with exponential growth.
func ExponentialBackoff(initial time.Duration, max time.Duration) func(int) time.Duration {
	return func(attempt int) time.Duration {
		wait := time.Duration(float64(initial) * math.Pow(2, float64(attempt-1)))
		if wait > max {
			return max
		}
		return wait
	}
}
