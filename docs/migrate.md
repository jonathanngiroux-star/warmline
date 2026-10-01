# Migrate — dry-run reports

`warmline migrate --from=<esp> --dry-run --input <export.json>` reads an ESP export file and prints a diff of what a Warmline migration would add, remove, change, fail to map, and put at risk. It never calls the ESP API and never mutates anything.

## Diff schema

JSON output (default `--format=text` prints the same data human-readable):

```json
{
  "source": "sendgrid",
  "dry_run": true,
  "input": "path/to/export.json",
  "added":    [{ "key": "domain.mail.example.com", "to": "mail.example.com" }],
  "removed":  [],
  "changed":   [],
  "unmapped": [{ "key": "webhook.event.dropped", "note": "..." }],
  "risks":    [{ "key": "ip_pools", "note": "..." }],
  "reputation_sim": { "status": "planned", "note": "..." }
}
```

- `added` — concepts Warmline will create locally (authenticated domains → DKIM records)
- `removed` / `changed` — concepts that map with a semantic change (populated from v0.1's second source)
- `unmapped` — source concepts with **no** canonical Warmline equivalent; reported, never silently dropped
- `risks` — real things at the source that Warmline cannot carry over: IP pools (Warmline simulates warmup, never operates pools), suppression lists (live in the ESP API — export them before cutover), legacy templates, webhook OAuth flows
- `reputation_sim` — reserved object; the trajectory engine (W5–7) fills it from your declared volume/bounce/complaint plan

## Sources

| Source | Status |
|---|---|
| SendGrid | dry-run report on fixture exports |
| Postmark | planned (v0.1) |
| Mailgun | backlog |

## CI gate

The committed fixture (`testdata/fixtures/sendgrid/sample-export.json`) must produce a machine-stable report: same input → same keys, same lengths, same order. CI fails if the mapping table regresses (e.g. an event silently becomes mapped or unmapped without the test contract being consciously updated).
