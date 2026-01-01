package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// Store wraps whatsmeow's sqlstore and adds app-specific tables.
type Store struct {
	db        *sql.DB
	container *sqlstore.Container
	log       waLog.Logger
}

// New creates a new Store with the given database path.
func New(dbPath string, log waLog.Logger) (*Store, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create store directory: %w", err)
	}

	// Open database
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create whatsmeow container
	container := sqlstore.NewWithDB(db, "sqlite3", log.Sub("whatsmeow"))

	// Upgrade whatsmeow schema
	if err := container.Upgrade(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to upgrade whatsmeow schema: %w", err)
	}

	s := &Store{
		db:        db,
		container: container,
		log:       log.Sub("Store"),
	}

	// Create app-specific tables
	if err := s.createTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create app tables: %w", err)
	}

	return s, nil
}

// Container returns the whatsmeow sqlstore container.
func (s *Store) Container() *sqlstore.Container {
	return s.container
}

// DB returns the underlying database connection.
func (s *Store) DB() *sql.DB {
	return s.db
}

// GetDevice returns an existing device or creates a new one.
func (s *Store) GetDevice() (*store.Device, error) {
	devices, err := s.container.GetAllDevices(context.Background())
	if err != nil {
		return nil, err
	}

	if len(devices) > 0 {
		return devices[0], nil
	}

	return s.container.NewDevice(), nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// createTables creates all app-specific tables.
func (s *Store) createTables() error {
	_, err := s.db.Exec(schema)
	return err
}

// Exec executes a query without returning rows.
func (s *Store) Exec(query string, args ...interface{}) (sql.Result, error) {
	return s.db.Exec(query, args...)
}

// Query executes a query that returns rows.
func (s *Store) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return s.db.Query(query, args...)
}

// QueryRow executes a query that returns a single row.
func (s *Store) QueryRow(query string, args ...interface{}) *sql.Row {
	return s.db.QueryRow(query, args...)
}

// Begin starts a transaction.
func (s *Store) Begin() (*sql.Tx, error) {
	return s.db.Begin()
}
