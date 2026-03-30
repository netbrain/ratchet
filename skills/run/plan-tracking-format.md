# GitHub Plan Tracking Issue Format

> This file is the extracted plan tracking issue format spec from `skills/run/SKILL.md`.
> It is loaded on demand when the orchestrator interacts with the GitHub plan tracking issue.
> For the orchestrator flow, see `skills/run/SKILL.md`.

---

## GitHub Plan Tracking Issue

When the `github-issues` progress adapter is enabled, a single GitHub issue mirrors
`plan.yaml` as a human-readable roadmap. This tracking issue serves as a backup for
`plan.yaml` and enables deterministic recovery without LLM assistance.

**Canonical body format:**

The tracking issue body has two layers: human-readable markdown (visible on GitHub) and hidden HTML comment metadata (for machine parsing). All HTML comments MUST be single-line — multi-line comments render as visible text on GitHub.

```markdown
<!-- ratchet-plan-tracking -->
<!-- epic_name: My Project -->
<!-- epic_description: Build the core API and frontend -->

# My Project

Build the core API and frontend

**Progress:** 1/2 milestones complete

---

## Milestone 1: Foundation
<!-- milestone_id: 1 -->
<!-- milestone_status: done -->
<!-- milestone_done_when: All core APIs passing tests -->
<!-- milestone_depends_on: [] -->

All core APIs passing tests

**Status:** complete

- [x] issue-1: Setup project scaffold — PR #42
<!-- issue_ref: issue-1 -->
<!-- issue_status: done -->
<!-- issue_pairs: ["code-quality"] -->
<!-- issue_depends_on: [] -->
<!-- issue_phase_status: {"plan":"done","test":"done","build":"done","review":"done","harden":"done"} -->
<!-- issue_branch: ratchet/foundation/issue-1 -->
<!-- issue_pr: 42 -->

## Milestone 2: Features
<!-- milestone_id: 2 -->
<!-- milestone_status: in_progress -->
<!-- milestone_done_when: Feature complete and reviewed -->
<!-- milestone_depends_on: [1] -->

Feature complete and reviewed

**Status:** in progress · **Tracking:** #165

- [ ] issue-2: Implement core feature
<!-- issue_ref: issue-2 -->
<!-- issue_status: pending -->
<!-- issue_pairs: ["code-quality","security"] -->
<!-- issue_depends_on: [] -->
<!-- issue_phase_status: {"plan":"pending","test":"pending","build":"pending","review":"pending","harden":"pending"} -->
<!-- issue_branch: null -->
<!-- issue_pr: null -->
```

**Critical rendering rule:** All JSON values inside HTML comments (`issue_pairs`, `issue_depends_on`, `issue_phase_status`, `milestone_depends_on`) MUST be compact single-line JSON. Multi-line JSON like `[\n  "foo"\n]` breaks GitHub's comment hiding and renders as visible text. The `sync-plan.sh` script uses `compact_json()` to enforce this.

**HTML comment metadata rules:**
- Every milestone block MUST have `milestone_id`, `milestone_status`, `milestone_done_when`, `milestone_depends_on`
- Every issue block MUST have `issue_ref`, `issue_status`, `issue_pairs`, `issue_depends_on`, `issue_phase_status`, `issue_branch`, `issue_pr`, `issue_progress_ref`
- The `<!-- ratchet-plan-tracking -->` sentinel on line 1 identifies the issue for deterministic parsing
- Checkbox state (`[x]` vs `[ ]`) reflects `issue_status == "done"` but is NOT the parse source — `issue_status` in the HTML comment is authoritative
- Fields NOT stored (not recoverable from tracking issue): `files`, `debates` arrays — these are runtime artifacts

**Sync helper — existence-guarded call pattern:**
```bash
if [ -f .claude/ratchet-scripts/progress/github-issues/sync-plan.sh ]; then
  bash .claude/ratchet-scripts/progress/github-issues/sync-plan.sh \
    || echo "Warning: plan tracking issue sync failed (non-blocking)" >&2
fi
```
Adapter failures NEVER block pipeline execution. Always wrap sync calls with `|| echo "Warning..."` or equivalent.
