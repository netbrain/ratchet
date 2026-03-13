---
name: ratchet:init
description: Analyze project and generate tailored agent pairs through codebase analysis and human interview
---

# /ratchet:init — Project Onboarding

Initialize Ratchet for the current project. You execute this entire flow inline — do NOT spawn subagents or tasks for the interview. You ARE the analyst.

## Prerequisites
- No existing `.ratchet/` directory (use `/ratchet:pair` to add pairs to an existing setup)

## Execution Steps

### Step 1: Check Prerequisites

Check if `.ratchet/` already exists. If so, inform the user and suggest `/ratchet:pair` instead.

### Step 2: Codebase Scan (silent — no user interaction)

Before asking the human anything, scan whatever exists in the project:

- Package manifests, lock files, build configs
- CI/CD pipelines
- Documentation (README, ADRs, design docs, CONTRIBUTING)
- Directory structure (top 3 levels)
- Test infrastructure — test directories, config files, coverage setup
- Linters, formatters, type checkers, security scanners
- Infrastructure files (Docker, Terraform, Helm, etc.)

Adapt your scan to what's actually in the repo. DO NOT ask the human for information you can read from the codebase.

If the project is empty or has no code yet, skip this step — the interview IS the discovery phase.

### Step 3: Interview (inline — talk directly to the user)

Use `AskUserQuestion` for every question. The interview adapts based on whether code exists.

**If code exists**: Present what you learned from the scan, then ask about things you CANNOT infer:
- What the human wants to improve or is concerned about
- Pain points, compliance requirements, priorities
- Derive your options from what you actually found — not from a template

**If greenfield (no code)**:
- "What are you building?" — let them describe it in their own words
- Based on their answer, ask follow-ups about scope, constraints, audience, deployment
- **Suggest a stack and methodology** with rationale — don't just ask "what language?"
- Let them accept, modify, or override your suggestion

Rules:
- **Always use `AskUserQuestion`** — never present choices as plain text
- Ask at most **3-5 focused questions**. Listen and adapt, don't run a questionnaire.
- For greenfield: suggest, don't just ask. Be opinionated with rationale.
- Wait for the user to respond before proceeding to the next step.

### Step 3b: Suggest Ecosystem Integrations (when relevant)

Based on what you learned, consider whether any complementary tools would benefit this project. Only suggest what genuinely fits — this is not a checklist. Use `AskUserQuestion` if you have a relevant suggestion:

- **PromptFoo** — for projects with many agent pairs where eval/regression testing of agent quality matters
- **OpenViking** — for large projects where cross-phase context management is complex
- **Agency Agents** — for projects spanning specialized domains where pre-built expert personas save time
- **Impeccable** — for frontend projects where design quality is a concern

If none are relevant, skip this step entirely. Don't mention tools that don't fit.

### Step 4: Internal Debate — Argue the Approach

Before presenting anything to the user, hold an internal debate about the best approach. Think through competing strategies like an angel and devil on the user's shoulder:

**For each major decision** (stack choice, methodology, component structure, workflow preset), argue both sides:
- **Advocate**: Why this approach fits the user's stated goals, constraints, and context
- **Challenger**: What could go wrong, what's being over-engineered, what simpler alternative exists

Produce **2-3 distinct approach options** that represent meaningfully different tradeoffs. Not minor variations — real strategic choices. Examples of the kind of tradeoffs to surface:
- Rigorous TDD everywhere vs. TDD for core logic + traditional for glue code
- Many focused pairs vs. fewer broad pairs
- Full phase pipeline vs. lightweight review-only to start
- Strict guards that block vs. advisory-only to avoid friction early

Each option should have: a name, a brief description, the tradeoffs (pros/cons), and who it's best for.

### Step 5: Present Options to the User

Use `AskUserQuestion` to present the approach options. Put the full comparison in the question text:

```
Based on what I learned, here are three approaches:

**Option A: [Name]**
[Description]. Phases: [which]. Pairs: [how many, what kind].
+ [pro]
+ [pro]
- [con]

**Option B: [Name]**
[Description]. Phases: [which]. Pairs: [how many, what kind].
+ [pro]
+ [pro]
- [con]

**Option C: [Name]**
...

Which approach fits best?
```

Options: `"Option A: [Name]"`, `"Option B: [Name]"`, `"Option C: [Name]"`, `"Let's discuss / mix and match"`

If "Let's discuss": use follow-up `AskUserQuestion` calls to refine. The user may want pieces from different options.

### Step 6: Finalize Configuration (iterative — do NOT skip to a final config)

Finalize the configuration through conversation, one concern at a time. Do NOT jump to a complete config — walk through each area with the user.

**6a. Components** — present the proposed components with scope globs and workflow presets. Use `AskUserQuestion`:
- Question: "[component list with scopes and workflows]. Do these groupings make sense?"
- Options: `"Looks good"`, `"Modify"`, `"Add/remove components"`

**6b. Pairs — discuss each one.** For each proposed pair, use `AskUserQuestion` to validate:
- What quality dimension does this pair focus on?
- What should the adversarial specifically look for? Ask the user — they know their domain. E.g., "For the file-watching pair, what edge cases matter most? Lock files? Rapid successive writes? Symlinks?"
- What validation commands should the adversarial run? Suggest based on the stack but ask if there are others.
- Is the phase assignment right? Explain why you chose it and let the user adjust.

Don't present all pairs at once for rubber-stamping. Walk through them — the user's input here directly shapes the agent prompts, which is the most important output of init.

**Ecosystem-inspired pairs:** After discussing the initial pairs, consider whether ecosystem projects suggest additional quality dimensions the user hasn't thought of. Draw from Impeccable's design expertise (information hierarchy, glanceability, accessibility) for frontend pairs and Agency Agents' specialist personas (security, performance, observability) for domain-specific pairs. Present these as suggestions with the inspiration source explained — e.g., "Drawing from Impeccable's design principles, a dashboard-ux pair could evaluate whether status information is glanceable and color-coded effectively." Let the user decide whether to add them.

**6c. Guards — discuss what checks matter.** Use `AskUserQuestion`:
- Present what you inferred from the stack (e.g., "I'd suggest `go vet`, `go test`, `gofmt` as blocking guards on the build phase")
- Ask what's missing: "Are there other checks you run or want to run? Linters, security scanners, benchmarks?"
- For each guard, confirm: blocking or advisory? Which phase? Which components?
- Options: `"These guards are good"`, `"Add more"`, `"Modify"`, `"Skip guards for now"`

**6d. Progress tracking:**
- Question: "How do you want to track milestone progress?"
- Options: `"None (just local)"`, `"Markdown files in .ratchet/progress/"`, `"GitHub Issues (requires gh CLI)"`, `"Other / configure later"`

**6e. Final review** — only after walking through each area, present the complete config for approval:
- Question: "[full formatted config]. Everything look right?"
- Options: `"Approve"`, `"Modify [section]"`, `"Start over"`

Wait for approval before proceeding.

### Step 7: Build Epic

Based on everything learned, propose a development roadmap:
- **For greenfield projects, Milestone 1 is always "Workflow Validation"** — a minimal vertical slice that proves the Ratchet pipeline works end-to-end. Pick the simplest possible feature that exercises all configured pairs and guards. The acceptance criteria should focus on the workflow functioning correctly (debates reach consensus, guards pass, phases gate properly), not on feature completeness. Real project work starts at Milestone 2.
- Break remaining milestones by dependency and priority
- Each milestone has: name, description, which pairs are relevant, what "done" looks like
- Present the epic to the human using `AskUserQuestion` for approval:
  - Question: "Proposed roadmap: [formatted milestone list]. Approve this epic?"
  - Options: `"Approve"`, `"Modify milestones"`, `"Start over"`
- The epic is a living document — it evolves as the project develops

plan.yaml format:
```yaml
epic:
  name: "<project name>"
  description: "<one-line description>"
  milestones:
    - id: 1
      name: "<milestone name>"
      description: "<what this milestone delivers>"
      pairs: [<relevant-pair-names>]
      status: pending        # pending | in_progress | done
      phase_status:           # tracks progress through phases
        plan: pending         # pending | in_progress | done
        test: pending
        build: pending
        review: pending
        harden: pending
      done_when: "<concrete acceptance criteria>"
      progress_ref: null     # set by progress adapter when milestone starts
  current_focus: null
```

### Step 8: Generate

For each approved pair, write:
- `.ratchet/project.yaml` — project profile with stack, architecture, testing spec
- `.ratchet/plan.yaml` — development roadmap with milestones
- `.ratchet/pairs/<name>/generative.md` — builder agent
- `.ratchet/pairs/<name>/adversarial.md` — critic agent
- `.ratchet/workflow.yaml` — v2 workflow configuration with pairs, components, guards:

```yaml
version: 2
max_rounds: 3
escalation: human  # human | orchestrator | both

progress:
  adapter: none  # none | markdown | github-issues | linear | jira

components:
  - name: <component-name>
    scope: "<file-glob>"
    workflow: tdd  # tdd | traditional | review-only

pairs:
  - name: <pair-name>
    component: <component-name>
    phase: review  # plan | test | build | review | harden
    scope: "<file-glob>"
    enabled: true
  # ... more pairs

guards: []  # populated based on testing spec
```

Create the `.ratchet/` directory structure:
```
.ratchet/
├── project.yaml
├── workflow.yaml
├── plan.yaml
├── pairs/
│   └── <pair-name>/
│       ├── generative.md
│       └── adversarial.md
├── debates/
├── reviews/
└── scores/
```

IMPORTANT:
- If code exists, scan it FIRST — never ask what you can read
- For existing projects, the interview focuses on what the human wants to improve
- For greenfield projects, the interview discovers intent, then you suggest stack and methodology
- Generated agent pair definitions must contain PROJECT-SPECIFIC knowledge (not generic templates)
- Generative agents get tools: Read, Grep, Glob, Bash, Write, Edit
- Adversarial agents get tools: Read, Grep, Glob, Bash with disallowedTools: Write, Edit
- Adversarial agents must know the exact validation commands available in this project
- Scope each pair to specific file globs — tight scope leads to deep analysis

### Step 9: Verify Output

After generation, verify:
- `.ratchet/project.yaml` exists and contains valid stack/testing info
- `.ratchet/workflow.yaml` exists with `version: 2` and at least one pair registered
- Each registered pair has both `generative.md` and `adversarial.md` in `.ratchet/pairs/`
- All directories created: `debates/`, `reviews/`, `scores/`

### Step 10: Report

Present a summary:
```
Ratchet initialized for [project name]

Stack: [language] / [framework] / [database]
Architecture: [pattern]

Pairs created:
  [pair-name] — [scope] — [quality dimension]
  [pair-name] — [scope] — [quality dimension]
  ...
```

Then use `AskUserQuestion` to guide the user on what to do next:
- Options:
  - "Start first debate (/ratchet:run)" — begin the epic workflow
  - "Add more pairs (/ratchet:pair)"
  - "Done for now"
