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
