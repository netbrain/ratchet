# Ratchet — Debate-Driven Quality Plugin for Claude Code

Ratchet improves code quality through **paired generative and adversarial agents that debate** until they reach consensus on code readiness.

## How It Works

1. **You define quality dimensions** — API contracts, DB performance, test coverage, etc.
2. **Ratchet generates agent pairs** — a builder (generative) and critic (adversarial) per dimension
3. **Agents debate** — the builder defends the code, the critic attacks it with evidence (running tests, benchmarks, linters)
4. **Consensus = ready** — when both agree, code passes. Disagreement escalates to an orchestrator or human.
5. **Ratchet tightens** — performance reviews after each debate feed into sharper agent prompts over time

## Installation

### Global (all projects)

```bash
git clone <repo> && cd ratchet
./install.sh --global
```

### Project-local

```bash
cd /your/project
/path/to/ratchet/install.sh --local
```

### Uninstall

```bash
./install.sh --uninstall --global   # remove global install
./install.sh --uninstall --local    # remove project-local install
```

### Options

| Flag | Description |
|------|-------------|
| `--global` | Install to `~/.claude/` (all projects) |
| `--local` | Install to `.claude/` (current project only) |
| `--uninstall` | Remove ratchet from the chosen scope |
| `--no-git-hooks` | Skip git pre-commit hook installation |

## Quick Start

```bash
# Initialize for your project (analyzes codebase, interviews you, generates pairs)
/ratchet:init

# Run debates on your current changes
/ratchet:run

# View a debate transcript
/ratchet:debate [id]

# Cast a human verdict on an escalated debate
/ratchet:verdict [id] accept|reject|modify

# View quality metrics
/ratchet:score

# Generate tests from debate findings
/ratchet:gen-tests

# Tighten agent pairs based on debate performance
/ratchet:tighten
```

## Commands

| Command | Description |
|---------|-------------|
| `/ratchet:init` | Analyze project + interview human → generate tailored agent pairs |
| `/ratchet:pair [name]` | Add a new pair post-init |
| `/ratchet:run [pair\|all]` | Run pairs against code changes — the core debate workflow |
| `/ratchet:debate [id]` | View/continue an ongoing debate |
| `/ratchet:verdict [id] [decision]` | Human-in-the-loop: cast deciding vote |
| `/ratchet:score [pair]` | Quality metrics and trends |
| `/ratchet:gen-tests [id\|pair]` | Generate tests from debate findings |
| `/ratchet:tighten [pair\|all]` | Tighten the ratchet — sharpen agents from debate lessons |

## Architecture

```
┌──────────┐     produces      ┌──────────────┐
│Generative├────────────────►  │   Artifact    │
│  Agent   │                   │ (code/tests)  │
└────┬─────┘                   └───────┬───────┘
     │                                 │
     │         ┌─────────────┐         │
     │◄────────┤ Adversarial │◄────────┘
     │ critique│   Agent     │ reviews
     │         └─────────────┘
     │
     ▼
┌─────────┐  agree?  ┌────────┐  yes  ┌────────┐
│  Round  ├─────────►│Consensus├──────►│ Accept │
│  N+1    │          └────┬───┘        └────────┘
└─────────┘               │ no (max rounds)
                          ▼
                   ┌─────────────┐
                   │ Orchestrator│──► Final verdict
                   │  or Human   │
                   └─────────────┘
```

### Key Agents

- **Analyst** — reads codebase, interviews human, generates tailored pairs, reviews agent performance
- **Orchestrator** — impartial tiebreaker, reads full debate transcripts, renders verdicts
- **Generative** (per pair) — builds/reviews code, full tool access
- **Adversarial** (per pair) — critiques code, can run tests but cannot edit source

### Debate Protocol

Each round:
1. Generative agent reviews/proposes changes
2. Adversarial agent critiques with evidence (test output, benchmarks)
3. Adversarial renders verdict: ACCEPT, CONDITIONAL_ACCEPT, or REJECT
4. If REJECT → next round (up to max_rounds) → escalate if no consensus

### Quality Ratchet

Once a standard is met, it can't regress:
- Pre-commit hooks block commits during unresolved escalations
- Score tracking shows quality trends over time
- `/ratchet:tighten` sharpens agents from accumulated debate lessons

## Project Runtime (`.ratchet/`)

Created per-project by `/ratchet:init`:

```
.ratchet/
├── project.yaml          # Project profile (stack, architecture, testing spec)
├── config.yaml           # Pairs definition, max rounds, escalation policy
├── pairs/                # Generated agent pair definitions
├── debates/              # Active and completed debate transcripts
├── reviews/              # Agent performance reviews
└── scores/               # Historical quality metrics
```

## Compatibility

- **Standalone**: Full functionality without other plugins
- **With PAUL**: Run pairs during Apply phase, feed results into Unify
- **With GSD**: TaskCompleted hook enforces consensus before task closure
