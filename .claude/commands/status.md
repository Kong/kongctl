# status

Show the current development status, progress, and next steps.

## Steps

1. Read the current active development from `docs/plan/index.md`:
   - Find the "Current Active Stage" section
   - Note the feature name and stage

2. Navigate to the current feature folder and read `execution-plan-steps.md`:
   - Find the Progress Summary table
   - Count completed vs total steps
   - Identify the next "Not Started" step

3. Check for any work in progress:
   - Look for steps marked as "In Progress"
   - Note if any steps are blocked

4. Run a quick git status to check for uncommitted changes:
   - Run `git status --short`

5. Display a comprehensive status report including:
   - Current feature/stage name
   - Overall progress (X/Y steps completed)
   - Current step in progress (if any)
   - Next step to implement
   - Any blockers or issues
   - Uncommitted changes (if any)

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