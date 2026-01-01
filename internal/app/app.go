package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"orion-agent/internal/auth"
	"orion-agent/internal/client"
	"orion-agent/internal/config"
	"orion-agent/internal/event"
	"orion-agent/internal/handler"
	"orion-agent/internal/logger"
	"orion-agent/internal/service"
	"orion-agent/internal/store"
	"orion-agent/internal/sync"
)

// App is the main application orchestrator.
type App struct {
	Config      *config.Config
	Log         *logger.Logger
	Store       *store.Store
	Client      *client.Client
	Dispatcher  *event.Dispatcher
	JIDService  *service.JIDService
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

	// Create client
	waClient, err := client.New(cfg, appStore, log)
	if err != nil {
		appStore.Close()
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	// Create dispatcher
	dispatcher := event.NewDispatcher(log)

	// Create JID service (uses ContactStore for PN/LID mappings)
	jidService := service.NewJIDService(waClient.Underlying(), contactStore, log)

	// Create sync service with ALL stores
	syncService := sync.NewSyncService(
		waClient.Underlying(),
		jidService,
		contactStore,
		groupStore,
		chatStore,
		blocklistStore,
		privacyStore,
		newsletterStore,
		log,
	)

	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		Config:       cfg,
		Log:          log,
		Store:        appStore,
		Client:       waClient,
		Dispatcher:   dispatcher,
		JIDService:   jidService,
		SyncService:  syncService,
		ContactStore: contactStore,
		ChatStore:    chatStore,
		MessageStore: messageStore,
		ReceiptStore: receiptStore,
		GroupStore:   groupStore,
		ctx:          ctx,
		cancel:       cancel,
	}

	// Register event handler
	waClient.AddEventHandler(app.handleEvent)

	// Register DataHandler for persistence (normalizes JIDs and saves to DB)
	dataHandler := handler.NewDataHandler(
		ctx,
		log,
		jidService,
		messageStore,
		contactStore,
		chatStore,
		groupStore,
		newsletterStore,
		receiptStore,
		reactionStore,
		callStore,
		pollStore,
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
	qrHandler := auth.NewQRHandler(a.Log)
	return qrHandler.HandleQRChannel(qrChan)
}

// handleEvent is the main event handler that routes to dispatcher.
// Uses comprehensive coalescence to fill ALL missing data from events.
func (a *App) handleEvent(evt interface{}) {
	// Handle connection events and coalescence
	switch e := evt.(type) {
	case *events.Connected:
		a.Log.Infof("Connected to WhatsApp")
		// Trigger full sync on connect
		go func() {
			if err := a.SyncService.FullSync(a.ctx); err != nil {
				a.Log.Warnf("Initial sync failed: %v", err)
			}
			// Start periodic scheduler
			a.SyncService.StartScheduler(sync.DefaultSchedulerConfig())
		}()

	case *events.PairSuccess:
		a.Log.Infof("Paired successfully as %s", e.ID)
		a.JIDService.StoreMappingFromEvent(e.ID, e.LID)

	// =====================================================
	// COMPREHENSIVE COALESCENCE - Fill ALL missing data
	// =====================================================

	case *events.Message:
		// Coalescence: sync sender + group if unknown
		go a.SyncService.OnNewMessage(a.ctx, e.Info.Chat, e.Info.Sender, e.Info.IsGroup)

	case *events.Receipt:
		// Coalescence: sync receipt sender
		go a.SyncService.OnNewContact(a.ctx, e.Sender)

	case *events.Presence:
		// Coalescence: sync contact from presence
		go a.SyncService.OnPresenceUpdate(a.ctx, e.From)

	case *events.ChatPresence:
		// Coalescence: sync sender from typing status
		go a.SyncService.OnChatPresenceUpdate(a.ctx, e.Chat, e.Sender)

	case *events.PushName:
		// Coalescence: sync profile pic on push name update
		go a.SyncService.OnPushNameUpdate(a.ctx, e.JID)

	case *events.Picture:
		// Coalescence: fetch full picture info
		go a.SyncService.OnPictureUpdate(a.ctx, e.JID, e.PictureID)

	case *events.JoinedGroup:
		// Coalescence: full sync for new group
		go a.SyncService.OnGroupJoined(a.ctx, e.JID)

	case *events.GroupInfo:
		// Coalescence: sync group info changes + participants
		go func() {
			a.SyncService.OnGroupInfoChange(a.ctx, e.JID)
			// Sync all mentioned participants
			var participantJIDs []types.JID
			if e.Join != nil {
				participantJIDs = append(participantJIDs, e.Join...)
			}
			if e.Leave != nil {
				participantJIDs = append(participantJIDs, e.Leave...)
			}
			if len(participantJIDs) > 0 {
				a.SyncService.OnGroupParticipantsChange(a.ctx, e.JID, participantJIDs)
			}
		}()

	case *events.HistorySync:
		// Coalescence: batch sync contacts/groups from history
		go func() {
			// Collect JIDs from history sync
			var contactJIDs []types.JID
			var groupJIDs []types.JID
			if e.Data != nil && e.Data.Conversations != nil {
				for _, conv := range e.Data.Conversations {
					if conv.ID != nil {
						jid, _ := types.ParseJID(*conv.ID)
						switch jid.Server {
						case types.GroupServer:
							groupJIDs = append(groupJIDs, jid)
						case types.HiddenUserServer, types.DefaultUserServer:
							contactJIDs = append(contactJIDs, jid)
						}
					}
				}
			}
			a.SyncService.OnHistorySyncContacts(a.ctx, contactJIDs)
			a.SyncService.OnHistorySyncGroups(a.ctx, groupJIDs)
		}()

	case *events.CallOffer:
		// Coalescence: sync caller info
		go a.SyncService.OnCallReceived(a.ctx, e.CallCreator)

	case *events.Blocklist:
		// Coalescence: refresh blocklist
		go a.SyncService.OnBlocklistChange(a.ctx)

	case *events.PrivacySettings:
		// Coalescence: refresh privacy settings
		go a.SyncService.OnPrivacySettingsChange(a.ctx)

	case *events.BusinessName:
		// Coalescence: sync contact with business name
		go a.SyncService.OnNewContact(a.ctx, e.JID)

	case *events.Contact:
		// Coalescence: sync full contact info
		go a.SyncService.OnNewContact(a.ctx, e.JID)

	case *events.NewsletterJoin:
		// Coalescence: newsletter info
		go a.SyncService.OnNewsletterMessage(a.ctx, e.ID)
	}

	// Route to dispatcher for normal handling
	a.Dispatcher.Handle(evt)
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
