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

7. Create pull request for review:
   - Push current branch to remote if not already pushed
   - Use `/create-pr` command to create PR with proper description
   - Or provide manual PR creation guidance

8. Provide transition guidance:
   - If next feature exists: explain how to start it after PR is merged
   - If no next feature: explain how to add a new feature
   - Recommend waiting for PR review and merge before starting next work

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

Pull Request:
- Branch pushed to origin
- PR created: https://github.com/Kong/kongctl/pull/123
- Title: "feat: implement declarative configuration stage 1"

Next Steps:
-----------
1. Wait for PR review and approval
2. Merge PR when approved
3. After merge, start next feature:
   - Use /start-session (will create new branch from updated main)
   - Or if no next feature planned, wait for PM planning

Current branch remains active for any review feedback changes.
Use /create-pr if you need to update the PR description or create additional PRs.
```