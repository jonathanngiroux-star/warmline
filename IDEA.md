# Warmline

**Category:** Transactional email wrapper + reputation simulation
**Cohort:** Faster-to-cash (wrapper) / higher-upside if you take deliverability ops
**Ceiling note:** Wrapper economics look like ~$100M ARR, not $1B. That is a feature.

## Pitch

Self-hosted MTA + managed deliverability harness that simulates IP warmup, bounce/complaint prediction, and DKIM rotation. Replaces SendGrid / Postmark / Mailgun at 3–4× savings ($300–800/mo vs $200–2k/mo).

## Weekly job

Backend engineers at 50–500 person SaaS firms watch reputation dashboards, beg for IP warmup schedules, and debug deliverability after a marketing send pollutes the transactional domain.

## Who pays

Infra / backend leads who own the transactional email line item and will not hire a dedicated email-ops person.

## Why incumbents fail

Managed IP pools are the premium. Self-hosted Postfix / Rspamd / OpenDKIM has no automated warmup, no reputation simulation, no bounce classification. Postal stalled. Mailpit is dev-only.

## Why this is still open

Warmup, bounce handling, and deliverability dashboards are a wide-open operational layer. No credible open project ships `migrate --from=sendgrid --dry-run` with a reputation *trajectory* simulation.

## MVP (8–12 weeks)

- Go binary wrapping zoneMTA or Haraka
- SQLite queue
- `warmline migrate --from=sendgrid --dry-run` + reputation trajectory sim
- Webhook normalization
- Cloud tier: managed relay pool (Resend/Postmark/etc.), not “you run 24/7 email ops”

## License

MIT on self-hosted core. **BSL/ELv2 on the cloud/deliverability layer.** MIT-only here is how a hyperscaler ships “managed deliverability” at 40% price and strips you.

## Pricing

- Cloud — $300–800/mo (dedicated IP, 500k–5M emails, reputation SLA)
- Overage — ~$0.10 / 1k
- Self-host core — free (staging / test)

## 90-day evidence

- Pilots that can show 3–4× cost savings on a real volume band
- Dry-run migration + reputation sim is demoable and CI-checkable
- ≥2 managed relay partnerships (single vendor is an investor “no”)
- Deliverability SLA target ≥99.5%
- 10+ paying teams by the same Q1 2027 bar as other wedges; two design partners ≠ a motion

## Living-income path

10 pilots × $500 = $5k MRR by month 6. ~3.8% conversion on a few hundred self-hosts → ~15 Cloud teams ≈ $7.5k MRR by month 12.

## Investor “no”

One relay partner; SLA <99.5%; cloud tier requires the founder to be on-call for email; MIT on the layer that is actually scarce.

## Fastest death

Founder becomes a 24/7 MTA operator. Or AWS SES / Google launch a warmup product and your license lets them copy the wrapper.
