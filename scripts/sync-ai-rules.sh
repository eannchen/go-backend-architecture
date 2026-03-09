#!/usr/bin/env bash
# Syncs AGENTS.md (source of truth) to .cursor/rules/*.mdc and .claude/rules/*.md.
# Run from repo root after editing AGENTS.md. Cursor and Claude Code will use the generated rules.

set -e
cd "$(git rev-parse --show-toplevel)"
mkdir -p .cursor/rules .claude/rules
# Remove generated files so renamed/removed sections in AGENTS.md don't leave stale rules
rm -f .cursor/rules/*.mdc .claude/rules/*.md

awk '
  BEGIN { first_hr = 0; name = "" }
  /^---$/ {
    first_hr = 1
    if (name != "") {
      close(out_cursor)
      close(out_claude)
      name = ""
    }
    next
  }
  /^# / && first_hr {
    if (name != "") {
      close(out_cursor)
      close(out_claude)
    }
    title = substr($0, 3)
    name = tolower(title)
    gsub(/[^a-z0-9]+/, "-", name)
    gsub(/^-|-$/, "", name)
    out_cursor = ".cursor/rules/" name ".mdc"
    out_claude = ".claude/rules/" name ".md"
    # Cursor .mdc
    print "---" > out_cursor
    print "description: " title > out_cursor
    print "alwaysApply: true" > out_cursor
    print "---" > out_cursor
    print "" > out_cursor
    print $0 > out_cursor
    # Claude .md
    print "---" > out_claude
    print "description: " title > out_claude
    print "---" > out_claude
    print "" > out_claude
    print $0 > out_claude
    next
  }
  name != "" {
    print $0 >> out_cursor
    print $0 >> out_claude
  }
  END {
    if (name != "") {
      close(out_cursor)
      close(out_claude)
    }
  }
' AGENTS.md

echo "Synced AGENTS.md -> .cursor/rules and .claude/rules"
