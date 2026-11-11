#!/usr/bin/env bash
# Counts where public repositories put agent files, using GitHub code search
# with your own `gh` login. Read-only: it only runs searches.
#
#   tools/survey/survey.sh > docs/survey/$(date +%F).tsv
#
# Counts are GitHub's estimates (code search covers default branches of
# indexed repositories), so compare rows with each other, not as totals.
# Code search allows about 10 requests a minute, so it pauses between them.
set -euo pipefail

command -v gh >/dev/null || { echo "needs the GitHub CLI (gh), signed in" >&2; exit 1; }
gh auth status >/dev/null 2>&1 || { echo "run 'gh auth login' first" >&2; exit 1; }

# label<TAB>query: what each location holds, then the search
queries=$(cat <<'Q'
skills, shared (.agents)	filename:SKILL.md path:.agents/skills
skills, Claude Code	filename:SKILL.md path:.claude/skills
skills, Codex	filename:SKILL.md path:.codex/skills
skills, Gemini CLI	filename:SKILL.md path:.gemini/skills
skills, GitHub	filename:SKILL.md path:.github/skills
skills, XDG config	filename:SKILL.md path:.config/agents/skills
subagents, shared (.agents)	path:.agents/agents extension:md
subagents, Claude Code	path:.claude/agents extension:md
subagents, Codex	path:.codex/agents extension:toml
subagents, Gemini CLI	path:.gemini/agents extension:md
memory, shared (.agents)	path:.agents/memory
memory, MEMORY.md anywhere	filename:MEMORY.md
instructions, AGENTS.md	filename:AGENTS.md
instructions, CLAUDE.md	filename:CLAUDE.md
instructions, GEMINI.md	filename:GEMINI.md
instructions, .agents/AGENTS.md	filename:AGENTS.md path:.agents
instructions, XDG config	filename:AGENTS.md path:.config/agents
prompts, Copilot	path:.github/prompts extension:md
prompts, Claude commands	path:.claude/commands extension:md
prompts, Gemini commands	path:.gemini/commands extension:toml
prompts, Codex	path:.codex/prompts extension:md
workflows, Windsurf	path:.windsurf/workflows extension:md
workflows, Cline	path:.clinerules/workflows extension:md
patterns, Fabric	filename:system.md path:fabric/patterns
Q
)

printf 'date\tlabel\tquery\tcount\n'
today=$(date +%F)
first=1
while IFS=$'\t' read -r label q; do
  [ -z "$label" ] && continue
  [ "$first" = 1 ] || sleep 7
  first=0
  n=$(gh api -X GET search/code -f q="$q" -f per_page=1 --jq .total_count 2>/dev/null || echo error)
  printf '%s\t%s\t%s\t%s\n' "$today" "$label" "$q" "$n"
done <<<"$queries"
