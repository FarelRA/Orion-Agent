package store

import (
	"database/sql"
	"time"
)

// SyncState represents the state of a sync operation.
type SyncState struct {
	SyncType     string
	LastSyncAt   time.Time
	SyncProgress int
	SyncData     string
}

// SyncStateStore handles sync state persistence.
type SyncStateStore struct {
	store *Store
}

// NewSyncStateStore creates a new SyncStateStore.
func NewSyncStateStore(s *Store) *SyncStateStore {
	return &SyncStateStore{store: s}
}

// Put updates a sync state.
func (s *SyncStateStore) Put(state *SyncState) error {
	_, err := s.store.Exec(`
		INSERT INTO orion_sync_state (sync_type, last_sync_at, sync_progress, sync_data)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(sync_type) DO UPDATE SET
			last_sync_at = excluded.last_sync_at,
			sync_progress = excluded.sync_progress,
			sync_data = excluded.sync_data
	`, state.SyncType, state.LastSyncAt.Unix(), state.SyncProgress, state.SyncData)
	return err
}

// Get retrieves the state for a sync type.
func (s *SyncStateStore) Get(syncType string) (*SyncState, error) {
	row := s.store.QueryRow(`
		SELECT sync_type, last_sync_at, sync_progress, sync_data
		FROM orion_sync_state WHERE sync_type = ?
	`, syncType)

	var state SyncState
	var ts int64
	var progress sql.NullInt64
	var data sql.NullString

	err := row.Scan(&state.SyncType, &ts, &progress, &data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	state.LastSyncAt = time.Unix(ts, 0)
	if progress.Valid {
		state.SyncProgress = int(progress.Int64)
	}
	state.SyncData = data.String

	return &state, nil
}

// GetAll retrieves all sync states.
func (s *SyncStateStore) GetAll() ([]*SyncState, error) {
	rows, err := s.store.Query(`
		SELECT sync_type, last_sync_at, sync_progress, sync_data
		FROM orion_sync_state
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var states []*SyncState
	for rows.Next() {
		var state SyncState
		var ts int64
		var progress sql.NullInt64
		var data sql.NullString

		if err := rows.Scan(&state.SyncType, &ts, &progress, &data); err != nil {
			return nil, err
		}

		state.LastSyncAt = time.Unix(ts, 0)
		if progress.Valid {
			state.SyncProgress = int(progress.Int64)
		}
		state.SyncData = data.String

		states = append(states, &state)
	}
	return states, nil
}
