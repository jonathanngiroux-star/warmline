package store

import (
	"encoding/json"
	"fmt"
	"testing"
)

// ListComplaints: never-tested path. Must return [] (not JSON null),
// newest first, and honor the limit.
func TestListComplaints(t *testing.T) {
	db := openTestDB(t)
	if rows, err := db.ListComplaints(10); err != nil {
		t.Fatalf("empty list: %v", err)
	} else if len(rows) != 0 {
		t.Fatalf("fresh db complaints = %d, want 0", len(rows))
	}
	for i := 0; i < 3; i++ {
		// complaints reference messages (FK): enqueue first
		id, err := db.Enqueue(fmt.Sprintf("c%d@example.net", i), "body", nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.RecordComplaint(id, fmt.Sprintf("c%d@example.net", i)); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := db.ListComplaints(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("limited list = %d, want 2", len(rows))
	}
	// newest first
	if rows[0].Recipient != "c2@example.net" {
		t.Errorf("newest first: got %q, want c2@example.net", rows[0].Recipient)
	}
	all, err := db.ListComplaints(0) // 0 -> default 50
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("default limit list = %d, want 3", len(all))
	}
}

// The empty list MUST be non-nil so JSON surfaces marshal as [] and
// not null (JS clients crash on .length of null).
func TestListComplaintsEmptyNeverNil(t *testing.T) {
	db := openTestDB(t)
	rows, err := db.ListComplaints(10)
	if err != nil {
		t.Fatal(err)
	}
	if rows == nil {
		t.Fatal("empty list must be non-nil so JSON encodes as [] not null")
	}
	b, _ := json.Marshal(rows)
	if string(b) == "null" {
		t.Fatal("empty complaints must marshal as [], got null")
	}
}
