package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.mau.fi/whatsmeow/types/events"

	"orion-agent/internal/data/store"
	"orion-agent/internal/infra/config"
	"orion-agent/internal/infra/logger"
	"orion-agent/internal/service/agent"
	"orion-agent/internal/service/event"
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
	Dispatcher   *event.Dispatcher
	JIDService   *utils.Utils
	SyncService  *sync.SyncService
	SendService  *send.SendService
	AgentService *agent.Service

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
	summaryStore := store.NewSummaryStore(appStore)
	settingsStore := store.NewSettingsStore(appStore, cfg)

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

	// Create send service
	sendService := send.NewSendService(waClient.Underlying(), appUtils, messageStore, reactionStore, pollStore, log)

	// Create tool store
	toolStore := store.NewToolStore(appStore.DB())

	// Create agent service
	agentService := agent.NewService(cfg, appStore, settingsStore, summaryStore, toolStore, sendService, log)

	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		Config:       cfg,
		Log:          log,
		Store:        appStore,
		Client:       waClient,
		Dispatcher:   dispatcher,
		JIDService:   appUtils,
		SyncService:  syncService,
		SendService:  sendService,
		AgentService: agentService,
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
		a.JIDService.StoreMappingFromEvent(e.ID, e.LID)
		a.AgentService.SetOwnJID(e.ID)

	case *events.Message:
		// Process through agent after persistence
		go a.processAgentMessage(e)
	}

	// Route to event dispatcher for persistence
	a.Dispatcher.Handle(evt)
	// Route to sync service for coalescence
	a.SyncService.Handle(evt)
}

// processAgentMessage processes a message through the AI agent.
func (a *App) processAgentMessage(e *events.Message) {
	// Skip if from self
	if e.Info.IsFromMe {
		return
	}

	// Skip old/history messages based on config (0 = no limit)
	maxAge := a.Config.AI.MaxMessageAge
	if maxAge > 0 && time.Since(e.Info.Timestamp) > time.Duration(maxAge)*time.Second {
		return
	}

	// Extract text content
	text := ""
	if e.Message.GetConversation() != "" {
		text = e.Message.GetConversation()
	} else if ext := e.Message.GetExtendedTextMessage(); ext != nil {
		text = ext.GetText()
	}

	// Skip empty messages
	if text == "" {
		return
	}

	// Extract mentioned JIDs
	var mentionedJIDs []string
	if ext := e.Message.GetExtendedTextMessage(); ext != nil && ext.ContextInfo != nil {
		mentionedJIDs = ext.ContextInfo.MentionedJID
	}

	// Process through agent
	err := a.AgentService.ProcessMessage(
		a.ctx,
		e.Info.Chat,
		e.Info.Sender,
		e.Info.ID,
		text,
		mentionedJIDs,
		e.Info.IsFromMe,
	)
	if err != nil {
		a.Log.Warnf("Agent processing failed: %v", err)
	}
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
