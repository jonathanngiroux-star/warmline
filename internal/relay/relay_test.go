package relay

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jonathanngiroux-star/warmline/internal/mta"
	"github.com/jonathanngiroux-star/warmline/internal/store"
)

// startFakeRelay runs an mta.Server as the user's "own SMTP relay".
func startFakeRelay(t *testing.T) (*mta.Server, string) {
	t.Helper()
	srv := mta.New()
	if err := srv.Listen("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	go srv.Serve()
	t.Cleanup(func() { srv.Close() })
	return srv, srv.Addr()
}

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestDeliverMarksSentAndRecordsAttempt(t *testing.T) {
	fake, addr := startFakeRelay(t)
	db := openTestStore(t)
	if _, err := db.Enqueue("user@example.net", "From: app@example.com\r\nSubject: t\r\n\r\nbody\r\n", nil); err != nil {
		t.Fatal(err)
	}

	r := New(Config{Addr: addr})
	n, err := r.WorkOnce(db)
	if err != nil {
		t.Fatalf("work once: %v", err)
	}
	if n != 1 {
		t.Fatalf("delivered = %d, want 1", n)
	}

	// The fake relay received the message.
	inbox := fake.Inbox()
	if len(inbox) != 1 {
		t.Fatalf("relay inbox = %d, want 1", len(inbox))
	}
	if inbox[0].To[0] != "user@example.net" {
		t.Errorf("relay recipient = %v", inbox[0].To)
	}
	if inbox[0].Data != "From: app@example.com\r\nSubject: t\r\n\r\nbody\r\n" {
		t.Errorf("relay data = %q", inbox[0].Data)
	}

	// Store: message sent, attempt logged.
	msg, err := db.Dequeue()
	if err != nil {
		t.Fatal(err)
	}
	if msg != nil {
		t.Errorf("queue should be empty after delivery, got %+v", msg)
	}
	var status string
	if err := db.DB().QueryRow("SELECT status FROM messages LIMIT 1").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "sent" {
		t.Errorf("status = %q, want sent", status)
	}
	var attempts int
	if err := db.DB().QueryRow("SELECT COUNT(*) FROM attempts").Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
}

func TestDeliverFailureRequeues(t *testing.T) {
	db := openTestStore(t)
	if _, err := db.Enqueue("user@example.net", "body\r\n", nil); err != nil {
		t.Fatal(err)
	}

	// Point at a dead port: delivery must fail, message must be back to
	// queued, attempt recorded with the error.
	r := New(Config{Addr: "127.0.0.1:1"})
	n, err := r.WorkOnce(db)
	if err == nil {
		t.Fatal("delivery to dead port should error")
	}
	if n != 0 {
		t.Errorf("delivered = %d, want 0", n)
	}

	var status string
	if err := db.DB().QueryRow("SELECT status FROM messages LIMIT 1").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "queued" {
		t.Errorf("status after failed delivery = %q, want queued (retryable)", status)
	}
	var attempts int
	if err := db.DB().QueryRow("SELECT COUNT(*) FROM attempts").Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (failure logged)", attempts)
	}
}

func TestWorkOnceEmptyQueue(t *testing.T) {
	db := openTestStore(t)
	r := New(Config{Addr: "127.0.0.1:1"})
	n, err := r.WorkOnce(db)
	if err != nil {
		t.Fatalf("empty queue should not error: %v", err)
	}
	if n != 0 {
		t.Errorf("delivered = %d, want 0", n)
	}
}

func TestConfigValidation(t *testing.T) {
	if err := (Config{}).Validate(); err == nil {
		t.Error("empty config should fail")
	}
	if err := (Config{Addr: "relay.example.com:587", Username: "u", Password: "p"}).Validate(); err != nil {
		t.Errorf("valid config rejected: %v", err)
	}
}

// Envelope-from fallback: a From: header with a display name ("From: Acme
// <a@b.com>") must extract the addr-spec only — relays reject display
// names in the envelope.
func TestDeliverFromHeaderWithDisplayName(t *testing.T) {
	fake, addr := startFakeRelay(t)
	db := openTestStore(t)
	if _, err := db.Enqueue("user@example.net",
		"From: Acme Notices <app@example.com>\r\nSubject: t\r\n\r\nbody\r\n", nil); err != nil {
		t.Fatal(err)
	}
	r := New(Config{Addr: addr})
	if _, err := r.WorkOnce(db); err != nil {
		t.Fatalf("work once: %v", err)
	}
	inbox := fake.Inbox()
	if len(inbox) != 1 {
		t.Fatalf("relay inbox = %d, want 1", len(inbox))
	}
	if inbox[0].From != "app@example.com" {
		t.Errorf("envelope-from = %q, want addr-spec only (app@example.com)", inbox[0].From)
	}
}

// LF-only line endings in the stored body must still yield a From header.
func TestDeliverFromHeaderLFOnly(t *testing.T) {
	fake, addr := startFakeRelay(t)
	db := openTestStore(t)
	if _, err := db.Enqueue("user@example.net", "From: app@example.com\nSubject: t\n\nbody\n", nil); err != nil {
		t.Fatal(err)
	}
	r := New(Config{Addr: addr})
	if _, err := r.WorkOnce(db); err != nil {
		t.Fatalf("work once: %v", err)
	}
	inbox := fake.Inbox()
	if len(inbox) != 1 || inbox[0].From != "app@example.com" {
		t.Errorf("LF-only body: from = %v", inbox)
	}
}

// splitHostPort: 0% coverage but a live caller (deliver). IPv6-safe:
// splits on the LAST colon; rejects malformed forms by naming them.
func TestSplitHostPort(t *testing.T) {
	cases := []struct {
		in         string
		host, port string
		wantErr    bool
	}{
		{"smtp.example.com:587", "smtp.example.com", "587", false},
		{"127.0.0.1:2525", "127.0.0.1", "2525", false},
		{"[::1]:587", "[::1]", "587", false},
		{"smtp.example.com", "", "", true},  // no port
		{":587", "", "", true},              // no host
		{"smtp.example.com:", "", "", true}, // empty port
	}
	for _, c := range cases {
		host, port, err := splitHostPort(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("splitHostPort(%q): want error, got %q %q", c.in, host, port)
			} else if !strings.Contains(err.Error(), c.in) {
				t.Errorf("error must name the offending addr: %v", err)
			}
			continue
		}
		if err != nil {
			t.Errorf("splitHostPort(%q): %v", c.in, err)
			continue
		}
		if host != c.host || port != c.port {
			t.Errorf("splitHostPort(%q) = %q,%q; want %q,%q", c.in, host, port, c.host, c.port)
		}
	}
}
