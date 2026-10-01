# Webhooks — canonical event mapping

ESP webhook payloads are normalized into one Warmline canonical event before they touch the queue store. One mapping table (`webhooks.SendgridEventKind`) drives both the live normalizer and the migrate dry-run report — an event cannot change mapping status in one place without the other noticing in CI.

## Canonical event

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

## SendGrid mapping

| SendGrid event | Warmline event | Notes |
|---|---|---|
| processed | processed | |
| delivered | delivered | |
| open | open | |
| click | click | |
| bounce | bounce | SendGrid `bounce_type` pre-classifies hard/soft; falls back to SMTP-code taxonomy |
| deferred | deferred | classified soft |
| spam_report | spam_report | |
| unsubscribe | unsubscribe | |
| group_unsubscribe | unsubscribe | aliased |
| dropped | **unmapped** | ESP-side pre-queue discard — reported, never silent |
| group_resubscribe | **unmapped** | unsubscribe-group management, not a delivery event |

Postmark mapping (migrate dry-run): `delivery`, `open`, `click`, `bounce`, `spam_complaint` map; `subscription_change` is reported as a risk.

## Ingest endpoint

`serve` exposes `POST /hooks/sendgrid`: body must be a JSON array of events (SendGrid's format); bounces and spam complaints are recorded in the queue store. Events with no canonical mapping are counted in the response as `unmapped` and dropped loudly, never silently:

```json
{"accepted": 2, "unmapped": ["dropped"]}
```
