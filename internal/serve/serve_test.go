package serve

import (
	"encoding/json"
	"path/filepath"
	"testing"

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
