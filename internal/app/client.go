package app

import (
	"context"
	"fmt"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"

	appstore "orion-agent/internal/data/store"
	"orion-agent/internal/infra/config"
)

// Client wraps whatsmeow.Client with additional functionality.
type Client struct {
	WAClient *whatsmeow.Client
	Device   *store.Device
	Store    *appstore.Store
	Log      waLog.Logger
	Config   *config.Config

	connected bool
	handlers  []func(interface{})
}

// NewClient creates a new Client.
func NewClient(cfg *config.Config, appStore *appstore.Store, log waLog.Logger) (*Client, error) {
	// Get or create device
	device, err := appStore.GetDevice()
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	// Create whatsmeow client
	waClient := whatsmeow.NewClient(device, log.Sub("whatsmeow"))
	waClient.EnableAutoReconnect = true
	waClient.AutoTrustIdentity = true

	c := &Client{
		WAClient: waClient,
		Device:   device,
		Store:    appStore,
		Log:      log.Sub("Client"),
		Config:   cfg,
	}

	return c, nil
}

// AddEventHandler adds an event handler function.
func (c *Client) AddEventHandler(handler func(interface{})) {
	c.handlers = append(c.handlers, handler)
	c.WAClient.AddEventHandler(handler)
}

// Connect connects to WhatsApp.
func (c *Client) Connect(ctx context.Context) error {
	if c.IsLoggedIn() {
		c.Log.Infof("Already logged in, connecting...")
		return c.WAClient.Connect()
	}

	// Need to pair
	c.Log.Infof("Not logged in, need to pair with QR code")
	return c.WAClient.Connect()
}

// Disconnect disconnects from WhatsApp.
func (c *Client) Disconnect() {
	c.WAClient.Disconnect()
	c.setConnected(false)
}

// IsLoggedIn returns true if the client has stored credentials.
func (c *Client) IsLoggedIn() bool {
	return c.Device.ID != nil
}

// IsConnected returns true if currently connected to WhatsApp.
func (c *Client) IsConnected() bool {
	return c.connected
}

func (c *Client) setConnected(connected bool) {
	c.connected = connected
}

// GetJID returns the client's JID.
func (c *Client) GetJID() types.JID {
	if c.Device.ID != nil {
		return *c.Device.ID
	}
	return types.JID{}
}

// GetLID returns the client's LID.
func (c *Client) GetLID() types.JID {
	return c.Device.LID
}

// GetQRChannel returns a channel for QR code events.
func (c *Client) GetQRChannel(ctx context.Context) (<-chan whatsmeow.QRChannelItem, error) {
	qrChan, err := c.WAClient.GetQRChannel(ctx)
	if err != nil {
		return nil, err
	}
	return qrChan, nil
}

// SendPresence sends presence update.
func (c *Client) SendPresence(ctx context.Context, presence types.Presence) error {
	return c.WAClient.SendPresence(ctx, presence)
}

// Underlying returns the underlying whatsmeow.Client for advanced usage.
func (c *Client) Underlying() *whatsmeow.Client {
	return c.WAClient
}
