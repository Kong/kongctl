# Kongctl Development Planning

This folder contains the complete planning and implementation tracking for all kongctl features and development efforts.

## 🎯 Current Active Stage: Stage 8 - External Resources 🚧

**Stage 8: External Resources** 🚧 **IN PROGRESS**
- **Requirements**: [008-external-resources/description.md](008-external-resources/description.md) ✅ Complete
- **Implementation Guide**: [008-external-resources/execution-plan-steps.md](008-external-resources/execution-plan-steps.md) ✅ Complete
- **Technical Overview**: [008-external-resources/execution-plan-overview.md](008-external-resources/execution-plan-overview.md) ✅ Complete
- **Architecture Decisions**: [008-external-resources/execution-plan-adrs.md](008-external-resources/execution-plan-adrs.md) ✅ Complete
- **Goal**: Enable referencing external resources managed by other Kong tools (decK, Terraform, Kong Operator) without taking ownership

**Progress**: 2/8 steps completed (25%) 🚧 **IN PROGRESS**
- **Dependencies**: Stage 7 completion ✅ Met
- **Current Task**: Step 3 - External Resource Resolver (Step 2 completed with all 13 adapters)

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

#### Stage 5: Sync Command Implementation ✅ Completed
**Goal**: Implement full state reconciliation with DELETE operations

| Document | Purpose | Status |
|----------|---------|---------|
| [description.md](005-dec-cfg-sync/description.md) | Requirements | ✅ Created |
| [execution-plan-overview.md](005-dec-cfg-sync/execution-plan-overview.md) | Technical approach | ✅ Created |
| [execution-plan-steps.md](005-dec-cfg-sync/execution-plan-steps.md) | **Implementation guide** | ✅ Created |
| [execution-plan-adrs.md](005-dec-cfg-sync/execution-plan-adrs.md) | Architecture decisions | ✅ Created |

**Implementation Status**: 7/7 steps completed (100%) ✅ **COMPLETED**
- Step 1: Create sync command structure ✅ (command already existed)
- Step 2: Add sync mode to planner ✅ (functionality already implemented)
- Step 3: Implement DELETE operation planning ✅ (validation already inline)
- Step 4: Add portal DELETE execution ✅ (implementation already complete)
- Step 5: Add API resource DELETE execution ✅ (implementation already complete, tests added)
- Step 6: Implement confirmation prompts ✅ (reused existing confirmation functionality)
- Step 7: Integration tests ✅ (functional testing complete)
- **Dependencies**: Stage 3 completion ✅ Met
- **Key deliverables achieved**: 
  - ✅ Sync command with full DELETE support for all resource types
  - ✅ Managed resource detection using KONGCTL-managed labels
  - ✅ Protected resource handling blocks deletions
  - ✅ Clear warnings for destructive operations
  - ✅ Confirmation prompts with DELETE resource listing
  - ✅ Empty configuration support (delete all managed resources)
  - ✅ Resource monikers for clear DELETE identification
  - ✅ API version deletion support
  - ✅ Bug fixes for API publication sync issues
  - ✅ Debug logging with --log-level debug flag
  - ✅ Consistent sync behavior across all resource types

#### Stage 6: Namespace-Based Resource Management ✅ Completed
**Goal**: Enable multiple teams to safely manage their own resources within a shared Konnect organization

| Document | Purpose | Status |
|----------|---------|---------|
| [description.md](006-namespace-resource-management/description.md) | Requirements | ✅ Created |
| [execution-plan-overview.md](006-namespace-resource-management/execution-plan-overview.md) | Technical approach | ✅ Created |
| [execution-plan-steps.md](006-namespace-resource-management/execution-plan-steps.md) | **Implementation guide** | ✅ Created |
| [execution-plan-adrs.md](006-namespace-resource-management/execution-plan-adrs.md) | Architecture decisions | ✅ Created |

**Implementation Status**: 15/15 steps completed (100%) ✅ **COMPLETED**
- **Dependencies**: Stage 5 completion ✅ Met
- **Completed deliverables**: 
  - ✅ Namespace field in kongctl section
  - ✅ File-level defaults via _defaults.kongctl.namespace  
  - ✅ Namespace-based resource filtering
  - ✅ Multi-namespace operations in single command
  - ✅ Namespace isolation during sync
  - ✅ Clear namespace visibility in output
  - ✅ Namespace validation
  - ✅ Integration tests
  - ✅ Documentation and examples
- **Key achievements**:
  - Teams can now safely manage resources in isolation
  - Clear namespace visibility in all commands
  - Comprehensive examples for various use cases

#### Stage 7: Testing, Documentation, and Core Improvements ✅ Completed
**Goal**: Complete essential testing, documentation, and core improvements for production readiness

| Document | Purpose | Status |
|----------|---------|---------|
| [description.md](007-dec-cfg-various/description.md) | Requirements | ✅ Complete |
| [execution-plan-overview.md](007-dec-cfg-various/execution-plan-overview.md) | Technical approach | ✅ Complete |
| [execution-plan-steps.md](007-dec-cfg-various/execution-plan-steps.md) | **Implementation guide** | ✅ Complete |
| [execution-plan-adrs.md](007-dec-cfg-various/execution-plan-adrs.md) | Architecture decisions | ✅ Complete |

**Implementation Status**: 15/15 steps completed (100%) ✅ **COMPLETED**
- **Dependencies**: Stages 1-6 completion ✅ Met
- **Key deliverables achieved**:
  - ✅ Login command migration to Konnect-first
  - ✅ Gateway product renamed to on-prem
  - ✅ Get command for portals with Konnect-first pattern
  - ✅ Get command for APIs with Konnect-first pattern
  - ✅ Get command for auth strategies with Konnect-first pattern
  - ✅ All imperative commands (get, list, create, del) support Konnect-first pattern
  - ✅ Comprehensive documentation with minimal help text and extended help system
  - ✅ Apply command integration tests infrastructure (SDK factory mocking enabled)
  - ✅ Sync command integration tests with comprehensive test scenarios
  - ✅ Error scenario integration tests covering API errors, network failures, protection violations, and configuration validation
  - ✅ Enhanced error messages with resource context, actionable hints, and proper portal creation error handling
  - ✅ Improved plan summary display with visual separators, field-level changes, protected resource warnings, and comprehensive statistics
  - ✅ Progress indicators for long operations with rich console reporting and namespace support
  - ✅ Dump command migration to public SDK with verified functionality
  - ✅ Code quality and refactoring completed through previous sessions

#### Stage 8: External Resources 🚧 In Progress
**Goal**: Enable referencing external resources managed by other Kong tools (decK, Terraform, Kong Operator) without taking ownership

| Document | Purpose | Status |
|----------|---------|---------|
| [description.md](008-external-resources/description.md) | Requirements | ✅ Complete |
| [execution-plan-overview.md](008-external-resources/execution-plan-overview.md) | Technical approach | ✅ Complete |
| [execution-plan-steps.md](008-external-resources/execution-plan-steps.md) | **Implementation guide** | ✅ Complete |
| [execution-plan-adrs.md](008-external-resources/execution-plan-adrs.md) | Architecture decisions | ✅ Complete |

**Implementation Status**: 2/8 steps completed (25%) 🚧 **IN PROGRESS**
- **Dependencies**: Stage 7 completion ✅ Met
- **Completed Steps**:
  - ✅ Step 1: Schema and Configuration - External resource types with Resolution naming theme
  - ✅ Step 2: Resource Type Registry - All 13 adapters fully implemented with SDK integration
- **Current Task**: Step 3 - External Resource Resolver implementation
- **Key deliverables planned**:
  - ✅ Schema and configuration for external resource definitions (Resolution theme)
  - Resource type registry mapping to SDK operations
  - External resource resolver with selector-based matching
  - Reference resolution integration for implicit ID resolution
  - Dependency handling for parent-child resource relationships
  - Planning phase integration with pre-resolution
  - Comprehensive error handling and validation
  - Integration testing and documentation

**Implementation Notes**:
- Used "Resolution" naming theme (ResolutionMetadata, ResolutionRegistry, ResolutionAdapter) to avoid stuttering and clarify purpose
- External resources do not have Kongctl metadata as they cannot be protected/namespaced (owned by external system)
- Registry expanded to include all portal and API child resource types
- Added support for ce_service (core entity service) with control_plane parent requirement
- Step 2 complete with all 13 adapters fully implemented:
  - Top-level: portal, api, control_plane, application_auth_strategy
  - Child resources: ce_service, portal children (4 types), API children (4 types)
  - All adapters have GetByID and GetBySelector methods with SDK integration

#### Future Work 💭
**Goal**: Capture ideas for future enhancements

| Document | Purpose | Status |
|----------|---------|---------|
| [README.md](future/README.md) | Future work index | ✅ Created |
| [configuration-discovery.md](future/configuration-discovery.md) | Show unmanaged fields | ✅ Created |

**Note**: Features moved here are nice-to-have but not critical for core functionality

## Current Implementation Priority

### 🎯 Current Status

**Stage 8: External Resources - In Progress 🚧**

**Completed Stages**:
- Stage 1: Configuration Format & Basic CLI ✅ **COMPLETED**
- Stage 2: Plan Generation with Label Management ✅ **COMPLETED**
- Stage 3: Plan Execution ✅ **COMPLETED**
- Stage 4: API Resources and Multi-Resource Support ✅ **COMPLETED**
- Stage 5: Sync Command Implementation ✅ **COMPLETED**
- Stage 6: Namespace-Based Resource Management ✅ **COMPLETED**
- Stage 7: Testing, Documentation, and Core Improvements ✅ **COMPLETED**

**Current Stage**:
- Stage 8: External Resources 🚧 **IN PROGRESS**

**Key Achievements**:
- Complete declarative configuration management system
- Support for Portals, APIs, and all child resources
- Full sync/apply/plan/diff command suite
- Namespace-based resource isolation
- Comprehensive error handling and UX improvements
- Konnect-first command structure
- Extensive integration test coverage
- Production-ready implementation

**Current Work**: 
- Step 1 completed: Schema and configuration with Resolution naming theme
- Step 2 completed: All 13 resource type adapters fully implemented with SDK integration
- Ready for Step 3: External Resource Resolver implementation
- External resources will enable integration with other Kong declarative tools

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

## Planning Reorganization (2025-07-24)

Additional reorganization to introduce namespace-based resource management:

1. **Stage 6 redefined** - Changed from "Various Improvements" to "Namespace-Based Resource Management"
2. **Stage 7 created** - Moved "Various Improvements and Testing" to Stage 7
3. **Namespace feature prioritized** - Addresses critical multi-team use case

This change reflects the importance of enabling multiple teams to safely manage resources within a shared Konnect organization.

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