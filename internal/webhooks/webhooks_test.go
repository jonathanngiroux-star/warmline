package webhooks

import (
	"os"
	"testing"
	"time"

	"github.com/jonathanngiroux-star/warmline/internal/queue"
)

const sendgridEventsFixture = "../../testdata/fixtures/sendgrid/event-webhook.json"

func loadFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(sendgridEventsFixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return raw
}

func TestNormalizeSendgrid(t *testing.T) {
	events, unmapped, err := NormalizeSendgrid(loadFixture(t))
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if len(events) != 8 {
		t.Fatalf("events = %d, want 8 (9 fixture events, 1 unmapped)", len(events))
	}
	if len(unmapped) != 1 || unmapped[0] != "dropped" {
		t.Errorf("unmapped = %v, want [dropped]", unmapped)
	}
	if events[0].Kind != queue.EventDelivered {
		t.Errorf("events[0].Kind = %q, want delivered", events[0].Kind)
	}
	// Deterministic timestamp conversion: unix epoch -> RFC3339 UTC.
	wantAt := time.Unix(1759305600, 0).UTC().Format(time.RFC3339)
	if events[0].At != wantAt {
		t.Errorf("events[0].At = %q, want %q", events[0].At, wantAt)
	}
	if events[0].ESPEventID != "SGEV1" {
		t.Errorf("events[0].ESPEventID = %q, want SGEV1", events[0].ESPEventID)
	}
}

func TestNormalizeSendgridBounce(t *testing.T) {
	events, _, err := NormalizeSendgrid(loadFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	var bounce *queue.CanonicalEvent
	for i := range events {
		if events[i].Kind == queue.EventBounce {
			bounce = &events[i]
		}
	}
	if bounce == nil {
		t.Fatal("no bounce event in output")
	}
	if bounce.BounceClass != queue.BounceHard {
		t.Errorf("bounce class = %q, want hard", bounce.BounceClass)
	}
	if bounce.SMTPCode != "5.1.1" {
		t.Errorf("bounce smtp_code = %q, want 5.1.1", bounce.SMTPCode)
	}
	if bounce.Recipient != "user2@example.net" {
		t.Errorf("bounce recipient = %q, want user2@example.net", bounce.Recipient)
	}
	if bounce.Diagnostic == "" {
		t.Error("bounce diagnostic is empty")
	}
}

func TestNormalizeSendgridDeferredAndAliases(t *testing.T) {
	events, _, err := NormalizeSendgrid(loadFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[queue.EventKind]int{}
	for _, ev := range events {
		kinds[ev.Kind]++
	}
	if kinds[queue.EventDeferred] != 1 {
		t.Errorf("deferred count = %d, want 1", kinds[queue.EventDeferred])
	}
	// group_unsubscribe must alias to canonical unsubscribe, not vanish.
	if kinds[queue.EventUnsub] != 2 {
		t.Errorf("unsubscribe count = %d, want 2 (unsubscribe + group_unsubscribe)", kinds[queue.EventUnsub])
	}
	if kinds[queue.EventSpamReport] != 1 {
		t.Errorf("spam_report count = %d, want 1", kinds[queue.EventSpamReport])
	}
	var deferred *queue.CanonicalEvent
	for i := range events {
		if events[i].Kind == queue.EventDeferred {
			deferred = &events[i]
		}
	}
	if deferred == nil || deferred.SMTPCode != "4.2.2" {
		t.Errorf("deferred smtp_code missing or wrong: %+v", deferred)
	}
}

func TestNormalizeSendgridInvalidInput(t *testing.T) {
	if _, _, err := NormalizeSendgrid([]byte(`{"not":"an array"}`)); err == nil {
		t.Error("single object should error: SendGrid posts an array")
	}
	if _, _, err := NormalizeSendgrid([]byte(`not json`)); err == nil {
		t.Error("garbage should error")
	}
}

func TestSendgridEventKindContract(t *testing.T) {
	// The migrate report and the webhook normalizer must share one
	// mapping table — a drift between them is a fidelity bug.
	for name, want := range map[string]queue.EventKind{
		"processed":         queue.EventProcessed,
		"delivered":         queue.EventDelivered,
		"bounce":            queue.EventBounce,
		"group_unsubscribe": queue.EventUnsub,
	} {
		got, ok := SendgridEventKind(name)
		if !ok || got != want {
			t.Errorf("SendgridEventKind(%q) = %q,%v want %q,true", name, got, ok, want)
		}
	}
	if _, ok := SendgridEventKind("dropped"); ok {
		t.Error("dropped must be unmapped (ESP-side pre-queue discard)")
	}
}

// A bounce event with bounce_type absent must classify via the SMTP-code
// taxonomy, not silently become unknown.
func TestNormalizeSendgridBounceNoPreclass(t *testing.T) {
	payload := `[{"event":"bounce","email":"u@example.net","sg_event_id":"E1","sg_message_id":"M1","timestamp":1759305600,"status":"5.1.1","reason":"550 5.1.1 user unknown"}]`
	events, unmapped, err := NormalizeSendgrid([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	if len(unmapped) != 0 {
		t.Errorf("unmapped = %v", unmapped)
	}
	if len(events) != 1 || events[0].BounceClass != queue.BounceHard {
		t.Errorf("bounce without preclass should classify hard via 5.1.1: %+v", events)
	}
}

// A soft-classified bounce (bounce_type: soft) with a 5xx code is still
// soft per SendGrid's own label — the ESP's classification wins.
func TestNormalizeSendgridBounceSoftPreclass(t *testing.T) {
	payload := `[{"event":"bounce","email":"u@example.net","bounce_type":"soft","status":"4.2.1","reason":"450 busy","sg_event_id":"E1","sg_message_id":"M1","timestamp":1759305600}]`
	events, _, err := NormalizeSendgrid([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	if events[0].BounceClass != queue.BounceSoft {
		t.Errorf("soft preclass should hold: %+v", events[0])
	}
}

// Events missing a timestamp must still normalize (At stays empty, not
// a zero-date string).
func TestNormalizeSendgridMissingTimestamp(t *testing.T) {
	payload := `[{"event":"delivered","email":"u@example.net","sg_event_id":"E1","sg_message_id":"M1"}]`
	events, _, err := NormalizeSendgrid([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d", len(events))
	}
	if events[0].At != "" {
		t.Errorf("At should be empty when timestamp absent, got %q", events[0].At)
	}
	if events[0].Recipient != "u@example.net" {
		t.Errorf("recipient lost: %+v", events[0])
	}
}

// An empty event array is a legal SendGrid webhook body.
func TestNormalizeSendgridEmptyArray(t *testing.T) {
	events, unmapped, err := NormalizeSendgrid([]byte(`[]`))
	if err != nil {
		t.Fatalf("empty array must not error: %v", err)
	}
	if len(events) != 0 || len(unmapped) != 0 {
		t.Errorf("want empty results, got %d events, %v unmapped", len(events), unmapped)
	}
}

// Duplicate unmapped event names are reported once, not per event.
func TestNormalizeSendgridUnmappedDedup(t *testing.T) {
	payload := `[{"event":"dropped","email":"a@x.com"},{"event":"dropped","email":"b@x.com"}]`
	_, unmapped, err := NormalizeSendgrid([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	if len(unmapped) != 1 || unmapped[0] != "dropped" {
		t.Errorf("unmapped = %v, want [dropped] once", unmapped)
	}
}
