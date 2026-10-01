# Webhooks — canonical event mapping (planned)

W8 milestone: ingest ESP-shaped webhook events and normalize them into the Warmline canonical event before they touch the queue store.

Canonical event (stable shape, defined in `internal/queue`):

```json
{
  "type": "bounce",
  "message_id": "…",
  "esp_event_id": "…",
  "at": "RFC3339",
  "recipient": "…",
  "smtp_code": "550",
  "diagnostic": "…",
  "bounce_class": "hard|soft|block|unknown"
}
```

Bounce taxonomy: `hard` (permanent 5xx), `soft` (temporary 4xx), `block` (ISP/reputation-level), `unknown`.

The SendGrid mapping table lives in `internal/migrate/sendgrid` (see [migrate.md](migrate.md) for the dry-run contract — events with no canonical equivalent are reported `unmapped`, never silently dropped). A local webhook tester + the full per-ESP mapping tables land with W8.
