Use sequential-thinking mcp and all its tools that you will need about the problem and how to solve it. 
You must ultrathink for the solution and use reasoning.

You must consider edge cases and follow best coding practices for everything. Never do bandaid fixes.

## Configuration

STEP 1: You must use the investigator subagent (pass to it the full path of the created task-{id} 
directory) that returns you a "INVESTIGATION_REPORT.md" file.

STEP 2: You must use the code-flow-mapper subagent (pass to it the full path of the created task-{id} 
directory) that returns you a "FLOW_REPORT.md" file.

STEP 3: You must use the planner subagent (pass to it the full path of the task directory that 
contains the 2 reports made by the 2 subagents) that reads both reports and creates a "PLAN.md".

STEP 4: After all three subagents finish, enter plan mode and read the "PLAN.md" file and present the plan 
to the user so that they can either accept or adjust it.

Problem: $ARGUMENTS
