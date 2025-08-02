# Flow Report: Delete Portals Command Implementation

## Executive Summary

This report maps the complete execution flow for implementing the `delete portals` command in kongctl. The analysis traces the code path from CLI invocation through command registration, argument processing, SDK calls, and output formatting. The report also identifies patterns for name resolution, confirmation prompts, and error handling that should be incorporated into the implementation.

## Command Execution Flow

### 1. Entry Point and Command Registration

```
main.go:main()
  └─> root.Execute(ctx, iostreams, buildInfo)
      └─> rootCmd.ExecuteContext(ctx)
          └─> addCommands() registers all verb commands
              └─> del.NewDeleteCmd() creates delete command
```

### 2. Delete Command Structure

The delete command (`/internal/cmd/root/verbs/del/del.go`) creates a hierarchical structure:

```
delete (verb)
  ├─> konnect (product - explicit usage)
  │    ├─> gateway
  │    ├─> portal  <-- Currently returns empty command
  │    ├─> api
  │    └─> auth-strategy
  │
  └─> gateway (direct Konnect-first pattern)
```

**Key Implementation Details:**
- Delete command has aliases: `d`, `D`, `del`, `rm`, `DEL`, `RM`
- Sets verb in context via `PersistentPreRun`
- Supports both explicit (`delete konnect portal`) and Konnect-first (`delete portal`) patterns

### 3. Konnect Product Command Flow

The konnect command (`/internal/cmd/root/products/konnect/konnect.go`) handles product-specific setup:

```
konnect.NewKonnectCmd(verb)
  ├─> Sets up PersistentPreRunE:
  │    ├─> Sets Product context value
  │    ├─> Sets SDK factory
  │    └─> Binds flags (PAT, base URL, etc.)
  │
  └─> Adds product commands:
       ├─> gateway.NewGatewayCmd()
       ├─> portal.NewPortalCmd()  <-- Line 155
       ├─> api.NewAPICmd()
       └─> authstrategy.NewAuthStrategyCmd()
```

### 4. Portal Command Registration

The portal command (`/internal/cmd/root/products/konnect/portal/portal.go`) currently handles only Get and List verbs:

```go
func NewPortalCmd(verb verbs.VerbValue, ...) (*cobra.Command, error) {
    switch verb {
    case verbs.Get:
        return newGetPortalCmd(...)
    case verbs.List:
        return newGetPortalCmd(...)
    case verbs.Delete:
        return &baseCmd, nil  // Currently returns empty command
    }
}
```

### 5. Direct Konnect-First Pattern

For better UX, the delete command also supports direct usage without "konnect":

```
del/gateway.go:NewDirectGatewayCmd()
  ├─> Creates gateway command with Konnect context
  ├─> Adds Konnect-specific flags (PAT, base URL)
  └─> Sets up SDK factory in preRunE
```

This pattern should be extended for portals to support `delete portal <id>` directly.

## Delete Operation Flow (Based on Control Plane)

### 1. Delete Control Plane Implementation

The delete control plane command provides the reference pattern:

```go
deleteControlPlane.go flow:
  ├─> validate() - Basic validation
  ├─> run() - Main execution:
  │    ├─> Get ID from args[0]
  │    ├─> Get SDK from helper
  │    ├─> Call SDK: DeleteControlPlane(ctx, id)
  │    ├─> Handle errors with proper attributes
  │    └─> Format and output response
  └─> No confirmation prompt (needs addition)
```

### 2. Portal SDK Delete Operation

The SDK provides the delete method with force option:

```go
// From vendor/.../operations/deleteportal.go
type DeletePortalRequest struct {
    PortalID string           // Portal ID
    Force    *QueryParamForce // Optional force deletion
}

// Force=true: Deletes portal and all API publications
// Force=false: Fails if APIs are published
```

## Name Resolution Pattern

The getPortal command demonstrates name-to-ID resolution:

```go
getPortal.go name resolution flow:
  ├─> Check if argument is UUID (regex pattern)
  ├─> If UUID: Use directly
  └─> If not UUID:
       └─> runListByName():
            ├─> Paginate through all portals
            ├─> Filter by exact name match
            └─> Return portal or error
```

**UUID Regex Pattern:**
```regex
^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$
```

## Confirmation Prompt Pattern

From declarative commands (apply/sync):

### 1. Flag Registration
```go
cmd.Flags().Bool("auto-approve", false, "Skip confirmation prompt")
```

### 2. Prompt Logic
```go
if !dryRun && !autoApprove {
    // Handle stdin special case for /dev/tty
    if !ConfirmExecution(plan, stdout, stderr, stdin) {
        return fmt.Errorf("delete cancelled")
    }
}
```

### 3. ConfirmExecution Function
```go
common/prompts.go:ConfirmExecution()
  ├─> Shows WARNING for DELETE operations
  ├─> Groups deletions by namespace
  ├─> Prompts: "Do you want to continue? Type 'yes' to confirm:"
  ├─> Handles interrupt signals gracefully
  └─> Returns true only if user types "yes"
```

## Error Handling Pattern

Consistent error handling across commands:

```go
// SDK errors are converted to attributes
attrs := cmd.TryConvertErrorToAttrs(err)
return cmd.PrepareExecutionError("Failed to delete Portal", err, cmd, attrs...)
```

## Output Formatting Pattern

All commands support multiple output formats:

```go
// Get output format from helper
outType, err := helper.GetOutputFormat()

// Create printer based on format
printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
defer printer.Flush()

// Print response
printer.Print(response)
```

Supported formats:
- TEXT (default)
- JSON
- YAML

## Implementation Requirements for Delete Portal

### 1. Create deletePortal.go

Location: `/internal/cmd/root/products/konnect/portal/deletePortal.go`

Structure:
```go
type deletePortalCmd struct {
    *cobra.Command
    force       bool
    autoApprove bool
}
```

### 2. Implementation Flow

```
deletePortal command flow:
  ├─> Validate arguments (0 or 1 for name/ID)
  ├─> Resolve name to ID if necessary
  ├─> Get portal details (for confirmation display)
  ├─> Show confirmation prompt (unless --auto-approve)
  │    ├─> Display portal name and ID
  │    ├─> Warn if APIs are published
  │    └─> Suggest --force if needed
  ├─> Call SDK DeletePortal with force flag
  └─> Handle response and output
```

### 3. Update portal.go

Update the switch statement to return delete command:
```go
case verbs.Delete:
    return newDeletePortalCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
```

### 4. Add Direct Delete Portal Support

Create a new file for direct portal delete (similar to gateway.go) to support:
```bash
kongctl delete portal <id|name>
```

### 5. Help Text Updates

Update delete command examples to include portal operations:
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

## Component Interaction Diagram

```
CLI Input: "kongctl delete portal my-portal --force"
    │
    ├─> main.go
    │     └─> root.Execute()
    │
    ├─> root.go
    │     └─> rootCmd.ExecuteContext()
    │           └─> delete command registered
    │
    ├─> del/del.go (delete verb)
    │     └─> PersistentPreRun: Set verb context
    │
    ├─> konnect/konnect.go (product)
    │     └─> PersistentPreRunE: Set product context, SDK factory
    │
    ├─> portal/portal.go
    │     └─> NewPortalCmd(verbs.Delete)
    │
    ├─> portal/deletePortal.go
    │     ├─> Parse arguments
    │     ├─> Name resolution (if needed)
    │     ├─> Confirmation prompt
    │     └─> SDK call
    │
    └─> SDK: DeletePortal(ctx, id, force)
          └─> HTTP DELETE /v1/portals/{id}?force=true
```

## Key Patterns and Best Practices

1. **Command Helper Pattern**: Use `cmd.BuildHelper()` for consistent access to config, logger, SDK, etc.

2. **Error Handling**: Always wrap SDK errors with `PrepareExecutionError` for consistent formatting

3. **Output Formatting**: Support TEXT, JSON, and YAML outputs using `cli.Format`

4. **Flag Binding**: Use Viper for configuration binding with proper config paths

5. **Context Values**: Pass product, verb, and SDK factory through context

6. **Confirmation Prompts**: Always include for destructive operations unless --auto-approve is set

7. **Name Resolution**: Support both UUID and name inputs for better UX

## Conclusion

The kongctl codebase provides a well-structured foundation for implementing the delete portals command. The implementation should follow established patterns from the delete control-plane command while incorporating name resolution from getPortal and confirmation prompts from the declarative commands. The modular architecture makes it straightforward to add new operations while maintaining consistency across the CLI.