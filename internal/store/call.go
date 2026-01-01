package store

import (
	"database/sql"
	"time"

	"go.mau.fi/whatsmeow/types"
)

// Call represents a voice/video call.
type Call struct {
	CallID          string
	CallerLID       types.JID
	GroupJID        types.JID // empty for 1:1 calls
	CallType        string    // "audio", "video"
	IsVideo         bool
	IsGroup         bool
	Timestamp       time.Time
	DurationSeconds int
	Outcome         string // "pending", "answered", "missed", "rejected", "busy"
}

// CallStore handles call operations.
type CallStore struct {
	store *Store
}

// NewCallStore creates a new CallStore.
func NewCallStore(s *Store) *CallStore {
	return &CallStore{store: s}
}

// Put saves or updates a call.
func (s *CallStore) Put(c *Call) error {
	callType := "audio"
	if c.IsVideo {
		callType = "video"
	}

	var groupJID sql.NullString
	if !c.GroupJID.IsEmpty() {
		groupJID.String = c.GroupJID.String()
		groupJID.Valid = true
	}

	_, err := s.store.Exec(`
		INSERT INTO orion_calls (
			call_id, caller_lid, group_jid, call_type, is_video, is_group,
			timestamp, duration_seconds, outcome
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(call_id) DO UPDATE SET
			duration_seconds = COALESCE(excluded.duration_seconds, orion_calls.duration_seconds),
			outcome = excluded.outcome
	`,
		c.CallID, c.CallerLID.String(), groupJID, callType, boolToInt(c.IsVideo), boolToInt(c.IsGroup),
		c.Timestamp.Unix(), c.DurationSeconds, c.Outcome,
	)
	return err
}

// Get retrieves a call by ID.
func (s *CallStore) Get(callID string) (*Call, error) {
	row := s.store.QueryRow(`
		SELECT call_id, caller_lid, group_jid, call_type, is_video, is_group,
			timestamp, duration_seconds, outcome
		FROM orion_calls WHERE call_id = ?
	`, callID)

	return s.scanCall(row)
}

// GetRecent retrieves recent calls.
func (s *CallStore) GetRecent(limit int) ([]*Call, error) {
	rows, err := s.store.Query(`
		SELECT call_id, caller_lid, group_jid, call_type, is_video, is_group,
			timestamp, duration_seconds, outcome
		FROM orion_calls
		ORDER BY timestamp DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var calls []*Call
	for rows.Next() {
		c, err := s.scanCallRow(rows)
		if err != nil {
			return nil, err
		}
		calls = append(calls, c)
	}

	return calls, nil
}

// UpdateOutcome updates the outcome and duration of a call.
func (s *CallStore) UpdateOutcome(callID string, outcome string, durationSeconds int) error {
	_, err := s.store.Exec(`
		UPDATE orion_calls SET outcome = ?, duration_seconds = ?
		WHERE call_id = ?
	`, outcome, durationSeconds, callID)
	return err
}

func (s *CallStore) scanCall(row *sql.Row) (*Call, error) {
	var c Call
	var callerLIDStr string
	var groupJID sql.NullString
	var isVideo, isGroup int
	var ts int64

	err := row.Scan(
		&c.CallID, &callerLIDStr, &groupJID, &c.CallType, &isVideo, &isGroup,
		&ts, &c.DurationSeconds, &c.Outcome,
	)
	if err != nil {
		return nil, err
	}

	c.CallerLID, _ = types.ParseJID(callerLIDStr)
	if groupJID.Valid {
		c.GroupJID, _ = types.ParseJID(groupJID.String)
	}
	c.IsVideo = isVideo == 1
	c.IsGroup = isGroup == 1
	c.Timestamp = time.Unix(ts, 0)

	return &c, nil
}

func (s *CallStore) scanCallRow(rows *sql.Rows) (*Call, error) {
	var c Call
	var callerLIDStr string
	var groupJID sql.NullString
	var isVideo, isGroup int
	var ts int64

	err := rows.Scan(
		&c.CallID, &callerLIDStr, &groupJID, &c.CallType, &isVideo, &isGroup,
		&ts, &c.DurationSeconds, &c.Outcome,
	)
	if err != nil {
		return nil, err
	}

	c.CallerLID, _ = types.ParseJID(callerLIDStr)
	if groupJID.Valid {
		c.GroupJID, _ = types.ParseJID(groupJID.String)
	}
	c.IsVideo = isVideo == 1
	c.IsGroup = isGroup == 1
	c.Timestamp = time.Unix(ts, 0)

	return &c, nil
}
