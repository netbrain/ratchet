#!/usr/bin/env bash
# Custom Claude Code statusline for Ratchet framework
# Displays epic progress when Ratchet is active, falls back to default otherwise

set -euo pipefail

# Read session JSON from stdin
input=$(cat)

# Extract base session data
model=$(echo "$input" | jq -r '.model.display_name // "Claude"')
cwd=$(echo "$input" | jq -r '.workspace.current_dir // .cwd // "~"')
ctx_pct=$(echo "$input" | jq -r '.context_window.used_percentage // 0' | cut -d. -f1)
vim_mode=$(echo "$input" | jq -r '.vim.mode // ""')

# Check if we're in a Ratchet workspace
ratchet_active=false
workspace_name=""
epic_name=""
milestone_progress=""
issue_progress=""

# Walk up from CWD looking for .ratchet/plan.yaml
check_dir="$cwd"
while [ "$check_dir" != "/" ]; do
  if [ -f "$check_dir/.ratchet/plan.yaml" ]; then
    ratchet_active=true
    plan_yaml="$check_dir/.ratchet/plan.yaml"

    # Extract epic info
    epic_name=$(yq eval '.epic.name // "ratchet"' "$plan_yaml")

    # Calculate milestone progress
    total_milestones=$(yq eval '.epic.milestones | length' "$plan_yaml")
    done_milestones=$(yq eval '[.epic.milestones[] | select(.status == "done")] | length' "$plan_yaml")
    milestone_progress="$done_milestones/$total_milestones M"

    # Find current milestone and issue progress
    current_milestone=$(yq eval '.epic.milestones[] | select(.status == "in_progress")' "$plan_yaml")
    if [ -n "$current_milestone" ]; then
      milestone_id=$(echo "$current_milestone" | yq eval '.id')
      milestone_name=$(echo "$current_milestone" | yq eval '.name')

      total_issues=$(echo "$current_milestone" | yq eval '.issues | length')
      done_issues=$(echo "$current_milestone" | yq eval '[.issues[] | select(.status == "done")] | length')
      issue_progress="$done_issues/$total_issues I"

      # Check for current focus
      current_focus=$(yq eval '.epic.current_focus' "$plan_yaml")
      if [ "$current_focus" != "null" ]; then
        focus_milestone=$(echo "$current_focus" | yq eval '.milestone_id')
        focus_phase=$(echo "$current_focus" | yq eval '.phase')
        if [ "$focus_milestone" = "$milestone_id" ]; then
          issue_progress="$issue_progress [$focus_phase]"
        fi
      fi
    fi

    # Detect workspace name from directory structure
    workspace_name=$(basename "$check_dir")

    break
  fi
  check_dir=$(dirname "$check_dir")
done

# Build statusline output
if $ratchet_active; then
  # Ratchet is active - show epic progress

  # Color codes
  cyan="\033[36m"
  green="\033[32m"
  yellow="\033[33m"
  dim="\033[2m"
  reset="\033[0m"

  # Build progress bar for milestones
  if [ "$total_milestones" -gt 0 ]; then
    milestone_pct=$((done_milestones * 100 / total_milestones))
    bar_width=10
    filled=$((milestone_pct * bar_width / 100))
    bar=$(printf "▓%.0s" $(seq 1 $filled))$(printf "░%.0s" $(seq 1 $((bar_width - filled))))
  else
    bar="──────────"
    milestone_pct=0
  fi

  # Vim mode indicator
  vim_indicator=""
  if [ -n "$vim_mode" ]; then
    if [ "$vim_mode" = "NORMAL" ]; then
      vim_indicator=" ${cyan}[N]${reset}"
    else
      vim_indicator=" ${green}[I]${reset}"
    fi
  fi

  # Main statusline
  echo -e "${cyan}◬ ${reset}${epic_name}${dim} | ${reset}$bar ${milestone_progress} ${dim}|${reset} $issue_progress ${dim}|${reset} ${yellow}${model}${reset} ${ctx_pct}%${vim_indicator}"

  # Optional second line: warnings/discoveries
  discoveries=$(yq eval '[.epic.discoveries[] | select(.severity == "high")] | length' "$plan_yaml" 2>/dev/null || echo "0")
  blocked=$(yq eval '[.epic.milestones[].issues[] | select(.status == "blocked")] | length' "$plan_yaml" 2>/dev/null || echo "0")

  if [ "$discoveries" -gt 0 ] || [ "$blocked" -gt 0 ]; then
    warnings=""
    [ "$blocked" -gt 0 ] && warnings="${warnings}${yellow}⚠ ${blocked} blocked${reset} "
    [ "$discoveries" -gt 0 ] && warnings="${warnings}${dim}🔍 ${discoveries} discoveries${reset}"
    echo -e "$warnings"
  fi

else
  # Not in Ratchet workspace - show default statusline

  # Color codes
  yellow="\033[33m"
  dim="\033[2m"
  reset="\033[0m"

  # Build context progress bar
  bar_width=10
  filled=$((ctx_pct * bar_width / 100))
  bar=$(printf "▓%.0s" $(seq 1 $filled))$(printf "░%.0s" $(seq 1 $((bar_width - filled))))

  # Vim mode indicator
  vim_indicator=""
  if [ -n "$vim_mode" ]; then
    if [ "$vim_mode" = "NORMAL" ]; then
      vim_indicator=" ${cyan}[N]${reset}"
    else
      vim_indicator=" ${green}[I]${reset}"
    fi
  fi

  # Shorten CWD if too long
  short_cwd=$(echo "$cwd" | sed "s|$HOME|~|" | awk -F/ '{
    if (NF > 3) {
      print $1 "/" $2 "/…/" $(NF-1) "/" $NF
    } else {
      print $0
    }
  }')

  echo -e "${dim}$short_cwd ${reset}${dim}|${reset} ${yellow}${model}${reset} $bar ${ctx_pct}%${vim_indicator}"
fi
