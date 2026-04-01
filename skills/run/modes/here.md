# --here Modifier (in-session execution)

`--here` is a **modifier**, not a mode. It modifies how the resolved mode executes
by keeping all work in the current session — no worktree isolation, no agent
spawning. The human is interactively present and serves as the quality gate.

**Restriction:** `--here` is valid ONLY in top-level human-interactive sessions.
Spawned agents (issue pipelines, debate-runners, continuation agents) MUST NOT
claim `--here`. If a spawned agent receives `--here`, it MUST ignore the flag
and execute normally.

**Forbidden combinations:**
- `--here --unsupervised` → FORBIDDEN. In-session execution requires human presence;
  unsupervised mode removes it. If both are passed, halt with error:
  `"--here and --unsupervised are mutually exclusive. --here requires human interaction."`
- `--here --go` → FORBIDDEN (since `--go` is shorthand for `--unsupervised --auto-pr`).
  Same error as above.

**Allowed combinations and behavior:**
- `--here --quick "<desc>"` → Follows Mode Q behavior (single generative pass,
  auto-commit). The orchestrator executes the generative work directly in the
  current session instead of spawning a generative agent. Auto-commits on
  completion (Mode Q behavior).
- `--here --auto-pr` → Auto-commit all changes and create a PR without prompting.
  No `AskUserQuestion` for the commit step.
- `--here --issue <ref>` → Execute the issue's pipeline in the current session
  on the current branch. Skip worktree creation. Read the issue's phase_status
  from plan.yaml and execute the current phase directly.
- `--here` (alone, no other mode flag) → Read plan.yaml normally (Step 1b),
  determine focus (Step 2), then execute directly in the current session.

**How `--here` modifies execution:**

1. **No worktree isolation**: Work happens on the current branch. No `isolation: "worktree"`
   on Agent tool calls (because no Agent tool calls are made).
2. **No agent spawning**: The orchestrator performs the generative work directly.
   The AGENT GATE is bypassed — the orchestrator MAY use Write and Edit on source
   files. The Source Code Boundary carve-out applies.
3. **Guards**: Run guards with `milestone-id=0` and `phase=build` when no
   milestone/phase context exists (e.g., `--here` alone without `--issue`).
   When context exists (e.g., `--here --issue <ref>`), use the actual
   milestone ID and current phase.
4. **Plan.yaml**: Read normally (Step 1b). Skip only with `--here --quick`
   (Mode Q skips plan.yaml regardless).
5. **Commit behavior**:
   - `--here --quick`: Auto-commit (Mode Q behavior).
   - `--here --auto-pr`: Auto-commit + auto-PR (no prompt).
   - `--here` alone: Prompt the user via `AskUserQuestion`:
     ```
     Changes complete. What would you like to do?
     ```
     Options: `"Commit changes (Recommended)"`, `"Create PR"`, `"Review changes first"`, `"Done — leave uncommitted"`
6. **Execution log**: Write with `mode: in-session`:
   ```bash
   EXEC_ID="in-session-$(date +%Y%m%dT%H%M%S)"
   mkdir -p .ratchet/executions
   cat > ".ratchet/executions/${EXEC_ID}.yaml" <<EOF
   id: "${EXEC_ID}"
   mode: in-session
   component: <detected-or-resolved-component>
   issue: "<issue-ref or null>"
   started: "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
   resolved: null
   guard_results: []
   files_modified: []
   EOF
   ```
