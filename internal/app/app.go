package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.mau.fi/whatsmeow/types/events"

	"orion-agent/internal/data/store"
	"orion-agent/internal/infra/config"
	"orion-agent/internal/infra/logger"
	"orion-agent/internal/service/event"
	"orion-agent/internal/service/sync"
	"orion-agent/internal/utils"
)

// App is the main application orchestrator.
type App struct {
	Config      *config.Config
	Log         *logger.Logger
	Store       *store.Store
	Client      *Client
	Dispatcher  *event.Dispatcher
	JIDService  *utils.Utils
	SyncService *sync.SyncService

	// Sub-stores for convenience
	ContactStore *store.ContactStore
	ChatStore    *store.ChatStore
	MessageStore *store.MessageStore
	ReceiptStore *store.ReceiptStore
	GroupStore   *store.GroupStore

	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new App instance.
func New(cfg *config.Config) (*App, error) {
	log := logger.New("orion", cfg.LogLevel)
	log.Infof("Initializing Orion Agent...")

	// Ensure store path exists
	if err := cfg.EnsureStorePath(); err != nil {
		return nil, fmt.Errorf("failed to ensure store path: %w", err)
	}

	// Create store
	dbPath := cfg.StorePath + "/orion.db"
	appStore, err := store.New(dbPath, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	// Create sub-stores
	contactStore := store.NewContactStore(appStore)
	chatStore := store.NewChatStore(appStore)
	messageStore := store.NewMessageStore(appStore)
	receiptStore := store.NewReceiptStore(appStore)
	groupStore := store.NewGroupStore(appStore)
	blocklistStore := store.NewBlocklistStore(appStore)
	privacyStore := store.NewPrivacyStore(appStore)
	newsletterStore := store.NewNewsletterStore(appStore)
	reactionStore := store.NewReactionStore(appStore)
	callStore := store.NewCallStore(appStore)
	pollStore := store.NewPollStore(appStore)
	labelStore := store.NewLabelStore(appStore)

	// Create client
	waClient, err := NewClient(cfg, appStore, log)
	if err != nil {
		appStore.Close()
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	// Create dispatcher
	dispatcher := event.NewDispatcher(log)

	// Create utils (JID normalization, etc.)
	appUtils := utils.New(contactStore, waClient.Underlying())

	// Create sync state store
	syncStateStore := store.NewSyncStateStore(appStore)

	// Create sync service with ALL stores
	syncService := sync.NewSyncService(
		waClient.Underlying(),
		appUtils,
		contactStore,
		groupStore,
		chatStore,
		blocklistStore,
		privacyStore,
		newsletterStore,
		syncStateStore,
		log,
	)

	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		Config:       cfg,
		Log:          log,
		Store:        appStore,
		Client:       waClient,
		Dispatcher:   dispatcher,
		JIDService:   appUtils,
		SyncService:  syncService,
		ContactStore: contactStore,
		ChatStore:    chatStore,
		MessageStore: messageStore,
		ReceiptStore: receiptStore,
		GroupStore:   groupStore,
		ctx:          ctx,
		cancel:       cancel,
	}

	// Set up sync dispatcher for coalescence
	syncService.SetDispatcher(ctx)

	// Register event handler
	waClient.AddEventHandler(app.handleEvent)

	// Register DataHandler for persistence (normalizes JIDs and saves to DB)
	dataHandler := event.NewDataHandler(
		ctx,
		log,
		appUtils,
		messageStore,
		contactStore,
		chatStore,
		groupStore,
		newsletterStore,
		receiptStore,
		reactionStore,
		callStore,
		pollStore,
		labelStore,
		privacyStore,
		blocklistStore,
	)
	dispatcher.Register(dataHandler)

	return app, nil
}

// Run starts the application.
func (a *App) Run() error {
	a.Log.Infof("Starting Orion Agent...")

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Connect
	if err := a.connect(); err != nil {
		return err
	}

	a.Log.Infof("Orion Agent is running. Press Ctrl+C to stop.")

	// Wait for signal
	<-sigChan
	a.Log.Infof("Shutting down...")

	return a.Shutdown()
}

// connect handles the connection flow including QR pairing if needed.
func (a *App) connect() error {
	if a.Client.IsLoggedIn() {
		a.Log.Infof("Using existing session...")
		return a.Client.Connect(a.ctx)
	}

	// Need QR pairing
	a.Log.Infof("No existing session, starting QR pairing...")

	qrChan, err := a.Client.GetQRChannel(a.ctx)
	if err != nil {
		return fmt.Errorf("failed to get QR channel: %w", err)
	}

	// Connect (will trigger QR generation)
	if err := a.Client.Connect(a.ctx); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Handle QR
	qrHandler := NewQRHandler(a.Log)
	return qrHandler.HandleQRChannel(qrChan)
}

// handleEvent is the main event handler that routes events to services.
func (a *App) handleEvent(evt interface{}) {
	// Handle special app-level events
	switch e := evt.(type) {
	case *events.Connected:
		a.Log.Infof("Connected to WhatsApp")

	case *events.PairSuccess:
		a.Log.Infof("Paired successfully as %s", e.ID)
		a.JIDService.StoreMappingFromEvent(e.ID, e.LID)
	}

	// Route to event dispatcher for persistence
	a.Dispatcher.Handle(evt)
	// Route to sync service for coalescence
	a.SyncService.Handle(evt)
}

// Shutdown gracefully shuts down the application.
func (a *App) Shutdown() error {
	a.cancel()
	a.SyncService.StopScheduler()
	a.Client.Disconnect()
	return a.Store.Close()
}

// RegisterHandler registers an event handler with the dispatcher.
func (a *App) RegisterHandler(h event.Handler) {
	a.Dispatcher.Register(h)
}
