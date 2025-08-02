# Stage 5: Sync Command Implementation - Technical Overview

## Overview
This stage implements the sync command, which performs full state
reconciliation including DELETE operations. The sync command is the only
command that can delete resources, making it critical for maintaining true
desired state.

## Technical Approach

### 1. Command Structure
The sync command follows the same pattern as apply but with sync mode:
- Accepts plan file or configuration directory
- Supports dry-run, auto-approve, and output formats
- Uses the same executor infrastructure
- Distinct behavior: includes DELETE operations

### 2. Plan Generation Mode
Extend existing planner to support sync mode:
```go
type PlanMode string

const (
    PlanModeApply PlanMode = "apply"  // CREATE/UPDATE only
    PlanModeSync  PlanMode = "sync"   // CREATE/UPDATE/DELETE
)
```

### 3. DELETE Operation Support
Add DELETE support to executor:
- Portal DELETE operations
- API resource DELETE operations
- Child resource DELETE operations
- Protection validation
- Managed resource verification

### 4. Confirmation Flow
Enhanced confirmation for destructive operations:
- Clear DELETE warnings
- List resources to be deleted
- Require explicit "yes" confirmation
- Auto-approve flag for automation

### 5. Resource Deletion Order
Proper dependency handling for deletions:
- Delete child resources before parents
- Delete in reverse dependency order
- Handle missing resources gracefully

## Architecture Decisions

### Why Separate Sync Command?
- Clear distinction between safe (apply) and destructive (sync) operations
- Explicit user intent required for deletions
- Easier to reason about in CI/CD pipelines
- Follows industry patterns (Terraform apply vs destroy)

### Protection Handling
- Protected resources cannot be deleted
- Must explicitly set protected: false first
- Fail-fast during planning if protected resources would be deleted
- Clear error messages guide users

### Managed Resource Requirement
- Only delete resources with KONGCTL/managed label
- Prevents accidental deletion of manually created resources
- Consistent with overall label-based management approach

## Implementation Order

1. **Command Setup** - Create sync command structure
2. **Mode Support** - Add sync mode to planner
3. **DELETE Planning** - Generate DELETE operations in plan
4. **Portal Deletions** - Implement portal DELETE execution
5. **API Deletions** - Implement API resource DELETE execution
6. **Confirmation UI** - Enhanced prompts with warnings
7. **Integration** - Full sync workflow testing

## Testing Strategy

### Unit Tests
- Plan generation in sync mode
- DELETE operation execution
- Protection validation
- Managed resource checks

### Integration Tests
- Full sync workflow
- Mixed CREATE/UPDATE/DELETE operations
- Protected resource handling
- Dry-run behavior
- Output format verification

### Edge Cases
- Already deleted resources
- Protected resources in deletion set
- Unmanaged resources
- Dependency ordering
- Partial failures

## Success Criteria
1. Sync command successfully deletes unmatched resources
2. Protected resources block deletion with clear errors
3. Only managed resources are deleted
4. Proper confirmation flow with warnings
5. Auto-approve works for automation
6. All output formats supported
7. Dry-run shows deletions without executing