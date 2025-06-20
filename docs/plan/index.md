# Declarative Configuration Implementation Plan

This folder contains the complete planning and implementation tracking for kongctl's declarative configuration feature.

## 🎯 Current Active Stage: Stage 1 ⏳ In Progress

**Stage 1: Configuration Format & Basic CLI**
- **Implementation Guide**: [001-execution-plan-steps.md](001-execution-plan-steps.md) ← **Start here**
- **Architecture Decisions**: [001-execution-plan-adrs.md](001-execution-plan-adrs.md)
- **Technical Overview**: [001-execution-plan-overview.md](001-execution-plan-overview.md)
- **Requirements**: [001-dec-cfg-cfg-format-basic-cli.md](001-dec-cfg-cfg-format-basic-cli.md)

**Implementation Status**: 0/7 steps completed  
**Next Step**: Step 1 - Add Verb Constants  
**Progress Tracking**: See Progress Summary in implementation guide above

## Quick Start for Implementation

1. **Read the process**: [process.md](process.md) - Understand the document structure
2. **Check current progress**: Use implementation guide linked above
3. **Start implementing**: Follow the next "Not Started" step with resolved dependencies

## Stage Overview

### Stage 1: Configuration Format & Basic CLI ⏳ In Progress
**Goal**: Establish YAML configuration format and integrate basic commands into kongctl

| Document | Purpose | Status |
|----------|---------|---------|
| [001-dec-cfg-cfg-format-basic-cli.md](001-dec-cfg-cfg-format-basic-cli.md) | Requirements from PM | ✅ Complete |
| [001-execution-plan-overview.md](001-execution-plan-overview.md) | Technical approach | ✅ Complete |
| [001-execution-plan-steps.md](001-execution-plan-steps.md) | **Implementation guide** | 📋 Ready for implementation |
| [001-execution-plan-adrs.md](001-execution-plan-adrs.md) | Architecture decisions | ✅ Complete |

**Implementation Status**: 0/7 steps completed
- **Next step**: Step 1 - Add Verb Constants
- **Estimated effort**: Small to medium implementation
- **Key deliverables**: Command stubs, YAML loading, basic validation

### Stage 2: Plan Generation & Execution 🔮 Future
**Goal**: Implement plan generation, validation, and execution

| Document | Purpose | Status |
|----------|---------|---------|
| 002-* | TBD | 🔮 Not yet planned |

**Implementation Status**: Not started
- **Dependencies**: Stage 1 completion
- **Key deliverables**: Reference resolution, plan documents, apply/sync commands

### Stage 3: Advanced Features 🔮 Future
**Goal**: Label management, drift detection, safety features

| Document | Purpose | Status |
|----------|---------|---------|
| 003-* | TBD | 🔮 Not yet planned |

**Implementation Status**: Not started
- **Dependencies**: Stage 2 completion
- **Key deliverables**: Label tracking, protection features, advanced validation

## Current Implementation Priority

### ⭐ Immediate Focus: Stage 1
The immediate priority is completing Stage 1. All planning documents are ready for implementation.

**Start here**: [001-execution-plan-steps.md](001-execution-plan-steps.md) - Progress Summary

### 🎯 Entry Points for Claude Code

1. **Check progress**: Progress Summary table shows current status
2. **Find next step**: Look for first "Not Started" step with resolved dependencies  
3. **Get context**: Read step details and reference ADRs as needed
4. **Update status**: Mark step "In Progress" before starting, "Completed" when done

## Key Design Decisions (Stage 1)

- **Per-resource reference mappings** instead of global mappings (ADR-001-008)
- **Separate `ref` field** for cross-resource references (ADR-001-003) 
- **SDK type embedding** to avoid duplication (ADR-001-002)
- **Type-specific ResourceSet** for clarity and safety (ADR-001-001)

## Testing Strategy

- **Test-first approach** for business logic
- **Focus on our code**, not third-party libraries
- **Integration tests** for command execution
- **Comprehensive validation** for reference patterns

## Stage Transition Process

When moving between stages, update this file to reflect the new current active stage:

1. **Complete current stage**: Mark all steps as "Completed" in current execution plan
2. **Update Current Active Stage section**: Change stage number, links, and status
3. **Update Stage Overview**: Move completed stage to show "✅ Completed" 
4. **Create new stage documents**: Follow naming convention (002-*, 003-*, etc.)

### Example Transition to Stage 2:
```markdown
## 🎯 Current Active Stage: Stage 2 ⏳ In Progress
**Stage 2: Plan Generation & Execution**
- **Implementation Guide**: [002-execution-plan-steps.md](002-execution-plan-steps.md) ← **Start here**
```

## File Organization Best Practices

- **Stage-specific files**: All documents numbered by stage (001-*, 002-*, etc.)
- **Progress tracking**: Status fields maintained in execution plan steps
- **Cross-references**: Documents link to each other for context
- **Living documents**: Updated during implementation with notes and decisions
- **Current stage indicator**: This index.md file always shows the active stage

---

**📖 For detailed development process**: See [process.md](process.md)  
**🚀 Ready to implement?** Check the "Current Active Stage" section above for current implementation guide