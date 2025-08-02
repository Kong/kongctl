# Investigation Report: Delete Portals Command Implementation

## Executive Summary

This report investigates the kongctl codebase to understand how to implement the `delete portals` command. The investigation reveals that the delete command infrastructure is already in place, but the portal-specific delete functionality is missing. The implementation should follow established patterns from other delete commands (e.g., delete gateway control-plane) and incorporate features from apply/sync commands for confirmation prompts.

## Investigation Findings

### 1. Delete Command Structure

#### Current State
- The delete command is located at: `/internal/cmd/root/verbs/del/`
- Main delete command file: `del.go`
- Gateway-specific delete commands: `gateway.go`
- The delete command supports both direct usage (`delete gateway control-plane`) and explicit product usage (`delete konnect gateway control-plane`)

#### Key Components
```go
// Main delete command registration in del.go
func NewDeleteCmd() (*cobra.Command, error) {
    cmd := &cobra.Command{
        Use:     "delete",
        Aliases: []string{"d", "D", "del", "rm", "DEL", "RM"},
        // ...
    }
    // Adds konnect subcommand
    // Adds gateway command directly for Konnect-first pattern
}
```

### 2. Portal Command Structure

#### Current Implementation
- Portal commands are located at: `/internal/cmd/root/products/konnect/portal/`
- Main portal command file: `portal.go`
- Currently implements: `Get` and `List` verbs only
- Missing: `Delete` verb implementation

#### Portal Command Registration Pattern
```go
func NewPortalCmd(verb verbs.VerbValue, ...) (*cobra.Command, error) {
    switch verb {
    case verbs.Get:
        return newGetPortalCmd(...)
    case verbs.List:
        return newGetPortalCmd(...)
    case verbs.Delete:
        return &baseCmd, nil // Currently returns empty command
    }
}
```

### 3. Delete Control Plane Implementation (Reference)

The delete control plane command provides a good reference pattern:

#### Location
`/internal/cmd/root/products/konnect/gateway/controlplane/deleteControlPlane.go`

#### Key Implementation Details
1. Accepts ID as argument
2. Uses SDK to perform deletion
3. Outputs result based on output format
4. No confirmation prompt (needs to be added)

```go
func (c *deleteControlPlaneCmd) run(helper cmd.Helper) error {
    id := helper.GetArgs()[0]
    sdk, err := helper.GetKonnectSDK(cfg, logger)
    res, err := sdk.GetControlPlaneAPI().DeleteControlPlane(ctx, id)
    // Output handling...
}
```

### 4. Portal SDK Methods

#### Delete Portal Operation
Location: `/vendor/github.com/Kong/sdk-konnect-go/models/operations/deleteportal.go`

Key features:
- Accepts `PortalID` (string)
- Optional `Force` parameter (true/false)
- If `Force=true`, automatically deletes all API publications
- If `Force=false`, deletion only succeeds if no APIs are published

```go
type DeletePortalRequest struct {
    PortalID string           // ID of the portal
    Force    *QueryParamForce // Optional force deletion
}
```

### 5. Portal Name Resolution Pattern

From `getPortal.go`, the pattern for resolving portal names to IDs:

1. Check if argument is UUID using regex
2. If not UUID, search by name using ListPortals with pagination
3. Filter results by name since SDK doesn't support name filtering

```go
isUUID, _ := regexp.MatchString(`^[a-fA-F0-9]{8}-...`, id)
if !isUUID {
    portal, err := runListByName(id, sdk.GetPortalAPI(), helper, cfg)
    // Use portal.ID for deletion
}
```

### 6. Confirmation Prompt Implementation

From declarative commands (`/internal/declarative/common/prompts.go`):

#### Key Features
1. `ConfirmExecution` function shows warnings for DELETE operations
2. Groups deletions by namespace
3. Requires user to type "yes" to confirm
4. Handles interrupt signals gracefully
5. Special handling for stdin input (uses /dev/tty for interactive prompt)

#### Auto-Approve Flag Pattern
From `sync.go` and `apply.go`:
```go
cmd.Flags().Bool("auto-approve", false, "Skip confirmation prompt")

// In execution:
if !dryRun && !autoApprove {
    if !common.ConfirmExecution(...) {
        return fmt.Errorf("delete cancelled")
    }
}
```

### 7. Required Files to Create/Modify

1. **New file**: `/internal/cmd/root/products/konnect/portal/deletePortal.go`
   - Implement delete portal command following control plane pattern
   - Add name-to-ID resolution
   - Add confirmation prompt with --auto-approve flag

2. **Modify**: `/internal/cmd/root/products/konnect/portal/portal.go`
   - Update switch statement to return delete command for `verbs.Delete`

3. **Modify**: `/internal/cmd/root/verbs/del/del.go`
   - Update help text to include portal deletion example

## Implementation Recommendations

### 1. Delete Portal Command Structure

```go
type deletePortalCmd struct {
    *cobra.Command
    force       bool
    autoApprove bool
}
```

### 2. Implementation Flow

1. Validate arguments (0 or 1 argument for name/ID)
2. Resolve name to ID if necessary
3. Show confirmation prompt (unless --auto-approve)
4. Call SDK DeletePortal with appropriate force flag
5. Handle response and output

### 3. Confirmation Prompt

For consistency with other tools, the prompt should:
- Show portal name and ID
- Warn if APIs are published (suggest --force flag)
- Require "yes" confirmation
- Support --auto-approve to skip

### 4. Force Flag Handling

- Add `--force` flag to force deletion even with published APIs
- If not set and portal has published APIs, deletion will fail
- Prompt should mention this when applicable

### 5. Help Text Updates

Update delete command examples to include:
```
# Delete a portal by ID
kongctl delete portal <id>

# Delete a portal by name
kongctl delete portal <name>

# Force delete a portal with published APIs
kongctl delete portal <id> --force

# Delete without confirmation
kongctl delete portal <id> --auto-approve
```

## Conclusion

The kongctl codebase has all the necessary infrastructure to implement the delete portals command. The implementation should follow the established patterns from the delete control-plane command while incorporating the confirmation prompt functionality from the apply/sync commands. The SDK provides the necessary DeletePortal method with force deletion support.