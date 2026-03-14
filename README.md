# Ratchet — Debate-Driven Development Workflow Engine for Claude Code

Ratchet turns AI code generation into a structured development process. Every phase of development — planning, testing, building, reviewing, hardening — is driven by **paired agents that debate** until they reach consensus.

## How It Works

1. **Initialize** — Ratchet scans your project (or interviews you for greenfield), debates the approach internally, and presents 2-3 strategy options with tradeoffs
2. **Phase-gated debates** — work proceeds through ordered phases: `plan → test → build → review → harden`. Each phase must pass before the next begins
3. **Agent pairs debate** — a builder (generative) and critic (adversarial) argue each phase. The critic runs real validation commands as evidence
4. **Guards gate advancement** — deterministic checks (lint, tests, security scans) run at phase boundaries. Blocking guards must pass to advance
5. **Commit or PR** — when a milestone completes, Ratchet packages the work as a local commit or pull request
6. **Learn from feedback** — CI failures and PR review comments feed back into the system via retrospectives, improving agents and guards over time

## Installation

### With Nix (recommended)

```bash
# Global (all projects)
nix run github:netbrain/ratchet -- --global

# Project-local
cd /your/project
nix run github:netbrain/ratchet -- --local

# Uninstall
nix run github:netbrain/ratchet -- --uninstall --global
nix run github:netbrain/ratchet -- --uninstall --local
```

### With Nix (v2 branch — pre-release)

```bash
# Global
nix run github:netbrain/ratchet/v2 -- --global

# Project-local
cd /your/project
nix run github:netbrain/ratchet/v2 -- --local
```

### Manual

```bash
git clone -b v2 git@github.com:netbrain/ratchet.git && cd ratchet
./install.sh --global    # or --local from your project dir
```

### Options

| Flag | Description |
|------|-------------|
| `--global` | Install to `~/.claude/` (all projects) |
| `--local` | Install to `.claude/` (current project only) |
| `--uninstall` | Remove ratchet from the chosen scope |
| `--no-git-hooks` | Skip git pre-commit hook installation |

## Quick Start

### New project (greenfield)

```
/ratchet:init
```

Ratchet will:
1. Ask what you're building
2. Suggest a stack and methodology with rationale
3. Debate the approach internally (angel vs devil)
4. Present 2-3 options with pros/cons — you pick or mix-and-match
5. Generate workflow config, components, agent pairs, guards, and a milestone roadmap

Then start building:

```
/ratchet:run
```

Ratchet walks you through each phase of the first milestone. The generative agent does the work, the adversarial agent verifies it, guards run at phase boundaries.

Preview what would run without executing anything:

```
/ratchet:run --dry-run
```

Run the full plan end-to-end without human intervention (halts on human escalation or unrecoverable failures):

```
/ratchet:run --unsupervised             # auto-commits locally
/ratchet:run --unsupervised --auto-pr   # auto-creates PRs at milestone boundaries
```

### Existing project

```
/ratchet:init
```

Same flow, but Ratchet scans your codebase first — it reads your manifests, tests, CI config, and directory structure before asking you anything. The interview focuses on what you want to *improve*, not what already exists.

## Commands

| Command | Description |
|---------|-------------|
| `/ratchet:init` | Analyze project, interview human, generate workflow config and agent pairs |
| `/ratchet:run` | Execute phase-gated debates — the core workflow |
| `/ratchet:status` | Milestone and phase progress snapshot |
| `/ratchet:pair [name]` | Add a new agent pair |
| `/ratchet:guard` | Manage deterministic checks (list, add, run, override) |
| `/ratchet:debate [id]` | View or continue an ongoing debate |
| `/ratchet:verdict [id]` | Human-in-the-loop: cast deciding vote on escalated debate |
| `/ratchet:score [pair]` | Quality metrics and trends |
| `/ratchet:retro [pr]` | Retrospective — learn from CI failures and PR feedback |
| `/ratchet:gen-tests` | Generate tests from debate findings |
| `/ratchet:tighten [pair]` | Sharpen agents from debate lessons and retro findings |
| `/ratchet:advise` | On-demand workflow health check — pair effectiveness, scope gaps, guard recommendations |

## Workflow

### Phases

Every milestone progresses through ordered phases. Phase N must complete before phase N+1 begins:

| Phase | What happens | Generative agent's job | Adversarial agent's job |
|-------|-------------|----------------------|----------------------|
| **plan** | Produce a spec | Write acceptance criteria, design decisions, risks | Challenge gaps, untestable criteria, missing edge cases |
| **test** | Write failing tests | Create tests encoding the spec | Verify tests are correct and cover the spec |
| **build** | Implement | Write code that makes tests pass | Run tests, lint, review implementation |
| **review** | Quality review | Fix issues, improve code | Find bugs, logic errors, convention violations |
| **harden** | Edge cases & security | Add validation, fix vulnerabilities | Run security scans, test edge cases |

Workflow presets control which phases apply:
- **tdd**: all 5 phases (plan → test → build → review → harden)
- **traditional**: skip test phase (plan → build → review → harden)
- **review-only**: review phase only

### Guards

Deterministic shell commands that run at phase boundaries:

```yaml
guards:
  - name: lint
    command: "eslint src/"
    phase: build
    blocking: true

  - name: security
    command: "semgrep --config=auto src/"
    phase: harden
    blocking: true

  - name: coverage
    command: "pytest --cov=src --cov-fail-under=80"
    phase: build
    blocking: false    # advisory — logs but doesn't block
```

### Adaptive Intelligence

Ratchet adapts how much structure to apply based on context:

**Pre-debate guards** — Guards can run before debates start (`timing: pre-debate`), catching lint/format failures before wasting debate cycles. Guards without a timing field default to post-debate (backward compatible).

```yaml
guards:
  - name: fmt-check
    command: "gofmt -l ."
    phase: build
    blocking: true
    timing: pre-debate    # fails fast — no debates if formatting is broken
```

**Adaptive round budgets** — Pairs can override the global `max_rounds`. Experienced pairs that rarely need more than one round can run lean:

```yaml
pairs:
  - name: api-quality
    component: backend
    phase: review
    scope: "src/api/**"
    max_rounds: 2         # this pair converges fast
    enabled: true
```

**Trivial fast-path** — The adversarial can issue `TRIVIAL_ACCEPT` for mechanical, obviously correct changes (typo fix, missing import, version bump). All-fast-path phases auto-advance without user confirmation.

**Phase regression** — The adversarial can issue `REGRESS` to send work backward when a later phase discovers a flaw in an earlier phase's output. Budget controlled by `max_regressions` — set globally as an integer, or per-phase as an object:

```yaml
max_regressions: 3                          # 3 regressions allowed for any phase
max_regressions:                            # or per-phase limits
  build: 3                                  # build can regress more (common during TDD)
  review: 1                                 # review regressions are expensive
  # unspecified phases fall back to 2
```

**Retro severity & recurrence** — Retrospective findings are classified by severity (critical/major/minor/noise). When the same gap recurs across retros, severity auto-escalates and findings are linked, giving `/ratchet:tighten` a priority queue.

**Cross-cutting scope** — Changed files are matched against all component scopes, not just the first match. Multi-component changes automatically trigger pairs from all relevant components. Pairs can use `scope: "auto"` to inherit their parent component's scope.

**Orchestrator learning** — Escalation rulings are stored in `.ratchet/escalations/`. When 3+ rulings exist in the same direction for the same pair and dispute type, the settled pattern is offered as a shortcut before spawning the orchestrator.

**Workflow health checks** — `/ratchet:advise` spawns the analyst for an on-demand assessment: pair effectiveness rankings, scope coverage gaps, guard recommendations, and workflow preset suggestions. Also runs automatically after each milestone completion.

### Feedback Loop

```
debates → guards → commit/PR → CI runs → /ratchet:retro → /ratchet:tighten
    ↑                                                            │
    └────────────────────────────────────────────────────────────┘
```

`/ratchet:retro` analyzes CI failures and PR review comments, identifies what Ratchet's debates missed, and proposes fixes (new guards, updated agent prompts, new pairs). `/ratchet:tighten` consumes retro findings to sharpen agents over time.

## Architecture

```
                        ┌─────────────────────────────┐
                        │         Epic Roadmap         │
                        │   milestone → milestone →    │
                        └──────────┬──────────────────┘
                                   │
                        ┌──────────▼──────────────────┐
                        │     Phase Gate Loop          │
                        │  plan → test → build →       │
                        │  review → harden             │
                        │  (REGRESS can send backward) │
                        └──────────┬──────────────────┘
                                   │ (per phase)
                        ┌──────────▼──────────────────┐
                        │  Pre-debate Guards           │
                        │  fmt ✓  lint ✓  (fail fast)  │
                        └──────────┬──────────────────┘
                                   │ (all pre-debate guards pass)
              ┌────────────────────▼───────────────────┐
              │              Debate Protocol            │
              │                                        │
              │  ┌───────────┐       ┌──────────────┐  │
              │  │Generative │◄─────►│ Adversarial  │  │
              │  │  (builds) │debate │  (critiques)  │  │
              │  └───────────┘       └──────────────┘  │
              │                                        │
              │  ACCEPT / TRIVIAL_ACCEPT → consensus   │
              │  REJECT → next round                   │
              │  REGRESS → return to earlier phase      │
              │  Max rounds → check precedent/escalate  │
              └────────────────────┬───────────────────┘
                                   │ (consensus reached)
                        ┌──────────▼──────────────────┐
                        │  Post-debate Guards          │
                        │  tests ✓  security ✓         │
                        └──────────┬──────────────────┘
                                   │ (all blocking guards pass)
                        ┌──────────▼──────────────────┐
                        │  Advance / analyst assess    │
                        │  or complete milestone       │
                        └─────────────────────────────┘
```

### Key Agents

- **Analyst** — scans codebase, interviews human, debates approach internally, generates tailored pairs and workflow config
- **Orchestrator** — impartial tiebreaker for escalated debates
- **Generative** (per pair) — builds/reviews code, has full tool access
- **Adversarial** (per pair) — critiques code, runs validation commands, cannot edit source

## Configuration

### workflow.yaml (v2)

```yaml
version: 2
max_rounds: 3
escalation: human       # human | orchestrator | both
max_regressions: 2      # integer (all phases) or object (per-phase)

progress:
  adapter: none          # none | markdown | github-issues

components:
  - name: backend
    scope: "src/api/**"
    workflow: tdd

  - name: frontend
    scope: "src/ui/**"
    workflow: traditional

pairs:
  - name: api-quality
    component: backend
    phase: review
    scope: "src/api/**"     # or "auto" to inherit component scope
    max_rounds: 2            # optional per-pair override
    enabled: true

guards:
  - name: lint
    command: "npm run lint"
    phase: build
    blocking: true
    timing: pre-debate       # pre-debate | post-debate (default)
    components: [backend, frontend]
```

### Project Runtime (`.ratchet/`)

```
.ratchet/
├── workflow.yaml        # Workflow config (v2) — components, phases, pairs, guards
├── project.yaml         # Project profile (stack, architecture, validation commands)
├── plan.yaml            # Epic roadmap with milestone/phase tracking
├── pairs/               # Generated agent pair definitions
│   └── <pair-name>/
│       ├── generative.md
│       └── adversarial.md
├── debates/             # Debate transcripts
├── guards/              # Guard execution results
├── reviews/             # Agent performance reviews
├── retros/              # Retrospective findings with severity and recurrence  (.gitignore)
├── escalations/         # Orchestrator rulings for precedent lookup            (.gitignore)
├── guards/              # Guard execution results                              (.gitignore)
├── reports/             # Health check reports from /ratchet:advise            (.gitignore)
├── progress/            # Local progress tracking (markdown adapter)           (.gitignore)
└── scores/              # Historical quality metrics (includes fast-path data) (.gitignore)
```

## Progress Tracking

Ratchet can track milestones in external systems:

| Adapter | Description |
|---------|-------------|
| `none` | No external tracking (default) |
| `markdown` | Local markdown files in `.ratchet/progress/` |
| `github-issues` | GitHub Issues via `gh` CLI |

Adapter failures never block debates. Auth is handled via environment (e.g., `gh auth`), never stored in config.

## Ecosystem

Ratchet is technology-agnostic, but during project setup the analyst may suggest complementary tools when they fit:

- [PromptFoo](https://github.com/promptfoo/promptfoo) — eval and regression testing for agent quality
- [OpenViking](https://github.com/volcengine/OpenViking) — persistent context management for complex projects
- [Agency Agents](https://github.com/msitarzewski/agency-agents) — specialist agent personas (security, QA, design)
- [Impeccable](https://github.com/pbakaus/impeccable) — design language skills for frontend quality
