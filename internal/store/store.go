// Package store is the SQLite queue store: messages, attempts, bounces,
// complaints, and DKIM selector state. One file, no server, no cgo.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/jonathanngiroux-star/warmline/internal/queue"
	_ "modernc.org/sqlite"
)

// Store wraps the SQLite database holding the Warmline queue.
type Store struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS messages (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  recipient   TEXT NOT NULL,
  body        TEXT NOT NULL,
  metadata    TEXT NOT NULL DEFAULT '{}',
  status      TEXT NOT NULL DEFAULT 'queued',
  relay       TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  sent_at     TEXT
);
CREATE INDEX IF NOT EXISTS idx_messages_status_created ON messages(status, created_at);

CREATE TABLE IF NOT EXISTS attempts (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  message_id  INTEGER NOT NULL REFERENCES messages(id),
  relay       TEXT NOT NULL,
  smtp_code   TEXT NOT NULL DEFAULT '',
  diagnostic  TEXT NOT NULL DEFAULT '',
  at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX IF NOT EXISTS idx_attempts_message ON attempts(message_id);

CREATE TABLE IF NOT EXISTS bounces (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  message_id  INTEGER NOT NULL REFERENCES messages(id),
  recipient   TEXT NOT NULL,
  smtp_code   TEXT NOT NULL DEFAULT '',
  diagnostic  TEXT NOT NULL DEFAULT '',
  class       TEXT NOT NULL DEFAULT 'unknown',
  at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX IF NOT EXISTS idx_bounces_message ON bounces(message_id);

CREATE TABLE IF NOT EXISTS complaints (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  message_id  INTEGER NOT NULL REFERENCES messages(id),
  recipient   TEXT NOT NULL,
  at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX IF NOT EXISTS idx_complaints_message ON complaints(message_id);

CREATE TABLE IF NOT EXISTS dkims (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  domain      TEXT NOT NULL,
  selector    TEXT NOT NULL,
  public_key  TEXT NOT NULL,
  created_at  TEXT NOT NULL,
  UNIQUE(domain, selector)
);
`

// Open opens (creating if needed) the SQLite queue database at path and
// applies the schema.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// Single-writer pragmas: WAL keeps reads concurrent with writes.
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("pragmas: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error { return s.db.Close() }

// Message is a queue row as the worker sees it.
type Message struct {
	ID        int64
	Recipient string
	Body      string
	Metadata  map[string]string
	Status    string
	Relay     string
	CreatedAt string
}

// Enqueue appends a message and returns its id.
func (s *Store) Enqueue(recipient, body string, metadata map[string]string) (int64, error) {
	if metadata == nil {
		metadata = map[string]string{}
	}
	meta, err := json.Marshal(metadata)
	if err != nil {
		return 0, fmt.Errorf("metadata marshal: %w", err)
	}
	var id int64
	err = s.db.QueryRow(
		`INSERT INTO messages (recipient, body, metadata) VALUES (?, ?, ?) RETURNING id`,
		recipient, body, string(meta),
	).Scan(&id)
	return id, err
}

// Dequeue claims the oldest queued message (FIFO) and marks it
// processing. Returns nil when the queue is empty.
func (s *Store) Dequeue() (*Message, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	row := tx.QueryRow(
		`SELECT id, recipient, body, metadata, status FROM messages
		 WHERE status = 'queued' ORDER BY created_at, id LIMIT 1`)
	var (
		msg     Message
		metaStr string
	)
	err = row.Scan(&msg.ID, &msg.Recipient, &msg.Body, &metaStr, &msg.Status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`UPDATE messages SET status='processing' WHERE id = ?`, msg.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	msg.Metadata = map[string]string{}
	if err := json.Unmarshal([]byte(metaStr), &msg.Metadata); err != nil {
		return nil, fmt.Errorf("metadata unmarshal: %w", err)
	}
	return &msg, nil
}

// MarkSent finalizes a claimed message with the relay that accepted it.
func (s *Store) MarkSent(id int64, relay string) error {
	_, err := s.db.Exec(
		`UPDATE messages SET status='sent', sent_at=strftime('%Y-%m-%dT%H:%M:%fZ','now'), relay=? WHERE id=?`,
		relay, id)
	return err
}

// RecordAttempt logs a delivery attempt against a message.
func (s *Store) RecordAttempt(messageID int64, relay, smtpCode, diagnostic string) error {
	_, err := s.db.Exec(
		`INSERT INTO attempts (message_id, relay, smtp_code, diagnostic) VALUES (?, ?, ?, ?)`,
		messageID, relay, smtpCode, diagnostic)
	return err
}

// RecordBounce stores a classified bounce.
func (s *Store) RecordBounce(messageID int64, recipient, smtpCode, diagnostic string, class queue.BounceClass) error {
	_, err := s.db.Exec(
		`INSERT INTO bounces (message_id, recipient, smtp_code, diagnostic, class) VALUES (?, ?, ?, ?, ?)`,
		messageID, recipient, smtpCode, diagnostic, string(class))
	return err
}

// RecordComplaint stores a spam complaint.
func (s *Store) RecordComplaint(messageID int64, recipient string) error {
	_, err := s.db.Exec(
		`INSERT INTO complaints (message_id, recipient) VALUES (?, ?)`,
		messageID, recipient)
	return err
}

// UpsertDKIM records a DKIM selector's public key for a domain. Calling
// again with a new selector is the rotation path.
func (s *Store) UpsertDKIM(domain, selector, publicKey, createdAt string) error {
	_, err := s.db.Exec(
		`INSERT INTO dkims (domain, selector, public_key, created_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(domain, selector) DO UPDATE SET public_key=excluded.public_key, created_at=excluded.created_at`,
		domain, selector, publicKey, createdAt)
	return err
}
