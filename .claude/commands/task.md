Use sequential-thinking mcp and all its tools that you will need about the problem and how to solve it. 
You must ultrathink for the solution and use reasoning.

You must consider edge cases and follow best coding practices for everything. Never do bandaid fixes.

## Configuration

The task directory has been automatically created by the hook. You need to understand which mode was used:
- GitHub Issue Mode: If the directory is in tasks/gh-XX/, read GITHUB_ISSUE.md for the issue details
- Planning Stage Mode: If the directory is in planning/XXX/tasks/task-YY/, reference the parent stage's execution-plan-steps.md
- Ad-hoc Mode: If the directory is in tasks/task-XX/, proceed with the provided problem

STEP 1: You must use the investigator subagent (pass to it the full path of the created task 
directory) that returns you a "INVESTIGATION_REPORT.md" file. If this is a GitHub issue, also 
include the issue details from GITHUB_ISSUE.md. If this is a planning stage task, reference 
the stage's execution-plan-steps.md file for context.

STEP 2: You must use the code-flow-mapper subagent (pass to it the full path of the created task 
directory) that returns you a "FLOW_REPORT.md" file.

STEP 3: You must use the planner subagent (pass to it the full path of the task directory that 
contains the 2 reports made by the 2 subagents) that reads both reports and creates a "PLAN.md".

STEP 4: After all three subagents finish, enter plan mode and read the "PLAN.md" file and present the plan 
to the user so that they can either accept or adjust it.

Problem: $ARGUMENTS