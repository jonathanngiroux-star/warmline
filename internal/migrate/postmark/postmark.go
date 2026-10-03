// Package postmark parses Postmark account exports and builds the
// Warmline dry-run migration report. Same Report schema as sendgrid;
// gaps listed, no fake 100%.
package postmark

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/jonathanngiroux-star/warmline/internal/migrate"
	"github.com/jonathanngiroux-star/warmline/internal/queue"
)

// Export is the subset of a Postmark account export Warmline reads.
type Export struct {
	ExportVersion string `json:"export_version"`
	Server        string `json:"server"`
	Domains       []struct {
		ID                 int64  `json:"id"`
		Name               string `json:"name"`
		DKIM               bool   `json:"dkim"`
		ReturnPath         bool   `json:"return_path"`
		VerificationStatus string `json:"verification_status"`
	} `json:"domains"`
	Senders []struct {
		Name    string `json:"name"`
		Email   string `json:"email"`
		ReplyTo string `json:"reply_to"`
	} `json:"senders"`
	Templates []struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		Active bool   `json:"active"`
	} `json:"templates"`
	MessageStreams []struct {
		Name         string `json:"name"`
		DeliveryType string `json:"delivery_type"`
	} `json:"message_streams"`
	Webhook struct {
		Enabled       bool     `json:"enabled"`
		URL           string   `json:"url"`
		MessageStream string   `json:"message_stream"`
		Events        []string `json:"events"`
	} `json:"webhook"`
	Stats struct {
		Last30d struct {
			Sent           int `json:"sent"`
			Bounces        int `json:"bounces"`
			SpamComplaints int `json:"spam_complaints"`
		} `json:"last_30d"`
	} `json:"stats"`
	Suppression struct {
		Bounces        int `json:"bounces"`
		SpamComplaints int `json:"spam_complaints"`
		Unsubscribes   int `json:"unsubscribes"`
	} `json:"suppression"`
}

// canonicalPostmarkEvents maps Postmark webhook event names to Warmline
// canonical kinds. Absent names surface as risks — never silent.
// Deliberately unmapped: `subscription_change` (recipient subscription
// management, not a delivery event).
var canonicalPostmarkEvents = map[string]queue.EventKind{
	"delivery":       queue.EventDelivered,
	"open":           queue.EventOpen,
	"click":          queue.EventClick,
	"bounce":         queue.EventBounce,
	"spam_complaint": queue.EventSpamReport,
}

// ParseFile reads and parses a Postmark export JSON file.
func ParseFile(path string) (*Export, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	e, err := ParseBytes(raw)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return e, nil
}

// ParseBytes parses a Postmark export from raw JSON bytes (embedded
// fixtures, GUI-loaded files).
func ParseBytes(raw []byte) (*Export, error) {
	var e Export
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// BuildReport turns the parsed export into the dry-run diff.
func (e *Export) BuildReport() (migrate.Report, error) {
	r := migrate.Report{
		Source:   "postmark",
		DryRun:   true,
		Added:    []migrate.Entry{},
		Removed:  []migrate.Entry{},
		Changed:  []migrate.Entry{},
		Unmapped: []migrate.Entry{},
		Risks:    []migrate.Entry{},
		ReputationSim: migrate.ReputationSim{
			Status: "planned",
			Note:   "reputation trajectory simulation ships in W5–7; this report reserves the field so CI and docs can pin the shape now",
		},
	}

	for _, d := range e.Domains {
		key := "domain." + d.Name
		notes := []string{}
		if !d.DKIM {
			notes = append(notes, "DKIM not enabled at source; Warmline requires your own DKIM key")
		}
		if !d.ReturnPath {
			notes = append(notes, "custom return-path not enabled; expect alignment loss")
		}
		r.Added = append(r.Added, migrate.Entry{
			Key:  key,
			To:   d.Name,
			Note: joinNotes(notes),
		})
		if d.VerificationStatus != "verified" {
			r.Risks = append(r.Risks, migrate.Entry{
				Key:  "domain.unverified",
				To:   d.Name,
				Note: "domain not verified at source — fix DNS at the source before migrating or your first send fails",
			})
		}
	}

	for _, ev := range e.Webhook.Events {
		if _, ok := canonicalPostmarkEvents[ev]; ok {
			continue
		}
		r.Risks = append(r.Risks, migrate.Entry{
			Key:  "webhook.event." + ev,
			Note: "no canonical Warmline event; events of this type would be invisible in the queue UI",
		})
	}

	if e.Suppression.Bounces > 0 || e.Suppression.SpamComplaints > 0 || e.Suppression.Unsubscribes > 0 {
		r.Risks = append(r.Risks, migrate.Entry{
			Key:  "suppression",
			To:   e.Suppression.Bounces + e.Suppression.SpamComplaints + e.Suppression.Unsubscribes,
			Note: "suppression lists live in the ESP API; export them before cutover or you will re-mail hard bouncers",
		})
	}

	// Message streams have no direct Warmline queue equivalent beyond
	// metadata; flag broadcast streams (marketing-shaped) explicitly.
	for _, s := range e.MessageStreams {
		if s.Name == "broadcast" {
			r.Risks = append(r.Risks, migrate.Entry{
				Key:  "message_stream.broadcast",
				To:   s.Name,
				Note: "Warmline is transactional-only; a broadcast stream must stay at the ESP or move to a marketing tool",
			})
		}
	}

	return r, nil
}

func joinNotes(notes []string) string {
	out := ""
	for i, n := range notes {
		if i > 0 {
			out += "; "
		}
		out += n
	}
	return out
}
