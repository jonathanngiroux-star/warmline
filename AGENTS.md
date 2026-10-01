# AGENTS.md — Warmline working rules

Binding brief: `IDEA.md`. This file is the enforcement summary. If a request fights the wedge, refuse it and name the trap.

## The wedge (only this)

- Self-hosted MTA wrapper + deliverability harness: single Go binary + SQLite.
- `warmline migrate --from=sendgrid --dry-run` with **reputation trajectory simulation** — the migrate/simulate harness is the product, not the MTA.
- Bounce/complaint classification + webhook normalization (SendGrid shape first; Postmark next; Mailgun can wait).
- DKIM rotation *helpers* (config snippets — we are not the CA).
- IP warmup **simulation** (dry-run curves only). We do not operate dedicated IP pools for anyone.
- Outbound goes through *user-supplied* relay credentials (Resend/Postmark/SES/their SMTP). Warmline never resells email.
- Cold start in the 2-minute class, hard cap 15 minutes. Kubernetes is a bug.

## Monetization (binding override — series-wide)

- **No subscriptions. No invoices. No seats. No "contact sales." No Stripe, no payment processor, no usage billing. No Cloud Pro tier.**
- The only ask is optional donations:

  - Ethereum / USDC (ERC-20): `0x85ee7E71f762d772599cbF1EC20E651B30657521`
  - Bitcoin: `bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg`

- Both addresses go in README, `/donate`, and the UI footer. No other wallets without the maintainer saying so.
- Success metrics: self-host deploys, migrate dry-run usage, deliverability fixture quality, verifiable on-chain donations — **not ARR**. Do not use "MRR," "ARR signal," "10 Cloud Pro teams," or "$800/mo" language.
- Named substitute cost ($200–$2k/mo ESP spend) may appear in README as *why this exists* — never as a price we charge.
- If someone asks for a $300–800/mo cloud SKU, point at `docs/donate.md` and stop.

## Explicitly out of scope (refuse on sight)

- Billing of any kind; hosted IP pools as a product; multi-tenant relay.
- 24/7 founder-on-call for other people's deliverability.
- Marketing-blast platform, template marketplace, full ESP, campaign tools, CRM, "AI email writer."
- Hyperscale warmup-as-a-service before the migrate simulator is real.
- MIT-licensed "we are SendGrid" claims. Never imply Warmline guarantees inbox placement.

## Stack rules

- Go. SQLite default. MTA core: zoneMTA **or** Haraka behind a thin wrapper — one choice, documented in README, stuck to.
- One binary + `docker run` + embedded UI. MIT core. **No BSL/ELv2 SKU exists in this project** — do not invent one. A multi-tenant demo host, if ever added, is a donation-funded courtesy preview, not a product.
- GOVERNANCE.md: DCO-only; donation addresses listed; 4/4 consensus + 30-day notice for license or wallet changes.
- CI: GitHub Actions; fidelity gates on migrate dry-run; golden fixture → deterministic simulation JSON.
- Solo-maintainable surface area is a hard constraint. Every added file is a liability.

## Working rules

1. Migration + simulation outrank "better MTA." Always.
2. Every feature answers yes to one of: does this help migrate, simulate reputation, classify bounces, deploy in minutes, or make donate addresses visible? If no → do not build.
3. Simulation is labeled simulation. Deterministic on the fixture: same input → same curve. No deliverability guarantees, ever.
4. Fixtures are PII-scrubbed, metadata-only, no live secrets.
5. Output patches, commands, and file paths. Not essays.

## MVP order (do not reorder without saying why)

W1–2 binary + queue + MTA wrap → W3–4 SendGrid migrate skeleton (fixture, JSON diff: `added`/`removed`/`changed`/`unmapped`/`risks`) → W5–7 reputation trajectory simulation (JSON + markdown curve, risk bands, CI golden) → W8 webhook normalization (SendGrid → canonical) → W9 Postmark dry-run → W10–11 DKIM rotation helpers + bounce classes → W12 docs.

## Definition of done for v0.1

One binary, SQLite queue, documented `docker run` ≤15 min. `warmline donate` prints both addresses exactly. SendGrid dry-run migrate + JSON diff. Deterministic reputation simulation on a golden fixture. Canonical webhook mapper for one ESP. README states: free self-host, donations only, simulation ≠ SLA. No Stripe, no seats, no hosted IP pool. No second product.
