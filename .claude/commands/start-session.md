# start-session

Initialize a new development session by establishing context and checking project health.

## Steps

1. First, establish clean git environment:
   - Run `git status` to see current branch and changes
   - If not on main branch, switch to main: `git checkout main`
   - If on main with uncommitted changes, ask user how to handle them
   - Update main branch: `git pull origin main`
   - Run `git log --oneline -5` to see recent commits

2. Create feature branch (if starting new work):
   - Identify current feature from `docs/plan/index.md`
   - Create feature branch: `git checkout -b feature/[feature-name]` 
   - Example: `git checkout -b feature/001-dec-cfg-cfg-format-basic-cli`
   - Or continue on existing feature branch if work is in progress

3. Verify the project builds successfully:
   - Run `make build`
   - If build fails, report the error and suggest fixes

4. Check the current development stage:
   - Read `docs/plan/index.md` to find the "Current Active Stage" section
   - Identify which feature/stage is currently being worked on

5. Review recent progress:
   - Navigate to the current stage folder (e.g., `docs/plan/001-dec-cfg-cfg-format-basic-cli/`)
   - Open `execution-plan-steps.md` and check the Progress Summary table
   - Note which steps are completed and which are pending

6. Run baseline tests to verify starting state:
   - Run `make test`
   - Note any failing tests

7. Check for lint issues:
   - Run `make lint`
   - Note any lint warnings or errors

8. Summarize findings:
   - Report current branch and any uncommitted changes
   - Confirm build status
   - Show current feature/stage being developed
   - Display progress (X/Y steps completed)
   - Identify the next step to work on
   - Report any test failures or lint issues that need attention

## Example Output

```
Session initialized successfully!

Git Status:
- Branch: feature/001-dec-cfg-cfg-format-basic-cli
- Based on: main (up to date)
- Working directory: Clean
- Recent commits on main: [list last 5]

Build Health: ✅ Passing

Current Development: Stage 1 - Configuration Format & Basic CLI
Progress: 3/7 steps completed

Next Step: Step 4 - Create command stubs

Quality Status:
- Tests: ✅ All passing
- Lint: ✅ No issues

Ready to continue development. Use /implement-next to work on Step 4.
```