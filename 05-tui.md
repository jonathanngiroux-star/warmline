# Project: Warmline
# Step 5 of 7: Build a full TUI
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

This file is **step 5**. Previous steps are assumed done. Later steps are out of scope until this step's definition of done is met.

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

Build a full TUI. GUI stays. Backend stays. Do not add a second data model.

`warmline tui`: queue depth, last bounce classes, run simulate on a fixture, migrate summary, donate. Same SQLite. Keyboard complete.

Use Bubble Tea or an equivalent in-repo TUI. `warmline tui` must run against the same SQLite (or stated data dir) as the Wails GUI. Donate screen required. No mouse-only actions. No subscription UI.

Wizard (required in this step):
- First `tui` launch with an empty data dir opens a step-by-step wizard before the main screen.
- Same job as the Wails wizard: setup, then one core workflow, then the main screen with something created.
- Keys for Back / Next are documented. No mouse-only step.
- A key reopens the wizard from the main screen.
- Donate addresses on the last wizard step and the donate screen.

## Definition of done

- [ ] TUI covers the workflows listed above
- [ ] First launch runs the wizard without the README
- [ ] Donate screen shows both addresses exactly
- [ ] README says how to launch the TUI and the wizard keys
- [ ] Wails GUI still builds

## First response required

1. Add the TUI package and `warmline tui`.
2. Implement the screens against existing store APIs.
3. Implement the first-run wizard and the reopen key.
4. Document keys.
5. Stop before a new feature that is not on the list.
