# Planning Process and Document Structure

This document explains how planning documents are organized in the `docs/plan/` folder and the development process they support.

## Document Organization

### File Naming Convention

All planning documents follow a stage-based naming pattern:
- `XXX-description.md` where XXX is the stage number (001, 002, etc.)
- Example: `001-execution-plan-steps.md` for Stage 1 implementation steps

### Document Types by Stage

Each stage of development has the following document types:

#### 1. High-Level Planning (`XXX-dec-cfg-cfg-format-basic-cli.md`)
- Product manager provided requirements
- High-level goals and deliverables
- Initial technical direction
- Success criteria

#### 2. Execution Plan Overview (`XXX-execution-plan-overview.md`)
- Technical approach and design decisions
- Code examples and architecture
- Package structure and patterns
- References to detailed ADRs

#### 3. Execution Plan Steps (`XXX-execution-plan-steps.md`)
- **Primary implementation guide**
- Step-by-step implementation plan
- Code examples for each step
- Test requirements
- Commit messages
- **Status tracking for progress**

#### 4. Architecture Decision Records (`XXX-execution-plan-adrs.md`)
- Stage-specific ADRs numbered as ADR-XXX-YYY
- Technical decisions with context and rationale
- Alternative approaches considered
- Consequences of decisions

#### 5. Process Documentation (`process.md`)
- This document explaining the planning structure
- Development process guidelines
- Claude Code usage instructions

## Status Tracking System

### Execution Plan Progress

The `XXX-execution-plan-steps.md` files contain a **Progress Summary** table and individual step status tracking:

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

| Stage | Description | Status | Key Documents |
|-------|-------------|--------|---------------|
| 001 | Configuration Format & Basic CLI | In Progress | 001-execution-plan-*.md |
| 002+ | Future stages | Not Started | TBD |

### Stage Transition Process

When completing a stage and moving to the next:

1. **Complete current stage**:
   - Mark all steps as "Completed" in current execution-plan-steps.md
   - Ensure all deliverables are implemented and tested
   - Create final commit for stage completion

2. **Update index.md**:
   - Change "Current Active Stage" section to next stage
   - Update stage number, status, and document links
   - Move completed stage to "✅ Completed" in Stage Overview
   - Update implementation status and next steps

3. **Create new stage documents**:
   - Follow naming convention: XXX-execution-plan-*.md
   - Create ADRs, steps, overview documents for new stage
   - Set up Progress Summary table in new execution-plan-steps.md

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

1. **Start with stage requirements**: Read `XXX-dec-cfg-cfg-format-basic-cli.md`
2. **Understand architecture**: Review `XXX-execution-plan-overview.md`
3. **Follow implementation guide**: Use `XXX-execution-plan-steps.md` as primary guide
4. **Reference decisions**: Consult `XXX-execution-plan-adrs.md` for context
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
- Stage-based organization prevents confusion

### Future Enhancements
1. **Template Structure**: Create templates for future stages
2. **Cross-Stage Dependencies**: Document how stages relate to each other
3. **Integration Testing**: Add stage-level integration test requirements
4. **Rollback Plans**: Document how to undo partially implemented stages

## Quick Reference

### Current Stage 1 Status
- **Requirements**: Defined in `001-dec-cfg-cfg-format-basic-cli.md`
- **Architecture**: Documented in `001-execution-plan-overview.md`
- **Implementation**: Tracked in `001-execution-plan-steps.md`
- **Decisions**: Recorded in `001-execution-plan-adrs.md`

### Key Entry Points for Implementation
1. **Progress Summary** in `001-execution-plan-steps.md` - Shows what's done/todo
2. **Step 1** onwards in same file - Detailed implementation guidance
3. **ADR references** when context needed for understanding decisions

This structure provides a complete roadmap for implementing declarative configuration features while maintaining clear documentation of decisions and progress.