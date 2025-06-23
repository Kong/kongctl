# Kongctl Development Planning

This folder contains the complete planning and implementation tracking for all kongctl features and development efforts.

## 🎯 Current Active Stage: Stage 1 ⏳ In Progress

**Stage 1: Configuration Format & Basic CLI**
- **Implementation Guide**: [001-dec-cfg-cfg-format-basic-cli/execution-plan-steps.md](001-dec-cfg-cfg-format-basic-cli/execution-plan-steps.md) ← **Start here**
- **Architecture Decisions**: [001-dec-cfg-cfg-format-basic-cli/execution-plan-adrs.md](001-dec-cfg-cfg-format-basic-cli/execution-plan-adrs.md)
- **Technical Overview**: [001-dec-cfg-cfg-format-basic-cli/execution-plan-overview.md](001-dec-cfg-cfg-format-basic-cli/execution-plan-overview.md)
- **Requirements**: [001-dec-cfg-cfg-format-basic-cli/description.md](001-dec-cfg-cfg-format-basic-cli/description.md)

**Implementation Status**: 0/7 steps completed  
**Next Step**: Step 1 - Add Verb Constants  
**Progress Tracking**: See Progress Summary in implementation guide above

## Quick Start for Implementation

1. **For Users**: [user-guide.md](user-guide.md) - Commands to direct Claude Code effectively
2. **For Claude Code**: [implementation-guide.md](implementation-guide.md) - Implementation workflow and document structure
3. **Check current progress**: Use implementation guide linked above
4. **Start implementing**: Follow the next "Not Started" step

### 🎮 Directing Claude Code

Use custom commands to streamline development:
- `/start-session` - Begin a new development session
- `/status` - Check current progress
- `/implement-next` - Implement the next step
- See [user-guide.md](user-guide.md) for all commands

## Feature Overview

### Declarative Configuration Feature

The first major feature being implemented is declarative configuration management, broken into the following stages:

#### Stage 1: Configuration Format & Basic CLI ⏳ In Progress
**Goal**: Establish YAML configuration format and integrate basic commands into kongctl

| Document | Purpose | Status |
|----------|---------|---------|
| [description.md](001-dec-cfg-cfg-format-basic-cli/description.md) | Requirements from PM | ✅ Complete |
| [execution-plan-overview.md](001-dec-cfg-cfg-format-basic-cli/execution-plan-overview.md) | Technical approach | ✅ Complete |
| [execution-plan-steps.md](001-dec-cfg-cfg-format-basic-cli/execution-plan-steps.md) | **Implementation guide** | 📋 Ready for implementation |
| [execution-plan-adrs.md](001-dec-cfg-cfg-format-basic-cli/execution-plan-adrs.md) | Architecture decisions | ✅ Complete |

**Implementation Status**: 0/7 steps completed
- **Next step**: Step 1 - Add Verb Constants
- **Estimated effort**: Small to medium implementation
- **Key deliverables**: Command stubs, YAML loading, basic validation

#### Stage 2: Plan Labels 🔮 Future
**Goal**: Implement plan label functionality

| Document | Purpose | Status |
|----------|---------|---------|
| [description.md](002-dec-cfg-plan-labels/description.md) | Requirements from PM | ✅ Complete |
| execution-plan-*.md | Implementation docs | 🔮 Not yet planned |

**Implementation Status**: Not started
- **Dependencies**: Stage 1 completion
- **Key deliverables**: Reference resolution, plan documents, apply/sync commands

#### Stage 3: Plan Execution 🔮 Future
**Goal**: Implement plan execution functionality

| Document | Purpose | Status |
|----------|---------|---------|
| [description.md](003-dec-cfg-plan-exec/description.md) | Requirements from PM | ✅ Complete |
| execution-plan-*.md | Implementation docs | 🔮 Not yet planned |

**Implementation Status**: Not started
- **Dependencies**: Stage 2 completion
- **Key deliverables**: Plan execution, validation, error handling

#### Stage 4: Multi-Resource 🔮 Future
**Goal**: Support for multiple resources in plans

| Document | Purpose | Status |
|----------|---------|---------|
| [description.md](004-dec-cfg-multi-resource/description.md) | Requirements from PM | ✅ Complete |
| execution-plan-*.md | Implementation docs | 🔮 Not yet planned |

**Implementation Status**: Not started
- **Dependencies**: Stage 3 completion
- **Key deliverables**: Multi-resource support, dependency management

## Current Implementation Priority

### ⭐ Immediate Focus: Stage 1
The immediate priority is completing Stage 1. All planning documents are ready for implementation.

**Start here**: [001-dec-cfg-cfg-format-basic-cli/execution-plan-steps.md](001-dec-cfg-cfg-format-basic-cli/execution-plan-steps.md) - Progress Summary

### 🎯 Entry Points for Claude Code

1. **Check progress**: Progress Summary table shows current status
2. **Find next step**: Look for first "Not Started" step with resolved dependencies  
3. **Get context**: Read step details and reference ADRs as needed
4. **Update status**: Mark step "In Progress" before starting, "Completed" when done

## Key Design Decisions

### Declarative Configuration (Stage 1)
- **Per-resource reference mappings** instead of global mappings (ADR-001-008)
- **Separate `ref` field** for cross-resource references (ADR-001-003) 
- **SDK type embedding** to avoid duplication (ADR-001-002)
- **Type-specific ResourceSet** for clarity and safety (ADR-001-001)

## General Testing Strategy

- **Test-first approach** for business logic
- **Focus on our code**, not third-party libraries
- **Integration tests** for command execution
- **Feature-specific validation** based on requirements

## Stage Transition Process

When moving between stages, update this file to reflect the new current active stage:

1. **Complete current stage**: Mark all steps as "Completed" in current execution plan
2. **Update Current Active Stage section**: Change stage number, links, and status
3. **Update Stage Overview**: Move completed stage to show "✅ Completed" 
4. **Create new stage documents**: Follow naming convention (002-*, 003-*, etc.)

### Example Transition to Stage 2:
```markdown
## 🎯 Current Active Stage: Stage 2 ⏳ In Progress
**Stage 2: Plan Labels**
- **Implementation Guide**: [002-dec-cfg-plan-labels/execution-plan-steps.md](002-dec-cfg-plan-labels/execution-plan-steps.md) ← **Start here**
```

## Planning Organization

- **Feature folders**: Each development effort has its own folder named after the PM's plan
- **Consistent naming**: Within each folder: `description.md`, `execution-plan-*.md`
- **Progress tracking**: Status fields maintained in execution plan steps
- **Cross-references**: Documents link to each other for context
- **Living documents**: Updated during implementation with notes and decisions
- **Current work indicator**: This index.md file always shows the active development effort

### Future Features

As new features are planned, they will be added to this index with their own folders and tracking. Examples might include:
- Authentication enhancements
- New resource types support
- Performance optimizations
- Plugin system implementation
- CLI UX improvements

---

**📖 For detailed development process**: See [implementation-guide.md](implementation-guide.md)  
**🚀 Ready to implement?** Check the "Current Active Stage" section above for current implementation guide