# Step 5-dry: Dry-Run Preview

If `--dry-run` is specified, produce a formatted preview and stop. No agents are spawned, no debates created, no files modified.

## Token Cost Estimation

After building the dependency graph (Step 3), compute estimated tokens per issue using these formulas:

**Base tokens by pipeline mode:**
| Pipeline mode | Base tokens | Phases | Rationale |
|---|---|---|---|
| `solo` | 20k | (single pass) | Single generative pass, no adversarial |
| `review` | 40k | review | Review-only debate (1 phase, gen + adv) |
| `hotfix` | 60k | build, review | Fast-track fix (2 phases) |
| `secure` | 60k | review, harden | Security hardening (2 phases) |
| `standard` | 80k | plan, build, review, harden | Standard pipeline (4 phases) |
| `full` | 160k | plan, test, build, review, harden | Full pipeline (5 phases) |

**Scaling factors:**
- **Pairs**: Multiply base by the number of pairs assigned to the issue. Each pair runs its own debate/execution.
- **Guards**: Add 2k per guard (both pre-execution and post-execution) assigned to the issue's phases. Guards invoke external commands and produce output that consumes context.
- **Max rounds** (debate mode only): The base already accounts for typical round counts. For `max_rounds > 3`, scale the debate portion by `max_rounds / 3`.

**Formula:**
```
issue_tokens = (base_tokens × pair_count × round_scale) + (2000 × guard_count)

where:
  base_tokens  = mode lookup from table above
  pair_count   = number of pairs assigned to the issue
  round_scale  = max(1, max_rounds / 3) for debate strategies, 1 for solo
  guard_count  = number of guards matching the issue's component and phases
```

**Cost estimation** uses current API rates (update these when rates change):
- Opus input: $15 / 1M tokens, output: $75 / 1M tokens (assume 30% input, 70% output)
- Sonnet input: $3 / 1M tokens, output: $15 / 1M tokens (assume 30% input, 70% output)
- Debate mode uses both opus (generative) and sonnet (adversarial) — estimate 60% opus, 40% sonnet by token volume
- Solo mode uses opus only

```
# Blended rate per 1k tokens (debate mode):
#   opus:   0.30 × $0.015 + 0.70 × $0.075 = $0.057/1k tokens
#   sonnet: 0.30 × $0.003 + 0.70 × $0.015 = $0.0114/1k tokens
#   blended = 0.60 × $0.057 + 0.40 × $0.0114 = $0.0388/1k tokens
#
# Solo mode (opus only):
#   $0.057/1k tokens
```

## Dry-Run Output Format

```
Dry-Run Preview
═══════════════

Milestone: [name] — [description]

Issues ([N] total, [N] ready to run in parallel):

  [ref]: [title]
    Strategy: debate
    Phase: [current phase]
    Pairs: [pair-name], [pair-name]
    Pre-execution guards: [guard-name] (blocking)
    Post-execution guards: [guard-name] (advisory)

  [ref]: [title]  (solo)
    Strategy: solo
    Phase: [current phase]
    Pairs: [pair-name]
    Post-execution guards: [guard-name] (blocking)
    Promote on guard failure: [yes|no]

  [ref]: [title]  (depends on [dep-ref])
    Phase: pending — waiting for dependency
    Pairs: [pair-name]

Phase flow per issue: [phase1] → [phase2] → ... → [phaseN]

Token & Cost Estimates
──────────────────────
  Issue       Mode       Pairs  Guards  Est. Tokens   Est. Cost
  ─────       ────       ─────  ──────  ───────────   ─────────
  [ref]       standard   2      3       ~166k         ~$6.44
  [ref]       solo       1      2       ~24k          ~$1.37
  [ref]       review     1      1       ~42k          ~$1.63
  ─────       ────       ─────  ──────  ───────────   ─────────
  Total                                 ~232k         ~$9.44
```

**In `--unsupervised` mode**: Log the token and cost estimates to stdout but do not block execution. The estimates are informational — they help operators audit spend after the fact. Do not present the `AskUserQuestion` confirmation (unsupervised auto-selects "Run for real").

**In supervised mode**: Include the cost table in the `AskUserQuestion` confirmation:

Question text:
```
Dry-run complete. Estimated cost: ~$[total] ([total_tokens]k tokens).

[cost table from above]

Proceed?
```

Options: `"Run for real (Recommended)"`, `"Done for now"`
