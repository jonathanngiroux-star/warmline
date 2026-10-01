// Package sendgrid parses SendGrid account exports and builds the
// Warmline dry-run migration report.
package sendgrid

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/jonathanngiroux-star/warmline/internal/migrate"
	"github.com/jonathanngiroux-star/warmline/internal/webhooks"
)

// Export is the subset of a SendGrid account export Warmline reads.
// Fields are metadata-only; exports are PII-scrubbed fixtures or the
// operator's own export file.
type Export struct {
	ExportVersion string `json:"export_version"`
	Account       struct {
		ID           string `json:"id"`
		SubuserCount int    `json:"subuser_count"`
	} `json:"account"`
	Subusers []struct {
		Username string `json:"username"`
		Disabled bool   `json:"disabled"`
	} `json:"subusers"`
	AuthDomains []struct {
		ID           int64  `json:"id"`
		Domain       string `json:"domain"`
		ParentDomain string `json:"parent_domain"`
		Subdomain    string `json:"subdomain"`
		CustomDKIM   bool   `json:"custom_dkim"`
		Default      bool   `json:"default"`
		DNSRecords   []struct {
			Host  string `json:"host"`
			Type  string `json:"type"`
			Data  string `json:"data"`
			Valid bool   `json:"valid"`
		} `json:"dns_records"`
	} `json:"authenticated_domains"`
	IPPools []struct {
		Name         string `json:"name"`
		Shared       bool   `json:"shared"`
		WarmupStatus string `json:"warmup_status"`
		IPs          []struct {
			IP     string `json:"ip"`
			Warmup struct {
				StartDate string `json:"start_date"`
				Status    string `json:"status"`
			} `json:"warmup"`
		} `json:"ips"`
	} `json:"ip_pools"`
	Webhook struct {
		Enabled bool     `json:"enabled"`
		URL     string   `json:"url"`
		OAuth   bool     `json:"oauth"`
		Events  []string `json:"events"`
	} `json:"webhook"`
	Templates []struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Generation string `json:"generation"`
	} `json:"templates"`
	ASMGroups []struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		IsDefault bool   `json:"is_default"`
	} `json:"asm_groups"`
	SenderIdentities []struct {
		Nickname string `json:"nickname"`
		From     struct {
			Email string `json:"email"`
			Name  string `json:"name"`
		} `json:"from"`
		ReplyTo struct {
			Email string `json:"email"`
			Name  string `json:"name"`
		} `json:"reply_to"`
	} `json:"sender_identities"`
	Suppression struct {
		Bounces     int `json:"bounces"`
		Blocks      int `json:"blocks"`
		Invalid     int `json:"invalid"`
		SpamReports int `json:"spam_reports"`
	} `json:"suppression"`
	Stats struct {
		Last30d struct {
			Requests    int `json:"requests"`
			Delivered   int `json:"delivered"`
			Bounces     int `json:"bounces"`
			SpamReports int `json:"spam_reports"`
		} `json:"last_30d"`
	} `json:"stats"`
	EventSamples []map[string]any `json:"event_samples"`
}

// ParseFile reads and parses a SendGrid export JSON file.
func ParseFile(path string) (*Export, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var e Export
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &e, nil
}

// BuildReport turns the parsed export into the dry-run diff.
func (e *Export) BuildReport() (migrate.Report, error) {
	r := migrate.Report{
		Source:   "sendgrid",
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

	// Authenticated domains → Warmline DKIM domain records.
	for _, d := range e.AuthDomains {
		key := "domain." + d.Domain
		notes := []string{}
		invalid := 0
		for _, rec := range d.DNSRecords {
			if !rec.Valid {
				invalid++
			}
		}
		if invalid > 0 {
			notes = append(notes, fmt.Sprintf("%d DNS records invalid at export time", invalid))
		}
		if !d.CustomDKIM {
			notes = append(notes, "DKIM was ESP-signed (shared); Warmline will require your own DKIM key")
		}
		r.Added = append(r.Added, migrate.Entry{
			Key:  key,
			To:   d.Domain,
			Note: joinNotes(notes),
		})
	}

	// Webhook event mapping — one shared table with the webhook
	// normalizer (webhooks.SendgridEventKind). Events absent from it are
	// reported unmapped, never silently dropped.
	for _, ev := range e.Webhook.Events {
		if _, ok := webhooks.SendgridEventKind(ev); ok {
			continue
		}
		r.Unmapped = append(r.Unmapped, migrate.Entry{
			Key:  "webhook.event." + ev,
			Note: "no canonical Warmline event yet; dropped events would be invisible in the queue UI",
		})
	}

	// Risk entries: real source concepts Warmline cannot carry over
	// because it does not operate IP pools, and live-API-only state.
	for _, pool := range e.IPPools {
		if len(pool.IPs) > 0 || !pool.Shared {
			r.Risks = append(r.Risks, migrate.Entry{
				Key:  "ip_pools",
				To:   pool.Name,
				Note: "Warmline simulates warmup but never operates IP pools; your sending IP is whatever your relay/host provides",
			})
			break // one entry per export, not per pool
		}
	}
	if e.Suppression.Bounces > 0 || e.Suppression.Blocks > 0 || e.Suppression.Invalid > 0 || e.Suppression.SpamReports > 0 {
		r.Risks = append(r.Risks, migrate.Entry{
			Key:  "suppression",
			To:   e.Suppression.Bounces + e.Suppression.Blocks + e.Suppression.Invalid,
			Note: "suppression lists live in the ESP API; export them via the ESP before cutover or you will re-mail hard bouncers",
		})
	}
	for _, tpl := range e.Templates {
		if tpl.Generation == "legacy" {
			r.Risks = append(r.Risks, migrate.Entry{
				Key:  "templates.legacy",
				To:   tpl.ID,
				Note: "legacy template engine has no Warmline equivalent; reauthor in your app or keep it at the ESP until rewritten",
			})
			break // one entry per export naming the first legacy template
		}
	}
	if e.Webhook.OAuth {
		r.Risks = append(r.Risks, migrate.Entry{
			Key:  "webhook.oauth",
			Note: "SendGrid signed-webhook OAuth flow has no direct Warmline equivalent; use a shared secret or mTLS at your receiver",
		})
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
