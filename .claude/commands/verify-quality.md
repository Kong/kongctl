# verify-quality

Run all quality gates to ensure code meets project standards.

## Steps

1. Run all quality gates:
   - `make build` (must pass)
   - `make lint` (zero issues required) 
   - `make test` (all tests must pass)
   - `make test-integration` (if CLI changes)

2. Check git status for uncommitted changes

3. Provide summary report:
   - Overall quality status (PASS/FAIL)
   - Individual check results
   - Issues requiring fixes
   - Next steps

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