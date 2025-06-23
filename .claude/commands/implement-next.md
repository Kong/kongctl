# implement-next

Implement the next available step in the current development plan with full quality checks.

## Steps

1. First, identify the current feature and next step:
   - Read `docs/plan/index.md` to find current active stage
   - Navigate to the feature folder's `execution-plan-steps.md`
   - Find the first step with status "Not Started"
   - Verify all dependencies are met (if any)

2. Update the step status to "In Progress":
   - Edit the step's status field in `execution-plan-steps.md`
   - Update the Progress Summary table

3. Read and understand the step requirements:
   - Read the full step description
   - Note any code examples provided
   - Check for ADR references and read them if needed
   - Understand test requirements

4. Implement the step following the guidance:
   - Write the code as specified in the step
   - Follow the exact patterns and examples provided
   - Add any required tests

5. Run quality gates:
   - Run `make build` - must pass
   - Run `make lint` - must have zero issues
   - Run `make test` - all tests must pass
   - If the step involves CLI commands, run `make test-integration`

6. If all quality gates pass:
   - Update step status to "Completed"
   - Update Progress Summary table
   - Add any implementation notes to the step if decisions were made

7. If quality gates fail:
   - Fix the issues
   - Re-run quality gates
   - Only mark as completed when all checks pass

8. Commit the work:
   - Stage the changes
   - Use the commit message template from the step (if provided)
   - Or create a descriptive commit message following project conventions

9. Report completion:
   - Show what was implemented
   - Confirm all quality checks passed
   - Show the updated progress
   - Identify the next step to work on

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