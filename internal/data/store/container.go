package store

// Container provides unified access to all stores.
// This is a convenience wrapper that groups all store types together.
type Container struct {
	// Core store
	Store *Store

	// Entity stores - ALL 12 stores
	Contacts    *ContactStore
	Chats       *ChatStore
	Messages    *MessageStore
	Receipts    *ReceiptStore
	Groups      *GroupStore
	Blocklist   *BlocklistStore
	Privacy     *PrivacyStore
	Newsletters *NewsletterStore
	Reactions   *ReactionStore
	Calls       *CallStore
	Polls       *PollStore
	Labels      *LabelStore
	SyncState   *SyncStateStore
}

// NewContainer creates a new Container with all sub-stores initialized.
func NewContainer(s *Store) *Container {
	return &Container{
		Store:       s,
		Contacts:    NewContactStore(s),
		Chats:       NewChatStore(s),
		Messages:    NewMessageStore(s),
		Receipts:    NewReceiptStore(s),
		Groups:      NewGroupStore(s),
		Blocklist:   NewBlocklistStore(s),
		Privacy:     NewPrivacyStore(s),
		Newsletters: NewNewsletterStore(s),
		Reactions:   NewReactionStore(s),
		Calls:       NewCallStore(s),
		Polls:       NewPollStore(s),
		Labels:      NewLabelStore(s),
		SyncState:   NewSyncStateStore(s),
	}
}

// Close closes the underlying store.
func (c *Container) Close() error {
	return c.Store.Close()
}

// Stats returns statistics about stored entities.
type Stats struct {
	Contacts    int
	Chats       int
	Messages    int
	Groups      int
	Newsletters int
	Blocked     int
	Receipts    int
	Reactions   int
	Calls       int
	Polls       int
	Labels      int
}

// GetStats returns current entity counts.
func (c *Container) GetStats() (*Stats, error) {
	stats := &Stats{}
	var err error

	// Count contacts
	if err = c.Store.QueryRow(`SELECT COUNT(*) FROM orion_contacts`).Scan(&stats.Contacts); err != nil {
		return nil, err
	}

	// Count chats
	if err = c.Store.QueryRow(`SELECT COUNT(*) FROM orion_chats`).Scan(&stats.Chats); err != nil {
		return nil, err
	}

	// Count messages
	if err = c.Store.QueryRow(`SELECT COUNT(*) FROM orion_messages`).Scan(&stats.Messages); err != nil {
		return nil, err
	}

	// Count groups
	if err = c.Store.QueryRow(`SELECT COUNT(*) FROM orion_groups`).Scan(&stats.Groups); err != nil {
		return nil, err
	}

	// Count newsletters
	if err = c.Store.QueryRow(`SELECT COUNT(*) FROM orion_newsletters`).Scan(&stats.Newsletters); err != nil {
		return nil, err
	}

	// Count blocked
	if err = c.Store.QueryRow(`SELECT COUNT(*) FROM orion_blocklist`).Scan(&stats.Blocked); err != nil {
		return nil, err
	}

	// Count receipts
	if err = c.Store.QueryRow(`SELECT COUNT(*) FROM orion_message_receipts`).Scan(&stats.Receipts); err != nil {
		return nil, err
	}

	// Count reactions
	if err = c.Store.QueryRow(`SELECT COUNT(*) FROM orion_reactions`).Scan(&stats.Reactions); err != nil {
		return nil, err
	}

	// Count calls
	if err = c.Store.QueryRow(`SELECT COUNT(*) FROM orion_calls`).Scan(&stats.Calls); err != nil {
		return nil, err
	}

	// Count polls
	if err = c.Store.QueryRow(`SELECT COUNT(*) FROM orion_polls`).Scan(&stats.Polls); err != nil {
		return nil, err
	}

	// Count polls
	if err = c.Store.QueryRow(`SELECT COUNT(*) FROM orion_polls`).Scan(&stats.Polls); err != nil {
		return nil, err
	}

	// Count labels
	if err = c.Store.QueryRow(`SELECT COUNT(*) FROM orion_labels`).Scan(&stats.Labels); err != nil {
		return nil, err
	}

	return stats, nil
}
