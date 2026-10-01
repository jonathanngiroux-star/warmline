# Warmline

Warmline is a self-hosted transactional-email wrapper + reputation-simulation harness for backend/infra leads who are tired of the weekly ESP job: watching reputation dashboards, begging for IP warmup, and doing bounce archaeology after marketing polluted the transactional domain.

Teams spend $200–$2,000/month on SendGrid / Postmark / Mailgun largely to rent that operational layer. Warmline is not a hosted ESP and never will be — it is a **free, MIT-licensed local harness** that answers the questions you currently pay an ESP to answer:

- `warmline migrate --from=sendgrid --dry-run` — what actually moves, what cannot move, and what will bite you
- `warmline simulate` — reputation trajectory and warmup curves, deterministic, from your own declared numbers
- bounce/complaint classification in a local SQLite queue, with normalized webhooks
- DKIM rotation helpers — config snippets, not a CA

**Warmline is free to self-host. If it saves you a SendGrid week, [donate](docs/donate.md).** There is no paid tier, no subscription, no "contact sales," and no feature will ever be gated on money.

## Status

Pre-release, under active development. All of the following is working today, covered by tests and exercised end-to-end:

- `warmline version`, `warmline donate`
- `warmline migrate --from=sendgrid|postmark --dry-run` (+ `--format=json`) with machine-stable reports
- `warmline simulate --plan plan.json` — deterministic reputation trajectory (JSON or markdown), golden-fixture gated
- `warmline dkim generate|rotate` — RSA-2048/Ed25519 keygen + rotation checklists
- `warmline serve` — local SMTP submission → SQLite queue → HTTP UI (`/` queue stats, `/donate`, `/hooks/sendgrid` webhook ingest), optional outbound drain into a relay **you** supply (`--relay`, see [docs/relay.md](docs/relay.md))

Not in v0.1 scope: Postmark/Mailgun webhook normalization, Mailgun migrate. (Outbound delivery exists via user-supplied relay — it is your relay, your keys; Warmline operates none.)

## Quick start (from source)

```sh
git clone https://github.com/jonathanngiroux-star/warmline
cd warmline
go build -o warmline ./cmd/warmline
./warmline version
./warmline donate
./warmline migrate --from=sendgrid --dry-run --input testdata/fixtures/sendgrid/sample-export.json
```

Docker:

```sh
docker build -t warmline .
docker run -d -p 8080:8080 -p 2525:2525 -v warmline-data:/data warmline
# UI: http://localhost:8080  (queue stats, /donate)
```

Measured cold start: **0.7s** from `docker run` to UI serving (the target is the 2-minute class; hard cap 15 minutes). First build with no cache ~54s. Configuration via `WARMLINE_DB` / `WARMLINE_SMTP` / `WARMLINE_HTTP` env vars or `--db` / `--smtp` / `--http` flags; the image defaults binds to `0.0.0.0` so port mappings work.

`serve` accepts local SMTP submissions into the SQLite queue; it does **not** deliver mail — outbound is user-supplied-relay only. The SQLite file is the only state.

## How it works

- **Single Go binary, SQLite queue, no cgo** — one file of state, one process, no Kubernetes.
- **Outbound goes through your own relay credentials** (Resend / Postmark / SES / your SMTP server). Warmline never resells email and never operates IP pools.
- **Migration is the product.** The dry-run diff (`added` / `removed` / `changed` / `unmapped` / `risks`) is machine-readable JSON so CI can gate a cutover on it. Bounces/complaints from ESP webhooks land in the local queue store even for mail the ESP sent before cutover.
- **The webhook normalizer and the migrate report share one mapping table** — an event can't silently change mapping status between the dry-run and production ingest.
- **Reputation simulation is simulation.** Warmup curves and risk bands are deterministic functions of your declared volume plan, bounce rate, complaint rate, and IP age. Warmline does not guarantee inbox placement and never will — anyone who promises that is selling you something.

## MTA core

Choice made in week 1 and stuck to: **zoneMTA** behind a thin Go wrapper. zoneMTA is a Node.js MTA; the wrapper contract is deliberately narrow (accept local SMTP submit, enqueue to SQLite, log attempts) so the migrate/simulate harness never depends on MTA internals. The wrap itself lands with `serve` (W1–2 milestone); nothing in the current CLI depends on it.

## Docs

- [docs/donate.md](docs/donate.md) — donation addresses (ETH/USDC, BTC)
- [docs/migrate.md](docs/migrate.md) — dry-run reports and the diff schema
- [docs/simulate.md](docs/simulate.md) — reputation simulation (planned)
- [docs/webhooks.md](docs/webhooks.md) — canonical event mapping
- [docs/relay.md](docs/relay.md) — outbound relay configuration (bring your own)

## License

MIT — see [LICENSE](LICENSE). Governance: [GOVERNANCE.md](GOVERNANCE.md) (DCO-only contributions; donation wallet changes need 4/4 consensus + 30-day notice).

## Contributing

DCO only — every commit carries `Signed-off-by`. No CLA. Keep the surface area solo-maintainable: every added file is a liability.
