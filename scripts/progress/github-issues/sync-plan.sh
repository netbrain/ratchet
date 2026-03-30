#!/usr/bin/env bash
# GitHub Issues progress adapter — sync-plan
# Syncs plan.yaml state to the GitHub tracking issue body.
# Usage:
#   sync-plan.sh [<plan-yaml-path>]           — render plan.yaml and update tracking issue
#   sync-plan.sh --recover [<plan-yaml-path>] — parse tracking issue body back to plan.yaml
# Requires: gh CLI authenticated, yq installed
#
# Non-blocking: exits 0 on any failure (logs warning to stderr).
set -euo pipefail

command -v gh >/dev/null 2>&1 || { echo "Warning: gh CLI is required but not found — skipping sync" >&2; exit 0; }
command -v yq >/dev/null 2>&1 || { echo "Warning: yq is required but not found — skipping sync" >&2; exit 0; }

RECOVER=false
PLAN_YAML=".ratchet/plan.yaml"

# Parse arguments
while [ "$#" -gt 0 ]; do
    case "$1" in
        --recover)
            RECOVER=true
            shift
            ;;
        -*)
            echo "Warning: Unknown flag $1 — skipping sync" >&2
            exit 0
            ;;
        *)
            PLAN_YAML="$1"
            shift
            ;;
    esac
done

if [ ! -f "$PLAN_YAML" ]; then
    echo "Warning: plan.yaml not found at $PLAN_YAML — skipping sync" >&2
    exit 0
fi

# Validate YAML parses
yq eval '.' "$PLAN_YAML" >/dev/null 2>&1 || {
    echo "Warning: $PLAN_YAML is not valid YAML — skipping sync" >&2
    exit 0
}

# Read the tracking issue number from plan.yaml (epic.progress_ref)
PROGRESS_REF=$(yq eval '.epic.progress_ref // ""' "$PLAN_YAML")
if [ -z "$PROGRESS_REF" ] || [ "$PROGRESS_REF" = "null" ]; then
    echo "Warning: epic.progress_ref not set in $PLAN_YAML — skipping sync" >&2
    exit 0
fi

# ---------------------------------------------------------------------------
# --recover mode: parse tracking issue body back to plan.yaml structure
# ---------------------------------------------------------------------------
if [ "$RECOVER" = "true" ]; then
    # Fetch the tracking issue body
    ISSUE_BODY=$(gh issue view "$PROGRESS_REF" --json body --jq '.body' 2>/dev/null) || {
        echo "Warning: Could not fetch issue $PROGRESS_REF — skipping recover" >&2
        exit 0
    }

    # Verify it is a ratchet tracking issue
    if ! printf '%s' "$ISSUE_BODY" | grep -q '<!-- ratchet-plan-tracking -->'; then
        echo "Warning: Issue $PROGRESS_REF does not appear to be a ratchet tracking issue — skipping recover" >&2
        exit 0
    fi

    # Parse epic metadata from HTML comments using sed (portable, no -P needed)
    EPIC_NAME=$(printf '%s' "$ISSUE_BODY" | sed -n 's/<!-- epic_name: \(.*\) -->/\1/p' | head -1) || true
    EPIC_DESC=$(printf '%s' "$ISSUE_BODY" | sed -n 's/<!-- epic_description: \(.*\) -->/\1/p' | head -1) || true

    # Build recovered YAML into a temp file
    TMP_YAML=$(mktemp)
    trap 'rm -f "$TMP_YAML"' EXIT

    {
        printf 'epic:\n'
        printf '  name: "%s"\n' "$EPIC_NAME"
        printf '  description: "%s"\n' "$EPIC_DESC"
        printf '  progress_ref: "%s"\n' "$PROGRESS_REF"
        printf '  milestones:\n'

        # Parse each milestone block from the issue body
        ms_id=""
        ms_name=""
        ms_status="pending"
        ms_done_when=""
        ms_depends_on="[]"
        in_ms=false

        while IFS= read -r line; do
            case "$line" in
                "## Milestone "*)
                    # Flush previous milestone before starting new one
                    if [ "$in_ms" = "true" ] && [ -n "$ms_id" ]; then
                        printf '    - id: %s\n' "$ms_id"
                        printf '      name: "%s"\n' "$ms_name"
                        printf '      status: %s\n' "$ms_status"
                        printf '      done_when: "%s"\n' "$ms_done_when"
                        printf '      depends_on: %s\n' "$ms_depends_on"
                        printf '      issues: []\n'
                    fi
                    in_ms=true
                    # Extract name: "## Milestone N: Name" — strip prefix up to ": "
                    ms_name=$(printf '%s' "$line" | sed 's/^## Milestone [^:]*: //')
                    ms_id=""
                    ms_status="pending"
                    ms_done_when=""
                    ms_depends_on="[]"
                    ;;
                "<!-- milestone_id: "*)
                    ms_id=$(printf '%s' "$line" | sed 's/<!-- milestone_id: \(.*\) -->/\1/')
                    ;;
                "<!-- milestone_status: "*)
                    ms_status=$(printf '%s' "$line" | sed 's/<!-- milestone_status: \(.*\) -->/\1/')
                    ;;
                "<!-- milestone_done_when: "*)
                    ms_done_when=$(printf '%s' "$line" | sed 's/<!-- milestone_done_when: \(.*\) -->/\1/')
                    ;;
                "<!-- milestone_depends_on: "*)
                    ms_depends_on=$(printf '%s' "$line" | sed 's/<!-- milestone_depends_on: \(.*\) -->/\1/')
                    ;;
            esac
        done <<EOF
$ISSUE_BODY
EOF

        # Flush last milestone
        if [ "$in_ms" = "true" ] && [ -n "$ms_id" ]; then
            printf '    - id: %s\n' "$ms_id"
            printf '      name: "%s"\n' "$ms_name"
            printf '      status: %s\n' "$ms_status"
            printf '      done_when: "%s"\n' "$ms_done_when"
            printf '      depends_on: %s\n' "$ms_depends_on"
            printf '      issues: []\n'
        fi
    } > "$TMP_YAML"

    # Validate the generated YAML
    yq eval '.' "$TMP_YAML" >/dev/null 2>&1 || {
        echo "Warning: Recovered YAML is invalid — skipping recover write" >&2
        exit 0
    }

    # Atomically write back to plan.yaml
    mv "$TMP_YAML" "$PLAN_YAML"
    echo "Recovered plan.yaml from tracking issue $PROGRESS_REF"
    exit 0
fi

# ---------------------------------------------------------------------------
# Normal sync mode: render plan.yaml and update tracking issue body
# ---------------------------------------------------------------------------
EPIC_NAME=$(yq eval '.epic.name // ""' "$PLAN_YAML")
EPIC_DESCRIPTION=$(yq eval '.epic.description // ""' "$PLAN_YAML")

# Compact JSON helper — yq's tojson can output pretty-printed multi-line JSON
# which breaks HTML comments on GitHub (only single-line comments are hidden).
# This collapses any multi-line JSON to a single line.
compact_json() {
    tr -d '\n' | sed 's/  */ /g'
}

render_body() {
    local plan="$1"

    # --- Hidden metadata (machine-readable, invisible on GitHub) ---
    printf '<!-- ratchet-plan-tracking -->\n'
    printf '<!-- epic_name: %s -->\n' "$EPIC_NAME"
    printf '<!-- epic_description: %s -->\n' "$EPIC_DESCRIPTION"
    printf '\n'

    # --- Human-readable roadmap ---
    printf '# %s\n\n' "$EPIC_NAME"
    printf '%s\n\n' "$EPIC_DESCRIPTION"

    # Epic progress summary
    local milestone_count done_count
    milestone_count=$(yq eval '.epic.milestones | length' "$plan")
    done_count=$(yq eval '[.epic.milestones[] | select(.status == "done")] | length' "$plan")
    printf '**Progress:** %s/%s milestones complete\n\n' "$done_count" "$milestone_count"
    printf '%s\n' '---'

    local i=0
    while [ "$i" -lt "$milestone_count" ]; do
        local ms_id ms_name ms_status ms_done_when ms_depends_on ms_desc ms_github_issue
        ms_id=$(yq eval ".epic.milestones[$i].id" "$plan")
        ms_name=$(yq eval ".epic.milestones[$i].name" "$plan")
        ms_status=$(yq eval ".epic.milestones[$i].status // \"pending\"" "$plan")
        ms_done_when=$(yq eval ".epic.milestones[$i].done_when // \"\"" "$plan")
        ms_depends_on=$(yq eval ".epic.milestones[$i].depends_on // [] | tojson" "$plan" | compact_json)
        ms_desc=$(yq eval ".epic.milestones[$i].description // \"\"" "$plan")
        ms_github_issue=$(yq eval ".epic.milestones[$i].github_issue // null" "$plan")

        # Milestone status badge
        local ms_badge
        case "$ms_status" in
            done)        ms_badge="complete" ;;
            in_progress) ms_badge="in progress" ;;
            *)           ms_badge="pending" ;;
        esac

        printf '\n## Milestone %s: %s\n' "$ms_id" "$ms_name"

        # Hidden metadata
        printf '<!-- milestone_id: %s -->\n' "$ms_id"
        printf '<!-- milestone_status: %s -->\n' "$ms_status"
        printf '<!-- milestone_done_when: %s -->\n' "$ms_done_when"
        printf '<!-- milestone_depends_on: %s -->\n' "$ms_depends_on"
        printf '\n'

        # Human-readable description and status
        if [ -n "$ms_desc" ] && [ "$ms_desc" != "null" ]; then
            printf '%s\n\n' "$ms_desc"
        fi
        printf '**Status:** %s' "$ms_badge"
        if [ "$ms_github_issue" != "null" ] && [ -n "$ms_github_issue" ]; then
            printf ' · **Tracking:** #%s' "$ms_github_issue"
        fi
        printf '\n\n'

        local issue_count
        issue_count=$(yq eval ".epic.milestones[$i].issues | length" "$plan")

        local j=0
        while [ "$j" -lt "$issue_count" ]; do
            local iss_ref iss_title iss_status iss_pairs iss_depends_on
            local iss_phase_status iss_branch iss_pr iss_checkbox iss_detail
            iss_ref=$(yq eval ".epic.milestones[$i].issues[$j].ref // \"\"" "$plan")
            iss_title=$(yq eval ".epic.milestones[$i].issues[$j].title // \"\"" "$plan")
            iss_status=$(yq eval ".epic.milestones[$i].issues[$j].status // \"pending\"" "$plan")
            iss_pairs=$(yq eval ".epic.milestones[$i].issues[$j].pairs // [] | tojson" "$plan" | compact_json)
            iss_depends_on=$(yq eval ".epic.milestones[$i].issues[$j].depends_on // [] | tojson" "$plan" | compact_json)
            iss_phase_status=$(yq eval ".epic.milestones[$i].issues[$j].phase_status // {} | tojson" "$plan" | compact_json)
            iss_branch=$(yq eval ".epic.milestones[$i].issues[$j].branch // null" "$plan")
            iss_pr=$(yq eval ".epic.milestones[$i].issues[$j].pr // null" "$plan")

            if [ "$iss_status" = "done" ]; then
                iss_checkbox="x"
            else
                iss_checkbox=" "
            fi

            # Build detail suffix: PR link
            iss_detail=""
            if [ "$iss_pr" != "null" ] && [ -n "$iss_pr" ]; then
                case "$iss_pr" in
                    http*) iss_detail=" — [PR]($iss_pr)" ;;
                    *)     iss_detail=" — PR #$iss_pr" ;;
                esac
            fi

            # Render ref: numeric refs are GitHub issue links (#N), local refs shown as-is
            local iss_ref_display="$iss_ref"
            case "$iss_ref" in
                [0-9]*) iss_ref_display="#$iss_ref" ;;
            esac

            printf -- '- [%s] %s: %s%s\n' "$iss_checkbox" "$iss_ref_display" "$iss_title" "$iss_detail"

            # Hidden metadata (all on single lines for GitHub to hide)
            printf '<!-- issue_ref: %s -->\n' "$iss_ref"
            printf '<!-- issue_status: %s -->\n' "$iss_status"
            printf '<!-- issue_pairs: %s -->\n' "$iss_pairs"
            printf '<!-- issue_depends_on: %s -->\n' "$iss_depends_on"
            printf '<!-- issue_phase_status: %s -->\n' "$iss_phase_status"
            printf '<!-- issue_branch: %s -->\n' "$iss_branch"
            printf '<!-- issue_pr: %s -->\n' "$iss_pr"
            printf '\n'

            j=$((j + 1))
        done

        i=$((i + 1))
    done
}

# Write body to temp file for atomic gh issue edit (avoids shell quoting issues with large bodies)
TMP_BODY=$(mktemp)
trap 'rm -f "$TMP_BODY"' EXIT
render_body "$PLAN_YAML" > "$TMP_BODY"

gh issue edit "$PROGRESS_REF" --body-file "$TMP_BODY" >/dev/null 2>&1 || {
    echo "Warning: Failed to update tracking issue $PROGRESS_REF — continuing" >&2
    exit 0
}

echo "Synced plan.yaml to tracking issue $PROGRESS_REF"
