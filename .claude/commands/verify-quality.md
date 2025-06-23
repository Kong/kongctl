# verify-quality

Run all quality gates to ensure code meets project standards.

## Steps

1. Run the build check:
   - Execute `make build`
   - Report success or capture error output

2. Run the lint check:
   - Execute `make lint`
   - Report any lint issues found
   - Provide suggestions for common lint fixes if issues found

3. Run unit tests:
   - Execute `make test`
   - Report test results (passed/failed)
   - Show failed test details if any

4. Check if integration tests are needed:
   - Look at recent changes with `git diff --name-only`
   - If changes involve CLI commands or API interactions, run integration tests
   - Execute `make test-integration` if needed
   - Report results

5. Check for uncommitted changes:
   - Run `git status`
   - List any modified or untracked files

6. Provide a summary report with:
   - Overall quality status (PASS/FAIL)
   - Individual check results
   - Specific issues that need fixing
   - Suggested next steps

## Example Output

```
Quality Verification Report
==========================

🔨 Build Check: ✅ PASSED
   Successfully built kongctl binary

🔍 Lint Check: ❌ FAILED
   Found 2 issues:
   - internal/cmd/plan.go:15: unused variable 'config'
   - internal/cmd/apply.go:23: line too long (125 > 120)
   
   Fix suggestions:
   - Remove unused variable or use _ if intentional
   - Break long line into multiple lines

🧪 Unit Tests: ✅ PASSED
   All 45 tests passed
   Coverage: 78.3%

🔌 Integration Tests: ✅ PASSED
   All 12 integration tests passed

📁 Git Status: Clean
   No uncommitted changes

Overall Status: ❌ FAILED
Action Required: Fix lint issues before proceeding

Run 'make lint' after fixes to verify.
```