---
name: ratchet:init
description: Analyze project and generate tailored agent pairs through codebase analysis and human interview
---

# /ratchet:init — Project Onboarding

Initialize Ratchet for the current project. You execute this entire flow inline — do NOT spawn subagents or tasks for the interview. You ARE the analyst.

## Prerequisites
- No existing `.ratchet/` directory in the current scope (use `/ratchet:pair` to add pairs to an existing setup)

## Execution Steps

### Step 1: Check Prerequisites

Check if `.ratchet/` already exists in CWD. If so, inform the user and suggest `/ratchet:pair` instead.

**Workspace detection**: Also check if a parent directory has `.ratchet/workflow.yaml` with a `workspaces` key. If so, this is a workspace within an existing multi-project setup. Check whether CWD is already registered as a workspace:
- If registered and has `.ratchet/` → already initialized, suggest `/ratchet:pair`
- If registered but no `.ratchet/` → proceed with init for this workspace
- If NOT registered → proceed with init, and after generating config, auto-register this workspace in the root `workflow.yaml`'s `workspaces` array

**Multi-project root init**: If the user runs `/ratchet:init` at the repo root and the project contains multiple distinct subprojects (detected by multiple `go.mod`, `package.json`, or similar manifests in subdirectories), use `AskUserQuestion`:
- Question: "This repo contains multiple projects: [list subdirs with manifests]. Set up with per-project workspaces?"
- Options: `"Yes — create workspace config (Recommended)"`, `"No — treat as single project"`, `"Let me pick which subdirs"`

If workspaces: create root `.ratchet/workflow.yaml` with only `version`, `workspaces`, and shared policy fields (models, escalation, max_rounds). No pairs, components, or guards at root. Then run workspace-level init for each workspace.

### Step 2: Codebase Scan (silent — no user interaction)

Before asking the human anything, scan whatever exists in the project:

- Package manifests, lock files, build configs
- CI/CD pipelines (`.github/workflows/*.yml`, `Jenkinsfile`, `.gitlab-ci.yml`, `.circleci/config.yml`, etc.) — extract every command/step that acts as a quality gate (lint, test, build, security scan, type check, format check). These become guard candidates.
- Documentation (README, ADRs, design docs, CONTRIBUTING)
- Directory structure (top 3 levels)
- Test infrastructure — test directories, config files, coverage setup
- Linters, formatters, type checkers, security scanners
- Infrastructure files (Docker, Terraform, Helm, etc.)

Adapt your scan to what's actually in the repo. DO NOT ask the human for information you can read from the codebase.

**Error handling for scan failures**: If specific scan targets are missing, skip that category silently and proceed:
```bash
# CI detection — skip gracefully if no CI config found
if ls .github/workflows/*.yml .github/workflows/*.yaml 2>/dev/null | head -1 > /dev/null; then
  # parse pipeline files for guard candidates
else
  # No CI config found — skip CI guard mirroring, note for interview
fi
```
Do NOT abort the scan because one source is missing — gather what you can and note gaps for the interview.

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

Use `AskUserQuestion` to present the approach options. Put the full comparison in the question text.

IMPORTANT: `AskUserQuestion` renders as a terminal selector, NOT as markdown. Do NOT use markdown formatting (`**bold**`, `#` headers, `- ` lists). Use plain text with simple indentation and line breaks:

```
Based on what I learned, here are three approaches:

Option A: [Name]
  [Description]. Phases: [which]. Pairs: [how many, what kind].
  Pros: [pro], [pro]
  Cons: [con]

Option B: [Name]
  [Description]. Phases: [which]. Pairs: [how many, what kind].
  Pros: [pro], [pro]
  Cons: [con]

Option C: [Name]
  ...

Which approach fits best?
```

Options: `"Option A: [Name] (Recommended)"`, `"Option B: [Name]"`, `"Option C: [Name]"`, `"Let's discuss / mix and match"`

Mark the option you believe best fits the user's stated goals as "(Recommended)".

If "Let's discuss": use follow-up `AskUserQuestion` calls to refine. The user may want pieces from different options.

### Step 6: Finalize Configuration (iterative — do NOT skip to a final config)

Finalize the configuration through conversation, one concern at a time. Do NOT jump to a complete config — walk through each area with the user.

**6a. Phases and workflow presets** — before discussing components, explain how phases work and what presets are available. The user needs to understand this to make informed choices about their component workflows. Use `AskUserQuestion` with the explanation in the question text:

```
Ratchet organizes work into phases. Each milestone progresses through its assigned phases in order — a phase must complete before the next begins.

The five phases:

  plan    — Produce a spec. Generative writes acceptance criteria and design
            decisions. Adversarial challenges gaps and untestable criteria.

  test    — Write failing tests. Generative creates tests encoding the spec.
            Adversarial verifies tests are correct and cover the spec.

  build   — Implement. Generative writes code to make tests pass. Adversarial
            runs tests, lint, and reviews the implementation.

  review  — Quality review. Generative fixes issues. Adversarial looks for bugs,
            logic errors, and convention violations.

  harden  — Edge cases and security. Generative adds validation and fixes
            vulnerabilities. Adversarial runs security scans and tests edge cases.

Each component chooses a workflow preset that selects which phases apply:

  tdd (plan > test > build > review > harden)
    Full rigor. Best for core business logic, APIs, and anything where
    correctness matters most. The test phase ensures tests exist before
    implementation — true test-driven development.

  traditional (plan > build > review > harden)
    Skips the test phase. Tests are written during build alongside implementation.
    Good for glue code, integrations, and components where TDD adds friction
    without proportional value.

  review-only (review)
    Minimal — just review existing code. Good for legacy code, documentation,
    or configuration where the full pipeline is overkill.

A note on the review phase: the build adversarial already reviews code (it runs
tests and critiques implementation). A separate review phase adds value when you
want a different lens — e.g., a build pair focused on "does it work" and a review
pair focused on "is it maintainable/idiomatic." If your build pairs already do
thorough quality review, consider whether a dedicated review phase adds enough
value to justify the extra debate cycle. You can always add it later.

Examples:

  A REST API backend — tdd
    Core logic needs test coverage first. Plan defines the API contract,
    tests encode it, build implements it, review checks conventions,
    harden adds input validation and auth checks.

  A React frontend — traditional
    UI components are hard to TDD meaningfully. Plan defines the UX,
    build implements it, review checks accessibility and patterns,
    harden adds error boundaries and XSS prevention.

  Infrastructure/CI config — review-only
    Terraform modules, Dockerfiles, CI pipelines. No build step —
    just review what's there for correctness and security.

  CLI tool — tdd for core, traditional for scaffolding
    Use tdd for the command parsing and business logic components,
    traditional for the output formatting and help text components.

Which preset fits your project? (You can assign different presets to different
components in the next step.)
```

Options: `"Understood — let's assign presets to components (Recommended)"`, `"I have questions about phases"`, `"Skip phase discussion"`

If the user has questions, answer them before proceeding. The goal is informed consent — the user should understand what they're opting into.

**6b. Components** — present the proposed components with scope globs and workflow presets. Now that the user understands phases, propose which preset fits each component and why. Use `AskUserQuestion`:
- Question: "[component list with scopes and recommended workflows, with brief rationale for each preset choice]. Do these groupings make sense?"
- Options: `"Looks good (Recommended)"`, `"Modify"`, `"Add/remove components"`

**6c. Pairs — discuss each one.** For each proposed pair, use `AskUserQuestion` to validate:
- What quality dimension does this pair focus on?
- What should the adversarial specifically look for? Ask the user — they know their domain. E.g., "For the file-watching pair, what edge cases matter most? Lock files? Rapid successive writes? Symlinks?"
- What validation commands should the adversarial run? Suggest based on the stack but ask if there are others.
- Is the phase assignment right? Explain why you chose it and let the user adjust.

Don't present all pairs at once for rubber-stamping. Walk through them — the user's input here directly shapes the agent prompts, which is the most important output of init.

**Ecosystem-inspired pairs:** After discussing the initial pairs, consider whether ecosystem projects suggest additional quality dimensions the user hasn't thought of. Draw from Impeccable's design expertise (information hierarchy, glanceability, accessibility) for frontend pairs and Agency Agents' specialist personas (security, performance, observability) for domain-specific pairs. Present these as suggestions with the inspiration source explained — e.g., "Drawing from Impeccable's design principles, a dashboard-ux pair could evaluate whether status information is glanceable and color-coded effectively." Let the user decide whether to add them.

**6d. Guards — mirror CI and add what's missing.** Use `AskUserQuestion`:
- **Always include the built-in `no-generated-files` guard**: This framework guard prevents agents from committing build artifacts (generated Go code, node_modules, compiled CSS, protobuf stubs, etc.) that are derived from source code. It reads `project.yaml` to infer stack-specific patterns and supports project-specific extensions via `generated_file_patterns` in `workflow.yaml`. Always register it:
  ```yaml
  - name: no-generated-files
    command: "bash scripts/check-generated-files.sh"
    phase: review
    blocking: true
    timing: pre-debate
    components: []  # all components
  ```
  Present this as: "I'll include the built-in generated-files guard — it prevents committing build artifacts like [stack-specific examples based on project.yaml, e.g., '*_templ.go files' for Go, 'node_modules/' for Node]."
- **Start from CI**: For each quality gate command discovered in CI/CD pipelines during the codebase scan (Step 2), propose a matching guard. The goal is that every check CI runs should have a corresponding Ratchet guard so debates never produce code that will fail the pipeline. Present these as: "I found these checks in your CI pipeline — I'll mirror them as guards:"
  - Map CI steps to guard properties: lint/format commands → `timing: pre-debate`, `phase: build`, `blocking: true`; test commands → `timing: post-debate`, `phase: build`, `blocking: true`; security scans → `timing: post-debate`, `phase: harden`, `blocking: true`; type checks → `timing: pre-debate`, `phase: build`, `blocking: true`
- **Then suggest additions**: Based on the stack, suggest guards for checks that CI *doesn't* run but should (e.g., "Your CI doesn't run a security scanner — want to add one as an advisory guard?")
- For each guard, confirm: blocking or advisory? Which phase? Which components? What timing?
- Options: `"These guards are good (Recommended)"`, `"Add more"`, `"Modify"`, `"Skip guards for now"`

**6e. Progress tracking:**
- Question: "How do you want to track milestone progress?"
- Options: `"None (just local)"`, `"Markdown files in .ratchet/progress/"`, `"GitHub Issues (requires gh CLI)"`, `"Other / configure later"`

**6f. Debate publishing (only when adapter is `github-issues`):** After the user selects `"GitHub Issues"` in Step 6e, immediately ask a follow-up using `AskUserQuestion`:
- Question: "Publish debate rounds as GitHub issue comments?"
- Options:
  - `"Yes — post a summary when debates conclude (Recommended)"` — sets `publish_debates: summary`
  - `"Yes — post each round as a comment (per-round)"` — sets `publish_debates: per-round`
  - `"No — debates stay local only"` — sets `publish_debates: false`

Skip this step entirely if the adapter selected in Step 6e is anything other than `github-issues` (i.e., `none`, `markdown`, `linear`, or `jira`).

**6g. Final review** — only after walking through each area, present the complete config for approval:
- Question: "[full formatted config]. Everything look right?"
- Options: `"Approve (Recommended)"`, `"Modify [section]"`, `"Start over"`

Wait for approval before proceeding.

### Step 7: Build Epic

Based on everything learned, propose a development roadmap:
- **For greenfield projects, Milestone 1 is always "Workflow Validation"** — a minimal vertical slice that proves the Ratchet pipeline works end-to-end. Pick the simplest possible feature that exercises all configured pairs and guards. The acceptance criteria should focus on the workflow functioning correctly (debates reach consensus, guards pass, phases gate properly), not on feature completeness. Real project work starts at Milestone 2.
- Break remaining milestones by dependency and priority
- **Every milestone must have at least one issue.** Decompose milestones into issues that are independently executable and parallelizable. A simple milestone has one issue that IS the milestone. Complex milestones have 2-5 issues.
- Each issue has: ref, title, which pairs are relevant, dependencies on other issues
- Mark dependencies with `depends_on` — dependent issues wait for their dependencies to complete, then branch from the dependency's branch
- Present the epic to the human using `AskUserQuestion` for approval:
  - Question: "Proposed roadmap: [formatted milestone list with issues]. Approve this epic?"
  - Options: `"Approve (Recommended)"`, `"Modify milestones"`, `"Start over"`
- The epic is a living document — it evolves as the project develops

plan.yaml format:
```yaml
epic:
  name: "<project name>"
  description: "<one-line description>"
  progress_ref: null   # set after init when github-issues adapter creates the tracking issue
  milestones:
    - id: 1
      name: "<milestone name>"
      description: "<what this milestone delivers>"
      status: pending        # pending | in_progress | done
      done_when: "<concrete acceptance criteria>"
      depends_on: []         # milestone IDs this depends on (empty = Layer 0, runs in parallel with other Layer 0 milestones)
      progress_ref: null     # set by progress adapter when milestone starts
      regressions: 0         # regression counter for budget tracking
      issues:                # required — at least 1 issue per milestone
        - ref: "<issue reference, e.g. issue-1 or #480>"
          title: "<issue title>"
          pairs: [<pairs relevant to this issue>]
          depends_on: []     # refs of issues this depends on (within same milestone)
          phase_status:      # per-issue phase tracking
            plan: pending    # pending | in_progress | done
            test: pending
            build: pending
            review: pending
            harden: pending
          files: []          # populated during debates — files changed for this issue
          debates: []        # populated during debates — debate IDs for this issue
          branch: null       # git branch for this issue's worktree
          pr: null           # populated when PR is created — full PR URL
          progress_ref: null # populated by adapter — e.g., GitHub issue number for this work item
          status: pending    # pending | in_progress | done | blocked
  current_focus: null
  discoveries: []    # sidequests — each entry follows this schema:
                     # ref: "discovery-<type>-<timestamp>"   unique ID
                     # title: "<short description>"
                     # description: "<full context and action needed>"
                     # source: "<origin identifier>"         e.g. "pr-conflict-20"
                     # created_at: "<ISO timestamp>"
                     # severity: "high|medium|low"
                     # status: "pending|done"               set to done after /ratchet:run processes it
                     # issue_ref: "<issue-ref>"             direct ref to affected issue
                     # affected_scope: "<human description>"
                     # retro_type: null|"ci-failure"|"skipped-finding"
```

**Every milestone must have at least one issue.** A simple milestone with a single coherent deliverable has one issue that IS the milestone. This unifies the execution model — there are no special cases. Phase tracking lives on issues, not milestones. Milestone status is derived: `pending` (no issues started), `in_progress` (any issue started), `done` (all issues done).

**Parallel execution.** Independent issues (no `depends_on` relationships) run their full phase pipelines in parallel, each in an isolated git worktree. Each issue produces its own PR when complete. This means a milestone with 3 independent issues launches 3 parallel pipelines, each progressing through plan → test → build → review → harden independently.

**Dependencies.** When an issue has `depends_on: ["issue-A"]`, it waits until issue-A reaches `done` before starting. The dependent issue's worktree branches from issue-A's branch (not main), so it builds on top of the dependency's changes. The PR body states "Depends on [issue-A PR] being merged first."

**Issue decomposition guidance for the analyst.** When building the epic, decompose milestones into issues that are:
- Small enough to be independently reviewable (one PR each)
- Parallelizable where possible (minimize `depends_on`)
- Scoped to specific pairs (each issue lists which pairs are relevant)

For a simple milestone, create a single issue with the same name and description. For complex milestones, break into 2-5 issues. Never create issues so fine-grained that each one is a single file change — that defeats the purpose of structured debate.

**Milestone parallelism.** Milestones can declare `depends_on: [milestone-id]` to express inter-milestone dependencies. Milestones with no dependencies (or `depends_on: []`) are Layer 0 and run in parallel. Milestones whose dependencies are all complete become ready and run in the next batch. If no milestone has `depends_on`, milestones run sequentially (backward compatible — the default).

When decomposing the epic, consider which milestones are truly independent. For example, "Auth System" and "Data Layer" can often run in parallel, while "API Integration" depends on both. Use milestone dependencies to express this — the orchestrator will parallelize automatically.

If a progress adapter is configured, issues are populated during init by querying the tracker. For `github-issues`, the analyst can import existing issues that match the milestone's scope. Issues can also be added manually.

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
escalation: human  # human | tiebreaker | both

progress:
  adapter: none  # none | markdown | github-issues | linear | jira
  # publish_debates: false  # Only valid when adapter is github-issues.
  #   false (default) — debates stay local
  #   per-round       — post each debate round as a GitHub issue comment
  #   summary         — post a summary comment when the debate concludes
  # WARNING: If publish_debates is non-false and adapter is not github-issues,
  # ratchet:run will emit a warning and treat it as false.

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

**`publish_debates` field generation rule:** When writing `.ratchet/workflow.yaml`, apply this logic:
- If `progress.adapter` is `github-issues` AND the user selected `per-round` or `summary` in Step 6f: write `publish_debates: per-round` or `publish_debates: summary` (respectively) as a field under `progress`.
- Otherwise: omit `publish_debates` entirely (absence equals the default of `false`).

Example when user chose `per-round`:
```yaml
progress:
  adapter: github-issues
  publish_debates: per-round
```

Example when user chose `none` or any adapter other than `github-issues`:
```yaml
progress:
  adapter: none
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
├── scores/
├── retros/
├── escalations/
├── guards/
├── reports/
└── progress/
```

**Gitignore**: If the project is a git repo, append the following to `.gitignore` (create it if it doesn't exist). These are runtime artifacts — the tracked pair definitions, workflow config, and plan are the source of truth.

```
# Ratchet runtime artifacts (regenerable, environment-specific)
.ratchet/plan.yaml
.ratchet/debates/
.ratchet/reviews/
.ratchet/scores/
.ratchet/retros/
.ratchet/escalations/
.ratchet/guards/
.ratchet/reports/
.ratchet/progress/
.ratchet/worktrees/
.ratchet/locks/
.ratchet/archive/
.ratchet/issues/
```

**Error handling for file generation (Step 8)**: If any file write fails during generation:
```bash
# Verify each critical file was created
for f in .ratchet/project.yaml .ratchet/workflow.yaml .ratchet/plan.yaml; do
  test -f "$f" || { echo "Error: Failed to create $f" >&2; exit 1; }
done
```
If a write fails mid-generation, inform the user which files were created successfully and which failed. Do NOT leave a partially-generated `.ratchet/` directory without warning — the user must know the state is incomplete.

**GitHub Plan Tracking Issue (Step 8 — after all files written)**: If the `github-issues` adapter was selected in Step 6e, create the plan tracking issue immediately after generating `.ratchet/plan.yaml`:
```bash
if [ -f .claude/ratchet-scripts/progress/github-issues/create-plan-issue.sh ]; then
  tracking_issue_number=$(bash .claude/ratchet-scripts/progress/github-issues/create-plan-issue.sh \
    || echo "")
  if [ -n "$tracking_issue_number" ]; then
    # Store the tracking issue number as epic.progress_ref in plan.yaml
    yq eval -i ".epic.progress_ref = \"$tracking_issue_number\"" .ratchet/plan.yaml
    echo "Plan tracking issue created: #${tracking_issue_number}"
  else
    echo "Warning: Failed to create plan tracking issue (non-blocking). You can create it manually later." >&2
  fi
else
  echo "Note: create-plan-issue.sh not found — plan tracking issue not created. Install Ratchet scripts to enable this feature." >&2
fi
```
The tracking issue body mirrors `plan.yaml` as human-readable markdown with HTML comment metadata for deterministic recovery (see the "GitHub Plan Tracking Issue" section in `skills/run/SKILL.md` for the canonical format). The returned issue number is stored as `epic.progress_ref` in `plan.yaml` so the sync helper can update the correct issue on subsequent runs.

IMPORTANT:
- If code exists, scan it FIRST — never ask what you can read
- For existing projects, the interview focuses on what the human wants to improve
- For greenfield projects, the interview discovers intent, then you suggest stack and methodology
- Generated agent pair definitions must contain PROJECT-SPECIFIC knowledge (not generic templates)
- Generative agents get tools: Read, Grep, Glob, Bash, Write, Edit
- Adversarial agents get tools: Read, Grep, Glob, Bash with disallowedTools: Write, Edit
- Adversarial agents must know the exact validation commands available in this project
- Scope each pair to specific file globs — tight scope leads to deep analysis
- **Guilty until proven innocent**: Both generative and adversarial agent prompts MUST encode the principle that test failures on a PR branch are caused by the PR unless definitively proven otherwise. Generative agents must fix failures, not dismiss them. Adversarial agents must reject dismissals that lack evidence of the failure existing on master.
- **`publish_debates` runtime warning**: When `/ratchet:run` processes a workflow where `publish_debates` is `per-round` or `summary` but `progress.adapter` is NOT `github-issues`, it MUST emit a warning to stderr and treat `publish_debates` as `false`. This is a misconfiguration — `publish_debates` only applies to the `github-issues` adapter. The init skill prevents this by only asking the question when the adapter is `github-issues`, but the warning provides a safety net for manually edited configs.

### Step 9: Verify Output

After generation, verify:
- `.ratchet/project.yaml` exists and contains valid stack/testing info
- `.ratchet/workflow.yaml` exists with `version: 2` and at least one pair registered
- `.gitignore` contains the Ratchet runtime artifact entries (if git repo), including `.ratchet/plan.yaml`
- Each registered pair has both `generative.md` and `adversarial.md` in `.ratchet/pairs/`
- All directories created: `debates/`, `reviews/`, `scores/`

### Step 9b: GitHub Repository Settings (git repos with `gh` CLI available)

After verifying output, check whether the repository has recommended settings for a Ratchet workflow. Only run this step if: (1) the project is a git repo, (2) `gh` CLI is available, and (3) the repo has a GitHub remote.

**Authentication check**: Before any `gh` commands, verify authentication:
```bash
if ! gh auth status >/dev/null 2>&1; then
  echo "Warning: GitHub CLI is not authenticated. Skipping repository settings check." >&2
  echo "Run 'gh auth login' to enable GitHub integration features." >&2
  # Skip this entire step — do not proceed to Detection
fi
```
If the auth check fails, skip Step 9b entirely and proceed to Step 10. Do not prompt the user to authenticate here — init can complete without GitHub settings. The warning is informational.

**Detection**: Query current settings:
```bash
gh repo view --json deleteBranchOnMerge,mergeCommitAllowed,squashMergeAllowed,rebaseMergeAllowed \
  2>/dev/null || echo "SKIP"
```

If the query fails (no remote, permission issues), skip this step silently.

**Issues enabled check**: If the user selected the `github-issues` progress adapter in Step 6e, also query `hasIssuesEnabled` and warn if issues are disabled on the repo — the adapter requires GitHub Issues to be enabled. Add `hasIssuesEnabled` to the `--json` fields list in that case only.

**Evaluate settings** and branch based on whether changes are needed:

If all settings already match recommendations, print an informational message ("GitHub repo settings already match Ratchet recommendations") and skip to the next step — no `AskUserQuestion` needed since there is no decision.

If any settings need changing, **present findings** via `AskUserQuestion`. Build the question text by listing each setting and its current vs recommended state:

```
GitHub repo settings review:

Ratchet creates worktree branches and PRs per issue. These settings
keep the repo clean and complement the debate workflow:

  Auto-delete head branches:  [ON/OFF — Ratchet branches pile up without this]
  Squash merge:               [ON/OFF — keeps main history clean, one commit per issue]
  Merge commits:              [ON/OFF — recommend OFF when squash is on]
  Rebase merge:               [ON/OFF — recommend OFF when squash is on]

Want me to apply the recommended settings?
```

Options:
- `"Apply recommended settings (Recommended)"`
- `"Skip — I'll configure manually"`

**Apply** (if user approves): Use `gh repo edit` flags:
```bash
gh repo edit \
  --delete-branch-on-merge \
  --enable-squash-merge \
  --enable-merge-commit=false \
  --enable-rebase-merge=false \
  || echo "Warning: could not update repo settings. Configure manually in Settings > General." >&2
```

Only include flags for settings that need changing — don't re-apply settings already correct. Note: changing repository settings requires write access. If the command fails with a permission error, suggest the user ask a repo admin or configure manually in Settings > General.

**Branch protection** is not applied automatically (requires admin permissions and more complex configuration). If the default branch has no protection rules, append an advisory note:

```
Note: Consider adding branch protection to your default branch:
  Settings > Branches > Add rule > [branch name]
  Recommended: Require PR, require status checks, no force push
```

This is informational only — do not attempt to create branch protection rules via the API (it requires admin scope and varies by plan).

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
  - "Start first debate (/ratchet:run) (Recommended)" — begin the epic workflow
  - "Add more pairs (/ratchet:pair)"
  - "Done for now"
