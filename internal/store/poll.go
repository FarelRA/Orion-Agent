package store

import (
	"database/sql"
	"encoding/json"
	"time"

	"go.mau.fi/whatsmeow/types"
)

// Poll represents poll metadata stored with the message.
// The actual poll question/options are in the Message struct.
// This store tracks votes.
type PollVote struct {
	MessageID       string    // Poll message ID
	ChatJID         types.JID // Chat where poll was sent
	VoterLID        types.JID
	SelectedOptions []string // Option names selected
	Timestamp       time.Time
}

// PollStore handles poll vote operations.
type PollStore struct {
	store *Store
}

// NewPollStore creates a new PollStore.
func NewPollStore(s *Store) *PollStore {
	return &PollStore{store: s}
}

// PutVote saves or updates a poll vote.
func (s *PollStore) PutVote(v *PollVote) error {
	optionsJSON, _ := json.Marshal(v.SelectedOptions)

	_, err := s.store.Exec(`
		INSERT INTO orion_poll_votes (message_id, chat_jid, voter_lid, selected_options, timestamp)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(message_id, chat_jid, voter_lid) DO UPDATE SET
			selected_options = excluded.selected_options,
			timestamp = excluded.timestamp
	`, v.MessageID, v.ChatJID.String(), v.VoterLID.String(), optionsJSON, v.Timestamp.Unix())
	return err
}

// GetVotes returns all votes for a poll.
func (s *PollStore) GetVotes(messageID string, chatJID types.JID) ([]PollVote, error) {
	rows, err := s.store.Query(`
		SELECT message_id, chat_jid, voter_lid, selected_options, timestamp
		FROM orion_poll_votes
		WHERE message_id = ? AND chat_jid = ?
	`, messageID, chatJID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var votes []PollVote
	for rows.Next() {
		v, err := s.scanVote(rows)
		if err != nil {
			return nil, err
		}
		votes = append(votes, *v)
	}

	return votes, nil
}

// GetVoteByVoter returns a specific voter's vote.
func (s *PollStore) GetVoteByVoter(messageID string, chatJID, voterLID types.JID) (*PollVote, error) {
	row := s.store.QueryRow(`
		SELECT message_id, chat_jid, voter_lid, selected_options, timestamp
		FROM orion_poll_votes
		WHERE message_id = ? AND chat_jid = ? AND voter_lid = ?
	`, messageID, chatJID.String(), voterLID.String())

	var v PollVote
	var chatJIDStr, voterLIDStr string
	var optionsJSON []byte
	var ts int64

	err := row.Scan(&v.MessageID, &chatJIDStr, &voterLIDStr, &optionsJSON, &ts)
	if err != nil {
		return nil, err
	}

	v.ChatJID, _ = types.ParseJID(chatJIDStr)
	v.VoterLID, _ = types.ParseJID(voterLIDStr)
	json.Unmarshal(optionsJSON, &v.SelectedOptions)
	v.Timestamp = time.Unix(ts, 0)

	return &v, nil
}

// DeleteVote removes a vote (when voter retracts).
func (s *PollStore) DeleteVote(messageID string, chatJID, voterLID types.JID) error {
	_, err := s.store.Exec(`
		DELETE FROM orion_poll_votes WHERE message_id = ? AND chat_jid = ? AND voter_lid = ?
	`, messageID, chatJID.String(), voterLID.String())
	return err
}

func (s *PollStore) scanVote(rows *sql.Rows) (*PollVote, error) {
	var v PollVote
	var chatJIDStr, voterLIDStr string
	var optionsJSON []byte
	var ts int64

	err := rows.Scan(&v.MessageID, &chatJIDStr, &voterLIDStr, &optionsJSON, &ts)
	if err != nil {
		return nil, err
	}

	v.ChatJID, _ = types.ParseJID(chatJIDStr)
	v.VoterLID, _ = types.ParseJID(voterLIDStr)
	json.Unmarshal(optionsJSON, &v.SelectedOptions)
	v.Timestamp = time.Unix(ts, 0)

	return &v, nil
}
