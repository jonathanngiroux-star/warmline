// Package webhooks normalizes ESP event webhook payloads into Warmline
// canonical events. SendgridEventKind is the single mapping table shared
// with the migrate dry-run report (internal/migrate/sendgrid imports it)
// so the report and the normalizer can never drift.
package webhooks

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jonathanngiroux-star/warmline/internal/queue"
)

// SendgridEventKind maps a SendGrid webhook event name to its canonical
// kind. Names absent from this table are unmapped and reported by both
// the migrate dry-run and NormalizeSendgrid — never silently dropped.
// Deliberately unmapped: `dropped` (ESP-side pre-queue discard, never
// enters a Warmline queue) and `group_resubscribe` (unsubscribe-group
// management, not a delivery event).
func SendgridEventKind(name string) (queue.EventKind, bool) {
	switch name {
	case "processed":
		return queue.EventProcessed, true
	case "delivered":
		return queue.EventDelivered, true
	case "open":
		return queue.EventOpen, true
	case "click":
		return queue.EventClick, true
	case "bounce":
		return queue.EventBounce, true
	case "deferred":
		return queue.EventDeferred, true
	case "spam_report":
		return queue.EventSpamReport, true
	case "unsubscribe":
		return queue.EventUnsub, true
	case "group_unsubscribe":
		return queue.EventUnsub, true
	}
	return "", false
}

// NormalizeSendgrid parses a SendGrid event webhook body (a JSON array
// of event objects) and returns canonical events plus the names of any
// unmapped event types found.
func NormalizeSendgrid(body []byte) ([]queue.CanonicalEvent, []string, error) {
	var raw []map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, nil, fmt.Errorf("sendgrid webhook body must be a JSON array of events: %w", err)
	}
	events := make([]queue.CanonicalEvent, 0, len(raw))
	unmappedSet := map[string]bool{}
	var unmapped []string
	for _, item := range raw {
		name, _ := item["event"].(string)
		kind, ok := SendgridEventKind(name)
		if !ok {
			if !unmappedSet[name] {
				unmappedSet[name] = true
				unmapped = append(unmapped, name)
			}
			continue
		}
		ev := queue.CanonicalEvent{
			Kind:       kind,
			ESPEventID: str(item["sg_event_id"]),
			Recipient:  str(item["email"]),
		}
		// Message identity: SendGrid's sg_message_id carries a
		// system suffix (…=<system>-<id>); keep the raw value, it is
		// the ESP-side correlation key.
		ev.MessageID = str(item["sg_message_id"])
		if ts, ok := num(item["timestamp"]); ok {
			ev.At = time.Unix(int64(ts), 0).UTC().Format(time.RFC3339)
		}
		switch kind {
		case queue.EventBounce:
			ev.SMTPCode = str(item["status"])
			ev.Diagnostic = str(item["reason"])
			// SendGrid pre-classifies: "bounce_type": hard|soft.
			switch str(item["bounce_type"]) {
			case "hard":
				ev.BounceClass = queue.BounceHard
			case "soft":
				ev.BounceClass = queue.BounceSoft
			default:
				ev.BounceClass = queue.ClassifyBounce(ev.SMTPCode, ev.Diagnostic)
			}
		case queue.EventDeferred:
			ev.SMTPCode = str(item["status"])
			ev.Diagnostic = str(item["reason"])
			ev.BounceClass = queue.BounceSoft
		}
		if kind == queue.EventBounce || kind == queue.EventDeferred || kind == queue.EventSpamReport {
			// keep metadata light: attempt number for deferred is
			// occasionally useful in the queue UI
			if a := str(item["attempt_num"]); a != "" {
				ev.Metadata = map[string]string{"attempt_num": a}
			}
		}
		events = append(events, ev)
	}
	return events, unmapped, nil
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func num(v any) (float64, bool) {
	f, ok := v.(float64)
	return f, ok
}
