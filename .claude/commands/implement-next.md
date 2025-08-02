# implement-next

Implement the next available step in the current development plan with full quality checks.

## Steps

1. Identify next step:
   - Read `planning/index.md` → current active stage
   - Find first "Not Started" step in execution-plan-steps.md
   - Verify dependencies are met

2. Update status to "In Progress" and implement:
   - Follow step guidance and code examples exactly
   - Add required tests

3. Run quality gates:
   - `make build && make lint && make test`
   - `make test-integration` if CLI commands involved

4. Complete:
   - Mark as "Completed" only if all checks pass
   - Update Progress Summary table
   - Commit with provided message template
   - Report progress and identify next step

## Example Output

```
Implementing Step 4: Create command stubs
========================================

Step Status: Not Started → In Progress

Implementation:
- Created stub commands for plan, apply, and export
- Added command registration to root command
- Implemented basic help text

Quality Gates:
✅ Build: Passing
✅ Lint: No issues
✅ Tests: All passing (15/15)
✅ Integration Tests: Passing

Step Status: In Progress → Completed

Progress Update: 4/7 steps completed (57%)

Changes committed with message:
"feat(cmd): add stub implementations for plan, apply, and export commands"

Next Step: Step 5 - Implement YAML loading
Use /implement-next to continue.
```