package send

import (
	"orion-agent/internal/utils"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

// Content is the interface all sendable content types must implement.
type Content interface {
	// ToMessage converts the content to a whatsmeow Message protobuf.
	ToMessage() (*waE2E.Message, error)

	// MediaType returns the media type for upload, or TypeUnknown for non-utils.
	MediaType() utils.Type
}

// SendResult contains the result of a successful send operation.
type SendResult struct {
	MessageID types.MessageID
	ServerID  types.MessageServerID
	Timestamp time.Time
	Recipient types.JID
	Sender    types.JID
	DebugInfo whatsmeow.MessageDebugTimings
}

// SendOption is a functional option for configuring send operations.
type SendOption func(*sendConfig)

// sendConfig holds configuration for a send operation.
type sendConfig struct {
	ID          types.MessageID
	Timeout     time.Duration
	Peer        bool
	MediaHandle string
}

// WithID sets a custom message ID.
func WithID(id types.MessageID) SendOption {
	return func(c *sendConfig) {
		c.ID = id
	}
}

// WithTimeout sets a custom timeout for the send operation.
func WithTimeout(timeout time.Duration) SendOption {
	return func(c *sendConfig) {
		c.Timeout = timeout
	}
}

// WithPeer forces peer-to-peer sending (no group optimization).
func WithPeer() SendOption {
	return func(c *sendConfig) {
		c.Peer = true
	}
}

// WithMediaHandle sets a pre-existing media handle for reusing uploads.
func WithMediaHandle(handle string) SendOption {
	return func(c *sendConfig) {
		c.MediaHandle = handle
	}
}

// applyOptions applies all options to a config.
func applyOptions(opts []SendOption) *sendConfig {
	cfg := &sendConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// toSendRequestExtra converts sendConfig to whatsmeow.SendRequestExtra.
func (c *sendConfig) toSendRequestExtra() whatsmeow.SendRequestExtra {
	extra := whatsmeow.SendRequestExtra{}
	if c.ID != "" {
		extra.ID = c.ID
	}
	if c.Timeout > 0 {
		extra.Timeout = c.Timeout
	}
	extra.Peer = c.Peer
	if c.MediaHandle != "" {
		extra.MediaHandle = c.MediaHandle
	}
	return extra
}
