package store

import (
	"time"

	"go.mau.fi/whatsmeow/types"
)

// Label represents a WhatsApp Business label.
type Label struct {
	ID           string
	Name         string
	Color        int
	SortOrder    int
	PredefinedID int
	Deleted      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// LabelAssociation represents a label assignment to a chat or message.
type LabelAssociation struct {
	LabelID    string
	TargetType string // "chat" or "message"
	TargetJID  types.JID
	MessageID  string // Only for message associations
	Timestamp  time.Time
}

// LabelStore handles label operations.
type LabelStore struct {
	store *Store
}

// NewLabelStore creates a new LabelStore.
func NewLabelStore(s *Store) *LabelStore {
	return &LabelStore{store: s}
}

// Put creates or updates a label.
func (s *LabelStore) Put(label *Label) error {
	now := time.Now().Unix()
	createdAt := now
	if !label.CreatedAt.IsZero() {
		createdAt = label.CreatedAt.Unix()
	}

	_, err := s.store.Exec(`
		INSERT INTO orion_labels (id, name, color, sort_order, predefined_id, deleted, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			color = excluded.color,
			sort_order = excluded.sort_order,
			predefined_id = excluded.predefined_id,
			deleted = excluded.deleted,
			updated_at = excluded.updated_at
	`,
		label.ID,
		label.Name,
		label.Color,
		label.SortOrder,
		label.PredefinedID,
		boolToInt(label.Deleted),
		createdAt,
		now,
	)
	return err
}

// Get retrieves a label by ID.
func (s *LabelStore) Get(id string) (*Label, error) {
	row := s.store.QueryRow(`
		SELECT id, name, color, sort_order, predefined_id, deleted, created_at, updated_at
		FROM orion_labels WHERE id = ?
	`, id)

	var label Label
	var deleted int
	var createdAt, updatedAt int64

	err := row.Scan(&label.ID, &label.Name, &label.Color, &label.SortOrder,
		&label.PredefinedID, &deleted, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	label.Deleted = deleted == 1
	label.CreatedAt = time.Unix(createdAt, 0)
	label.UpdatedAt = time.Unix(updatedAt, 0)

	return &label, nil
}

// GetAll retrieves all labels.
func (s *LabelStore) GetAll() ([]*Label, error) {
	rows, err := s.store.Query(`
		SELECT id, name, color, sort_order, predefined_id, deleted, created_at, updated_at
		FROM orion_labels WHERE deleted = 0 ORDER BY sort_order
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var labels []*Label
	for rows.Next() {
		var label Label
		var deleted int
		var createdAt, updatedAt int64

		err := rows.Scan(&label.ID, &label.Name, &label.Color, &label.SortOrder,
			&label.PredefinedID, &deleted, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		label.Deleted = deleted == 1
		label.CreatedAt = time.Unix(createdAt, 0)
		label.UpdatedAt = time.Unix(updatedAt, 0)
		labels = append(labels, &label)
	}

	return labels, nil
}

// Delete marks a label as deleted.
func (s *LabelStore) Delete(id string) error {
	now := time.Now().Unix()
	_, err := s.store.Exec(`UPDATE orion_labels SET deleted = 1, updated_at = ? WHERE id = ?`, now, id)
	return err
}

// AssociateChat associates a label with a chat.
func (s *LabelStore) AssociateChat(labelID string, chatJID types.JID, timestamp time.Time) error {
	_, err := s.store.Exec(`
		INSERT INTO orion_label_associations (label_id, target_type, target_jid, message_id, timestamp)
		VALUES (?, 'chat', ?, '', ?)
		ON CONFLICT(label_id, target_type, target_jid, COALESCE(message_id, '')) DO UPDATE SET
			timestamp = excluded.timestamp
	`, labelID, chatJID.String(), timestamp.Unix())
	return err
}

// AssociateMessage associates a label with a message.
func (s *LabelStore) AssociateMessage(labelID string, chatJID types.JID, messageID string, timestamp time.Time) error {
	_, err := s.store.Exec(`
		INSERT INTO orion_label_associations (label_id, target_type, target_jid, message_id, timestamp)
		VALUES (?, 'message', ?, ?, ?)
		ON CONFLICT(label_id, target_type, target_jid, COALESCE(message_id, '')) DO UPDATE SET
			timestamp = excluded.timestamp
	`, labelID, chatJID.String(), messageID, timestamp.Unix())
	return err
}

// RemoveChatAssociation removes a label from a chat.
func (s *LabelStore) RemoveChatAssociation(labelID string, chatJID types.JID) error {
	_, err := s.store.Exec(`
		DELETE FROM orion_label_associations 
		WHERE label_id = ? AND target_type = 'chat' AND target_jid = ?
	`, labelID, chatJID.String())
	return err
}

// RemoveMessageAssociation removes a label from a message.
func (s *LabelStore) RemoveMessageAssociation(labelID string, chatJID types.JID, messageID string) error {
	_, err := s.store.Exec(`
		DELETE FROM orion_label_associations 
		WHERE label_id = ? AND target_type = 'message' AND target_jid = ? AND message_id = ?
	`, labelID, chatJID.String(), messageID)
	return err
}

// GetChatsForLabel retrieves all chats with a specific label.
func (s *LabelStore) GetChatsForLabel(labelID string) ([]types.JID, error) {
	rows, err := s.store.Query(`
		SELECT target_jid FROM orion_label_associations
		WHERE label_id = ? AND target_type = 'chat'
	`, labelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jids []types.JID
	for rows.Next() {
		var jidStr string
		if err := rows.Scan(&jidStr); err != nil {
			return nil, err
		}
		if jid, err := types.ParseJID(jidStr); err == nil {
			jids = append(jids, jid)
		}
	}
	return jids, nil
}

// GetLabelsForChat retrieves all labels for a specific chat.
func (s *LabelStore) GetLabelsForChat(chatJID types.JID) ([]*Label, error) {
	rows, err := s.store.Query(`
		SELECT l.id, l.name, l.color, l.sort_order, l.predefined_id, l.deleted, l.created_at, l.updated_at
		FROM orion_labels l
		JOIN orion_label_associations a ON l.id = a.label_id
		WHERE a.target_type = 'chat' AND a.target_jid = ? AND l.deleted = 0
		ORDER BY l.sort_order
	`, chatJID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var labels []*Label
	for rows.Next() {
		var label Label
		var deleted int
		var createdAt, updatedAt int64

		err := rows.Scan(&label.ID, &label.Name, &label.Color, &label.SortOrder,
			&label.PredefinedID, &deleted, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		label.Deleted = deleted == 1
		label.CreatedAt = time.Unix(createdAt, 0)
		label.UpdatedAt = time.Unix(updatedAt, 0)
		labels = append(labels, &label)
	}
	return labels, nil
}
