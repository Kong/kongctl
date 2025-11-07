# status

Show the current development status, progress, and next steps.

## Steps

1. Read current development from `planning/index.md`
2. Check Progress Summary in current execution-plan-steps.md
3. Run `git status --short` for uncommitted changes
4. Display report:
   - Current feature/stage and progress (X/Y steps)
   - Steps in progress or next to implement
   - Any blockers or uncommitted changes

## Example Output

```
Current Development Status
========================

Feature: Declarative Configuration - Stage 1
Description: Configuration Format & Basic CLI

Progress: 3/7 steps completed (43%)
[███████████████░░░░░░░░░░░░░░░░░] 

✅ Completed Steps:
- Step 1: Add Verb Constants
- Step 2: Define Command Structure
- Step 3: Add Command Factories

🔄 In Progress:
- None

📋 Next Step:
- Step 4: Create command stubs
  Status: Not Started
  Description: Add stub implementations for plan, apply, and export commands

Git Status: Clean (or list changes)

Ready to continue with: /implement-next
```