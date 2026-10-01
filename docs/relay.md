# Relay — outbound is yours

Warmline's queue holds mail locally. To actually deliver, you point it at a relay **you** supply: your own SMTP server, or credentials you rent from any provider. Warmline never operates relays, never resells email, and never sees your credentials outside the process you run.

Configure via flags or env vars:

| Flag | Env | Meaning |
|---|---|---|
| `--relay` | `WARMLINE_RELAY` | `host:port` of your relay |
| `--relay-user` | `WARMLINE_RELAY_USER` | username (optional) |
| `--relay-pass` | `WARMLINE_RELAY_PASS` | password (optional — prefer the env var so it stays out of shell history) |
| `--relay-from` | `WARMLINE_RELAY_FROM` | envelope-from override (optional) |

## Examples

Your own Postfix on the same box:

```sh
warmline serve --relay 127.0.0.1:587
```

Amazon SES (SMTP endpoints, credentials from your AWS account):

```sh
export WARMLINE_RELAY=email-smtp.us-east-1.amazonaws.com:587
export WARMLINE_RELAY_USER=AKIA...SMTP-USER
export WARMLINE_RELAY_PASS=...smtp-password...
warmline serve --relay-from=notifications@yourdomain.com
```

Any commercial SMTP provider you already pay (Postmark, Mailgun, Resend, Gmail workspace relay — all speak SMTP):

```sh
WARMLINE_RELAY=smtp.postmarkapp.com:587 \
WARMLINE_RELAY_USER=your-server-token \
WARMLINE_RELAY_PASS=your-secret \
warmline serve
```

## Behavior

- The drain loop claims one queued message at a time, delivers it, records an `attempts` row, marks it `sent` (or requeues on failure with the error recorded — the queue is the retry loop).
- `STARTTLS` is used when the relay offers it; `AUTH PLAIN` when credentials are set.
- Without `--relay`, `serve` still accepts and queues submissions; nothing leaves the host.
- TLS certificate verification uses system roots; relay credentials live in your environment, not in any Warmline database.

## What Warmline is not

Not an ESP, not a shared IP pool, not your deliverability department. The reputation simulation (`docs/simulate.md`) models warmup risk; it does not make your relay reputable.
