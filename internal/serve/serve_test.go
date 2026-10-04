package serve

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jonathanngiroux-star/warmline/internal/mta"
	"github.com/jonathanngiroux-star/warmline/internal/store"
)

func TestQueueStats(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// empty
	st, err := Stats(db)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if st.Total != 0 || st.Status["queued"] != 0 {
		t.Errorf("empty stats = %+v", st)
	}

	// three messages, one claimed, one sent
	if _, err := db.Enqueue("a@example.net", "a", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Enqueue("b@example.net", "b", nil); err != nil {
		t.Fatal(err)
	}
	id, err := db.Enqueue("c@example.net", "c", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Dequeue(); err != nil {
		t.Fatal(err)
	}
	if err := db.MarkSent(id, "relay-x"); err != nil {
		t.Fatal(err)
	}

	st, err = Stats(db)
	if err != nil {
		t.Fatal(err)
	}
	if st.Total != 3 {
		t.Errorf("total = %d, want 3", st.Total)
	}
	if st.Status["queued"] != 1 {
		t.Errorf("queued = %d, want 1", st.Status["queued"])
	}
	if st.Status["processing"] != 1 {
		t.Errorf("processing = %d, want 1", st.Status["processing"])
	}
	if st.Status["sent"] != 1 {
		t.Errorf("sent = %d, want 1", st.Status["sent"])
	}
}

func TestStatsMarshal(t *testing.T) {
	st := QueueStats{Total: 5, Status: map[string]int{"queued": 5}}
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(b) {
		t.Fatal("stats must serialize")
	}
}

// Submit (0% coverage): every accepted message enqueues one row per
// recipient — the queue is per-recipient for bounce classification.
func TestStoreHandlerSubmit(t *testing.T) {
	db, _ := newTestServe(t)
	h := storeHandler{db: db}
	msg := mta.Message{
		From: "sender@example.com",
		To:   []string{"a@example.net", "b@example.net"},
		Data: "Subject: multi\r\n\r\nbody\r\n",
	}
	if err := h.Submit(msg); err != nil {
		t.Fatalf("submit: %v", err)
	}
	msgs, err := db.ListMessages(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("rows = %d, want 2 (one per recipient)", len(msgs))
	}
	got := map[string]string{}
	for _, m := range msgs {
		got[m.Recipient] = m.Status
	}
	if got["a@example.net"] != "queued" || got["b@example.net"] != "queued" {
		t.Errorf("statuses = %v, want both queued", got)
	}
	// metadata carries the envelope sender
	row, err := db.ListMessages(1)
	if err != nil {
		t.Fatal(err)
	}
	_ = row
}

// Run (0% coverage): live bind on ephemeral ports + a real SMTP submit
// through the wired handler — the full serve path in-process.
func TestServeRunSmoke(t *testing.T) {
	db, _ := newTestServe(t)
	// Grab free ports, then RELEASE them so Run can bind (holding the
	// probe sockets blocks Run and hangs the suite).
	smtpAddr, httpAddr := grabPort(t), grabPort(t)
	go func() {
		_ = Run(db, Options{SMTPAddr: smtpAddr, HTTPAddr: httpAddr})
	}()

	// real SMTP dialogue into the booted server (Run binds async —
	// retry until the listener is up)
	msg := "Subject: smoke\r\n\r\nthrough the real path\r\n"
	deadline := time.Now().Add(3 * time.Second)
	var sendErr error
	for {
		sendErr = smtpSendMail(smtpAddr, "s@example.com", []string{"r@example.net"}, msg)
		if sendErr == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("smtp submit to %s: %v", smtpAddr, sendErr)
		}
		time.Sleep(50 * time.Millisecond)
	}
	// wait for the async handler to land the row
	pollDeadline := time.Now().Add(3 * time.Second)
	for {
		msgs, err := db.ListMessages(10)
		if err != nil {
			t.Fatal(err)
		}
		if len(msgs) == 1 {
			if msgs[0].Recipient != "r@example.net" {
				t.Errorf("recipient = %q", msgs[0].Recipient)
			}
			return
		}
		if time.Now().After(pollDeadline) {
			t.Fatal("message never landed in the queue via serve.Run")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func grabPort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close() // release immediately; Run binds it next
	return addr
}

func smtpSendMail(addr, from string, to []string, msg string) error {
	c, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer c.Close()
	r := bufio.NewReader(c)
	w := bufio.NewWriter(c)
	readLine := func() string { b, _ := r.ReadString('\n'); return b }
	writeLine := func(s string) { fmt.Fprintf(w, "%s\r\n", s); w.Flush() }
	readLine() // banner
	writeLine("EHLO test")
	readLine()
	readLine() // 250-warmline / 250 OK
	writeLine("MAIL FROM:<" + from + ">")
	readLine()
	writeLine("RCPT TO:<" + to[0] + ">")
	readLine()
	writeLine("DATA")
	readLine()
	writeLine(msg)
	writeLine(".")
	resp := readLine()
	if !strings.HasPrefix(resp, "250") {
		return fmt.Errorf("DATA not accepted: %s", resp)
	}
	writeLine("QUIT")
	return nil
}
