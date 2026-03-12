---
name: ratchet:init
description: Analyze project and generate tailored agent pairs through codebase analysis and human interview
---

# /ratchet:init — Project Onboarding

Initialize Ratchet for the current project by analyzing the codebase and interviewing the human to generate tailored quality agent pairs.

## Prerequisites
- No existing `.ratchet/` directory (use `/ratchet:pair` to add pairs to an existing setup)

## Execution Steps

### Step 1: Check Prerequisites

Check if `.ratchet/` already exists. If so, inform the user and suggest `/ratchet:pair` instead.

### Step 2: Launch Analyst Agent

Spawn the **analyst** agent with the following task:

```
Generate a Ratchet configuration for this project. Follow this protocol:

1. CODEBASE SCAN FIRST — Before asking the human anything, silently scan the project:
   - Read: README.md, CLAUDE.md, go.mod, package.json, Cargo.toml, pyproject.toml, Makefile,
     flake.nix, docker-compose.yml, .github/workflows/*, any project plan files
   - Scan directory structure (ls key directories)
   - Identify: languages, frameworks, database, architecture patterns
   - Determine: existing test infrastructure, CI setup, exact test/lint/build commands
   - Look for: project plans, roadmaps, ADRs, design docs in the repo
   - DO NOT ask the human for information you can read from the codebase.

2. TARGETED INTERVIEW — Only ask about things you CANNOT infer from the code:
   - Skip questions about language, framework, stack, test commands — you already know these.
   - DO ask about:
     - "What are your biggest quality concerns?" (subjective, can't be inferred)
     - "What breaks most often or worries you most?" (experience-based)
     - "Any compliance or regulatory requirements?" (external constraints)
     - "Any areas where code review consistently catches issues?" (team knowledge)
   - If the codebase is empty/new with no manifests, THEN ask about intended stack and purpose.
   - Present what you learned from the scan as context: "I see this is a Go project using
     gorilla/mux with PostgreSQL. Your CI runs golangci-lint and Playwright E2E tests..."
   - Ask at most 2-3 focused questions, not a generic questionnaire.
   - Adapt follow-ups based on answers.

3. SYNTHESIZE — Combine codebase scan + interview answers to identify quality dimensions.

4. PROPOSE PAIRS — Present each proposed pair to the human with rationale. Wait for approval.

5. BUILD EPIC — Based on everything learned, propose a development roadmap:
   - Break the project into milestones (ordered by dependency and priority)
   - Each milestone has: name, description, which pairs are relevant, what "done" looks like
   - Present the epic to the human for approval using AskUserQuestion
   - The epic is a living document — it evolves as the project develops
   - Write to .ratchet/plan.yaml

   Example plan.yaml:
   ```yaml
   epic:
     name: "todoapp"
     description: "Go + htmx + templ todo application with SQLite"
     milestones:
       - id: 1
         name: "Project scaffold"
         description: "Go module, main entry point, basic server startup"
         pairs: [handler-quality]
         status: pending
         done_when: "Server starts, responds to health check"
       - id: 2
         name: "Data layer"
         description: "SQLite schema, models, repository with CRUD"
         pairs: [data-integrity]
         status: pending
         done_when: "All CRUD operations work with integration tests"
       - id: 3
         name: "Handlers + templates"
         description: "HTTP handlers wired to repo, templ views with htmx"
         pairs: [handler-quality, template-htmx]
         status: pending
         done_when: "Full UI flow: list, create, toggle, delete"
       - id: 4
         name: "Input hardening"
         description: "Validation, fuzz targets, edge case coverage"
         pairs: [fuzz-resilience]
         status: pending
         done_when: "All inputs validated, fuzz targets pass 30s runs"
     current_focus: null
   ```

6. GENERATE — For each approved pair, write:
   - .ratchet/project.yaml — project profile with stack, architecture, testing spec
   - .ratchet/plan.yaml — development roadmap with milestones
   - .ratchet/pairs/<name>/generative.md — builder agent
   - .ratchet/pairs/<name>/adversarial.md — critic agent
   - .ratchet/config.yaml — registers all approved pairs

Create the .ratchet/ directory structure:
  .ratchet/
  ├── project.yaml
  ├── config.yaml
  ├── plan.yaml
  ├── pairs/
  │   └── <pair-name>/
  │       ├── generative.md
  │       └── adversarial.md
  ├── debates/
  ├── reviews/
  └── scores/

IMPORTANT:
- The codebase scan is MANDATORY and comes FIRST — never ask what you can read
- The interview is for subjective/experience-based questions only
- For new/empty projects with no code or manifests, the interview covers stack and purpose too
- Generated agent pair definitions must contain PROJECT-SPECIFIC knowledge (not generic templates)
- Generative agents get tools: Read, Grep, Glob, Bash, Write, Edit
- Adversarial agents get tools: Read, Grep, Glob, Bash with disallowedTools: Write, Edit
- Adversarial agents must know the exact test/lint/benchmark commands from the testing spec
- Scope each pair to specific file globs — tight scope leads to deep analysis
- Include the project's architecture patterns, ORM, framework conventions in agent prompts
```

### Step 3: Verify Output

After the analyst completes, verify:
- `.ratchet/project.yaml` exists and contains valid stack/testing info
- `.ratchet/config.yaml` exists with at least one pair registered
- Each registered pair has both `generative.md` and `adversarial.md` in `.ratchet/pairs/`
- All directories created: `debates/`, `reviews/`, `scores/`

### Step 4: Report

Present a summary to the user:
```
Ratchet initialized for [project name]

Stack: [language] / [framework] / [database]
Architecture: [pattern]

Pairs created:
  ✓ [pair-name] — [scope] — [quality dimension]
  ✓ [pair-name] — [scope] — [quality dimension]
  ...

Run /ratchet:run to start a debate on your current changes.
Run /ratchet:pair [name] to add more pairs.
```
