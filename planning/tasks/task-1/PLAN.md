# Implementation Plan: Delete Portals Command

## Overview

This plan details the implementation of the `delete portals` command for kongctl. The command will support deletion by both portal ID and name, include confirmation prompts with an --auto-approve flag, and follow established patterns from existing delete commands.

## Files to Create/Modify

### 1. New Files to Create

#### `/internal/cmd/root/products/konnect/portal/deletePortal.go`
Primary implementation file for the delete portal command.

#### `/internal/cmd/root/verbs/del/portal.go`
Direct portal delete support for Konnect-first pattern (enables `kongctl delete portal <id>`).

### 2. Files to Modify

#### `/internal/cmd/root/products/konnect/portal/portal.go`
Update the switch statement to return the delete command for `verbs.Delete`.

#### `/internal/cmd/root/verbs/del/del.go`
Update help text to include portal deletion examples.

## Implementation Details

### 1. deletePortal.go Implementation

```go
package portal

import (
    "context"
    "fmt"
    "os"
    "regexp"
    "strings"

    "github.com/Kong/kongctl/internal/cli"
    "github.com/Kong/kongctl/internal/cmd"
    "github.com/Kong/kongctl/internal/config"
    "github.com/Kong/kongctl/internal/iostreams"
    "github.com/Kong/kongctl/internal/konnect"
    "github.com/Kong/kongctl/internal/log"
    "github.com/Kong/kongctl/internal/verbs"
    "github.com/Kong/sdk-konnect-go/models/components"
    "github.com/Kong/sdk-konnect-go/models/operations"
    "github.com/spf13/cobra"
)

type deletePortalCmd struct {
    *cobra.Command
    force       bool
    autoApprove bool
}

func newDeletePortalCmd(
    verb verbs.VerbValue,
    baseCmd *cobra.Command,
    addParentFlags func(*cobra.Command),
    parentPreRun func(*cobra.Command, []string) error,
) *deletePortalCmd {
    deleteCmd := &deletePortalCmd{
        Command: &cobra.Command{
            Use:   "portal [ID or NAME]",
            Short: "Delete a portal",
            Long: `Delete a portal by ID or name.

If the portal has published APIs, the deletion will fail unless the --force flag is used.
Using --force will delete the portal and all its API publications.

A confirmation prompt will be shown before deletion unless --auto-approve is used.`,
            Example: `  # Delete a portal by ID
  kongctl delete portal 12345678-1234-1234-1234-123456789012

  # Delete a portal by name
  kongctl delete portal my-portal

  # Force delete a portal with published APIs
  kongctl delete portal my-portal --force

  # Delete without confirmation prompt
  kongctl delete portal my-portal --auto-approve`,
            Args:              cobra.MaximumNArgs(1),
            ValidArgsFunction: baseCmd.ValidArgsFunction,
            PreRunE:           parentPreRun,
        },
    }

    deleteCmd.RunE = func(cmd *cobra.Command, args []string) error {
        return deleteCmd.run(cmd, args)
    }

    // Add flags
    deleteCmd.Flags().BoolVar(&deleteCmd.force, "force", false, 
        "Force deletion even if the portal has published APIs")
    deleteCmd.Flags().BoolVar(&deleteCmd.autoApprove, "auto-approve", false,
        "Skip confirmation prompt")

    addParentFlags(deleteCmd.Command)

    return deleteCmd
}

func (c *deletePortalCmd) validate(helper cmd.Helper) error {
    args := helper.GetArgs()
    if len(args) == 0 {
        return fmt.Errorf("portal ID or name is required")
    }
    return nil
}

func (c *deletePortalCmd) run(cmd *cobra.Command, args []string) error {
    ctx := cmd.Context()
    helper := cmd.BuildHelper(ctx)

    if err := c.validate(helper); err != nil {
        return err
    }

    cfg := helper.GetConfig()
    logger := helper.GetLogger()

    // Get SDK
    sdk, err := helper.GetKonnectSDK(cfg, logger)
    if err != nil {
        return cmd.PrepareKongRequestError("Failed to create Konnect SDK", err, c.Command)
    }

    // Get portal ID (resolve name if necessary)
    portalID := args[0]
    var portal *components.Portal
    
    // Check if argument is UUID
    uuidRegex := regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)
    isUUID := uuidRegex.MatchString(portalID)
    
    if !isUUID {
        // Resolve name to ID
        logger.Debug("Resolving portal name to ID", log.String("name", portalID))
        resolvedPortal, err := c.resolvePortalByName(ctx, portalID, sdk.GetPortalAPI(), helper)
        if err != nil {
            return err
        }
        portal = resolvedPortal
        portalID = portal.ID
    } else {
        // Get portal details for confirmation
        getReq := operations.GetPortalRequest{
            PortalID: portalID,
        }
        getResp, err := sdk.GetPortalAPI().GetPortal(ctx, getReq)
        if err != nil {
            attrs := cmd.TryConvertErrorToAttrs(err)
            return cmd.PrepareExecutionError("Failed to get portal details", err, c.Command, attrs...)
        }
        if getResp.Portal == nil {
            return fmt.Errorf("portal not found: %s", portalID)
        }
        portal = getResp.Portal
    }

    // Show confirmation prompt unless --auto-approve
    if !c.autoApprove {
        if !c.confirmDeletion(portal, helper) {
            return fmt.Errorf("delete cancelled")
        }
    }

    // Prepare delete request
    deleteReq := operations.DeletePortalRequest{
        PortalID: portalID,
    }
    if c.force {
        force := operations.QueryParamForceTrue
        deleteReq.Force = &force
    }

    // Delete the portal
    logger.Info("Deleting portal", log.String("id", portalID), log.String("name", portal.Name))
    
    res, err := sdk.GetPortalAPI().DeletePortal(ctx, deleteReq)
    if err != nil {
        attrs := cmd.TryConvertErrorToAttrs(err)
        // Check if error is due to published APIs
        if !c.force && strings.Contains(err.Error(), "published") {
            attrs = append(attrs, log.String("suggestion", "Use --force to delete portal with published APIs"))
        }
        return cmd.PrepareExecutionError("Failed to delete portal", err, c.Command, attrs...)
    }

    // Format and output response
    outType, err := helper.GetOutputFormat()
    if err != nil {
        return err
    }

    printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
    if err != nil {
        return err
    }
    defer printer.Flush()

    // Create success response
    response := map[string]interface{}{
        "id":      portalID,
        "name":    portal.Name,
        "status":  "deleted",
        "message": fmt.Sprintf("Portal '%s' deleted successfully", portal.Name),
    }

    return printer.Print(response)
}

func (c *deletePortalCmd) resolvePortalByName(
    ctx context.Context,
    name string,
    api konnect.PortalAPI,
    helper cmd.Helper,
) (*components.Portal, error) {
    pageSize := int64(100)
    page := int64(1)
    
    for {
        req := operations.ListPortalsRequest{
            PageSize: &pageSize,
            Page:     &page,
        }
        
        res, err := api.ListPortals(ctx, req)
        if err != nil {
            attrs := cmd.TryConvertErrorToAttrs(err)
            return nil, cmd.PrepareExecutionError("Failed to list portals", err, c.Command, attrs...)
        }
        
        if res.ListPortalsResponse == nil || res.ListPortalsResponse.Data == nil {
            break
        }
        
        // Look for exact name match
        var matches []*components.Portal
        for _, portal := range res.ListPortalsResponse.Data {
            if portal.Name == name {
                matches = append(matches, &portal)
            }
        }
        
        if len(matches) > 1 {
            return nil, fmt.Errorf("multiple portals found with name '%s'. Please use ID instead", name)
        }
        
        if len(matches) == 1 {
            return matches[0], nil
        }
        
        // Check if there are more pages
        meta := res.ListPortalsResponse.Meta
        if meta == nil || meta.Page == nil || *meta.Page.Total <= page {
            break
        }
        
        page++
    }
    
    return nil, fmt.Errorf("portal not found: %s", name)
}

func (c *deletePortalCmd) confirmDeletion(portal *components.Portal, helper cmd.Helper) bool {
    streams := helper.GetStreams()
    
    // Print warning
    fmt.Fprintln(streams.Out, "\nWARNING: This will permanently delete the following portal:")
    fmt.Fprintf(streams.Out, "\n  Name: %s\n", portal.Name)
    fmt.Fprintf(streams.Out, "  ID:   %s\n", portal.ID)
    
    // Check if portal has published APIs (would need to call API to get this info)
    // For now, we'll include a general warning about the force flag
    if !c.force {
        fmt.Fprintln(streams.Out, "\nNote: If this portal has published APIs, the deletion will fail.")
        fmt.Fprintln(streams.Out, "      Use --force to delete the portal and all its API publications.")
    }
    
    fmt.Fprint(streams.Out, "\nDo you want to continue? Type 'yes' to confirm: ")
    
    // Handle input (check if stdin is piped)
    input := streams.In
    if f, ok := input.(*os.File); ok && f.Fd() == 0 {
        // stdin is piped, try to use /dev/tty
        tty, err := os.Open("/dev/tty")
        if err == nil {
            defer tty.Close()
            input = tty
        }
    }
    
    // Read user input
    var response string
    fmt.Fscanln(input, &response)
    
    return strings.ToLower(strings.TrimSpace(response)) == "yes"
}
```

### 2. Update portal.go

In `/internal/cmd/root/products/konnect/portal/portal.go`, update the switch statement:

```go
case verbs.Delete:
    return newDeletePortalCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
```

### 3. Create portal.go for Direct Delete Support

Create `/internal/cmd/root/verbs/del/portal.go`:

```go
package del

import (
    "github.com/Kong/kongctl/internal/cmd"
    "github.com/Kong/kongctl/internal/cmd/root/products"
    "github.com/Kong/kongctl/internal/cmd/root/products/konnect"
    "github.com/Kong/kongctl/internal/cmd/root/products/konnect/portal"
    "github.com/Kong/kongctl/internal/konnect/flags"
    "github.com/Kong/kongctl/internal/verbs"
    "github.com/spf13/cobra"
)

// NewDirectPortalCmd creates a direct portal command with Konnect context
func NewDirectPortalCmd() (*cobra.Command, error) {
    // Create base command
    portalCmd, err := portal.NewPortalCmd(
        verbs.Delete,
        cmd.RootConfig{},
        cmd.RootState{},
        func(c *cobra.Command) {
            // Add Konnect-specific flags
            flags.BindKonnectAuthFlags(c)
            flags.BindKonnectAPIFlags(c)
        },
        func(c *cobra.Command, args []string) error {
            // Set up Konnect context
            ctx := c.Context()
            ctx = products.WithProduct(ctx, products.KonnectProduct)
            ctx = konnect.WithSDKFactory(ctx, konnect.SDKFactory)
            c.SetContext(ctx)
            
            // Bind flags to viper
            if err := flags.BindKonnectAuthFlagsToViper(c); err != nil {
                return err
            }
            if err := flags.BindKonnectAPIFlagsToViper(c); err != nil {
                return err
            }
            
            return nil
        },
    )
    
    if err != nil {
        return nil, err
    }
    
    // Update command use for direct access
    portalCmd.Use = "portal [ID or NAME]"
    
    return portalCmd, nil
}
```

### 4. Update del.go

In `/internal/cmd/root/verbs/del/del.go`, add portal to the delete command:

1. Import the portal package
2. Add portal command in the command setup
3. Update help text examples

```go
// In imports
import (
    // ... existing imports ...
    "github.com/Kong/kongctl/internal/cmd/root/verbs/del/portal"
)

// In NewDeleteCmd function, after gateway command:
portalCmd, err := NewDirectPortalCmd()
if err != nil {
    return nil, fmt.Errorf("failed to create portal command: %w", err)
}
cmd.AddCommand(portalCmd)

// Update Long description to include portal examples:
Long: `Delete Konnect resources.

Examples:
  # Delete a gateway control plane
  kongctl delete gateway control-plane my-cp

  # Delete a portal by ID
  kongctl delete portal 12345678-1234-1234-1234-123456789012

  # Delete a portal by name
  kongctl delete portal my-portal

  # Force delete a portal with published APIs
  kongctl delete portal my-portal --force

  # Delete without confirmation
  kongctl delete portal my-portal --auto-approve`,
```

## Integration Points

### 1. Command Registration Flow
```
main.go → root.go → del.go → portal.go (direct) → deletePortal.go
                  ↓
              konnect.go → portal/portal.go → deletePortal.go
```

### 2. SDK Integration
- Uses `sdk.GetPortalAPI().DeletePortal(ctx, request)`
- Supports force parameter for deleting portals with published APIs
- Handles SDK errors with proper error attributes

### 3. Configuration Access
- Uses helper methods to access configuration
- Supports PAT authentication via flag or config
- Respects output format settings

## Testing Approach

### 1. Unit Tests

Create `/internal/cmd/root/products/konnect/portal/deletePortal_test.go`:

```go
package portal

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestDeletePortalCmd_Validate(t *testing.T) {
    tests := []struct {
        name    string
        args    []string
        wantErr bool
    }{
        {
            name:    "valid with ID",
            args:    []string{"12345678-1234-1234-1234-123456789012"},
            wantErr: false,
        },
        {
            name:    "valid with name",
            args:    []string{"my-portal"},
            wantErr: false,
        },
        {
            name:    "missing argument",
            args:    []string{},
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cmd := &deletePortalCmd{}
            // Set up mock helper with args
            // Test validate method
        })
    }
}

func TestDeletePortalCmd_UUIDRegex(t *testing.T) {
    uuidRegex := regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)
    
    tests := []struct {
        input string
        want  bool
    }{
        {"12345678-1234-1234-1234-123456789012", true},
        {"12345678-1234-1234-1234-12345678901G", false},
        {"my-portal", false},
        {"", false},
    }
    
    for _, tt := range tests {
        t.Run(tt.input, func(t *testing.T) {
            got := uuidRegex.MatchString(tt.input)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### 2. Integration Tests

Create `/test/integration/portal_delete_test.go`:

```go
// +build integration

package integration

import (
    "testing"
    "github.com/Kong/kongctl/test/testutil"
)

func TestDeletePortal_ByID(t *testing.T) {
    // Test deletion by ID with mock SDK
}

func TestDeletePortal_ByName(t *testing.T) {
    // Test name resolution and deletion
}

func TestDeletePortal_WithForce(t *testing.T) {
    // Test force deletion of portal with APIs
}

func TestDeletePortal_AutoApprove(t *testing.T) {
    // Test skipping confirmation prompt
}
```

### 3. Manual Testing Checklist

- [ ] Delete portal by ID with confirmation prompt
- [ ] Delete portal by name with confirmation prompt
- [ ] Delete portal with --auto-approve flag
- [ ] Delete portal with published APIs (should fail)
- [ ] Delete portal with published APIs using --force
- [ ] Test interrupt handling (Ctrl+C) during prompt
- [ ] Test with piped input (echo "yes" | kongctl delete portal)
- [ ] Test all output formats (TEXT, JSON, YAML)

## Error Handling

### 1. Error Scenarios

| Scenario | Error Message | Suggestion |
|----------|--------------|------------|
| Portal not found | "portal not found: {id/name}" | Check portal ID or name |
| Multiple portals with same name | "multiple portals found with name '{name}'. Please use ID instead" | Use portal ID |
| Portal has published APIs | "Failed to delete portal: portal has published APIs" | Use --force flag |
| No argument provided | "portal ID or name is required" | Provide portal ID or name |
| Network/API error | "Failed to delete portal: {error}" | Check network/credentials |

### 2. Error Handling Pattern

```go
if err != nil {
    attrs := cmd.TryConvertErrorToAttrs(err)
    // Add context-specific attributes
    return cmd.PrepareExecutionError("Failed to delete portal", err, c.Command, attrs...)
}
```

## Command Examples

### Basic Usage
```bash
# Delete by ID
kongctl delete portal 12345678-1234-1234-1234-123456789012

# Delete by name
kongctl delete portal my-developer-portal

# With PAT authentication
kongctl delete portal my-portal --pat $KONNECT_PAT
```

### Advanced Usage
```bash
# Force delete with published APIs
kongctl delete portal my-portal --force

# Skip confirmation prompt
kongctl delete portal my-portal --auto-approve

# Combine flags
kongctl delete portal my-portal --force --auto-approve

# Different output formats
kongctl delete portal my-portal --output json
kongctl delete portal my-portal --output yaml
```

### Direct Konnect Usage
```bash
# Explicit konnect product
kongctl delete konnect portal my-portal

# Direct usage (Konnect-first)
kongctl delete portal my-portal
```

## Implementation Order

1. **Phase 1: Core Implementation**
   - Create deletePortal.go with basic functionality
   - Update portal.go to wire up delete command
   - Implement name resolution

2. **Phase 2: Confirmation Prompt**
   - Add confirmation prompt logic
   - Implement --auto-approve flag
   - Handle interrupt signals

3. **Phase 3: Direct Delete Support**
   - Create portal.go in del package
   - Update del.go with portal command
   - Update help text

4. **Phase 4: Testing**
   - Write unit tests
   - Write integration tests
   - Perform manual testing

5. **Phase 5: Documentation**
   - Update command help text
   - Update README if needed
   - Add to command reference docs

## Success Criteria

- [ ] Delete portal by ID works correctly
- [ ] Delete portal by name resolves and deletes correctly
- [ ] Confirmation prompt shows portal details
- [ ] --auto-approve flag skips confirmation
- [ ] --force flag allows deletion of portals with APIs
- [ ] Error messages are clear and actionable
- [ ] All output formats work (TEXT, JSON, YAML)
- [ ] Direct delete command works (`delete portal`)
- [ ] Help text includes portal examples
- [ ] All tests pass (unit and integration)
- [ ] Manual testing checklist completed