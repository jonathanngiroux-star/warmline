// Package queue defines the canonical event model every ESP webhook
// mapper normalizes into, plus the SMTP bounce taxonomy used by the queue
// store and the queue UI.
package queue

// EventKind is the normalized event type across ESPs.
type EventKind string

const (
	EventProcessed  EventKind = "processed"
	EventDelivered  EventKind = "delivered"
	EventOpen       EventKind = "open"
	EventClick      EventKind = "click"
	EventBounce     EventKind = "bounce"
	EventDeferred   EventKind = "deferred"
	EventSpamReport EventKind = "spam_report"
	EventUnsub      EventKind = "unsubscribe"
)

// BounceClass is the Warmline bounce taxonomy.
type BounceClass string

const (
	BounceHard    BounceClass = "hard"
	BounceSoft    BounceClass = "soft"
	BounceBlock   BounceClass = "block" // ISP/reputation-level block, not recipient-level
	BounceUnknown BounceClass = "unknown"
)

// CanonicalEvent is the stable wire shape every ESP mapper produces.
// Field names are stability-critical: the webhook normalizer, migration
// report, and queue UI all read them.
type CanonicalEvent struct {
	Kind        EventKind         `json:"type"`
	MessageID   string            `json:"message_id"`
	ESPEventID  string            `json:"esp_event_id,omitempty"`
	At          string            `json:"at"` // RFC3339
	Recipient   string            `json:"recipient,omitempty"`
	SMTPCode    string            `json:"smtp_code,omitempty"`
	Diagnostic  string            `json:"diagnostic,omitempty"`
	BounceClass BounceClass       `json:"bounce_class,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ClassifyBounce maps an SMTP reply code + diagnostic to the Warmline
// bounce taxonomy. Accepts both reply codes ("550", "421") and enhanced
// status codes ("5.1.1", "4.2.1") — ESPs send both forms. Codes outside
// 4xx/5xx and unparsable junk are unknown.
func ClassifyBounce(code, diagnostic string) BounceClass {
	if !codeClassifiable(code) {
		return BounceUnknown
	}
	// 5xx = permanent → hard unless the diagnostic screams reputation or
	// blocklist, which makes it a block, not a recipient failure.
	if code[0] == '5' {
		if blockListed(diagnostic) {
			return BounceBlock
		}
		return BounceHard
	}
	// 4xx = temporary → soft (retry).
	return BounceSoft
}

// codeClassifiable reports whether code is a 4xx/5xx reply code ("550")
// or enhanced status code ("5.1.1", "4.2.2"): first char 4 or 5,
// remaining chars digits or dots, at least 3 chars total.
func codeClassifiable(code string) bool {
	if len(code) < 3 {
		return false
	}
	if code[0] != '4' && code[0] != '5' {
		return false
	}
	for i := 1; i < len(code); i++ {
		c := code[i]
		if (c < '0' || c > '9') && c != '.' {
			return false
		}
	}
	return true
}

// blockListed reports whether a diagnostic looks like an ISP/reputation
// block rather than a per-recipient problem.
func blockListed(diagnostic string) bool {
	d := diagnostic
	for _, marker := range []string{"block", "spam", "blacklist", "reputation", "policy"} {
		if containsFold(d, marker) {
			return true
		}
	}
	return false
}

func containsFold(s, sub string) bool {
	if len(sub) == 0 || len(s) < len(sub) {
		return false
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if equalFold(s[i:i+len(sub)], sub) {
			return true
		}
	}
	return false
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
