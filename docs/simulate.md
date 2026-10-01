# Simulate — reputation trajectory

`warmline simulate --plan plan.json` runs a deterministic reputation-trajectory simulation over a volume plan you declare. Same plan → same curve, every time — that determinism is CI-gated on a golden fixture.

## Plan schema

```json
{
  "volume_start": 500,
  "volume_target": 50000,
  "ramp_days": 14,
  "days": 21,
  "bounce_rate": 1.0,
  "complaint_rate": 0.05,
  "ip_age": "new"
}
```

- `volume_start` / `volume_target` — day-1 and steady-state messages/day
- `ramp_days` — linear warmup window; day 1 starts, day `ramp_days` hits target, then holds
- `days` — total simulated days
- `bounce_rate`, `complaint_rate` — percentages you declare (1.0 = 1%)
- `ip_age` — `new` or `aged`

## Output

`--format=json` (see `testdata/simulate/golden-curve.json` for the exact shape) or the default markdown table:

```
| Day | Volume | Block risk | Defer risk |
|---|---|---|---|
| 1 | 500 | high | high |
...
```

Risk bands (`low` / `elevated` / `high`) come from published ISP engagement guidance (Google/Yahoo 2024 sender rules: complaint ceiling 0.3%, bounce ceiling 2%) plus a new-IP prior that decays as the ramp completes without violations.

## What this is not

**A simulation, not a deliverability guarantee.** Every output carries the disclaimer. Warmline does not operate IP pools, does not see your real ISP feedback, and cannot promise inbox placement — anyone who does is selling something. Use it to sanity-check a warmup schedule before you commit a domain to it.

The migrate dry-run reserves `reputation_sim: { "status": "planned" }` for wiring a report directly into a simulation in a later milestone.
