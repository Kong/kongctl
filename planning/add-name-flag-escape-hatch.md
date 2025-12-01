# Add --name Flag to Resource Commands (Escape Hatch for Name Collisions)

## Overview

This document outlines the plan to add `--name` flags to all commands that
currently accept positional arguments for resource identifiers. This provides
an escape hatch for users when resource names collide with subcommand names.

## Problem Statement

### The Name Collision Issue

Kongctl uses a hierarchical command structure where parent commands can have
both positional arguments AND subcommands. Cobra's command resolution
prioritizes subcommand matching over positional arguments, creating a
namespace collision problem.

**Example Scenario:**

```bash
# User creates an API named "documents"
$ kongctl create api --name "documents"

# Later tries to retrieve it
$ kongctl get api documents
# ERROR: This executes the "documents" subcommand, NOT get the API

# Current workaround: Must use UUID
$ kongctl get api a1b2c3d4-5678-9abc-...
# This works, but requires knowing the UUID
```

### Why This Happens

1. **Cobra Resolution:** When parsing `get api documents`:
   - First checks if "documents" matches a subcommand → YES, it matches
   - Executes `get api documents` subcommand
   - Never considers "documents" as a positional arg to `get api`

2. **No Escape Mechanism:**
   - Quotes don't help: `get api "documents"` still matches subcommand
   - No `--` separator support in this context
   - No flag-based alternative currently exists

3. **Reserved Words:**
   - APIs: `documents`, `versions`, `implementations`, `publications`, `attributes`
   - Portals: `pages`, `applications`, `teams`, `developers`, `snippets`,
     `application-registrations`
   - Services/Routes: Any future child subcommands

### Impact Assessment

- **User Experience:** Frustrating when resource names are common words
- **Workaround Limitation:** UUID-only access requires users to look up UUIDs first
- **Documentation Burden:** Must warn users about reserved words
- **Scalability:** List of reserved words grows with each new subcommand

## Requirements

Based on user input, the implementation must:

1. **Add `--name` flag alongside positional args** - Both methods work
2. **Do NOT add `--id` flag** - UUID detection via `--name` or positional is sufficient
3. **Apply to ALL verbs** - get, delete, adopt, create (not just get)
4. **Error on conflict** - If both positional arg AND `--name` provided, return error
5. **Backwards compatible** - Existing positional arg usage continues working
6. **Escape hatch primary use** - Provides way to access resources with reserved names

## Scope: Commands Affected

### Total: 33+ command variations

#### Parent Resources (7 commands)
1. `get api` → Add `--name <name>`
2. `get portal` → Add `--name <name>`
3. `get control-plane` → Add `--name <name>`
4. `get auth-strategy` → Add `--name <name>`
5. `get service` → Add `--name <name>`
6. `get route` → Add `--name <name>`
7. `get consumer` → Add `--name <name>`

#### Portal Child Resources (7 commands)
8. `get portal applications` → Add `--name <name>`
9. `get portal developers` → Add `--name <email>`
10. `get portal teams` → Add `--name <name>`
11. `get portal pages` → Add `--name <slug-or-title>`
12. `get portal snippets` → Add `--name <name>`
13. `get portal application-registrations` → Add `--name <id>` (UUID only)
14. `delete portal applications` → Add `--name <name>`

#### API Child Resources (5 commands)
15. `get api versions` → Add `--name <version>`
16. `get api documents` → Add `--name <slug-or-title>`
17. `get api implementations` → Add `--name <identifier>`
18. `get api publications` → Add `--name <portal-id>`
19. `get api attributes` → Add `--name <key>`

#### Delete Commands (3 commands)
20. `delete portal` → Add `--name <name>`
21. `delete control-plane` → Add `--name <name>` (currently UUID-only)
22. `delete portal application-registrations` → Add `--name <id>`

#### Adopt Commands (4 commands)
23. `adopt api` → Add `--name <name>`
24. `adopt portal` → Add `--name <name>`
25. `adopt control-plane` → Add `--name <name>` (currently UUID-only)
26. `adopt auth-strategy` → Add `--name <name>`

#### Create Commands (1 command)
27. `create control-plane` → Add `--name <name>` (optional arg already exists)

## Technical Approach

### Pattern 1: Parent Resources (Most Common)

**Current implementation:**
```go
func (c *getAPICmd) runE(cobraCmd *cobra.Command, args []string) error {
    helper := cmd.BuildHelper(cobraCmd, args)

    // Accepts 0 or 1 args
    if len(helper.GetArgs()) == 1 {
        id := strings.TrimSpace(helper.GetArgs()[0])
        isUUID := util.IsValidUUID(id)

        if !isUUID {
            // Search by name
            api, err := runListByName(id, sdk.GetAPIAPI(), helper, cfg)
            // ...
        } else {
            // Get by UUID
            api, err := runGet(id, sdk.GetAPIAPI(), helper)
            // ...
        }
    }

    // List all APIs
    apis, err := runList(sdk.GetAPIAPI(), helper, cfg)
    // ...
}
```

**New implementation with --name flag:**
```go
func (c *getAPICmd) runE(cobraCmd *cobra.Command, args []string) error {
    helper := cmd.BuildHelper(cobraCmd, args)

    // Get flag value
    nameFlag := helper.GetCmd().Flags().Lookup("name")
    var nameFromFlag string
    if nameFlag != nil && nameFlag.Changed {
        nameFromFlag = nameFlag.Value.String()
    }

    // Validation: Cannot specify both positional arg AND --name flag
    if len(helper.GetArgs()) == 1 && nameFromFlag != "" {
        return &cmd.ConfigurationError{
            Err: fmt.Errorf("cannot specify both positional argument and --name flag; use one or the other"),
        }
    }

    // Determine identifier (from flag or positional arg)
    var identifier string
    if nameFromFlag != "" {
        identifier = nameFromFlag
    } else if len(helper.GetArgs()) == 1 {
        identifier = helper.GetArgs()[0]
    }

    // If identifier provided, get specific resource
    if identifier != "" {
        id := strings.TrimSpace(identifier)
        isUUID := util.IsValidUUID(id)

        if !isUUID {
            // Search by name
            api, err := runListByName(id, sdk.GetAPIAPI(), helper, cfg)
            // ...
        } else {
            // Get by UUID
            api, err := runGet(id, sdk.GetAPIAPI(), helper)
            // ...
        }
    }

    // No identifier: list all APIs
    apis, err := runList(sdk.GetAPIAPI(), helper, cfg)
    // ...
}
```

**Flag registration:**
```go
func newGetAPICmd(...) *getAPICmd {
    rv := getAPICmd{Command: baseCmd}
    rv.RunE = rv.runE

    // Add --name flag
    rv.Flags().String("name", "", "API name or UUID to retrieve")

    // Add child subcommands
    rv.AddCommand(newGetAPIDocumentsCmd(...))
    // ...

    return &rv
}
```

### Pattern 2: Child Resources (Requires Parent Identifier)

**Current implementation:**
```go
func (h apiDocumentsHandler) run(args []string) error {
    // Validate parent identifier
    apiID, apiName := getAPIIdentifiers(cfg)
    if apiID == "" && apiName == "" {
        return &cmd.ConfigurationError{
            Err: fmt.Errorf("either --api-id or --api-name must be specified"),
        }
    }

    // Handle optional positional arg for specific document
    if len(args) == 1 {
        documentID := args[0]
        return h.getSingleDocument(documentID, ...)
    }

    // List all documents
    return h.listDocuments(...)
}
```

**New implementation with --name flag:**
```go
func newGetAPIDocumentsCmd(...) *cobra.Command {
    cmd := &cobra.Command{
        Use: "documents",
        RunE: func(cmd *cobra.Command, args []string) error {
            handler := apiDocumentsHandler{cmd: cmd}
            return handler.run(args)
        },
    }

    addAPIChildFlags(cmd)  // Adds --api-id and --api-name

    // Add --name flag for document identifier
    cmd.Flags().String("name", "", "Document ID, slug, or title to retrieve")

    return cmd
}

func (h apiDocumentsHandler) run(args []string) error {
    // Validate parent identifier
    apiID, apiName := getAPIIdentifiers(cfg)
    if apiID == "" && apiName == "" {
        return &cmd.ConfigurationError{
            Err: fmt.Errorf("either --api-id or --api-name must be specified"),
        }
    }

    // Get --name flag value
    nameFlag := h.cmd.Flags().Lookup("name")
    var nameFromFlag string
    if nameFlag != nil && nameFlag.Changed {
        nameFromFlag = nameFlag.Value.String()
    }

    // Validation: Cannot specify both positional arg AND --name flag
    if len(args) == 1 && nameFromFlag != "" {
        return &cmd.ConfigurationError{
            Err: fmt.Errorf("cannot specify both positional argument and --name flag; use one or the other"),
        }
    }

    // Determine identifier
    var identifier string
    if nameFromFlag != "" {
        identifier = nameFromFlag
    } else if len(args) == 1 {
        identifier = args[0]
    }

    // If identifier provided, get specific document
    if identifier != "" {
        return h.getSingleDocument(identifier, ...)
    }

    // No identifier: list all documents
    return h.listDocuments(...)
}
```

### Pattern 3: Delete Commands (Require Exactly 1 Arg)

**Current implementation:**
```go
func newDeletePortalCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:  "portal <portal-id|portal-name>",
        Args: cobra.ExactArgs(1),
        RunE: runDeletePortal,
    }
    return cmd
}

func runDeletePortal(cmd *cobra.Command, args []string) error {
    portalID := args[0]
    isUUID := util.IsValidUUID(portalID)

    if !isUUID {
        // Resolve name to UUID
        portal, err := resolvePortalByName(portalID)
        portalID = portal.ID
    }

    // Delete by UUID
    err := deletePortal(portalID)
    // ...
}
```

**New implementation with --name flag:**
```go
func newDeletePortalCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:  "portal [portal-id|portal-name]",  // Make arg optional
        Args: cobra.MaximumNArgs(1),              // 0 or 1 args allowed
        RunE: runDeletePortal,
    }

    // Add --name flag
    cmd.Flags().String("name", "", "Portal name or UUID to delete")

    return cmd
}

func runDeletePortal(cmd *cobra.Command, args []string) error {
    // Get --name flag value
    nameFlag := cmd.Flags().Lookup("name")
    var nameFromFlag string
    if nameFlag != nil && nameFlag.Changed {
        nameFromFlag = nameFlag.Value.String()
    }

    // Validation: Must provide exactly one identifier
    if len(args) == 0 && nameFromFlag == "" {
        return &cmd.ConfigurationError{
            Err: fmt.Errorf("must specify portal identifier via positional argument or --name flag"),
        }
    }

    // Validation: Cannot specify both
    if len(args) == 1 && nameFromFlag != "" {
        return &cmd.ConfigurationError{
            Err: fmt.Errorf("cannot specify both positional argument and --name flag; use one or the other"),
        }
    }

    // Determine identifier
    var identifier string
    if nameFromFlag != "" {
        identifier = nameFromFlag
    } else {
        identifier = args[0]
    }

    portalID := identifier
    isUUID := util.IsValidUUID(portalID)

    if !isUUID {
        // Resolve name to UUID
        portal, err := resolvePortalByName(portalID)
        portalID = portal.ID
    }

    // Delete by UUID
    err := deletePortal(portalID)
    // ...
}
```

### Pattern 4: Adopt Commands (Custom Args Validation)

**Current implementation:**
```go
func newAdoptAPICmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:  "api <api-id|api-name>",
        Args: func(cmd *cobra.Command, args []string) error {
            if len(args) != 1 {
                return fmt.Errorf("accepts 1 arg, received %d", len(args))
            }
            return nil
        },
        RunE: runAdoptAPI,
    }
    return cmd
}
```

**New implementation with --name flag:**
```go
func newAdoptAPICmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:  "api [api-id|api-name]",  // Make arg optional
        Args: func(cmd *cobra.Command, args []string) error {
            nameFlag := cmd.Flags().Lookup("name")
            var nameFromFlag string
            if nameFlag != nil && nameFlag.Changed {
                nameFromFlag = nameFlag.Value.String()
            }

            // Must provide exactly one identifier
            if len(args) == 0 && nameFromFlag == "" {
                return fmt.Errorf("must specify API identifier via positional argument or --name flag")
            }

            // Cannot provide both
            if len(args) > 0 && nameFromFlag != "" {
                return fmt.Errorf("cannot specify both positional argument and --name flag")
            }

            if len(args) > 1 {
                return fmt.Errorf("accepts at most 1 arg, received %d", len(args))
            }

            return nil
        },
        RunE: runAdoptAPI,
    }

    // Add --name flag
    cmd.Flags().String("name", "", "API name or UUID to adopt")

    return cmd
}

func runAdoptAPI(cmd *cobra.Command, args []string) error {
    // Get --name flag value
    nameFlag := cmd.Flags().Lookup("name")
    var identifier string
    if nameFlag != nil && nameFlag.Changed {
        identifier = nameFlag.Value.String()
    } else {
        identifier = args[0]
    }

    // Rest of adopt logic...
}
```

## Implementation Steps

### Phase 1: Create Helper Functions

**Goal:** Centralize the identifier resolution logic to avoid duplication

**File:** `internal/cmd/common/identifier_flags.go` (new file)

```go
package common

import (
    "fmt"
    "github.com/kong/kongctl/internal/cmd"
    "github.com/spf13/cobra"
)

// IdentifierSource represents where an identifier came from
type IdentifierSource int

const (
    IdentifierSourceNone IdentifierSource = iota
    IdentifierSourcePositional
    IdentifierSourceFlag
)

// GetResourceIdentifier retrieves a resource identifier from either
// positional args or --name flag, with conflict validation.
//
// Returns:
//   - identifier: the resource identifier (name or UUID)
//   - source: where the identifier came from
//   - error: if both are specified or neither is specified (when required)
func GetResourceIdentifier(
    cmd *cobra.Command,
    args []string,
    required bool,
) (identifier string, source IdentifierSource, err error) {
    // Get --name flag value
    nameFlag := cmd.Flags().Lookup("name")
    var nameFromFlag string
    if nameFlag != nil && nameFlag.Changed {
        nameFromFlag = nameFlag.Value.String()
    }

    // Get positional arg
    var posArg string
    if len(args) > 0 {
        posArg = args[0]
    }

    // Validation: Cannot specify both
    if posArg != "" && nameFromFlag != "" {
        return "", IdentifierSourceNone, &cmd.ConfigurationError{
            Err: fmt.Errorf("cannot specify both positional argument and --name flag; use one or the other"),
        }
    }

    // Validation: Must specify one if required
    if required && posArg == "" && nameFromFlag == "" {
        return "", IdentifierSourceNone, &cmd.ConfigurationError{
            Err: fmt.Errorf("must specify resource identifier via positional argument or --name flag"),
        }
    }

    // Return identifier and source
    if nameFromFlag != "" {
        return nameFromFlag, IdentifierSourceFlag, nil
    } else if posArg != "" {
        return posArg, IdentifierSourcePositional, nil
    }

    return "", IdentifierSourceNone, nil
}

// AddNameFlag adds a --name flag to a command with appropriate help text
func AddNameFlag(cmd *cobra.Command, resourceType string, description string) {
    if description == "" {
        description = fmt.Sprintf("%s name or UUID", resourceType)
    }
    cmd.Flags().String("name", "", description)
}
```

### Phase 2: Update Parent Resource Commands (GET)

Update each parent resource command following Pattern 1.

**Files to modify:**
1. `/internal/cmd/root/products/konnect/api/getAPI.go`
2. `/internal/cmd/root/products/konnect/portal/getPortal.go`
3. `/internal/cmd/root/products/konnect/gateway/controlplane/getControlPlane.go`
4. `/internal/cmd/root/products/konnect/authstrategy/getAuthStrategy.go`
5. `/internal/cmd/root/products/konnect/gateway/service/getService.go`
6. `/internal/cmd/root/products/konnect/gateway/route/getRoute.go`
7. `/internal/cmd/root/products/konnect/gateway/consumer/getConsumer.go`

**Changes for each file:**
1. Add `--name` flag in command initialization
2. Use `GetResourceIdentifier()` helper to get identifier
3. Update validation logic
4. Update help text examples to show both methods

### Phase 3: Update Child Resource Commands (GET)

Update each child resource command following Pattern 2.

**Files to modify:**
1. `/internal/cmd/root/products/konnect/portal/applications.go`
2. `/internal/cmd/root/products/konnect/portal/developers.go`
3. `/internal/cmd/root/products/konnect/portal/teams.go`
4. `/internal/cmd/root/products/konnect/portal/pages.go`
5. `/internal/cmd/root/products/konnect/portal/snippets.go`
6. `/internal/cmd/root/products/konnect/portal/application_registrations.go`
7. `/internal/cmd/root/products/konnect/api/versions.go`
8. `/internal/cmd/root/products/konnect/api/documents.go`
9. `/internal/cmd/root/products/konnect/api/implementations.go`
10. `/internal/cmd/root/products/konnect/api/publications.go`
11. `/internal/cmd/root/products/konnect/api/attributes.go`

### Phase 4: Update Delete Commands

Update delete commands following Pattern 3.

**Files to modify:**
1. `/internal/cmd/root/products/konnect/portal/deletePortal.go`
2. `/internal/cmd/root/products/konnect/gateway/controlplane/deleteControlPlane.go`
3. `/internal/cmd/root/products/konnect/portal/applications_delete.go`
4. `/internal/cmd/root/products/konnect/portal/application_registrations_delete.go`

**Special consideration:** These commands currently use `Args: cobra.ExactArgs(1)`.
This must change to `Args: cobra.MaximumNArgs(1)` or custom validation.

### Phase 5: Update Adopt Commands

Update adopt commands following Pattern 4.

**Files to modify:**
1. `/internal/cmd/root/products/konnect/adopt/api.go`
2. `/internal/cmd/root/products/konnect/adopt/portal.go`
3. `/internal/cmd/root/products/konnect/adopt/control_plane.go`
4. `/internal/cmd/root/products/konnect/adopt/auth_strategy.go`

**Special consideration:** These commands use custom `Args` validation functions.
The validation logic must be updated to handle `--name` flag.

### Phase 6: Update Create Commands

**Files to modify:**
1. `/internal/cmd/root/products/konnect/gateway/controlplane/createControlPlane.go`

**Note:** This command already accepts an optional positional arg for name.
Just add `--name` flag as alternative.

### Phase 7: Update Help Text and Examples

For each modified command, update:
1. Usage string to show `[identifier]` as optional when `--name` supported
2. Examples to demonstrate both positional and flag-based usage
3. Error messages to mention both options

**Example help text update:**
```go
getAPIsExample = normalizers.Examples(
    i18n.T("root.products.konnect.api.getAPIExamples",
        fmt.Sprintf(`
    # List all the APIs for the organization
    %[1]s get apis

    # Get details for an API with a specific ID using positional arg
    %[1]s get api 22cd8a0b-72e7-4212-9099-0764f8e9c5ac

    # Get details for an API with a specific name using positional arg
    %[1]s get api my-api

    # Get details for an API using --name flag (useful for reserved words)
    %[1]s get api --name documents
    %[1]s get api --name my-api

    # Get all the APIs using command aliases
    %[1]s get apis
    `, meta.CLIName)))
```

## Testing Strategy

### Unit Tests

For each modified command, add tests for:

1. **Positional arg only** (existing behavior)
   ```go
   func TestGetAPI_PositionalArg(t *testing.T) {
       // Test: kongctl get api my-api
   }
   ```

2. **Flag only** (new behavior)
   ```go
   func TestGetAPI_NameFlag(t *testing.T) {
       // Test: kongctl get api --name my-api
   }
   ```

3. **Reserved word escape** (critical use case)
   ```go
   func TestGetAPI_ReservedWordViaFlag(t *testing.T) {
       // Test: kongctl get api --name documents
       // Should get API named "documents", not execute subcommand
   }
   ```

4. **Conflict detection** (error case)
   ```go
   func TestGetAPI_ConflictError(t *testing.T) {
       // Test: kongctl get api my-api --name other-api
       // Should return error
   }
   ```

5. **Neither provided when optional** (list all)
   ```go
   func TestGetAPI_NoIdentifier(t *testing.T) {
       // Test: kongctl get api
       // Should list all APIs
   }
   ```

6. **Neither provided when required** (error case for delete/adopt)
   ```go
   func TestDeletePortal_NoIdentifier(t *testing.T) {
       // Test: kongctl delete portal
       // Should return error
   }
   ```

### Integration Tests

Add integration tests to verify:

1. **End-to-end with real API** (if safe)
   ```go
   func TestGetAPI_Integration_NameFlag(t *testing.T) {
       // Create API named "documents"
       // Retrieve via --name flag
       // Verify correct API returned
   }
   ```

2. **Help text accuracy**
   ```go
   func TestGetAPI_HelpText(t *testing.T) {
       // Verify --name flag appears in help output
       // Verify examples are correct
   }
   ```

### Manual Testing Checklist

For each command type, manually verify:

- [ ] Positional arg works (existing behavior)
- [ ] --name flag works
- [ ] Conflict returns clear error
- [ ] Help text is accurate
- [ ] Tab completion works (if applicable)
- [ ] Reserved word can be accessed via flag
- [ ] UUID works via both methods
- [ ] Name works via both methods

## Documentation Updates

### Files to Update

1. **CLAUDE.md** - Add guidance about --name flag pattern
2. **README.md** - Mention escape hatch in usage section
3. **planning/user-guide.md** - Document the name collision issue and solution

### Documentation Content

**Section to add to user guide:**

```markdown
## Avoiding Name Collisions with Subcommands

Some command names are reserved because they match subcommand names. For
example, you cannot retrieve an API named "documents" using:

```bash
# This executes the "documents" subcommand instead
kongctl get api documents
```

**Solution:** Use the `--name` flag to explicitly specify the resource name:

```bash
# This retrieves the API named "documents"
kongctl get api --name documents
```

### Reserved Words by Resource Type

**APIs:**
- documents, document, docs, doc
- versions, version, vs, ver
- implementations, implementation
- publications, publication
- attributes, attribute

**Portals:**
- pages, page, pgs
- applications, application, apps
- teams, team
- developers, developer, devs
- snippets, snippet, snip
- application-registrations, registration, registrations

**Tip:** When in doubt, use the `--name` flag for clarity.
```

## Rollout Plan

### Stage 1: Helper Functions and Tests
- Create helper functions in `internal/cmd/common/identifier_flags.go`
- Write comprehensive unit tests for helpers
- **Verification:** `make test` passes

### Stage 2: Parent Resources (GET)
- Update all 7 parent resource GET commands
- Add unit tests for each
- **Verification:** `make test` and manual testing

### Stage 3: Child Resources (GET)
- Update all 11 child resource GET commands
- Add unit tests for each
- **Verification:** `make test` and manual testing

### Stage 4: Delete Commands
- Update all 4 delete commands
- Add unit tests for each
- **Verification:** `make test` and manual testing

### Stage 5: Adopt Commands
- Update all 4 adopt commands
- Add unit tests for each
- **Verification:** `make test` and manual testing

### Stage 6: Create Commands
- Update create control-plane command
- Add unit tests
- **Verification:** `make test` and manual testing

### Stage 7: Documentation
- Update all documentation
- Update help text and examples
- **Verification:** Manual review

### Stage 8: Integration Testing
- Run full integration test suite
- Manual testing of real-world scenarios
- **Verification:** `make test-integration` passes

## Risk Assessment

### Low Risk
- **Backwards compatibility:** Positional args continue working
- **Incremental rollout:** Can deploy command-by-command
- **Clear error messages:** Users know what went wrong

### Medium Risk
- **Scope:** 33+ commands to modify (high effort)
- **Testing coverage:** Need thorough tests for all variations
- **Documentation:** Must be clear to avoid confusion

### Mitigations
- Use helper functions to ensure consistency
- Comprehensive test coverage before deployment
- Phased rollout with validation at each stage
- Clear error messages guide users to correct usage

## Success Criteria

1. ✅ Users can access resources with reserved names via `--name` flag
2. ✅ Positional arg behavior remains unchanged (backwards compatible)
3. ✅ Clear error when both positional and flag provided
4. ✅ All existing tests continue passing
5. ✅ New tests cover all flag variations
6. ✅ Documentation clearly explains the feature
7. ✅ Help text shows both usage methods

## Future Considerations

### Potential Enhancements (Out of Scope)

1. **--id flag for explicit UUID queries**
   - Currently rejected, but could be added later
   - Would provide semantic clarity: `--id <uuid>` vs `--name <name>`

2. **Deprecate positional args**
   - Could eventually require flags for all queries
   - Would eliminate ambiguity completely
   - Breaking change requiring major version bump

3. **Reserved word validation**
   - Validate resource names during creation
   - Warn or error if name matches subcommand
   - Requires changes to create/update commands

4. **Shell completion for --name**
   - Auto-complete resource names from API
   - Requires additional API calls
   - May impact performance

## Appendix: Complete File Inventory

### New Files
- `internal/cmd/common/identifier_flags.go` - Helper functions

### Modified Files (33+ total)

**Parent Resources (GET):**
1. `internal/cmd/root/products/konnect/api/getAPI.go`
2. `internal/cmd/root/products/konnect/portal/getPortal.go`
3. `internal/cmd/root/products/konnect/gateway/controlplane/getControlPlane.go`
4. `internal/cmd/root/products/konnect/authstrategy/getAuthStrategy.go`
5. `internal/cmd/root/products/konnect/gateway/service/getService.go`
6. `internal/cmd/root/products/konnect/gateway/route/getRoute.go`
7. `internal/cmd/root/products/konnect/gateway/consumer/getConsumer.go`

**Child Resources (GET):**
8. `internal/cmd/root/products/konnect/portal/applications.go`
9. `internal/cmd/root/products/konnect/portal/developers.go`
10. `internal/cmd/root/products/konnect/portal/teams.go`
11. `internal/cmd/root/products/konnect/portal/pages.go`
12. `internal/cmd/root/products/konnect/portal/snippets.go`
13. `internal/cmd/root/products/konnect/portal/application_registrations.go`
14. `internal/cmd/root/products/konnect/api/versions.go`
15. `internal/cmd/root/products/konnect/api/documents.go`
16. `internal/cmd/root/products/konnect/api/implementations.go`
17. `internal/cmd/root/products/konnect/api/publications.go`
18. `internal/cmd/root/products/konnect/api/attributes.go`

**Delete Commands:**
19. `internal/cmd/root/products/konnect/portal/deletePortal.go`
20. `internal/cmd/root/products/konnect/gateway/controlplane/deleteControlPlane.go`
21. `internal/cmd/root/products/konnect/portal/applications_delete.go`
22. `internal/cmd/root/products/konnect/portal/application_registrations_delete.go`

**Adopt Commands:**
23. `internal/cmd/root/products/konnect/adopt/api.go`
24. `internal/cmd/root/products/konnect/adopt/portal.go`
25. `internal/cmd/root/products/konnect/adopt/control_plane.go`
26. `internal/cmd/root/products/konnect/adopt/auth_strategy.go`

**Create Commands:**
27. `internal/cmd/root/products/konnect/gateway/controlplane/createControlPlane.go`

**Documentation:**
28. `CLAUDE.md`
29. `README.md`
30. `planning/user-guide.md`

### Test Files (New)
- `internal/cmd/common/identifier_flags_test.go`
- Unit test files for each modified command (33+ files)

## Questions for Implementation Session

1. Should we add integration tests that create resources with reserved names?
2. Should we validate reserved words at resource creation time?
3. Should error messages suggest using `--name` flag when collision detected?
4. Should we add shell completion for the `--name` flag?
5. Do we need to update any CI/CD pipelines or deployment scripts?
