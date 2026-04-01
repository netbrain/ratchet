# Ratchet — Debate-Driven Development Workflow Engine for Claude Code

Ratchet turns AI code generation into a structured development process. Every phase of development — planning, testing, building, reviewing, hardening — is driven by **paired agents that debate** until they reach consensus.

## How It Works

1. **Initialize** — Ratchet scans your project (or interviews you for greenfield), debates the approach internally, and presents 2-3 strategy options with tradeoffs
2. **Milestones decompose into issues** — each milestone is broken into independently executable issues. Independent issues run **in parallel**, each in an isolated git worktree. Independent milestones can also run in parallel when they declare `depends_on`
3. **Phase-gated debates** — each issue progresses through ordered phases: `plan → test → build → review → harden`. Phases are gated — each must pass before the next begins
4. **Agent pairs debate** — a builder (generative) and critic (adversarial) argue each phase. The critic runs real validation commands as evidence
5. **Guards gate advancement** — deterministic checks (lint, tests, security scans) run at phase boundaries. Blocking guards must pass to advance
6. **Issue → PR** — each issue produces its own PR when complete. Dependent issues state their merge order
7. **Learn from feedback** — CI failures, PR review comments, and debate patterns feed back into the system via `/ratchet:tighten`, improving agents and guards over time

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

### Manual

```bash
git clone git@github.com:netbrain/ratchet.git && cd ratchet
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
6. Offer to configure GitHub repo settings (auto-delete branches, squash merge, disable merge/rebase commits) via `gh repo edit` — when a GitHub remote is detected

Then start building:

```
/ratchet:run
```

Ratchet launches the first milestone's issues in parallel. Each issue progresses through its own phase pipeline — the generative agent does the work, the adversarial agent verifies it, guards run at phase boundaries. Each issue produces its own PR.

Preview what would run without executing anything:

```
/ratchet:run --dry-run
```

Quick fix — skip the full pipeline for small, well-understood changes:

```
/ratchet:run --quick "Fix off-by-one in src/parser.ts validateToken loop"
```

Run in the current session (no worktree isolation — you serve as quality gate):

```
/ratchet:run --here
/ratchet:run --here --quick "Add missing null check in utils.ts"
```

Run the full plan end-to-end without human intervention (halts on human escalation or unrecoverable failures):

```
/ratchet:run --unsupervised             # auto-commits locally
/ratchet:run --unsupervised --auto-pr   # auto-creates PRs per issue
/ratchet:run --go                       # shorthand for --unsupervised --auto-pr
```

### Existing project

```
/ratchet:init
```

Same flow, but Ratchet scans your codebase first — it reads your manifests, tests, CI config, and directory structure before asking you anything. The interview focuses on what you want to *improve*, not what already exists.

### Workspaces

For repos with multiple projects, Ratchet supports workspaces. Each workspace has its own `.ratchet/` with pairs, plans, and debates. A root config provides shared policy defaults.

```
/ratchet:init    # auto-detects multi-project structure, creates root + workspace configs
```

Root `workflow.yaml`:
```yaml
version: 2
workspaces:
  - path: monitor
    name: monitor
  - path: engine
    name: engine

# Shared policy — inherited by all workspaces (per-field override)
models:
  generative: opus
  adversarial: sonnet
escalation: human
max_rounds: 3
```

Work on a specific workspace:
```
/ratchet:run monitor         # from repo root — target workspace by name
/ratchet:status monitor      # workspace-level status
/ratchet:status              # overview from root
```

Or `cd` into a workspace directory — Ratchet finds the local `.ratchet/` and runs in single-project mode.

Workspaces are fully autonomous — they never share pairs, guards, or plans. The root only provides policy defaults (`models`, `escalation`, `max_rounds`, `max_regressions`, `pr_scope`) that workspaces can override per-field.

## Commands

| Command | Alias | Description |
|---------|-------|-------------|
| `/ratchet:init` | | Analyze project, interview human, generate workflow config and agent pairs |
| `/ratchet:run` | `/rr` | Execute phase-gated debates — the core workflow |
| `/ratchet:status` | `/rrs` | Milestone and phase progress snapshot |
| `/ratchet:tighten [pair\|pr N]` | `/rrt` | Analyze all improvement signals and sharpen the system |
| `/ratchet:pair [name]` | | Add a new agent pair |
| `/ratchet:guard` | | Manage deterministic checks (list, add, run, override) |
| `/ratchet:debate [id]` | | View or continue an ongoing debate |
| `/ratchet:verdict [id]` | | Human-in-the-loop: cast deciding vote on escalated debate |
| `/ratchet:score [pair]` | | Quality metrics and trends |
| `/ratchet:watch` | | Watch active PRs for merge conflicts, CI failures, and review comments |
| `/ratchet:sidequest` | | Log discoveries and sidequests during active work |
| `/ratchet:statusline` | | Configure the Ratchet status line in Claude Code |
| `/ratchet:update` | | Update Ratchet framework to the latest version |

**`/ratchet:run` flags:**

```
/ratchet:run                         # Resume from epic — propose next focus
/ratchet:run [pair-name]             # Run a specific pair against its scoped files
/ratchet:run [workspace]             # Target a specific workspace
/ratchet:run --milestone <id>        # Run a single milestone's pipeline
/ratchet:run --issue <ref>           # Run a single issue's pipeline
/ratchet:run --all-files             # Run all pairs against all files in scope
/ratchet:run --no-cache              # Force re-debate even if files haven't changed
/ratchet:run --no-auto-merge         # Disable auto-merging of prerequisite PRs
/ratchet:run --dry-run               # Preview what would run without executing
/ratchet:run --quick "<description>" # Quick-fix: skip plan, single generative pass
/ratchet:run --here                  # In-session execution, no worktree isolation
/ratchet:run --unsupervised          # End-to-end without human intervention
/ratchet:run --auto-pr               # Auto-create PRs per issue
/ratchet:run --go                    # Shorthand for --unsupervised --auto-pr
```

## Workflow

### Phases

Every issue progresses through ordered phases. Phase N must complete before phase N+1 begins:

| Phase | What happens | Generative agent's job | Adversarial agent's job |
|-------|-------------|----------------------|----------------------|
| **plan** | Produce a spec | Write acceptance criteria, design decisions, risks | Challenge gaps, untestable criteria, missing edge cases |
| **test** | Write failing tests | Create tests encoding the spec | Verify tests are correct and cover the spec |
| **build** | Implement | Write code that makes tests pass | Run tests, lint, review implementation |
| **review** | Quality review | Fix issues, improve code | Find bugs, logic errors, convention violations |
| **harden** | Edge cases & security | Add validation, fix vulnerabilities | Run security scans, test edge cases |

Pipeline presets control which phases apply:
- **full**: all 5 phases (plan → test → build → review → harden)
- **standard**: skip test phase (plan → build → review → harden)
- **review**: review phase only
- **hotfix**: build → review (skip planning and hardening)
- **secure**: review → harden (security-focused review)

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

**Rationalization-check guards** — Guards can bundle multiple assertions. The guard passes only if all checks pass:

```yaml
guards:
  - name: debate-artifacts-complete
    type: rationalization-check
    phase: review
    blocking: false
    checks:
      - assert: "All referenced debate IDs have artifact directories"
        command: |
          for id in $(yq -r '.. | .debates[]? // empty' .ratchet/plan.yaml); do
            [ -d ".ratchet/debates/$id" ] || { echo "MISSING: $id" >&2; exit 1; }
          done
      - assert: "Active debates have resolved timestamps"
        command: |
          for f in .ratchet/debates/*/meta.json; do
            resolved=$(jq -r '.resolved // "null"' "$f")
            [ "$resolved" != "null" ] || { echo "UNRESOLVED: $f" >&2; exit 1; }
          done
```

**Generated files guard** — Ratchet includes a built-in pre-commit hook (`scripts/check-generated-files.sh`) that blocks committing build artifacts and runtime state. It detects generated files via path patterns (e.g., `*_templ.go`, `node_modules/`, `dist/`) and content markers (e.g., `// Code generated ... DO NOT EDIT`). The guard is stack-aware — it reads `project.yaml` to infer ecosystem-specific patterns (Go, Node, Python, Rust, JVM). Projects can extend detection via `generated_file_patterns` in `workflow.yaml`. Auto-registered during `/ratchet:init`. Override with `RATCHET_ALLOW_GENERATED=1 git commit ...`.

### Adaptive Intelligence

Ratchet adapts how much structure to apply based on context:

**Pre-execution guards** — Guards can run before debates start (`timing: pre-execution`), catching lint/format failures before wasting debate cycles. Guards without a timing field default to `post-execution` (backward compatible).

```yaml
guards:
  - name: fmt-check
    command: "gofmt -l ."
    phase: build
    blocking: true
    timing: pre-execution    # fails fast — no debates if formatting is broken
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

**Shared resources** — Guards can declare resource dependencies (`requires: [postgres]`). Resources are defined with start/stop commands and an optional `singleton` flag. Singleton resources are file-locked so only one pipeline uses them at a time. Non-singleton resources are started once and shared freely. Resources are torn down when the milestone completes.

```yaml
resources:
  - name: postgres
    start: "docker compose up -d postgres"
    stop: "docker compose down postgres"
    singleton: true       # one pipeline at a time

  - name: redis
    start: "docker compose up -d redis"
    singleton: false      # shared freely

  - name: playwright
    start: "npx playwright install --with-deps"
    singleton: true       # never run more than one playwright process

guards:
  - name: integration-tests
    command: "npm run test:integration"
    phase: build
    blocking: true
    requires: [postgres, redis]

  - name: e2e-tests
    command: "npx playwright test"
    phase: harden
    blocking: true
    requires: [playwright]
```

**Parallel milestones** — Milestones can declare `depends_on` to form a dependency graph. Independent milestones (no dependencies) run in parallel, each handling its own issue DAG. If no milestones declare `depends_on`, they run sequentially (backward compatible).

```yaml
milestones:
  - id: 1
    name: "Auth System"
    depends_on: []          # Layer 0 — parallel with M2

  - id: 2
    name: "Data Layer"
    depends_on: []          # Layer 0 — parallel with M1

  - id: 3
    name: "API Integration"
    depends_on: [1, 2]      # Layer 1 — waits for both
```

**Round summarization** — For debates that reach 3+ rounds, prior round history is condensed to summaries. Only the most recent round is included in full context. This saves ~5-10k tokens per 3-round debate while preserving the full argument thread.

**Retro severity & recurrence** — Retrospective findings are classified by severity (critical/major/minor/noise). When the same gap recurs, severity auto-escalates and findings are linked, giving `/ratchet:tighten` a priority queue.

**Cross-cutting scope** — Changed files are matched against all component scopes, not just the first match. Multi-component changes automatically trigger pairs from all relevant components. Pairs can use `scope: "auto"` to inherit their parent component's scope.

**Tiebreaker learning** — Escalation rulings are stored in `.ratchet/escalations/`. When 3+ rulings exist in the same direction for the same pair and dispute type, the settled pattern is offered as a shortcut before spawning the tiebreaker.

**Workflow health checks** — `/ratchet:tighten` spawns the analyst for an on-demand assessment: pair effectiveness rankings, scope coverage gaps, guard recommendations, workflow preset suggestions, and PR/CI gap analysis. Also runs automatically after each milestone completion.

### Feedback Loop

```
debates → guards → commit/PR → CI runs → /ratchet:tighten
    ↑                                          │
    └──────────────────────────────────────────┘
```

`/ratchet:tighten` is the single entrypoint for improving the system. It analyzes CI failures, PR review comments, debate history, escalation patterns, and discoveries — identifies what Ratchet missed — and applies fixes: sharpened agent prompts, new guards, workflow config changes.

## Architecture

```
Epic Roadmap
    |
    +-- Milestone 1 (parallel)    +-- Milestone 2 (parallel)    +-- Milestone 3
    |   own agent                 |   own agent                 |   depends_on: [1, 2]
    |                             |                             |   starts when both complete
    |   Issue DAG:                |   Issue DAG:                |
    |     A --+                   |     D --+                   |
    |     B --+--> C              |     E --+                   |
    |                             |                             |
```

Each issue pipeline runs per-phase:

```
Pre-debate Guards (fail fast)
    |
    v
Debate Runner Agent (orchestrates -- never writes code)
    |
    +-- Generative Agent <--debate--> Adversarial Agent
    |
    |   ACCEPT / TRIVIAL_ACCEPT --> advance to next phase
    |   REJECT                  --> next round
    |   CONDITIONAL_ACCEPT      --> address conditions, then re-review
    |   REGRESS                 --> return to earlier phase
    |
    v
Post-debate Guards + phase advance
```

Shared resources with singleton locking:

```
Guards can declare: requires: [postgres, redis]

  postgres (singleton)  -- flock'd, one pipeline at a time
  redis (shared)        -- started once, shared freely
  playwright (singleton) -- flock'd, serialized access
```
```

The `/ratchet:run` skill uses a modular architecture — mode specs (`modes/quick-fix.md`, `modes/here.md`, `modes/epic-guided.md`, `modes/dry-run.md`) and pipeline modules (`issue-pipeline.md`, `unsupervised.md`, `pr-body.md`, `plan-tracking-format.md`) are loaded only when needed, keeping the base skill lean (~500 lines, ~3k tokens). The full skill with all modules loaded is ~1,500 lines/~7.5k tokens.

### Key Agents

- **Analyst** — scans codebase, interviews human, debates approach internally, generates tailored pairs and workflow config
- **Debate Runner** — orchestrates a single debate: spawns generative/adversarial agents, manages rounds, persists artifacts. Cannot write code itself
- **Tiebreaker** — impartial arbiter for escalated debates
- **Generative** (per pair) — builds/reviews code, has full tool access
- **Adversarial** (per pair) — critiques code, runs validation commands, cannot edit source

## Configuration

### workflow.yaml

```yaml
version: 2
max_rounds: 3
escalation: human       # human | tiebreaker | both | none | promote
max_regressions: 2      # integer (all phases) or object (per-phase)
pr_scope: issue         # issue | debate | phase | milestone

models:                  # optional — omit to inherit parent model
  debate_runner: sonnet  # protocol orchestration
  generative: opus       # writes code
  adversarial: sonnet    # reviews code
  tiebreaker: sonnet     # resolves escalations
  analyst: opus          # deep analysis

progress:
  adapter: none          # none | markdown | github-issues
  # publish_debates: per-round  # false | per-round | summary (github-issues only)

components:
  - name: backend
    scope: "src/api/**"
    pipeline: full
    strategy: debate          # default — generative + adversarial pair

  - name: frontend
    scope: "src/ui/**"
    pipeline: standard

  - name: schemas
    scope: "schemas/*.json"
    pipeline: review
    strategy: solo            # single agent, no adversarial review
    promote_on_guard_failure: true  # promote even if guards fail

pairs:
  - name: api-quality
    component: backend
    phase: review
    scope: "src/api/**"     # or "auto" to inherit component scope
    max_rounds: 2            # optional per-pair override
    enabled: true
    models:                  # optional per-pair override
      generative: opus
      adversarial: sonnet

guards:
  - name: lint
    command: "npm run lint"
    phase: build
    blocking: true
    timing: pre-execution       # pre-execution | post-execution (default)
    components: [backend, frontend]

  - name: integration-tests
    command: "npm run test:integration"
    phase: build
    blocking: true
    requires: [postgres]     # needs singleton resource

resources:
  - name: postgres
    start: "docker compose up -d postgres"
    stop: "docker compose down postgres"
    singleton: true          # one pipeline at a time
```

### Project Runtime (`.ratchet/`)

```
.ratchet/
├── workflow.yaml        # Workflow config (v2) — components, phases, pairs, guards
├── project.yaml         # Project profile (stack, architecture, validation commands)
├── plan.yaml            # Epic roadmap — milestones, issues, per-issue phase tracking
├── pairs/               # Generated agent pair definitions
│   └── <pair-name>/
│       ├── generative.md
│       └── adversarial.md
├── debates/             # Debate transcripts
├── executions/          # Execution logs (per-run metadata and timing)         (.gitignore)
├── guards/              # Guard execution results                              (.gitignore)
├── reviews/             # Agent performance reviews
├── retros/              # Retrospective findings with severity and recurrence  (.gitignore)
├── escalations/         # Tiebreaker rulings for precedent lookup              (.gitignore)
├── reports/             # Tighten reports and health assessments               (.gitignore)
├── progress/            # Local progress tracking (markdown adapter)           (.gitignore)
├── scores/              # Historical quality metrics (includes fast-path data) (.gitignore)
└── archive/             # Archived epic and plan history
```

## Progress Tracking

Ratchet can track milestones in external systems:

| Adapter | Description |
|---------|-------------|
| `none` | No external tracking (default) |
| `markdown` | Local markdown files in `.ratchet/progress/` |
| `github-issues` | GitHub Issues via `gh` CLI |

**Debate publishing** — When using the `github-issues` adapter, debate rounds can be posted as comments on the individual work issue (not the epic tracking issue). Configure via `publish_debates` in `workflow.yaml`:
- `false` (default) — debates stay local in `.ratchet/debates/`
- `per-round` — each round is posted as a comment immediately after completion
- `summary` — a single consolidated comment is posted when the debate finishes

Published comments include artifact inlining — files created or modified by the generative agent are embedded in collapsed `<details>` blocks, so GitHub readers see actual content rather than local file paths.

Adapter failures never block debates. Auth is handled via environment (e.g., `gh auth`), never stored in config.

## Ecosystem

Ratchet is technology-agnostic, but during project setup the analyst may suggest complementary tools when they fit:

- [PromptFoo](https://github.com/promptfoo/promptfoo) — eval and regression testing for agent quality
- [OpenViking](https://github.com/volcengine/OpenViking) — persistent context management for complex projects
- [Agency Agents](https://github.com/msitarzewski/agency-agents) — specialist agent personas (security, QA, design)
- [Impeccable](https://github.com/pbakaus/impeccable) — design language skills for frontend quality
