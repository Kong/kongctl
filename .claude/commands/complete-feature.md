# complete-feature

Finalize the current feature development and prepare for the next feature.

## Steps

1. Verify all steps are completed:
   - Read current feature's `execution-plan-steps.md`
   - Check that all steps have status "Completed"
   - If any steps are not completed, list them and stop

2. Run final quality checks:
   - Execute `make build`
   - Execute `make lint`
   - Execute `make test`
   - Execute `make test-integration`
   - All must pass before proceeding

3. Review all changes for the feature:
   - Run `git log --oneline` to see commits for this feature
   - Run `git diff main...HEAD` to see all changes
   - Summarize what was implemented

4. Create a feature completion commit:
   - Stage any remaining changes
   - Create commit with message like:
     ```
     feat: complete [feature name] implementation
     
     - All X steps completed
     - All tests passing
     - Ready for review
     ```

5. Update documentation:
   - In the feature's `execution-plan-steps.md`, ensure Progress Summary shows 100%
   - Add any final implementation notes

6. Update index.md:
   - Change the current feature's status to "✅ Completed"
   - If there's a next feature ready:
     - Update "Current Active Stage" section to the next feature
     - Update links and status information
   - If no next feature:
     - Update to indicate no active development

7. Provide transition guidance:
   - If next feature exists: explain how to start it
   - If no next feature: explain how to add a new feature
   - Suggest creating a PR for the completed work

## Example Output

```
Feature Completion: Declarative Configuration - Stage 1
======================================================

Step Verification: ✅ All 7 steps completed

Quality Gates:
✅ Build: Passing
✅ Lint: No issues  
✅ Tests: All 52 tests passing
✅ Integration: All 15 tests passing

Feature Summary:
- Added plan, apply, and export commands
- Implemented YAML configuration format
- Added resource validation
- Created comprehensive test suite

Commits for this feature:
- abc1234 feat(cmd): add verb constants
- def5678 feat(cmd): define command structure
- ghi9012 feat(cmd): add command factories
- jkl3456 feat(cmd): create command stubs
- mno7890 feat(config): implement YAML loading
- pqr1234 feat(validation): add resource validation
- stu5678 test: add integration tests

Documentation updated:
- Progress Summary shows 100% complete
- index.md updated to show feature as completed

Next Steps:
-----------
No active feature currently set in index.md.

To start a new feature:
1. Receive plan document from PM
2. Create folder: docs/plan/XXX-feature-name/
3. Add plan as description.md
4. Update index.md to set as current active stage
5. Use /start-session to begin development

Recommended: Create a PR for this completed feature before starting the next one.
```