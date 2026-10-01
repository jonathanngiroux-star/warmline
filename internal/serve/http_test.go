package serve

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jonathanngiroux-star/warmline/internal/store"
)

func newTestServe(t *testing.T) (*store.Store, http.Handler) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "serve.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	mux := http.NewServeMux()
	mux.HandleFunc("/", queuePage(db))
	mux.HandleFunc("/donate", donatePage())
	mux.HandleFunc("/hooks/sendgrid", sendgridHook(db))
	return db, mux
}

func TestQueuePage(t *testing.T) {
	db, mux := newTestServe(t)
	if _, err := db.Enqueue("a@example.net", "x", nil); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "total") {
		t.Errorf("queue page missing stats:\n%s", body)
	}
	if !strings.Contains(string(body), "/donate") {
		t.Error("queue page missing donate link")
	}
}

func TestDonatePage(t *testing.T) {
	_, mux := newTestServe(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/donate")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	for _, want := range []string{DonateEthereum, DonateBitcoin} {
		if !strings.Contains(string(body), want) {
			t.Errorf("donate page missing %s", want)
		}
	}
}

func TestSendgridHook(t *testing.T) {
	db, mux := newTestServe(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	payload := `[{"event":"delivered","email":"u1@example.net","sg_event_id":"E1","sg_message_id":"M1","timestamp":1759305600},{"event":"bounce","email":"u2@example.net","sg_event_id":"E2","sg_message_id":"M2","timestamp":1759305660,"status":"5.1.1","reason":"550 5.1.1 no such user","bounce_type":"hard"}]`
	resp, err := http.Post(srv.URL+"/hooks/sendgrid", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d: %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), `"accepted":2`) {
		t.Errorf("hook response wrong: %s", body)
	}
	// The bounce must be recorded in the store.
	var n int
	if err := db.DB().QueryRow("SELECT COUNT(*) FROM bounces WHERE recipient = ?", "u2@example.net").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("bounces for u2 = %d, want 1", n)
	}
}

func TestSendgridHookRejectsGarbage(t *testing.T) {
	_, mux := newTestServe(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/hooks/sendgrid", "application/json", strings.NewReader(`garbage`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("garbage payload status = %d, want 400", resp.StatusCode)
	}
}

func TestSendgridHookRejectsGet(t *testing.T) {
	_, mux := newTestServe(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/hooks/sendgrid")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET status = %d, want 405", resp.StatusCode)
	}
}
