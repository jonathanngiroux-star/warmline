package store

import (
	"encoding/json"
	"testing"
)

// Every list method must return a NON-nil slice on empty so JSON
// surfaces encode [] and not null (JS .length crashes). Pin all four.
func TestEmptyListsMarshalAsArray(t *testing.T) {
	db := openTestDB(t)

	msgs, err := db.ListMessages(10)
	if err != nil {
		t.Fatal(err)
	}
	if msgs == nil {
		t.Error("ListMessages: empty must be non-nil (JSON [] not null)")
	}
	if b, _ := json.Marshal(msgs); string(b) == "null" {
		t.Error("ListMessages: marshals as null")
	}

	bounces, err := db.ListBounces(10)
	if err != nil {
		t.Fatal(err)
	}
	if bounces == nil {
		t.Error("ListBounces: empty must be non-nil (JSON [] not null)")
	}
	if b, _ := json.Marshal(bounces); string(b) == "null" {
		t.Error("ListBounces: marshals as null")
	}

	complaints, err := db.ListComplaints(10)
	if err != nil {
		t.Fatal(err)
	}
	if complaints == nil {
		t.Error("ListComplaints: empty must be non-nil (JSON [] not null)")
	}

	dkims, err := db.ListDKIMs()
	if err != nil {
		t.Fatal(err)
	}
	if dkims == nil {
		t.Error("ListDKIMs: empty must be non-nil (JSON [] not null)")
	}
	if b, _ := json.Marshal(dkims); string(b) == "null" {
		t.Error("ListDKIMs: marshals as null")
	}
}
