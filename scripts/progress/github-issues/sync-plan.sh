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

render_body() {
    local plan="$1"
    local body

    body="$(printf '<!-- ratchet-plan-tracking -->\n')"
    body="${body}$(printf '<!-- epic_name: %s -->\n' "$EPIC_NAME")"
    body="${body}$(printf '<!-- epic_description: %s -->\n' "$EPIC_DESCRIPTION")"
    body="${body}$(printf '\n')"
    body="${body}$(printf '# %s — Ratchet Roadmap\n' "$EPIC_NAME")"

    local milestone_count
    milestone_count=$(yq eval '.epic.milestones | length' "$plan")

    local i=0
    while [ "$i" -lt "$milestone_count" ]; do
        local ms_id ms_name ms_status ms_done_when ms_depends_on
        ms_id=$(yq eval ".epic.milestones[$i].id" "$plan")
        ms_name=$(yq eval ".epic.milestones[$i].name" "$plan")
        ms_status=$(yq eval ".epic.milestones[$i].status // \"pending\"" "$plan")
        ms_done_when=$(yq eval ".epic.milestones[$i].done_when // \"\"" "$plan")
        ms_depends_on=$(yq eval ".epic.milestones[$i].depends_on // [] | tojson" "$plan")

        body="${body}$(printf '\n')"
        body="${body}$(printf '## Milestone %s: %s\n' "$ms_id" "$ms_name")"
        body="${body}$(printf '<!-- milestone_id: %s -->\n' "$ms_id")"
        body="${body}$(printf '<!-- milestone_status: %s -->\n' "$ms_status")"
        body="${body}$(printf '<!-- milestone_done_when: %s -->\n' "$ms_done_when")"
        body="${body}$(printf '<!-- milestone_depends_on: %s -->\n' "$ms_depends_on")"
        body="${body}$(printf '\n')"

        local issue_count
        issue_count=$(yq eval ".epic.milestones[$i].issues | length" "$plan")

        local j=0
        while [ "$j" -lt "$issue_count" ]; do
            local iss_ref iss_title iss_status iss_pairs iss_depends_on
            local iss_phase_status iss_branch iss_pr iss_checkbox
            iss_ref=$(yq eval ".epic.milestones[$i].issues[$j].ref // \"\"" "$plan")
            iss_title=$(yq eval ".epic.milestones[$i].issues[$j].title // \"\"" "$plan")
            iss_status=$(yq eval ".epic.milestones[$i].issues[$j].status // \"pending\"" "$plan")
            iss_pairs=$(yq eval ".epic.milestones[$i].issues[$j].pairs // [] | tojson" "$plan")
            iss_depends_on=$(yq eval ".epic.milestones[$i].issues[$j].depends_on // [] | tojson" "$plan")
            iss_phase_status=$(yq eval ".epic.milestones[$i].issues[$j].phase_status // {} | tojson" "$plan")
            iss_branch=$(yq eval ".epic.milestones[$i].issues[$j].branch // null" "$plan")
            iss_pr=$(yq eval ".epic.milestones[$i].issues[$j].pr // null" "$plan")

            if [ "$iss_status" = "done" ]; then
                iss_checkbox="x"
            else
                iss_checkbox=" "
            fi

            body="${body}$(printf -- '- [%s] %s: %s\n' "$iss_checkbox" "$iss_ref" "$iss_title")"
            body="${body}$(printf '<!-- issue_ref: %s -->\n' "$iss_ref")"
            body="${body}$(printf '<!-- issue_status: %s -->\n' "$iss_status")"
            body="${body}$(printf '<!-- issue_pairs: %s -->\n' "$iss_pairs")"
            body="${body}$(printf '<!-- issue_depends_on: %s -->\n' "$iss_depends_on")"
            body="${body}$(printf '<!-- issue_phase_status: %s -->\n' "$iss_phase_status")"
            body="${body}$(printf '<!-- issue_branch: %s -->\n' "$iss_branch")"
            body="${body}$(printf '<!-- issue_pr: %s -->\n' "$iss_pr")"
            body="${body}$(printf '\n')"

            j=$((j + 1))
        done

        i=$((i + 1))
    done

    printf '%s' "$body"
}

BODY=$(render_body "$PLAN_YAML")

# Write body to temp file for atomic gh issue edit (avoids shell quoting issues with large bodies)
TMP_BODY=$(mktemp)
trap 'rm -f "$TMP_BODY"' EXIT
printf '%s' "$BODY" > "$TMP_BODY"

gh issue edit "$PROGRESS_REF" --body-file "$TMP_BODY" >/dev/null 2>&1 || {
    echo "Warning: Failed to update tracking issue $PROGRESS_REF — continuing" >&2
    exit 0
}

echo "Synced plan.yaml to tracking issue $PROGRESS_REF"
