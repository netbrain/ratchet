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

Then start building:

```
/ratchet:run
```

Ratchet walks you through each phase of the first milestone. The generative agent does the work, the adversarial agent verifies it, guards run at phase boundaries.

### Existing project

```
/ratchet:init
```

Same flow, but Ratchet scans your codebase first — it reads your manifests, tests, CI config, and directory structure before asking you anything. The interview focuses on what you want to *improve*, not what already exists.

### Upgrading from v1

If you have an existing `.ratchet/config.yaml` from v1:

```
/ratchet:migrate
```

This converts your v1 config to v2 `workflow.yaml` with components, phases, and guards inferred from your existing setup.

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
| `/ratchet:migrate` | Upgrade v1 config.yaml to v2 workflow.yaml |

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
                        └──────────┬──────────────────┘
                                   │ (per phase)
              ┌────────────────────▼───────────────────┐
              │              Debate Protocol            │
              │                                        │
              │  ┌───────────┐       ┌──────────────┐  │
              │  │Generative │◄─────►│ Adversarial  │  │
              │  │  (builds) │debate │  (critiques)  │  │
              │  └───────────┘       └──────────────┘  │
              │                                        │
              │  Round N → ACCEPT / REJECT → Round N+1 │
              │  Max rounds → Escalate to orchestrator  │
              └────────────────────┬───────────────────┘
                                   │ (consensus reached)
                        ┌──────────▼──────────────────┐
                        │     Guards (deterministic)   │
                        │  lint ✓  tests ✓  security ✓ │
                        └──────────┬──────────────────┘
                                   │ (all blocking guards pass)
                        ┌──────────▼──────────────────┐
                        │    Advance to next phase     │
                        │    or complete milestone     │
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
    scope: "src/api/**"
    enabled: true

guards:
  - name: lint
    command: "npm run lint"
    phase: build
    blocking: true
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
├── retros/              # Retrospective findings (CI/PR feedback)
├── progress/            # Local progress tracking (markdown adapter)
└── scores/              # Historical quality metrics
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
