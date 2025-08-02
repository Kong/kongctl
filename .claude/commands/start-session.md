# start-session

Initialize a new development session by establishing context and checking project health.

## Steps

1. Establish git environment:
   - Run `git status` and `git log --oneline -5`
   - Switch to main if needed: `git checkout main && git pull origin main`
   - Create/switch to feature branch based on current work in `planning/index.md`

2. Verify project health:
   - Run `make build` (must pass)
   - Run `make test` and `make lint`

3. Check current development:
   - Read `planning/index.md` for "Current Active Stage"
   - Check Progress Summary in current execution-plan-steps.md

4. Report status:
   - Branch status and build health
   - Current feature and progress (X/Y steps)
   - Next step to work on
   - Any issues requiring attention

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