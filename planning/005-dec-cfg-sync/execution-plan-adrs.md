# Stage 5: Sync Command Implementation - Architecture Decision Records

## ADR-005-001: Separate Sync Command for Declarative Deletions

### Status
Proposed

### Context
We need a way to perform full state reconciliation including deletion of
resources that exist in the system but not in the configuration. This is
specifically for declarative configuration management, separate from imperative
delete commands like `kongctl delete control-plane`.

### Decision
Create a separate `sync` command for declarative configuration that performs
full state reconciliation including DELETE operations. The `apply` command
remains safe for declarative configuration and only performs CREATE/UPDATE
operations. This distinction is only for declarative configuration management -
imperative commands like `delete` continue to work as expected.

### Consequences
**Positive:**
- Clear separation of safe vs destructive declarative operations
- Explicit user intent required for declarative deletions
- Easier to reason about in CI/CD pipelines
- Follows established patterns (Terraform apply vs destroy)
- Doesn't affect imperative delete commands

**Negative:**
- Two declarative commands to learn instead of one
- Potential confusion about when to use which for declarative config

**Alternatives Considered:**
- Add --delete flag to apply command
- Always perform deletions in apply

## ADR-005-002: Protection Validation at Planning Time

### Status
Proposed

### Context
Protected resources should not be deleted. We need to decide when to validate
this constraint.

### Decision
Validate protection status during plan generation. If any protected resource
would be deleted, fail the planning phase with a clear error message.

### Consequences
**Positive:**
- Fail fast - users know immediately what's blocking
- No partial execution with protected resources
- Clear guidance on how to proceed

**Negative:**
- Cannot generate a plan showing protected deletions
- Users must unprotect resources before seeing full plan

**Alternatives Considered:**
- Validate at execution time
- Skip protected resources with warnings

## ADR-005-003: Managed Resource Requirement for Deletion

### Status
Proposed

### Context
We need to ensure kongctl only deletes resources it manages, not manually
created resources.

### Decision
Only delete resources that have the KONGCTL/managed label. Resources without
this label are ignored during sync operations.

### Consequences
**Positive:**
- Prevents accidental deletion of manual resources
- Consistent with label-based management approach
- Clear ownership model

**Negative:**
- Resources created before label implementation won't be deleted
- Manual label manipulation could cause issues

**Alternatives Considered:**
- Delete all resources not in configuration
- Add --force flag to delete unmanaged resources

## ADR-005-004: Confirmation Prompt Design

### Status
Proposed

### Context
DELETE operations are destructive and irreversible. We need appropriate
safeguards.

### Decision
In text output mode, show:
1. Standard plan summary
2. Explicit WARNING section listing resources to delete
3. Require typing "yes" to confirm (not just y or Y)
4. Support --auto-approve for automation

### Consequences
**Positive:**
- Clear understanding of destructive operations
- Deliberate action required
- Automation still possible with flag

**Negative:**
- Extra step in workflow
- Cannot use simple y/n confirmation

**Alternatives Considered:**
- No confirmation (rely on dry-run)
- Simple y/n prompt
- Require resource names in confirmation

## ADR-005-005: Not-Found Error Handling

### Status
Proposed

### Context
Resources might be deleted externally between plan generation and execution.

### Decision
Treat "not found" errors during DELETE operations as successful completion.
Report as "skipped - already deleted" rather than an error.

### Consequences
**Positive:**
- Idempotent operations
- Graceful handling of external changes
- Desired end state achieved

**Negative:**
- Might hide other issues
- Less visibility into external changes

**Alternatives Considered:**
- Treat as error
- Add warning but continue
- Re-fetch state before execution

## ADR-005-006: Deletion Order

### Status
Proposed

### Context
Resources have dependencies. Parents cannot be deleted before children.

### Decision
Delete resources in reverse dependency order:
1. API child resources (implementations, publications)
2. API versions
3. APIs
4. Portals

Within each level, order doesn't matter.

### Consequences
**Positive:**
- Respects resource dependencies
- Prevents cascade deletion issues
- Predictable behavior

**Negative:**
- More complex than arbitrary order
- Must maintain dependency knowledge

**Alternatives Considered:**
- Rely on backend cascade deletion
- Delete in any order with retry
- User-specified deletion order

## ADR-005-007: Sync Mode Plan Reuse

### Status
Proposed

### Context
Users might want to review a sync plan before execution, similar to apply.

### Decision
Allow sync command to accept a plan file generated in sync mode. The plan
command accepts a --mode flag to generate sync-mode plans.

### Consequences
**Positive:**
- Consistent with apply workflow
- Allows plan review and approval
- Supports GitOps workflows

**Negative:**
- Plans must track their mode
- Potential confusion if wrong plan used

**Alternatives Considered:**
- No plan file support for sync
- Separate plan-sync command
- Auto-detect mode from plan content