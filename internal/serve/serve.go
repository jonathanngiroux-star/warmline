// Package serve wires the local SMTP front, the SQLite queue, and a
// minimal HTTP UI into one process. One binary, one db file, no
// external dependencies. Outbound is user-supplied-relay only.
package serve

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/jonathanngiroux-star/warmline/internal/mta"
	"github.com/jonathanngiroux-star/warmline/internal/queue"
	"github.com/jonathanngiroux-star/warmline/internal/store"
	"github.com/jonathanngiroux-star/warmline/internal/webhooks"
)

// QueueStats is the queue summary the UI renders.
type QueueStats struct {
	Total  int            `json:"total"`
	Status map[string]int `json:"status"`
}

// Stats reads queue stats from the store.
func Stats(db *store.Store) (QueueStats, error) {
	st := QueueStats{Status: map[string]int{}}
	rows, err := db.DB().Query(`SELECT status, COUNT(*) FROM messages GROUP BY status`)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		var n int
		if err := rows.Scan(&s, &n); err != nil {
			return st, err
		}
		st.Status[s] = n
		st.Total += n
	}
	return st, nil
}

// Donate addresses — binding, mirrored from cmd/warmline (GOVERNANCE.md:
// 4/4 + 30-day notice to change).
const (
	DonateEthereum = "0x85ee7E71f762d772599cbF1EC20E651B30657521"
	DonateBitcoin  = "bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg"
)

// storeHandler adapts store.Store to mta.Handler: enqueue each accepted
// message. Multi-recipient submits enqueue one row per recipient — the
// queue is per-recipient for bounce classification.
type storeHandler struct{ db *store.Store }

func (h storeHandler) Submit(m mta.Message) error {
	for _, to := range m.To {
		if _, err := h.db.Enqueue(to, m.Data, map[string]string{"from": m.From}); err != nil {
			return err
		}
	}
	return nil
}

// Options configures Serve.
type Options struct {
	SMTPAddr string // e.g. "127.0.0.1:2525"
	HTTPAddr string // e.g. "127.0.0.1:8080"
}

// Run starts the SMTP front and the HTTP UI and blocks. Both listeners
// are wired to the same store; the store is opened by the caller.
func Run(db *store.Store, opts Options) error {
	// SMTP front
	smtp := mta.New()
	smtp.SetHandler(storeHandler{db: db})
	if err := smtp.Listen(opts.SMTPAddr); err != nil {
		return fmt.Errorf("smtp listen %s: %w", opts.SMTPAddr, err)
	}
	go smtp.Serve()

	// HTTP UI
	mux := http.NewServeMux()
	mux.HandleFunc("/", queuePage(db))
	mux.HandleFunc("/donate", donatePage())
	mux.HandleFunc("/hooks/sendgrid", sendgridHook(db))
	return http.ListenAndServe(opts.HTTPAddr, mux)
}

// DB exposes the underlying *sql.DB (used by Stats).
func dbHandle(db *store.Store) *sql.DB { return db.DB() }

func queuePage(db *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		st, err := Stats(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, pageShell("Queue"), "")
		fmt.Fprintf(w, "<h1>Warmline queue</h1><p>total: <strong>%d</strong></p><ul>", st.Total)
		for _, s := range []string{"queued", "processing", "sent"} {
			fmt.Fprintf(w, "<li>%s: %d</li>", s, st.Status[s])
		}
		fmt.Fprintf(w, "</ul><p><a href=\"/donate\">donate</a></p>")
		fmt.Fprint(w, footer())
	}
}

func donatePage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, pageShell("Donate"), "")
		fmt.Fprint(w, "<h1>Donate</h1><p>Warmline is free to self-host. If it saves you a SendGrid week, donate.</p>")
		fmt.Fprintf(w, "<p>Ethereum / USDC (ERC-20): <code>%s</code></p>", DonateEthereum)
		fmt.Fprintf(w, "<p>Bitcoin: <code>%s</code></p>", DonateBitcoin)
		fmt.Fprint(w, "<p>No feature is gated on donations.</p>")
		fmt.Fprint(w, footer())
	}
}

// sendgridHook accepts a SendGrid event webhook POST and stores canonical
// events. Bounces and complaints are recorded against the queue rows.
func sendgridHook(db *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		var body []byte
		var readErr error
		if r.Body != nil {
			body, readErr = io.ReadAll(r.Body)
			if readErr != nil {
				http.Error(w, "read body", http.StatusBadRequest)
				return
			}
		}
		events, unmapped, err := webhooks.NormalizeSendgrid(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for _, ev := range events {
			switch ev.Kind {
			case queue.EventBounce:
				_ = db.RecordBounce(0, ev.Recipient, ev.SMTPCode, ev.Diagnostic, ev.BounceClass)
			case queue.EventSpamReport:
				_ = db.RecordComplaint(0, ev.Recipient)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"accepted": len(events),
			"unmapped": unmapped,
		})
	}
}

func pageShell(title string) string {
	return fmt.Sprintf("<!DOCTYPE html><html><head><title>Warmline — %s</title><style>body{font-family:system-ui,sans-serif;max-width:640px;margin:2rem auto;padding:0 1rem}code{background:#f4f4f4;padding:2px 6px;border-radius:4px}</style></head><body>", title)
}

func footer() string {
	return fmt.Sprintf(`<hr><p style="font-size:0.85rem;color:#666">Free forever. Donations: <a href="/donate">ETH/USDC + BTC</a></p></body></html>`)
}
