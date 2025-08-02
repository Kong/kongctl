# Investigation Report: Adding `kongctl get me` Command

## Executive Summary

This report documents the investigation of the kongctl codebase to understand how to implement a new `kongctl get me` command that calls the Konnect `/users/me` endpoint using the SDK method `s.Me.GetUsersMe(ctx)`. The investigation reveals a well-structured command pattern that can be followed to implement this new functionality.

## Key Findings

### 1. Command Structure and Organization

**Location**: Commands are organized in `/internal/cmd/root/verbs/`

The get commands follow a consistent pattern:
- Main get command: `/internal/cmd/root/verbs/get/get.go`
- Direct commands for "Konnect-first" pattern: `/internal/cmd/root/verbs/get/portal.go`, `/internal/cmd/root/verbs/get/api.go`
- Product-specific implementations: `/internal/cmd/root/products/konnect/[resource]/`

### 2. Existing Get Command Patterns

#### Direct Command Pattern (Konnect-first)
Files like `portal.go` and `api.go` in `/internal/cmd/root/verbs/get/` implement the "Konnect-first" pattern where commands work directly at root level (e.g., `kongctl get portals` instead of `kongctl get konnect portals`).

**Pattern Structure:**
1. `NewDirect[Resource]Cmd()` function that:
   - Defines `addFlags` function for Konnect-specific flags (PAT, base-url, page-size)
   - Defines `preRunE` function to set up Konnect context
   - Creates the command using existing resource package
   - Binds flags to configuration

2. `bindFlags()` function that binds flags to configuration paths using Viper

#### Resource Implementation Pattern
Files like `/internal/cmd/root/products/konnect/portal/getPortal.go` show the actual command implementation:

**Key Components:**
1. **Text Display Records**: Custom structs for formatted text output
2. **Conversion Functions**: Convert SDK responses to display records
3. **Run Functions**: `runList()`, `runGet()`, `runListByName()` for different operations
4. **Command Structure**: Uses cobra.Command with validation, execution, and output formatting

### 3. SDK Usage and Authentication

#### SDK Interface
**Location**: `/internal/konnect/helpers/sdk.go`

The SDK interface (`SDKAPI`) currently includes methods like:
- `GetControlPlaneAPI()`
- `GetPortalAPI()` 
- `GetAPIAPI()`
- etc.

**Missing**: No `GetMeAPI()` method exists yet - this will need to be added.

#### SDK Initialization
**Location**: `/internal/cmd/root/products/konnect/common/common.go`

Authentication flow:
1. Checks for PAT token first (`cfg.GetString(PATConfigPath)`)
2. Falls back to refresh token from login command
3. Creates authenticated SDK client using `auth.GetAuthenticatedClient()`
4. Returns `&helpers.KonnectSDK{SDK: sdk}`

#### Me API Discovery
**Location**: `/vendor/github.com/Kong/sdk-konnect-go/me.go`

Found the target SDK method:
```go
func (s *Me) GetUsersMe(ctx context.Context, opts ...operations.Option) (*operations.GetUsersMeResponse, error)
```

**Response Structure**: `/vendor/github.com/Kong/sdk-konnect-go/models/operations/getusersme.go`
```go
type GetUsersMeResponse struct {
    ContentType string
    StatusCode  int
    RawResponse *http.Response
    User        *components.User
}
```

### 4. Authentication Handling

#### Flag Definitions
**Location**: `/internal/cmd/root/products/konnect/common/common.go`

Standard Konnect flags:
- `--pat`: Personal Access Token
- `--base-url`: Konnect API base URL (default: https://global.api.konghq.com)
- `--page-size`: Results per page (for list operations)

#### Flag Binding
Each command binds flags to configuration using Viper:
```go
err = cfg.BindFlag(common.PATConfigPath, f)
```

### 5. Output Formatting

#### Output Types
**Location**: `/internal/cmd/common/common.go`

Supported formats:
- `JSON` (json)
- `YAML` (yaml) 
- `TEXT` (text) - default

#### Output Implementation Pattern
Commands use segmentio/cli for formatting:
```go
printer, e := cli.Format(outType.String(), helper.GetStreams().Out)
if outType == cmdCommon.TEXT {
    printer.Print(textDisplayRecord)
} else {
    printer.Print(rawSDKResponse)
}
```

### 6. Command Registration

#### Main Get Command
**Location**: `/internal/cmd/root/verbs/get/get.go`

New commands are added in `NewGetCmd()`:
```go
// Add [resource] command directly for Konnect-first pattern
[resource]Cmd, err := NewDirect[Resource]Cmd()
if err != nil {
    return nil, err
}
cmd.AddCommand([resource]Cmd)
```

## Implementation Requirements

### 1. Files to Create/Modify

#### New Files Needed:
1. `/internal/cmd/root/verbs/get/me.go` - Direct me command implementation
2. `/internal/cmd/root/products/konnect/me/me.go` - Me command package
3. `/internal/cmd/root/products/konnect/me/getMe.go` - Get me implementation

#### Files to Modify:
1. `/internal/cmd/root/verbs/get/get.go` - Add me command registration
2. `/internal/konnect/helpers/sdk.go` - Add GetMeAPI() method to interface
3. `/internal/konnect/helpers/[new file]` - Implement Me API wrapper

### 2. Missing SDK Interface

The `SDKAPI` interface needs a new method:
```go
GetMeAPI() MeAPI
```

And corresponding implementation in `KonnectSDK` struct.

### 3. Command Structure

The me command should follow the single-resource pattern (no pagination needed):
- No need for `runList()` or `runListByName()` 
- Only needs `runGet()` equivalent for current user
- Should support all three output formats (text, json, yaml)

### 4. Text Display Format

For text output, create a `textDisplayRecord` struct with relevant user fields:
- ID
- Email 
- Name
- Organization info
- Creation/update timestamps
- etc.

## Architecture Recommendations

### 1. Follow Existing Patterns

The implementation should closely follow the portal/API command patterns:
- Use the same flag binding mechanism
- Implement the same preRunE pattern for context setup
- Use the same output formatting approach

### 2. Command Placement

Place the me command at the root level as a direct command:
- `kongctl get me` (preferred)
- Not `kongctl get konnect me` (follows Konnect-first pattern)

### 3. No Arguments Needed

Unlike other get commands that accept ID/name arguments, the me command should:
- Accept no arguments (always returns current user)
- Validate that no arguments are provided
- Always return the authenticated user's information

### 4. Error Handling

Follow existing error handling patterns:
- Use `cmd.PrepareExecutionError()` for execution errors
- Use `&cmd.ConfigurationError{}` for validation errors
- Bubble errors up to command level for consistent reporting

## Security Considerations

1. **Authentication Required**: Command must require valid PAT or auth token
2. **No Sensitive Data Logging**: Ensure user data is not logged at debug levels
3. **Follow Existing Auth Flow**: Use the same token validation as other commands

## Testing Strategy

Follow existing testing patterns:
1. **Unit Tests**: Test display record conversion and validation
2. **Integration Tests**: Test with real Konnect API (when applicable)
3. **Mock Testing**: Use existing SDK mock patterns for unit tests

## Conclusion

The kongctl codebase has a well-established pattern for implementing get commands. The me command implementation should follow the same patterns used by portal and API commands, with the main difference being that it doesn't require pagination or search functionality since it always returns the current user's information.

The key missing piece is the SDK interface method for accessing the Me API, which needs to be added to the helpers package. Once that's in place, the command implementation can follow the established patterns closely.