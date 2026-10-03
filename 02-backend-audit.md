# Project: Warmline
# Step 2 of 7: Full audit and tests of the backend
# Role: execute this step only. Do not skip ahead. Do not add a subscription.

You are building **Warmline**, a solo-maintainable transactional-email wrapper and reputation simulator.
Replace the weekly work behind SendGrid / Postmark / Mailgun — reputation dashboards, IP warmup begging, bounce archaeology — for backend/infra leads at 50–500 person SaaS teams.

**Monetization override (all Hermes projects, including this one):**
- No subscriptions. No Cloud Starter / Pro / Enterprise invoices. No seat math. No usage bills. No "contact sales."
- No payment processor, no Stripe, no invoices.
- The only ask is optional donations to the project addresses:

Ethereum / USDC (ERC-20): `0x85ee7E71f762d772599cbF1EC20E651B30657521`
Bitcoin: `bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg`

Put both addresses in README, `/donate`, and every UI footer (GUI and TUI). Do not add other wallets without the maintainer saying so. Do not gate features on a donation.
The original Reality Seed map sold hosted tiers. That path is closed. Do not pretend this is MRR. Measure success as self-host deploys, migrate-dry-run usage, fixture quality, cold-start time, and on-chain donations — not ARR.

Read this brief as binding. If a request fights the wedge, this step, or the donation rule, refuse and name the trap.

## Sequence (do not reorder)

1. Programming the backend
2. Full audit and tests of the backend
3. Build a full GUI
4. Full audit and tests of the GUI
5. Build a full TUI
6. Full audit and tests of the TUI
7. Full audit and tests of the whole system

This file is **step 2**. Previous steps are assumed done. Later steps are out of scope until this step's definition of done is met.

## Pitch

Infra leads will self-host a wrapper that dry-runs a SendGrid export and simulates warmup — then donate if it kills a weekly dashboard job. They will not get a Warmline invoice.

Named substitute (why it exists, not a price you charge): SendGrid / Postmark / Mailgun $200–$2,000/mo.

## Stack

Go, SQLite queue, zoneMTA or Haraka behind one wrapper (pick one and stick), SendGrid migrate + reputation simulation, webhook normalization. GUI and TUI later.
Binary: `warmline` commands include `serve`, `migrate`, `simulate`, `version`, `donate`.
Cold start: about 2 minutes, hard cap 15 minutes. Kubernetes is a bug.
License: MIT on wrapper, queue, migrate, simulator, webhook normalizer, GUI, TUI. No paid relay SKU.
GUI toolkit (step 3 only): [Wails](https://github.com/wailsapp/wails) v2. Desktop window. Go bindings. Not a browser-only admin site.
GOVERNANCE.md: DCO-only; donation addresses listed; no CLA surprise; 4/4 consensus + 30-day notice for license or wallet changes.

## Always out of scope

- You operating a multi-tenant relay or dedicated IP pool
- 24/7 founder-on-call for other people's deliverability
- Marketing-blast platform, template marketplace, full ESP
- Inbox-placement guarantees
- Subscription billing

Fastest death: founder becomes the MTA.

## Repo layout

```
warmline/
  IDEA.md
  AGENTS.md
  GOVERNANCE.md
  README.md
  LICENSE
  cmd/warmline/
  internal/{store,mta,queue,migrate/sendgrid,migrate/postmark,simulate,webhooks,serve,ui,tui}/
  testdata/fixtures/sendgrid/
  testdata/simulate/
  Dockerfile
  docs/{donate.md,fidelity/sendgrid.md,simulate.md,webhooks.md}
```

## This step

Full audit and tests of the backend. Fix bugs. Do not add features. Do not start the GUI or TUI.

Audit targets:
- Simulation determinism
- Bounce class mapping gaps explicit
- Webhook normalizer rejects unknown versions loudly
- Secrets not logged
- Donate exact
- No claim of inbox placement
- No call-home

Required:
- `go test ./...` (or the repo's equivalent) green
- Race detector on the store and ingest/serve paths
- Golden fixtures checked in; CI fails on drift
- A short `docs/audit/backend.md` listing bugs found and fixed
- Fuzz or table tests on parsers and migrate
- Confirm `warmline donate` still prints both addresses exactly and nothing else is a paywall

## Definition of done

- [ ] Backend test suite green, including race on hot paths
- [ ] Bugs found in this pass are fixed, not ticketed
- [ ] `docs/audit/backend.md` written
- [ ] No new product surface
- [ ] No GUI, no TUI

## First response required

1. Read the backend and list failure modes before editing.
2. Add the missing tests.
3. Fix what fails.
4. Write `docs/audit/backend.md`.
5. Stop.
