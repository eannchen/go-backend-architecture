---
name: sync-ai-rules
description: Run after AGENTS.md is modified to regenerate .cursor/rules and .claude/rules so Cursor and Claude Code see the same rules. Use when you or the user have just edited AGENTS.md.
---

# Sync AI rules

After changing `AGENTS.md`, run the sync script so Cursor and Claude Code get the updated rules.

## Steps

1. Run from the repository root:
   ```bash
   ./scripts/sync-ai-rules.sh
   ```
2. Confirm the script printed "Synced AGENTS.md -> .cursor/rules and .claude/rules".

No other steps. The script overwrites `.cursor/rules/*.mdc` and `.claude/rules/*.md` from the current content of `AGENTS.md`.
