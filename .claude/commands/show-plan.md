# show-plan

Display the current development plan including all steps and their status.

## Steps

1. Identify the current active feature:
   - Read `docs/plan/index.md` to find the current active stage
   - Note the feature name and folder

2. Read the feature requirements:
   - Navigate to the feature folder
   - Read `description.md` to understand PM requirements
   - Extract key goals and deliverables

3. Read the technical overview:
   - Open `execution-plan-overview.md` if it exists
   - Note the technical approach and architecture decisions

4. Read all implementation steps:
   - Open `execution-plan-steps.md`
   - For each step, note:
     - Step number and title
     - Current status
     - Brief description
     - Any blockers or dependencies

5. Compile and display a comprehensive plan showing:
   - Feature name and description
   - Key goals/deliverables
   - Technical approach summary
   - All steps with their status
   - Overall progress
   - Current focus

## Example Output

```
Development Plan: Declarative Configuration - Stage 1
=====================================================

Feature: Configuration Format & Basic CLI
Folder: docs/plan/001-dec-cfg-cfg-format-basic-cli/

Goals:
- Establish YAML configuration format
- Integrate basic commands (plan, apply, export)
- Implement resource validation
- Support basic workflows

Technical Approach:
- Embed SDK types in configuration structures
- Use type-specific ResourceSet for type safety
- Implement reference resolution with validation
- Create command stubs first, then add functionality

Implementation Steps:
--------------------
✅ Step 1: Add Verb Constants
   Status: Completed
   Define constants for plan, apply, sync operations

✅ Step 2: Define Command Structure  
   Status: Completed
   Create basic command hierarchy

✅ Step 3: Add Command Factories
   Status: Completed
   Implement factory functions for commands

📋 Step 4: Create command stubs [NEXT]
   Status: Not Started
   Add stub implementations for plan, apply, export

📋 Step 5: Implement YAML loading
   Status: Not Started
   Add YAML parsing and validation

📋 Step 6: Add resource validation
   Status: Not Started
   Implement resource type validation

📋 Step 7: Integration tests
   Status: Not Started
   Add end-to-end tests for commands

Progress: 3/7 steps completed (43%)

Current Focus: Ready to implement Step 4
Use /implement-next to begin work on command stubs.
```