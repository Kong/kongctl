# Planning Process and Document Structure

This document explains how planning documents are organized in the `docs/plan/` folder and the development process they support.

## Document Organization

### Folder Structure

Planning documents are organized in stage-specific folders:
- Each development effort has its own folder named after the product manager's plan name
- Example: `001-dec-cfg-cfg-format-basic-cli/` for declarative configuration Stage 1
- All feature-specific documents live within that feature's folder

Top-level process documentation remains in the root `docs/plan/` directory:
- `process.md` - This document explaining the planning structure
- `index.md` - Master dashboard showing current active stage
- `claude-code-guide.md` - Quick start guide for implementation
- `user-guide.md` - Commands and workflows for users to direct Claude Code

### Document Types by Feature

Each feature/development folder contains the following document types:

#### 1. High-Level Planning (`description.md`)
- Product manager provided requirements and specifications
- High-level goals and deliverables
- Initial technical direction
- Success criteria

#### 2. Execution Plan Overview (`execution-plan-overview.md`)
- Technical approach and design decisions
- Code examples and architecture
- Package structure and patterns
- References to detailed ADRs

#### 3. Execution Plan Steps (`execution-plan-steps.md`)
- **Primary implementation guide**
- Step-by-step implementation plan
- Code examples for each step
- Test requirements
- Commit messages
- **Status tracking for progress**

#### 4. Architecture Decision Records (`execution-plan-adrs.md`)
- Stage-specific ADRs numbered as ADR-XXX-YYY
- Technical decisions with context and rationale
- Alternative approaches considered
- Consequences of decisions

## Status Tracking System

### Execution Plan Progress

The `execution-plan-steps.md` files contain a **Progress Summary** table and individual step status tracking:

#### Status Values
- **Not Started** - Step has not been begun
- **In Progress** - Step is currently being worked on  
- **Completed** - Step has been fully implemented and tested
- **Blocked** - Step cannot proceed due to dependencies or issues
- **Skipped** - Step was intentionally skipped for this stage

#### Maintenance During Development
1. Update step status as work progresses
2. Add notes in step sections for implementation decisions
3. Update Progress Summary table to reflect current state
4. Mark dependencies as resolved when prerequisites complete

### Stage Status Overview

**Current Active Stage**: See [index.md](index.md) for the current active stage and implementation guide.

The index.md file serves as the **single source of truth** for:
- Which stage is currently active
- Direct links to current implementation documents
- Progress status and next steps
- Stage transition guidance

| Folder | Feature/Stage | Status |
|--------|---------------|--------|
| 001-dec-cfg-cfg-format-basic-cli/ | Declarative Config: Configuration Format & Basic CLI | In Progress |
| 002-dec-cfg-plan-labels/ | Declarative Config: Plan Labels | Not Started |
| 003-dec-cfg-plan-exec/ | Declarative Config: Plan Execution | Not Started |
| 004-dec-cfg-multi-resource/ | Declarative Config: Multi-Resource | Not Started |
| Future folders | Additional features as planned by PM | TBD |

### Development Transition Process

When completing a development effort and moving to the next:

1. **Complete current development**:
   - Mark all steps as "Completed" in current feature's execution-plan-steps.md
   - Ensure all deliverables are implemented and tested
   - Create final commit for feature completion

2. **Update index.md**:
   - Change "Current Active Stage" section to next development effort
   - Update feature name, status, and document links
   - Move completed feature to "✅ Completed" in Feature Overview
   - Update implementation status and next steps

3. **Prepare new development folder**:
   - Product manager provides plan document with name (e.g., `005-feature-name`)
   - Create folder: `docs/plan/005-feature-name/`
   - Add plan document as `description.md` in the folder
   - Create execution-plan-*.md documents as implementation progresses

4. **Maintain continuity**:
   - Reference completed stages in new planning documents
   - Document dependencies between stages
   - Preserve architectural decisions from previous stages

## ADR Numbering System

Architecture Decision Records use stage-specific numbering:
- Format: `ADR-XXX-YYY` where XXX is stage number, YYY is ADR number
- Example: `ADR-001-008` is the 8th ADR for Stage 1
- This prevents confusion between decisions made in different stages
- Each stage's ADRs start from 001

## Development Workflow

### For Implementers (Including Claude Code)

1. **Start with stage requirements**: Read `{stage-folder}/description.md`
2. **Understand architecture**: Review `{stage-folder}/execution-plan-overview.md`
3. **Follow implementation guide**: Use `{stage-folder}/execution-plan-steps.md` as primary guide
4. **Reference decisions**: Consult `{stage-folder}/execution-plan-adrs.md` for context
5. **Track progress**: Update status fields in execution plan steps
6. **Maintain quality**: Follow test requirements and commit message patterns

### Step-by-Step Implementation Process

1. **Check dependencies**: Ensure prerequisite steps are completed
2. **Update status**: Mark step as "In Progress"
3. **Implement code**: Follow the detailed implementation guide
4. **Write tests**: Follow test requirements in the step
5. **Commit changes**: Use provided commit message template
6. **Update status**: Mark step as "Completed"
7. **Update summary**: Reflect progress in Progress Summary table

## Best Practices

### For Planning Documents
- Keep stage-specific information in stage files
- Put general process information in this document
- Use consistent formatting and structure
- Include comprehensive code examples
- Provide clear test requirements

### For Implementation
- Always update status tracking
- Follow the exact step sequence
- Don't skip tests unless explicitly noted
- Use provided commit message templates
- Add implementation notes to steps when decisions change

### For Claude Code Specifically
- Start each session by reading the Progress Summary
- Focus on the next "Not Started" step with resolved dependencies
- Update status before and after working on each step
- Add notes to steps when making implementation decisions
- Reference ADRs when context is needed for decisions

## Suggested Improvements

### Current Structure Strengths
- Clear separation between planning and execution
- Comprehensive step-by-step guidance
- Status tracking for progress visibility
- Feature-based organization prevents confusion

### Future Enhancements
1. **Template Structure**: Create templates for future stages
2. **Cross-Stage Dependencies**: Document how stages relate to each other
3. **Integration Testing**: Add stage-level integration test requirements
4. **Rollback Plans**: Document how to undo partially implemented stages

## Quick Reference

### Example: Current Active Development
For the current active development effort (see [index.md](index.md)):
- **Folder**: Named after the PM's plan (e.g., `001-dec-cfg-cfg-format-basic-cli/`)
- **Requirements**: Defined in `{folder}/description.md`
- **Architecture**: Documented in `{folder}/execution-plan-overview.md`
- **Implementation**: Tracked in `{folder}/execution-plan-steps.md`
- **Decisions**: Recorded in `{folder}/execution-plan-adrs.md`

### Key Entry Points for Implementation
1. **Progress Summary** in current development's `execution-plan-steps.md` - Shows what's done/todo
2. **Step 1** onwards in same file - Detailed implementation guidance
3. **ADR references** when context needed for understanding decisions

This structure provides a complete roadmap for implementing any feature in kongctl while maintaining clear documentation of decisions and progress.