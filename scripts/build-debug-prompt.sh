#!/usr/bin/env bash
# Build the plain-text system prompt from the vjb-debug skill files.
# Concatenates SKILL.md and all referenced subfiles, strips YAML frontmatter,
# converts Claude Code subagent dispatch patterns to numbered checklist steps,
# and writes to pkg/vpwned/server/debug_prompt_embedded.txt.
#
# Exit codes:
#   0 — success
#   1 — skill file not found
#   2 — required frontmatter fields (version, last-updated) absent
set -euo pipefail

SKILL_FILE="${1:-.claude/skills/vjb-debug/SKILL.md}"
OUTPUT_FILE="pkg/vpwned/server/debug_prompt_embedded.txt"
SKILL_DIR="$(dirname "$SKILL_FILE")"

if [[ ! -f "$SKILL_FILE" ]]; then
  echo "ERROR: skill file not found: $SKILL_FILE" >&2
  exit 1
fi

# ── Extract frontmatter fields ─────────────────────────────────────────────────
VERSION=""
LAST_UPDATED=""
in_fm=0
fm_done=0
while IFS= read -r line; do
  if [[ "$line" == "---" && $fm_done -eq 0 ]]; then
    in_fm=$((in_fm + 1))
    [[ $in_fm -eq 2 ]] && fm_done=1
    continue
  fi
  if [[ $in_fm -eq 1 ]]; then
    if [[ "$line" =~ ^version:[[:space:]]*(.+)$ ]]; then
      VERSION="${BASH_REMATCH[1]}"
    fi
    if [[ "$line" =~ ^last-updated:[[:space:]]*(.+)$ ]]; then
      LAST_UPDATED="${BASH_REMATCH[1]}"
    fi
  fi
done < "$SKILL_FILE"

if [[ -z "$VERSION" || -z "$LAST_UPDATED" ]]; then
  echo "ERROR: SKILL.md frontmatter missing required fields (version and/or last-updated)" >&2
  exit 2
fi

# ── Strip YAML frontmatter from a file ────────────────────────────────────────
strip_frontmatter() {
  local file="$1"
  awk 'BEGIN{fm=0; done=0}
    /^---$/ && !done { fm++; if(fm==2){done=1}; next }
    done || fm==0 { print }' "$file"
}

# ── Collect referenced subfiles from SKILL.md in order of appearance ──────────
collect_subfiles() {
  grep -oE '\(([^)]+\.md)\)' "$SKILL_FILE" \
    | sed 's/^(//;s/)$//' \
    | grep -v '^http' \
    | sort -u \
    | while read -r rel; do
        local abs="${SKILL_DIR}/${rel}"
        [[ -f "$abs" ]] && echo "$abs"
      done
}

# ── Convert subagent dispatch → numbered checklist ────────────────────────────
# Matches lines containing subagent_type: or Agent( calls and rewrites them
convert_subagents() {
  local counter=0
  while IFS= read -r line; do
    if echo "$line" | grep -qE 'subagent_type:'; then
      counter=$((counter + 1))
      agent_name="$(echo "$line" | grep -oE 'subagent_type:\s*"?[^"}, ]+"?' \
                    | sed 's/subagent_type:[[:space:]]*//;s/"//g' | head -1)"
      echo "${counter}. **Run investigation steps for \`${agent_name}\`** (inline equivalent below):"
    else
      echo "$line"
    fi
  done
}

# ── Build output ───────────────────────────────────────────────────────────────
mkdir -p "$(dirname "$OUTPUT_FILE")"
{
  # First two lines parsed by the Go handler
  echo "version: ${VERSION}"
  echo "last_updated: ${LAST_UPDATED}"
  echo ""

  # Main skill file (frontmatter stripped)
  strip_frontmatter "$SKILL_FILE"

  # Inline each referenced subfile with a section separator
  while IFS= read -r subfile; do
    echo ""
    echo "---"
    echo ""
    strip_frontmatter "$subfile"
  done < <(collect_subfiles)
} | convert_subagents > "$OUTPUT_FILE"

echo "Built: $OUTPUT_FILE (version=${VERSION}, last_updated=${LAST_UPDATED})"
