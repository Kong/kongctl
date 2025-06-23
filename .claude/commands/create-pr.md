# create-pr

Create a pull request for the current feature branch.

## Steps

1. Verify current branch and changes:
   - Run `git status` to confirm on feature branch
   - Check that there are committed changes ready for review
   - If working directory is dirty, ask user to commit changes first

2. Push feature branch to remote:
   - Run `git push origin [current-branch]`
   - If branch doesn't exist on remote, use `git push -u origin [current-branch]`

3. Gather PR information:
   - Identify current feature from branch name and `docs/plan/index.md`
   - Read the feature's `description.md` for PM requirements
   - Read `execution-plan-steps.md` to see what was completed
   - Summarize implemented changes

4. Create pull request using GitHub CLI:
   - Use `gh pr create` with the following structure:
   - Title: Brief description of the feature/changes
   - Body template:
     ```markdown
     ## Summary
     [Brief description of what was implemented]
     
     ## Changes
     - [List key changes made]
     - [Reference completed steps from execution plan]
     
     ## Testing
     - [x] Build passes: `make build`
     - [x] Lint passes: `make lint`
     - [x] Tests pass: `make test`
     - [x] Integration tests pass: `make test-integration` (if applicable)
     
     ## Related Planning
     - Feature folder: `docs/plan/[feature-folder]/`
     - Requirements: [link to description.md]
     - Implementation plan: [link to execution-plan-steps.md]
     
     🤖 Generated with [Claude Code](https://claude.ai/code)
     ```

5. Report PR creation:
   - Show PR URL
   - Provide next steps for review process
   - Suggest any follow-up actions

## Example Output

```
Creating Pull Request
====================

Current branch: feature/001-dec-cfg-cfg-format-basic-cli
Changes pushed to origin ✅

Feature: Declarative Configuration - Stage 1
Implementation: 7/7 steps completed

Changes included:
- Added plan, apply, and export command stubs
- Implemented YAML configuration format
- Added resource validation
- Created comprehensive test suite

PR created successfully!
🔗 https://github.com/Kong/kongctl/pull/123

Title: "feat: implement declarative configuration stage 1"

Next steps:
1. Request review from team members
2. Address any review feedback
3. Merge when approved
4. Use /complete-feature to finalize and move to next stage

Branch remains active for any review feedback changes.
```