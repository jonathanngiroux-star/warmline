// Package relay drains the local queue into a USER-SUPPLIED relay
// (their own SMTP server, SES, Postmark, Resend — anything speaking
// SMTP). Warmline does not operate relays and never resells email.
package relay

import (
	"fmt"
	"net/smtp"
	"strings"

	"github.com/jonathanngiroux-star/warmline/internal/store"
)

// Config is the user-supplied relay endpoint.
type Config struct {
	Addr     string // host:port, e.g. smtp.example.com:587
	Username string // optional (PLAIN auth when set)
	Password string // optional
	From     string // optional envelope-from override; per-message from otherwise
}

// Validate rejects configs that cannot work.
func (c Config) Validate() error {
	if c.Addr == "" {
		return fmt.Errorf("relay addr is required (host:port)")
	}
	if (c.Username == "") != (c.Password == "") {
		return fmt.Errorf("username and password must be set together")
	}
	return nil
}

// Relay delivers queued messages to the configured relay.
type Relay struct {
	cfg Config
}

// New builds a Relay from config.
func New(cfg Config) *Relay { return &Relay{cfg: cfg} }

// WorkOnce claims one queued message, delivers it via the relay, and
// records the outcome (sent / retryable failure). Returns deliveries
// made (0 or 1). A delivery failure requeues the message for retry —
// Warmline's queue is the retry loop, and the failure is logged as an
// attempt row.
func (r *Relay) WorkOnce(db *store.Store) (int, error) {
	msg, err := db.Dequeue()
	if err != nil {
		return 0, err
	}
	if msg == nil {
		return 0, nil
	}
	if err := r.cfg.Validate(); err != nil {
		_ = db.Requeue(msg.ID)
		return 0, err
	}
	err = r.deliver(msg)
	if err != nil {
		_ = db.RecordAttempt(msg.ID, r.cfg.Addr, "", err.Error())
		_ = db.Requeue(msg.ID)
		return 0, fmt.Errorf("deliver id=%d: %w", msg.ID, err)
	}
	if err := db.MarkSent(msg.ID, r.cfg.Addr); err != nil {
		return 0, err
	}
	if err := db.RecordAttempt(msg.ID, r.cfg.Addr, "250", ""); err != nil {
		return 0, err
	}
	return 1, nil
}

// deliver speaks minimal SMTP submission to the relay.
func (r *Relay) deliver(msg *store.Message) error {
	cl, err := smtp.Dial(r.cfg.Addr)
	if err != nil {
		return err
	}
	defer cl.Close()
	if err := cl.Hello("warmline.local"); err != nil {
		return err
	}
	if ok, _ := cl.Extension("STARTTLS"); ok {
		if err := cl.StartTLS(nil); err != nil {
			return err
		}
	}
	if r.cfg.Username != "" {
		host, _, err := splitHostPort(r.cfg.Addr)
		if err != nil {
			return err
		}
		if err := cl.Auth(smtp.PlainAuth("", r.cfg.Username, r.cfg.Password, host)); err != nil {
			return err
		}
	}
	from := r.cfg.From
	if from == "" {
		from = msg.Metadata["from"]
	}
	if from == "" {
		from = addrSpec(headerValue(msg.Body, "From"))
	}
	if from == "" {
		return fmt.Errorf("no envelope-from: set relay --from or include From: in the message")
	}
	if err := cl.Mail(from); err != nil {
		return err
	}
	if err := cl.Rcpt(msg.Recipient); err != nil {
		return err
	}
	w, err := cl.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(msg.Body)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return cl.Quit()
}

func splitHostPort(addr string) (string, string, error) {
	i := strings.LastIndex(addr, ":")
	if i <= 0 || i == len(addr)-1 {
		return "", "", fmt.Errorf("relay addr %q must be host:port", addr)
	}
	return addr[:i], addr[i+1:], nil
}

// headerValue extracts a header value from a raw RFC 5322 message
// ("From: addr" in the header block). Returns "" when absent. Handles
// CRLF, LF, and CR line endings; stops at the first empty line.
func headerValue(raw, name string) string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	for _, line := range strings.Split(normalized, "\n") {
		if len(line) > len(name) && strings.EqualFold(line[:len(name)], name) && line[len(name)] == ':' {
			return strings.TrimSpace(line[len(name)+1:])
		}
	}
	return ""
}

// addrSpec extracts the bare address from a From: header value that may
// carry a display name ("Acme <a@b.com>") or be bare ("a@b.com").
// Relays reject display names in the envelope — this returns the
// addr-spec only.
func addrSpec(v string) string {
	if i := strings.LastIndex(v, "<"); i >= 0 {
		if j := strings.Index(v[i:], ">"); j > 0 {
			return v[i+1 : i+j]
		}
	}
	return v
}
