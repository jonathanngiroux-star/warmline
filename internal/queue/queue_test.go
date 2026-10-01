package queue

import (
	"encoding/json"
	"testing"
)

func TestClassifyBounce(t *testing.T) {
	tests := []struct {
		name       string
		code       string
		diagnostic string
		want       BounceClass
	}{
		{"550 user unknown", "550", "mailbox unavailable", BounceHard},
		{"553 no such user", "553", "requested action not taken: mailbox unavailable", BounceHard},
		{"554 permanent reject", "554", "delivery error: dd this user does not exist", BounceHard},
		{"421 graylist", "421", "try again later", BounceSoft},
		{"450 mailbox busy", "450", "mailbox unavailable", BounceSoft},
		{"452 insufficient storage", "452", "insufficient system storage", BounceSoft},
		{"empty code", "", "", BounceUnknown},
		{"non-smtp junk", "abc", "whatever", BounceUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyBounce(tc.code, tc.diagnostic); got != tc.want {
				t.Errorf("ClassifyBounce(%q, %q) = %q, want %q", tc.code, tc.diagnostic, got, tc.want)
			}
		})
	}
}

func TestCanonicalEventJSONShape(t *testing.T) {
	// The canonical event is the wire format every ESP mapper normalizes
	// into. Field names are stability-critical: webhook consumers and the
	// migration report depend on them.
	ev := CanonicalEvent{
		Kind:        EventBounce,
		MessageID:   "msg_01",
		ESPEventID:  "sg_123",
		Recipient:   "user@example.net",
		SMTPCode:    "550",
		Diagnostic:  "mailbox unavailable",
		BounceClass: BounceHard,
	}
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"type", "message_id", "esp_event_id", "at", "recipient", "smtp_code", "diagnostic", "bounce_class"} {
		if _, ok := m[key]; !ok {
			t.Errorf("canonical event JSON missing key %q in %s", key, b)
		}
	}
	if m["type"] != "bounce" {
		t.Errorf("type = %v, want bounce", m["type"])
	}
	if m["bounce_class"] != "hard" {
		t.Errorf("bounce_class = %v, want hard", m["bounce_class"])
	}
}

func TestCanonicalEventOmitsEmpty(t *testing.T) {
	// A delivered event carries no bounce detail — those fields must be
	// absent, not null/zero-value noise.
	ev := CanonicalEvent{Kind: EventDelivered, MessageID: "msg_01"}
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, absent := range []string{"smtp_code", "diagnostic", "bounce_class", "recipient", "esp_event_id", "metadata"} {
		if string(b) != "" && containsKey(b, absent) {
			t.Errorf("delivered event should omit %q: %s", absent, b)
		}
	}
}

func containsKey(b []byte, key string) bool {
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return false
	}
	_, ok := m[key]
	return ok
}

func TestClassifyBounceEnhancedCodes(t *testing.T) {
	tests := []struct {
		name string
		code string
		want BounceClass
	}{
		{"enhanced hard", "5.1.1", BounceHard},
		{"enhanced soft", "4.2.1", BounceSoft},
		{"enhanced hard 3-part", "5.7.1", BounceHard},
		{"reply code still works", "550", BounceHard},
		{"junk rejected", "abc", BounceUnknown},
		{"empty rejected", "", BounceUnknown},
		{"too short", "5.", BounceUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyBounce(tc.code, ""); got != tc.want {
				t.Errorf("ClassifyBounce(%q) = %q, want %q", tc.code, got, tc.want)
			}
		})
	}
}
