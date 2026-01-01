package store

import (
	"time"

	"go.mau.fi/whatsmeow/types"
)

// Receipt represents a message receipt.
type Receipt struct {
	MessageID    string
	ChatJID      types.JID
	RecipientLID types.JID
	ReceiptType  string // "sent", "delivered", "read", "played"
	Timestamp    time.Time
}

// ReceiptStore handles receipt operations.
type ReceiptStore struct {
	store *Store
}

// NewReceiptStore creates a new ReceiptStore.
func NewReceiptStore(s *Store) *ReceiptStore {
	return &ReceiptStore{store: s}
}

// Put stores a receipt.
func (s *ReceiptStore) Put(r *Receipt) error {
	_, err := s.store.Exec(`
		INSERT INTO orion_message_receipts (message_id, chat_jid, recipient_lid, receipt_type, timestamp)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(message_id, chat_jid, recipient_lid, receipt_type) DO UPDATE SET timestamp = excluded.timestamp
	`, r.MessageID, r.ChatJID.String(), r.RecipientLID.String(), r.ReceiptType, r.Timestamp.Unix())
	return err
}

// PutMany stores multiple receipts.
func (s *ReceiptStore) PutMany(receipts []Receipt) error {
	tx, err := s.store.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO orion_message_receipts (message_id, chat_jid, recipient_lid, receipt_type, timestamp)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(message_id, chat_jid, recipient_lid, receipt_type) DO UPDATE SET timestamp = excluded.timestamp
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, r := range receipts {
		if _, err := stmt.Exec(r.MessageID, r.ChatJID.String(), r.RecipientLID.String(), r.ReceiptType, r.Timestamp.Unix()); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetForMessage retrieves all receipts for a message.
func (s *ReceiptStore) GetForMessage(messageID string, chatJID types.JID) ([]Receipt, error) {
	rows, err := s.store.Query(`
		SELECT message_id, chat_jid, recipient_lid, receipt_type, timestamp
		FROM orion_message_receipts WHERE message_id = ? AND chat_jid = ?
		ORDER BY timestamp
	`, messageID, chatJID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var receipts []Receipt
	for rows.Next() {
		var msgID, chatJIDStr, recipientLIDStr, receiptType string
		var timestamp int64

		if err := rows.Scan(&msgID, &chatJIDStr, &recipientLIDStr, &receiptType, &timestamp); err != nil {
			return nil, err
		}

		chatJID, _ := types.ParseJID(chatJIDStr)
		recipientLID, _ := types.ParseJID(recipientLIDStr)

		receipts = append(receipts, Receipt{
			MessageID:    msgID,
			ChatJID:      chatJID,
			RecipientLID: recipientLID,
			ReceiptType:  receiptType,
			Timestamp:    time.Unix(timestamp, 0),
		})
	}
	return receipts, nil
}

// IsRead checks if a message has been read.
func (s *ReceiptStore) IsRead(messageID string, chatJID types.JID) (bool, error) {
	var count int
	err := s.store.QueryRow(`
		SELECT COUNT(*) FROM orion_message_receipts
		WHERE message_id = ? AND chat_jid = ? AND receipt_type = 'read'
	`, messageID, chatJID.String()).Scan(&count)
	return count > 0, err
}
