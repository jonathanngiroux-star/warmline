package store

import "database/sql"

// settings.go: a tiny key/value table for first-run UI state (whether
// the setup wizard has been shown). Keys are app-defined constants;
// values are short strings.

// Setting returns the value for key, or "" when unset.
func (s *Store) Setting(key string) string {
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return ""
	}
	if err != nil {
		return ""
	}
	return v
}

// SetSetting upserts key/value.
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		key, value)
	return err
}
