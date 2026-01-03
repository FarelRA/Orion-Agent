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
	"orion-agent/internal/service/agent"
	"orion-agent/internal/service/event"
	"orion-agent/internal/service/media"
	"orion-agent/internal/service/send"
	"orion-agent/internal/service/sync"
	"orion-agent/internal/utils"
)

// App is the main application orchestrator.
type App struct {
	Config       *config.Config
	Log          *logger.Logger
	Store        *store.Store
	Client       *Client
	Utils        *utils.Utils
	EventService *event.EventService
	SyncService  *sync.SyncService
	SendService  *send.SendService
	AgentService *agent.AgentService
	MediaService *media.MediaService

	// Sub-stores for convenience
	ContactStore    *store.ContactStore
	ChatStore       *store.ChatStore
	MessageStore    *store.MessageStore
	ReceiptStore    *store.ReceiptStore
	GroupStore      *store.GroupStore
	MediaCacheStore *store.MediaCacheStore

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
	toolStore := store.NewToolStore(appStore)
	summaryStore := store.NewSummaryStore(appStore)
	settingsStore := store.NewSettingsStore(appStore, cfg)
	mediaCacheStore := store.NewMediaCacheStore(appStore)

	// Create client
	waClient, err := NewClient(cfg, appStore, log)
	if err != nil {
		appStore.Close()
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	// Create utils
	appUtils := utils.New(contactStore, waClient.Underlying())

	// Create sync state store
	syncStateStore := store.NewSyncStateStore(appStore)

	// Create media service
	mediaService := media.NewMediaService(waClient.Underlying(), &cfg.Media, cfg.StorePath, mediaCacheStore, log)

	// Create sync service with ALL stores
	syncService := sync.NewSyncService(
		waClient.Underlying(),
		appUtils,
		mediaService,
		contactStore,
		groupStore,
		chatStore,
		blocklistStore,
		privacyStore,
		newsletterStore,
		syncStateStore,
		log,
	)

	// Create send service
	sendService := send.NewSendService(waClient.Underlying(), appUtils, messageStore, reactionStore, pollStore, log)

	// Create agent service
	agentService := agent.NewAgentService(cfg, appStore, settingsStore, summaryStore, toolStore, sendService, log)

	// Create event service with ALL stores
	eventService := event.NewEventService(
		log,
		appUtils,
		agentService, // Direct agent integration
		mediaService,
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

	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		Config:          cfg,
		Log:             log,
		Store:           appStore,
		Client:          waClient,
		EventService:    eventService,
		Utils:           appUtils,
		SyncService:     syncService,
		SendService:     sendService,
		AgentService:    agentService,
		ContactStore:    contactStore,
		ChatStore:       chatStore,
		MessageStore:    messageStore,
		ReceiptStore:    receiptStore,
		GroupStore:      groupStore,
		MediaCacheStore: mediaCacheStore,
		MediaService:    mediaService,
		ctx:             ctx,
		cancel:          cancel,
	}

	// Set up sync dispatcher for coalescence
	syncService.SetDispatcher(ctx)

	// Set up event dispatcher
	eventService.SetDispatcher(ctx)

	// Register event handler
	waClient.AddEventHandler(app.handleEvent)

	return app, nil
}

// Run starts the application.
func (a *App) Run() error {
	a.Log.Infof("Starting Orion Agent...")

	// Start media service
	a.MediaService.Start()

	// Setup signal handling to cancel context
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		a.Log.Infof("Received %v, initiating shutdown...", sig)
		a.cancel()
	}()

	// Connect (respects context cancellation)
	if err := a.connect(); err != nil {
		if a.ctx.Err() != nil {
			a.Log.Infof("Shutdown during startup")
			return a.Shutdown()
		}
		return err
	}

	a.Log.Infof("Orion Agent is running. Press Ctrl+C to stop.")

	// Wait for context cancellation
	<-a.ctx.Done()
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

	// Handle QR with context
	qrHandler := NewQRHandler(a.Log)
	return qrHandler.HandleQRChannel(a.ctx, qrChan)
}

// handleEvent is the main event handler that routes events to services.
func (a *App) handleEvent(evt interface{}) {
	// Handle special app-level events
	switch e := evt.(type) {
	case *events.Connected:
		a.Log.Infof("Connected to WhatsApp")
		// Set own JID for agent
		if a.Client.Underlying().Store.ID != nil {
			a.AgentService.SetOwnJID(*a.Client.Underlying().Store.ID)
		}

	case *events.PairSuccess:
		a.Log.Infof("Paired successfully as %s", e.ID)
		a.Utils.StoreMappingFromEvent(e.ID, e.LID)
		a.AgentService.SetOwnJID(e.ID)

	case *events.Message:
		// Message processing now happens in DataHandler via AgentProcessor
	}

	// Route to event service for persistence
	a.EventService.Handle(evt)
	// Route to sync service for coalescence
	a.SyncService.Handle(evt)
}

// Shutdown gracefully shuts down the application.
func (a *App) Shutdown() error {
	a.cancel()
	a.MediaService.Stop()
	a.SyncService.StopScheduler()
	a.Client.Disconnect()
	return a.Store.Close()
}
