package store

// Listing queries for the TUI/GUI read views. All limit-clamped, all
// newest-first (the operator wants the recent activity on top).

// MessageRow is one queue message for a read-only listing.
type MessageRow struct {
	ID        int64
	Recipient string
	Status    string
	Relay     string
	CreatedAt string
}

// ListMessages returns the most recent queue messages (any status).
func (s *Store) ListMessages(limit int) ([]MessageRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT id, recipient, status, relay, created_at
		 FROM messages ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MessageRow{}
	for rows.Next() {
		var r MessageRow
		if err := rows.Scan(&r.ID, &r.Recipient, &r.Status, &r.Relay, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// BounceRow is one classified bounce, newest first.
type BounceRow struct {
	ID         int64
	MessageID  int64 // 0 = ESP-originated (webhook), no local queue row
	Recipient  string
	SMTPCode   string
	Diagnostic string
	Class      string
	At         string
}

// ListBounces returns the most recent bounces.
func (s *Store) ListBounces(limit int) ([]BounceRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT id, COALESCE(message_id, 0), recipient, smtp_code, diagnostic, class, at
		 FROM bounces ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BounceRow{}
	for rows.Next() {
		var r BounceRow
		if err := rows.Scan(&r.ID, &r.MessageID, &r.Recipient, &r.SMTPCode, &r.Diagnostic, &r.Class, &r.At); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ComplaintRow is one spam complaint, newest first.
type ComplaintRow struct {
	ID        int64
	MessageID int64
	Recipient string
	At        string
}

// ListComplaints returns the most recent complaints.
func (s *Store) ListComplaints(limit int) ([]ComplaintRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT id, COALESCE(message_id, 0), recipient, at
		 FROM complaints ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ComplaintRow{}
	for rows.Next() {
		var r ComplaintRow
		if err := rows.Scan(&r.ID, &r.MessageID, &r.Recipient, &r.At); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DKIMRow is one DKIM selector record.
type DKIMRow struct {
	ID        int64
	Domain    string
	Selector  string
	PublicKey string
	CreatedAt string
}

// ListDKIMs returns all DKIM selector records, newest first.
func (s *Store) ListDKIMs() ([]DKIMRow, error) {
	rows, err := s.db.Query(
		`SELECT id, domain, selector, public_key, created_at
		 FROM dkims ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DKIMRow{}
	for rows.Next() {
		var r DKIMRow
		if err := rows.Scan(&r.ID, &r.Domain, &r.Selector, &r.PublicKey, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
