# Kongctl Development Planning

This folder contains the complete planning and implementation tracking for all kongctl features and development efforts.

## 🎯 Current Active Stage: Stage 5 (In Progress)

**Previous Stages Completed**:
- Stage 1: Configuration Format & Basic CLI ✅ 
- Stage 2: Plan Generation with Label Management ✅
- Stage 3: Plan Execution ✅
- Stage 4: API Resources and Multi-Resource Support ✅

**Stage 4: API Resources and Multi-Resource Support** ✅ Completed
- **Requirements**: [004-dec-cfg-multi-resource/description.md](004-dec-cfg-multi-resource/description.md) ✅ Available
- **Implementation Guide**: [004-dec-cfg-multi-resource/execution-plan-steps.md](004-dec-cfg-multi-resource/execution-plan-steps.md) ✅ Created
- **Technical Overview**: [004-dec-cfg-multi-resource/execution-plan-overview.md](004-dec-cfg-multi-resource/execution-plan-overview.md) ✅ Created
- **Architecture Decisions**: [004-dec-cfg-multi-resource/execution-plan-adrs.md](004-dec-cfg-multi-resource/execution-plan-adrs.md) ✅ Created
- **Goal**: Extend declarative configuration to support API resources with external content loading via YAML tags

**Progress**: All Steps 1-13 Complete ✅ **STAGE 4 COMPLETED**
**Next Stage**: Stage 5 - Sync Command Implementation

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
| [execution-plan-steps.md](001-dec-cfg-cfg-format-basic-cli/execution-plan-steps.md) | **Implementation guide** | ✅ Complete |
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

#### Stage 3: Plan Execution ✅ Completed
**Goal**: Implement plan execution functionality for apply command

| Document | Purpose | Status |
|----------|---------|---------|
| [description.md](003-dec-cfg-plan-exec/description.md) | Requirements from PM | ✅ Complete |
| [execution-plan-overview.md](003-dec-cfg-plan-exec/execution-plan-overview.md) | Technical approach | ✅ Complete |
| [execution-plan-steps.md](003-dec-cfg-plan-exec/execution-plan-steps.md) | **Implementation guide** | ✅ Complete |
| [execution-plan-adrs.md](003-dec-cfg-plan-exec/execution-plan-adrs.md) | Architecture decisions | ✅ Complete |

**Implementation Status**: 5a/11 original steps completed ✅ **COMPLETED**
- **Dependencies**: Stage 2 completion ✅ Met
- **Key deliverables achieved**: 
  - ✅ Mode-aware plan generation (apply vs sync modes)
  - ✅ Base executor package with progress reporting
  - ✅ Portal operations (CREATE/UPDATE)
  - ✅ Apply command with dry-run support
  - ✅ Protected resource handling with fail-fast
  - ✅ Output format support (text/json/yaml)
  - ✅ Configuration-based change detection for idempotency
  - ✅ Support for all portal fields
  - ✅ Protection label always present with true/false value
  - ✅ stdin support with interactive prompts via /dev/tty

**Note**: Remaining steps from original Stage 3 have been reorganized into Stages 5 and 6 for better focus and deliverability.

#### Stage 4: API Resources and Multi-Resource Support 🚧 In Progress
**Goal**: Support for API resources and their child resources with dependency handling

| Document | Purpose | Status |
|----------|---------|---------|
| [description.md](004-dec-cfg-multi-resource/description.md) | Requirements | ✅ Updated |
| [execution-plan-steps.md](004-dec-cfg-multi-resource/execution-plan-steps.md) | Implementation guide | ✅ Created |
| [execution-plan-overview.md](004-dec-cfg-multi-resource/execution-plan-overview.md) | Technical approach | ✅ Created |
| [execution-plan-adrs.md](004-dec-cfg-multi-resource/execution-plan-adrs.md) | Architecture decisions | ✅ Created |

**Implementation Status**: 13/13 steps completed (100%) ✅ **COMPLETED**
- Step 1: SDK Migration ✅ (Complete removal of internal SDK)
- Step 2: Resource Interfaces ✅
- Step 3: API Resource Implementation ✅
- Step 4: API Child Resource Types ✅ (Dual-mode configuration support)
- Step 5: YAML Tag System Architecture ✅
- Step 6: File Tag Resolver ✅ (With security measures and caching)
- Step 7: Tag System Integration ✅ (Dynamic base directory handling)
- Step 8: Extend planner for API resources ✅ (Full child resource support)
- Step 9: Create Integration Tests for API Resources ✅
- Step 10: Update plan command for file loading support ✅ (Enhanced extraction syntax)
- Step 11: Add cross-resource reference validation ✅ (External ID support)
- Step 12: Create Comprehensive Integration Tests ✅ (Full scenario coverage)
- Step 13: Add examples and documentation ✅ (API examples, YAML tags reference, troubleshooting guide)
- **Dependencies**: Stage 3 completion ✅ Met
- **Key deliverables**: 
  - API resource support (CREATE/UPDATE/DELETE)
  - API child resources (versions, publications, implementations)
  - YAML tag system with value extraction (!file, !file.extract)
  - External ID references for control planes and services
  - Dependency resolution and ordering
  - Cross-resource reference validation
  - Nested and separate file configuration support

#### Stage 5: Sync Command Implementation 🚧 In Progress
**Goal**: Implement full state reconciliation with DELETE operations

| Document | Purpose | Status |
|----------|---------|---------|
| [description.md](005-dec-cfg-sync/description.md) | Requirements | ✅ Created |
| [execution-plan-overview.md](005-dec-cfg-sync/execution-plan-overview.md) | Technical approach | ✅ Created |
| [execution-plan-steps.md](005-dec-cfg-sync/execution-plan-steps.md) | **Implementation guide** | ✅ Created |
| [execution-plan-adrs.md](005-dec-cfg-sync/execution-plan-adrs.md) | Architecture decisions | ✅ Created |

**Implementation Status**: 1/7 steps completed (14%)
- Step 1: Create sync command structure ✅ (command already existed)
- **Dependencies**: Stage 3 completion ✅ Met
- **Key deliverables**: 
  - Sync command with DELETE support
  - Managed resource detection
  - Protected resource handling for deletions
  - Clear warnings for destructive operations

#### Stage 6: Various Improvements and Testing 🔮 Future
**Goal**: Complete remaining improvements, UX enhancements, and comprehensive testing

| Document | Purpose | Status |
|----------|---------|---------|
| [description.md](006-dec-cfg-various/description.md) | Requirements | ✅ Created |
| execution-plan-*.md | Implementation docs | 🔮 Not yet created |

**Implementation Status**: Not started
- **Dependencies**: Stages 1-5 completion
- **Key deliverables**: 
  - Configuration discovery feature
  - Plan validation framework
  - Login command migration to Konnect-first
  - Comprehensive integration tests
  - Complete documentation updates
  - Various UX improvements
  - Extensive code review and refactoring

## Current Implementation Priority

### ⭐ Immediate Focus: Stage 4 Implementation
Stages 1-3 are complete. Stage 4 is in progress.

**Completed Stages**:
- Stage 1: Configuration Format & Basic CLI ✅ **COMPLETED**
- Stage 2: Plan Generation with Label Management ✅ **COMPLETED**
- Stage 3: Plan Execution ✅ **COMPLETED**

**Current Stage**:
- Stage 4: API Resources and Multi-Resource Support 🚧 **IN PROGRESS**
  - Step 1: Migrate to public Konnect SDK ✅ **COMPLETED** (Complete internal SDK removal)
  - Step 2: Create resource interfaces and base types ✅ **COMPLETED**
  - Step 3: Implement API resource type ✅ **COMPLETED**
  - Step 4: Implement API child resource types ✅ **COMPLETED** (Dual-mode configuration)
  - Step 5: Create YAML tag system architecture ✅ **COMPLETED**
  - Step 6: Implement file tag resolver with loading ✅ **COMPLETED** (With security and caching)
  - Step 7: Integrate tag system with resource loader ✅ **COMPLETED** (Dynamic base directory)
  - Step 8: Extend planner and executor for API resources ✅ **COMPLETED** (Full child resource support)
  - Step 9: Create Integration Tests for API Resources ✅ **COMPLETED**

**To continue implementation**: Work on Step 13 in [004-dec-cfg-multi-resource/execution-plan-steps.md](004-dec-cfg-multi-resource/execution-plan-steps.md)

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
- **Configuration-based change detection** (ADR-003-011) - only manage fields in user config
- **Protection label always present** with true/false value for consistency
- **stdin support with TTY separation** for Unix-like systems

### Multi-Resource Support (Stage 4)
- **API-centric design** - APIs as primary resources with child resources
- **Flexible configuration** - support both nested and separate file approaches
- **Reference-based dependencies** - resources reference each other by ref field
- **Topological ordering** - ensure correct creation/deletion order

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

## Planning Reorganization (2025-07-01)

The planning structure has been reorganized to improve focus and deliverability:

1. **Stage 3 marked complete** - Core apply command functionality is fully implemented
2. **Stage 4 refocused** - Changed from portal pages/specs to API resources and their children
3. **Stage 5 created** - Dedicated to sync command implementation
4. **Stage 6 created** - Various improvements, UX enhancements, and comprehensive testing

This reorganization better reflects the implementation priorities and natural grouping of features.

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
- Additional resource types (routes, services, consumers)
- Performance optimizations
- Plugin system implementation
- CLI UX improvements

---

**📖 For detailed development process**: See [implementation-guide.md](implementation-guide.md)  
**🚀 Ready to implement?** Check the "Current Active Stage" section above for current implementation guide