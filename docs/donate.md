# Donate

Warmline is free to self-host. If it saves you a SendGrid week, donate.

No feature is gated on donations — everything works whether you donate or not.

## Addresses

- **Ethereum / USDC (ERC-20):** `0x85ee7E71f762d772599cbF1EC20E651B30657521`
- **Bitcoin:** `bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg`

Run `warmline donate` to print both addresses from the binary itself.

## Attributing a donation to a GitHub handle (optional)

Donations are anonymous by default. If you want credit, sign a one-line message with the wallet you send from and include it in the transfer memo where the chain supports one:

```
donate warmline <github-handle> <UTC date>
```

Example (signed with the sending wallet's key):

```
donate warmline octocat 2026-10-01
```

For USDC transfers on Ethereum, append the message as text to any public gist or X post linking the transaction hash — on-chain memos are not standard for ERC-20 transfers, so this off-chain note is the attribution path. Do not send secrets, keys, or personal data in a memo.

## Where the money goes

Infrastructure for the demo host (if one exists), hardware, and maintainer time. Donation events are acknowledged in release notes when they can be verified on-chain. No subscription, invoice, or "pro" tier will ever be created from this money — see `GOVERNANCE.md` for the binding rule.
