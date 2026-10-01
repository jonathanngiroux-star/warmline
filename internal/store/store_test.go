package store

import (
	"path/filepath"
	"testing"

	"github.com/jonathanngiroux-star/warmline/internal/queue"
)

func openTestDB(t *testing.T) *Store {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSchemaCreated(t *testing.T) {
	db := openTestDB(t)
	for _, table := range []string{"messages", "attempts", "bounces", "complaints", "dkims"} {
		var name string
		// Table names come from our own list; not user input.
		err := db.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='" + table + "'").Scan(&name)
		if err != nil {
			t.Errorf("table %s missing: %v", table, err)
		}
	}
}

func TestEnqueueDequeue(t *testing.T) {
	db := openTestDB(t)
	id, err := db.Enqueue("user@example.net", "hello", map[string]string{"tenant": "acme"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if id == 0 {
		t.Fatal("enqueue returned id 0")
	}
	msg, err := db.Dequeue()
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if msg == nil {
		t.Fatal("dequeue returned nil, want the enqueued message")
	}
	if msg.ID != id {
		t.Errorf("dequeued id = %d, want %d", msg.ID, id)
	}
	if msg.Recipient != "user@example.net" {
		t.Errorf("recipient = %q, want user@example.net", msg.Recipient)
	}
	if msg.Body != "hello" {
		t.Errorf("body = %q, want hello", msg.Body)
	}
	if msg.Metadata["tenant"] != "acme" {
		t.Errorf("metadata[tenant] = %q, want acme", msg.Metadata["tenant"])
	}
}

func TestDequeueEmpty(t *testing.T) {
	db := openTestDB(t)
	msg, err := db.Dequeue()
	if err != nil {
		t.Fatalf("dequeue on empty queue: %v", err)
	}
	if msg != nil {
		t.Errorf("dequeue on empty queue returned %+v, want nil", msg)
	}
}

func TestDequeueFIFO(t *testing.T) {
	db := openTestDB(t)
	if _, err := db.Enqueue("a@example.net", "first", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Enqueue("b@example.net", "second", nil); err != nil {
		t.Fatal(err)
	}
	m1, err := db.Dequeue()
	if err != nil {
		t.Fatal(err)
	}
	m2, err := db.Dequeue()
	if err != nil {
		t.Fatal(err)
	}
	if m1.Recipient != "a@example.net" || m2.Recipient != "b@example.net" {
		t.Errorf("FIFO violated: got %q then %q", m1.Recipient, m2.Recipient)
	}
}

func TestDequeueMarksProcessing(t *testing.T) {
	// Dequeue claims a message (status=processing) so the next Dequeue
	// does not hand out the same message twice.
	db := openTestDB(t)
	if _, err := db.Enqueue("a@example.net", "x", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Dequeue(); err != nil {
		t.Fatal(err)
	}
	msg, err := db.Dequeue()
	if err != nil {
		t.Fatal(err)
	}
	if msg != nil {
		t.Errorf("second dequeue returned %+v, want nil (already claimed)", msg)
	}
}

func TestMarkResult(t *testing.T) {
	db := openTestDB(t)
	id, err := db.Enqueue("user@example.net", "hi", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Dequeue(); err != nil {
		t.Fatal(err)
	}
	if err := db.MarkSent(id, "relay-acme"); err != nil {
		t.Fatalf("mark sent: %v", err)
	}
	var status string
	if err := db.db.QueryRow("SELECT status FROM messages WHERE id = ?", id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "sent" {
		t.Errorf("status = %q, want sent", status)
	}
}

func TestRecordAttemptBounceComplaint(t *testing.T) {
	db := openTestDB(t)
	id, err := db.Enqueue("user@example.net", "hi", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.RecordAttempt(id, "relay-acme", "421", "try again later"); err != nil {
		t.Fatalf("record attempt: %v", err)
	}
	if err := db.RecordBounce(id, "user@example.net", "550", "mailbox unavailable", queue.BounceHard); err != nil {
		t.Fatalf("record bounce: %v", err)
	}
	if err := db.RecordComplaint(id, "user@example.net"); err != nil {
		t.Fatalf("record complaint: %v", err)
	}
	var n int
	if err := db.db.QueryRow("SELECT COUNT(*) FROM bounces").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("bounces count = %d, want 1", n)
	}
	if err := db.db.QueryRow("SELECT COUNT(*) FROM complaints").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("complaints count = %d, want 1", n)
	}
	if err := db.db.QueryRow("SELECT COUNT(*) FROM attempts").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("attempts count = %d, want 1", n)
	}
}

func TestBounceClassStored(t *testing.T) {
	db := openTestDB(t)
	id, err := db.Enqueue("user@example.net", "hi", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.RecordBounce(id, "user@example.net", "550", "mailbox unavailable", queue.BounceHard); err != nil {
		t.Fatal(err)
	}
	var class string
	if err := db.db.QueryRow("SELECT class FROM bounces WHERE message_id = ?", id).Scan(&class); err != nil {
		t.Fatal(err)
	}
	if class != "hard" {
		t.Errorf("stored bounce class = %q, want hard", class)
	}
}

func TestDKIMRoundTrip(t *testing.T) {
	db := openTestDB(t)
	err := db.UpsertDKIM("example.com", "s1", "MIIBIjANBgkq...public", "2026-09-30T00:00:00Z")
	if err != nil {
		t.Fatalf("upsert dkim: %v", err)
	}
	// Upsert with a new selector = rotation path.
	err = db.UpsertDKIM("example.com", "s2", "MIIBIjANBgkq...public2", "2026-10-01T00:00:00Z")
	if err != nil {
		t.Fatalf("upsert dkim rotation: %v", err)
	}
	rows, err := db.db.Query("SELECT selector FROM dkims WHERE domain = ? ORDER BY created_at", "example.com")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		got = append(got, s)
	}
	if len(got) != 2 || got[0] != "s1" || got[1] != "s2" {
		t.Errorf("dkim rows = %v, want [s1 s2]", got)
	}
}

func TestDBAccessor(t *testing.T) {
	db := openTestDB(t)
	if db.DB() == nil {
		t.Fatal("DB() returned nil")
	}
	var one int
	if err := db.DB().QueryRow("SELECT 1").Scan(&one); err != nil || one != 1 {
		t.Fatalf("DB() not queryable: %v", err)
	}
}
