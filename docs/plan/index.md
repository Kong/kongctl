# Kongctl Development Planning

This folder contains the complete planning and implementation tracking for all kongctl features and development efforts.

## 🎯 Current Active Stage: Stage 3 (In Progress - Step 5/13 Complete)

**Previous Stages Completed**:
- Stage 1: Configuration Format & Basic CLI ✅ 
- Stage 2: Plan Generation with Label Management ✅

**Stage 3: Plan Execution** 🚧 In Progress
- **Requirements**: [003-dec-cfg-plan-exec/description.md](003-dec-cfg-plan-exec/description.md) ✅ Available
- **Implementation Guide**: [003-dec-cfg-plan-exec/execution-plan-steps.md](003-dec-cfg-plan-exec/execution-plan-steps.md) ✅ **Start here**
- **Technical Overview**: [003-dec-cfg-plan-exec/execution-plan-overview.md](003-dec-cfg-plan-exec/execution-plan-overview.md) ✅ Complete
- **Architecture Decisions**: [003-dec-cfg-plan-exec/execution-plan-adrs.md](003-dec-cfg-plan-exec/execution-plan-adrs.md) ✅ Complete
- **Goal**: Execute plans generated in Stage 2, applying changes to Konnect

**Next Step**: Implement Step 5a - Fix Idempotency Issue

**Important Decision**: Moving to configuration-based change detection to fix idempotency issues (ADR-003-011)

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

#### Stage 1: Configuration Format & Basic CLI ✅ Completed
**Goal**: Establish YAML configuration format and integrate basic commands into kongctl

| Document | Purpose | Status |
|----------|---------|---------|
| [description.md](001-dec-cfg-cfg-format-basic-cli/description.md) | Requirements from PM | ✅ Complete |
| [execution-plan-overview.md](001-dec-cfg-cfg-format-basic-cli/execution-plan-overview.md) | Technical approach | ✅ Complete |
| [execution-plan-steps.md](001-dec-cfg-cfg-format-basic-cli/execution-plan-steps.md) | **Implementation guide** | 📋 Ready for implementation |
| [execution-plan-adrs.md](001-dec-cfg-cfg-format-basic-cli/execution-plan-adrs.md) | Architecture decisions | ✅ Complete |

**Implementation Status**: 7/7 steps completed (100%) ✅ **COMPLETED**
- **Completed**: All steps implemented and tested
- **Key deliverables achieved**: 
  - Command stubs for plan, sync, diff, export
  - YAML loading with multi-file support
  - Basic validation with fail-fast duplicate detection
  - Plan command integration with loader

#### Stage 2: Plan Generation with Label Management ✅ Completed
**Goal**: Build the planner that compares current vs desired state and generates plans with CREATE/UPDATE operations

| Document | Purpose | Status |
|----------|---------|---------|
| [description.md](002-dec-cfg-plan-labels/description.md) | Requirements from PM | ✅ Complete |
| [execution-plan-overview.md](002-dec-cfg-plan-labels/execution-plan-overview.md) | Technical approach | ✅ Complete |
| [execution-plan-steps.md](002-dec-cfg-plan-labels/execution-plan-steps.md) | **Implementation guide** | ✅ Complete |
| [execution-plan-adrs.md](002-dec-cfg-plan-labels/execution-plan-adrs.md) | Architecture decisions | ✅ Complete |

**Implementation Status**: 11/11 steps completed (100%) ✅ **COMPLETED**
- **Completed**: All steps implemented and tested
- **Key deliverables achieved**: 
  - ✅ Konnect API integration for fetching current portal state
  - ✅ Label management system (KONGCTL/managed, KONGCTL/config-hash, KONGCTL/protected)
  - ✅ Plan generation for CREATE and UPDATE operations
  - ✅ Plan serialization to JSON format
  - ✅ Reference resolution and dependency management
  - ✅ Protection status change handling
  - ✅ Plan command integration
  - ✅ Diff command with text/JSON/YAML output formats
  - ✅ Integration tests with dual-mode SDK support (mock/real)

#### Stage 3: Plan Execution ⏳ In Progress
**Goal**: Implement plan execution functionality

| Document | Purpose | Status |
|----------|---------|---------|
| [description.md](003-dec-cfg-plan-exec/description.md) | Requirements from PM | ✅ Complete |
| [execution-plan-overview.md](003-dec-cfg-plan-exec/execution-plan-overview.md) | Technical approach | ✅ Complete |
| [execution-plan-steps.md](003-dec-cfg-plan-exec/execution-plan-steps.md) | **Implementation guide** | ✅ Complete |
| [execution-plan-adrs.md](003-dec-cfg-plan-exec/execution-plan-adrs.md) | Architecture decisions | ✅ Complete |

**Implementation Status**: 5/13 steps completed (38%) - In Progress
- **Dependencies**: Stage 2 completion ✅ Met
- **Key deliverables**: 
  - Mode-aware plan generation (apply vs sync)
  - Separate apply and sync commands
  - Plan execution with progress reporting
  - Protected resource handling with fail-fast
  - Dry-run mode support
  - Output format support (text/json/yaml)
  - Konnect-first login migration
  - **NEW**: Configuration-based change detection for idempotency
  - **NEW**: Progressive configuration discovery

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

### ⭐ Immediate Focus: Stage 3 Implementation
Stages 1 and 2 are complete. Stage 3 planning is complete and ready for implementation.

**Completed Stages**:
- Stage 1: Configuration Format & Basic CLI ✅ **COMPLETED**
- Stage 2: Plan Generation with Label Management ✅ **COMPLETED**

**Current Stage**:
- Stage 3: Plan Execution ⏳ **In Progress** (5/13 steps)

**To begin implementation**: Start with Step 5a in [003-dec-cfg-plan-exec/execution-plan-steps.md](003-dec-cfg-plan-exec/execution-plan-steps.md)

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

### Plan Generation (Stage 2)
- **Label-based resource management** with KONGCTL/managed, KONGCTL/config-hash, KONGCTL/protected
- **Semantic change IDs** for human-readable plan identification
- **Minimal field storage** to reduce plan size and focus on actual changes
- **Protection status isolation** from regular field updates
- **Dependency-ordered execution** for proper resource creation order

### Plan Execution (Stage 3)
- **Mode-aware plan generation** with separate apply and sync modes
- **Fail-fast protection handling** - planning fails if protected resources would be modified
- **Execution-time validation** for protection status changes
- **Consistent confirmation prompts** requiring 'yes' for both commands
- **Output format support** (text/json/yaml) for CI/CD integration
- **Konnect-first login** migration for consistency
- **Configuration-based change detection** (ADR-003-011) - only manage fields in user config
- **Progressive configuration discovery** (ADR-003-012) - show unmanaged fields to users

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