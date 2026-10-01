# Warmline

Warmline is a self-hosted transactional-email wrapper + reputation-simulation harness for backend/infra leads who are tired of the weekly ESP job: watching reputation dashboards, begging for IP warmup, and doing bounce archaeology after marketing polluted the transactional domain.

Teams spend $200–$2,000/month on SendGrid / Postmark / Mailgun largely to rent that operational layer. Warmline is not a hosted ESP and never will be — it is a **free, MIT-licensed local harness** that answers the questions you currently pay an ESP to answer:

- `warmline migrate --from=sendgrid --dry-run` — what actually moves, what cannot move, and what will bite you
- `warmline simulate` — reputation trajectory and warmup curves, deterministic, from your own declared numbers
- bounce/complaint classification in a local SQLite queue, with normalized webhooks
- DKIM rotation helpers — config snippets, not a CA

**Warmline is free to self-host. If it saves you a SendGrid week, [donate](docs/donate.md).** There is no paid tier, no subscription, no "contact sales," and no feature will ever be gated on money.

## Status

Pre-release, under active development. Currently working:

- `warmline version`, `warmline donate`
- `warmline migrate --from=sendgrid --dry-run` (+ `--format=json`) on a PII-scrubbed fixture export
- SQLite queue store (messages, attempts, bounces, complaints, DKIM selectors)

Planned (v0.1): reputation trajectory simulation, webhook normalization for SendGrid, Postmark dry-run, DKIM rotation helpers, embedded queue UI, `serve`.

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
docker run --rm warmline version
```

`serve` (local SMTP submit + queue + UI) is W5–12 work; the cold-start target is ~2 minutes, hard cap 15. The SQLite file is the only state.

## How it works

- **Single Go binary, SQLite queue, no cgo** — one file of state, one process, no Kubernetes.
- **Outbound goes through your own relay credentials** (Resend / Postmark / SES / your SMTP server). Warmline never resells email and never operates IP pools.
- **Migration is the product.** The dry-run diff (`added` / `removed` / `changed` / `unmapped` / `risks`) is machine-readable JSON so CI can gate a cutover on it.
- **Reputation simulation is simulation.** Warmup curves and risk bands are deterministic functions of your declared volume plan, bounce rate, complaint rate, and IP age. Warmline does not guarantee inbox placement and never will — anyone who promises that is selling you something.

## MTA core

Choice made in week 1 and stuck to: **zoneMTA** behind a thin Go wrapper. zoneMTA is a Node.js MTA; the wrapper contract is deliberately narrow (accept local SMTP submit, enqueue to SQLite, log attempts) so the migrate/simulate harness never depends on MTA internals. The wrap itself lands with `serve` (W1–2 milestone); nothing in the current CLI depends on it.

## Docs

- [docs/donate.md](docs/donate.md) — donation addresses (ETH/USDC, BTC)
- [docs/migrate.md](docs/migrate.md) — dry-run reports and the diff schema
- [docs/simulate.md](docs/simulate.md) — reputation simulation (planned)
- [docs/webhooks.md](docs/webhooks.md) — canonical event mapping (planned)

## License

MIT — see [LICENSE](LICENSE). Governance: [GOVERNANCE.md](GOVERNANCE.md) (DCO-only contributions; donation wallet changes need 4/4 consensus + 30-day notice).

## Contributing

DCO only — every commit carries `Signed-off-by`. No CLA. Keep the surface area solo-maintainable: every added file is a liability.
