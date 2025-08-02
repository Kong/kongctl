# implement-step

Implement a specific step by number (e.g., /implement-step 3).

## Usage

This command requires a step number as an argument. The user should specify which step to implement.

## Steps

1. Parse the step number from the user's request:
   - Look for a number in the user's message
   - If no number found, ask the user to specify which step

2. Identify the current feature:
   - Read `planning/index.md` to find current active stage
   - Navigate to the feature folder's `execution-plan-steps.md`

3. Find the specified step:
   - Look for "## Step [NUMBER]:" in the execution plan
   - If step doesn't exist, report error and list available steps

4. Check step status and dependencies:
   - Verify the step exists and note its current status
   - If status is "Completed", ask if user wants to re-implement
   - Check if any dependencies are listed and verify they're completed

5. Follow the same implementation process as /implement-next:
   - Update status to "In Progress"
   - Read and understand step requirements
   - Implement following the guidance
   - Run quality gates
   - Update status to "Completed" if successful
   - Commit the work

6. Report completion and suggest next actions

## Example Output

```
Implementing Step 3: Add Command Factories
==========================================

Step found: ✅
Current status: Not Started
Dependencies: Step 1 ✅, Step 2 ✅

Step Status: Not Started → In Progress

Implementation:
[... implementation details ...]

Quality Gates:
✅ Build: Passing
✅ Lint: No issues
✅ Tests: All passing

Step Status: In Progress → Completed

Next available step: Step 4 - Create command stubs
Use /implement-next to continue, or /status to see overall progress.
```