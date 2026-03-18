#!/usr/bin/env bash
# GitHub Issues progress adapter — create-plan-issue
# Creates a GitHub tracking issue for a Ratchet plan (plan.yaml).
# Usage: create-plan-issue.sh <epic-name> <plan-yaml-path>
# Outputs: issue number (stdout)
# Requires: gh CLI authenticated, yq installed
set -euo pipefail

command -v gh >/dev/null 2>&1 || { echo "Error: gh CLI is required but not found" >&2; exit 1; }
command -v yq >/dev/null 2>&1 || { echo "Error: yq is required but not found" >&2; exit 1; }

EPIC_NAME="${1:?Usage: create-plan-issue.sh <epic-name> <plan-yaml-path>}"
PLAN_YAML="${2:?Usage: create-plan-issue.sh <epic-name> <plan-yaml-path>}"

if [ ! -f "$PLAN_YAML" ]; then
    echo "Error: plan.yaml not found at $PLAN_YAML" >&2
    exit 1
fi

# Validate YAML parses
yq eval '.' "$PLAN_YAML" >/dev/null 2>&1 || { echo "Error: $PLAN_YAML is not valid YAML" >&2; exit 1; }

EPIC_DESCRIPTION=$(yq eval '.epic.description // ""' "$PLAN_YAML")

# Render the plan as the canonical tracking issue body
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

# Write body to temp file to avoid ARG_MAX limits and shell quoting issues with large bodies
TMP_BODY=$(mktemp)
trap 'rm -f "$TMP_BODY"' EXIT
printf '%s' "$BODY" > "$TMP_BODY"

ISSUE_URL=$(gh issue create \
    --title "Ratchet Plan: ${EPIC_NAME}" \
    --body-file "$TMP_BODY" \
    --label "ratchet-plan" 2>&1) || {
    echo "Error: Failed to create GitHub issue: $ISSUE_URL" >&2
    exit 1
}

# Extract issue number from URL (https://github.com/owner/repo/issues/42 -> 42)
ISSUE_NUM=$(printf '%s' "$ISSUE_URL" | grep -oE '[0-9]+$') || true
if [ -z "$ISSUE_NUM" ]; then
    echo "Error: Could not extract issue number from gh output: $ISSUE_URL" >&2
    exit 1
fi

echo "$ISSUE_NUM"
