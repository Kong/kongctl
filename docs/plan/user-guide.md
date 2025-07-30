# User Guide for Directing Claude Code

This guide provides commands and patterns for users to effectively direct Claude Code through the kongctl development workflow.

## 🎯 Custom Commands for Claude Code

Claude Code custom commands are implemented in `.claude/commands/` and can be invoked with `/command-name`. Use these commands to direct Claude Code through common development tasks:

### Session Management Commands

#### `/task`
Follow the task based workflow which uses subagents and mcp servers for planning and implementation

#### `/start-session`
**Usage**: "Start a new development session with /start-session"
**What Claude Code will do**:
1. Run the session quick start checklist from CLAUDE.md
2. Check git status and recent commits
3. Verify build health
4. Review current active development in docs/plan/index.md
5. Report current progress and next steps

#### `/status`
**Usage**: "Show current development status with /status"
**What Claude Code will do**:
1. Display current feature/stage from index.md
2. Show progress summary from current execution-plan-steps.md
3. List completed and pending steps
4. Identify the next step to work on

### Implementation Commands

#### `/implement-next`
**Usage**: "Implement the next step with /implement-next"
**What Claude Code will do**:
1. Find the next "Not Started" step in current execution-plan-steps.md
2. Mark it as "In Progress"
3. Implement the step following the provided guidance
4. Run quality gates (build, lint, test)
5. Mark as "Completed" if all checks pass
6. Update Progress Summary

#### `/implement-step N`
**Usage**: "/implement-step 3" or "Implement step 3"
**What Claude Code will do**:
1. Navigate to the specified step number
2. Verify dependencies are met
3. Follow the same implementation process as /implement-next

#### `/verify-quality`
**Usage**: "Run quality verification with /verify-quality"
**What Claude Code will do**:
1. Run `make build`
2. Run `make lint`
3. Run `make test`
4. Run `make test-integration` if applicable
5. Report any issues found

### Planning Commands

#### `/show-plan`
**Usage**: "Show the current development plan with /show-plan"
**What Claude Code will do**:
1. Display the feature description from current description.md
2. Show the technical overview
3. List all steps with their current status

#### `/show-adrs`
**Usage**: "Show architecture decisions with /show-adrs"
**What Claude Code will do**:
1. List all ADRs for the current feature
2. Provide summaries of key decisions
3. Show decision status and reasoning

### Feature Transition Commands

#### `/complete-feature`
**Usage**: "Complete the current feature with /complete-feature"
**What Claude Code will do**:
1. Verify all steps are marked "Completed"
2. Run final quality checks
3. Create completion commit
4. Update index.md to show feature as completed
5. Create pull request for review
6. Provide instructions for next feature

#### `/create-pr`
**Usage**: "Create a pull request with /create-pr"
**What Claude Code will do**:
1. Push current branch to remote
2. Create GitHub PR with proper description
3. Include testing checklist and planning references
4. Provide PR URL and next steps

#### `/start-feature FOLDER-NAME`
**Usage**: "Start new feature with /start-feature 005-new-feature-name"
**What Claude Code will do**:
1. Create the feature folder if it doesn't exist
2. Help set up initial planning documents
3. Update index.md to set as current active development

## 📋 User Workflow Examples

### Starting a New Day of Development
```
User: Start a new development session with /start-session
Claude: [Runs checklist, reports status]

User: Show current development status with /status
Claude: [Shows current feature, progress, next steps]

User: Implement the next step with /implement-next
Claude: [Implements step, runs tests, updates tracking]
```

### Implementing Multiple Steps
```
User: Show the current plan with /show-plan
Claude: [Displays feature overview and all steps]

User: Implement step 2 with /implement-step 2
Claude: [Implements specific step]

User: Continue implementing with /implement-next
Claude: [Moves to next available step]

User: Run quality verification with /verify-quality
Claude: [Runs all quality checks]
```

### Completing a Feature
```
User: Show current development status with /status
Claude: [Shows all steps completed]

User: Complete the current feature with /complete-feature
Claude: [Finalizes feature, updates tracking]

User: Start new feature with /start-feature 005-auth-improvements
Claude: [Sets up new feature development]
```

## 🌿 Git Workflow

Our development follows a feature branch workflow:

### Branch Strategy
- **main**: Production-ready code
- **feature/[feature-name]**: Development branches for each feature
- Example: `feature/001-dec-cfg-cfg-format-basic-cli`

### Workflow Steps
1. **Start**: `/start-session` creates/switches to feature branch from latest main
2. **Develop**: Work on feature using `/implement-next` and other commands
3. **Complete**: `/complete-feature` creates PR for review
4. **Review**: Team reviews PR, provides feedback
5. **Merge**: PR gets merged to main after approval
6. **Next**: New `/start-session` starts fresh from updated main

### PR Guidelines
- Each feature gets its own PR
- Include comprehensive testing checklist
- Reference planning documents
- Wait for review before starting next feature

## 🚨 Important User Guidelines

### DO:
- Start each session with `/start-session` to establish context
- Use `/status` frequently to track progress
- Run `/verify-quality` after implementing multiple steps
- Commit changes regularly (ask Claude to commit after major milestones)
- Review ADRs with `/show-adrs` when making architectural decisions

### DON'T:
- Skip steps or implement out of order without good reason
- Forget to update status tracking (Claude should handle this automatically)
- Move to a new feature without completing the current one
- Ignore failing tests or lint issues

## 💡 Tips for Effective Sessions

1. **Start Small**: Use `/implement-next` to work step by step rather than trying to implement everything at once

2. **Verify Often**: Use `/verify-quality` after each step or two to catch issues early

3. **Track Progress**: Use `/status` at the beginning and end of each session

4. **Understand Context**: Use `/show-plan` and `/show-adrs` to understand the bigger picture

5. **Clean Commits**: Ask Claude to commit with descriptive messages after completing each step or logical unit of work

## 🔄 Session Continuity

When resuming work across Claude Code sessions:

```
User: Start a new development session with /start-session
User: Show what was done in the last session (check recent commits)
User: Show current development status with /status
User: Continue with /implement-next
```

## 🆘 Troubleshooting

If things go wrong:

```
User: Run quality verification with /verify-quality
User: [If issues found] Fix the build/lint/test issues
User: Show current development status with /status
User: [If step is partially complete] Review and complete the current step
```

## 📝 Custom Workflows

You can combine commands for specific workflows:

### Morning Standup Workflow
```
/start-session
/status
Show me what's planned for today
```

### End of Day Workflow
```
/status
/verify-quality
Commit all completed work with appropriate messages
Push changes to the planning branch
```

### Code Review Prep
```
/show-plan
/status
Show me all changes made for the current feature
/verify-quality
```

## 🔧 Command Implementation

The custom commands are implemented as markdown files in `.claude/commands/`:

- `start-session.md` - Session initialization with git branch setup
- `status.md` - Progress reporting and next steps
- `implement-next.md` - Step-by-step implementation with quality gates
- `implement-step.md` - Specific step implementation by number
- `verify-quality.md` - Comprehensive quality checks
- `show-plan.md` - Feature plan and step overview
- `show-adrs.md` - Architecture decision summaries
- `create-pr.md` - Pull request creation with proper formatting
- `complete-feature.md` - Feature finalization and PR workflow

Each command file contains detailed instructions for Claude Code to follow, ensuring consistent behavior and adherence to the development process.

---

**Remember**: These commands are designed to help Claude Code follow the established development process. Claude Code will understand these commands in the context of the planning documents and CLAUDE.md guidance.
