# Simulate — reputation trajectory (planned)

`warmline simulate` is the W5–7 milestone: a deterministic reputation-trajectory simulation over a volume plan you declare.

Planned shape:

```
warmline simulate --from=migrate-report  # or --fixture
```

Inputs (all declared by you, none fetched live):

- volume plan (messages/day ramp)
- bounce rate, complaint rate
- new vs aged sending IP

Outputs:

- JSON + markdown warmup curve (day-by-day)
- predicted block/defer risk bands

**This is a simulation, not a deliverability guarantee.** Same input always produces the same curve — that determinism is CI-gated on a golden fixture. Warmline does not operate IP pools and cannot promise inbox placement; anyone who does is selling something.

Until the engine ships, migrate reports carry `reputation_sim: { "status": "planned" }` so downstream tooling can pin the schema now.
