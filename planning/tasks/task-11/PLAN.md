# Implementation Plan: Adding `kongctl get me` Command

## Executive Summary

This plan provides detailed, step-by-step instructions for implementing a new `kongctl get me` command that calls the Konnect `/users/me` endpoint using the SDK method `s.Me.GetUsersMe(ctx)`. The command will be accessible as both `kongctl get me` (preferred, Konnect-first pattern) and `kongctl get konnect me`.

Based on comprehensive investigation of existing patterns, this implementation follows established conventions and maintains full compatibility with the existing codebase architecture.

## Implementation Overview

### Files to Create
1. `/internal/konnect/helpers/me.go` - MeAPI interface definition
2. `/internal/cmd/root/verbs/get/me.go` - Direct me command (Konnect-first pattern)
3. `/internal/cmd/root/products/konnect/me/me.go` - Me command package factory
4. `/internal/cmd/root/products/konnect/me/getMe.go` - Get me implementation

### Files to Modify
1. `/internal/konnect/helpers/sdk.go` - Add GetMeAPI() method to SDKAPI interface
2. `/internal/cmd/root/verbs/get/get.go` - Register new me command

## Step-by-Step Implementation

### Phase 1: SDK Interface Extension

#### Step 1.1: Create MeAPI Interface
**File**: `/internal/konnect/helpers/me.go`

```go
package helpers

import (
	"context"

	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// MeAPI interface for the Me API operations
type MeAPI interface {
	GetUsersMe(ctx context.Context, opts ...kkOps.Option) (*kkOps.GetUsersMeResponse, error)
}
```

**Purpose**: Defines the interface for Me API operations, following the same pattern as other API interfaces.

#### Step 1.2: Extend SDKAPI Interface
**File**: `/internal/konnect/helpers/sdk.go`

**Changes Required**:

1. Add `GetMeAPI() MeAPI` to the `SDKAPI` interface:
```go
type SDKAPI interface {
	GetControlPlaneAPI() ControlPlaneAPI
	GetPortalAPI() PortalAPI
	GetAPIAPI() APIAPI
	GetMeAPI() MeAPI  // ADD THIS LINE
	// ... other existing methods
}
```

2. Add implementation in `KonnectSDK` struct:
```go
func (k *KonnectSDK) GetMeAPI() MeAPI {
	if k.SDK == nil {
		return nil
	}
	return k.SDK.Me
}
```

**Validation**: 
- Ensure imports include the new MeAPI interface
- Verify no compilation errors
- Run `make build` to confirm changes compile correctly

### Phase 2: Command Implementation

#### Step 2.1: Create Me Command Package
**File**: `/internal/cmd/root/products/konnect/me/me.go`

```go
package me

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	CommandName = "me"
)

var (
	meUse   = CommandName
	meShort = i18n.T("root.products.konnect.me.meShort",
		"Get current user information")
	meLong = normalizers.LongDesc(i18n.T("root.products.konnect.me.meLong",
		`The me command retrieves information about the currently authenticated user.`))
	meExample = normalizers.Examples(
		i18n.T("root.products.konnect.me.meExamples",
			fmt.Sprintf(`
	# Get current user information
	%[1]s get me
	`, meta.CLIName)))
)

func NewMeCmd(verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     meUse,
		Short:   meShort,
		Long:    meLong,
		Example: meExample,
	}

	switch verb {
	case verbs.Get:
		return newGetMeCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	case verbs.List, verbs.Delete, verbs.Create, verbs.Add, verbs.Apply, verbs.Dump, verbs.Update, verbs.Help, verbs.Login,
		verbs.Plan, verbs.Sync, verbs.Diff, verbs.Export:
		return &baseCmd, nil
	}

	return &baseCmd, nil
}
```

**Purpose**: Command factory that creates the appropriate me command based on the verb, following the same pattern as portal and API commands.

#### Step 2.2: Create Get Me Implementation
**File**: `/internal/cmd/root/products/konnect/me/getMe.go`

```go
package me

import (
	"fmt"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

var (
	getMeShort = i18n.T("root.products.konnect.me.getMeShort",
		"Get current user information")
	getMeLong = i18n.T("root.products.konnect.me.getMeLong",
		`Use the get verb with the me command to retrieve information about the currently authenticated user.`)
	getMeExample = normalizers.Examples(
		i18n.T("root.products.konnect.me.getMeExamples",
			fmt.Sprintf(`
	# Get current user information
	%[1]s get me
	`, meta.CLIName)))
)

// Represents a text display record for current user
type textDisplayRecord struct {
	ID               string
	Email            string
	FullName         string
	PreferredName    string
	Active           string
	InferredRegion   string
	LocalCreatedTime string
	LocalUpdatedTime string
}

func userToDisplayRecord(u *kkComps.User) textDisplayRecord {
	missing := "n/a"

	var id, email, fullName, preferredName, active, inferredRegion string

	if u.ID != nil && *u.ID != "" {
		id = *u.ID
	} else {
		id = missing
	}

	if u.Email != nil && *u.Email != "" {
		email = *u.Email
	} else {
		email = missing
	}

	if u.FullName != nil && *u.FullName != "" {
		fullName = *u.FullName
	} else {
		fullName = missing
	}

	if u.PreferredName != nil && *u.PreferredName != "" {
		preferredName = *u.PreferredName
	} else {
		preferredName = missing
	}

	if u.Active != nil {
		if *u.Active {
			active = "true"
		} else {
			active = "false"
		}
	} else {
		active = missing
	}

	if u.InferredRegion != nil && *u.InferredRegion != "" {
		inferredRegion = *u.InferredRegion
	} else {
		inferredRegion = missing
	}

	var createdAt, updatedAt string
	if u.CreatedAt != nil {
		createdAt = u.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	} else {
		createdAt = missing
	}

	if u.UpdatedAt != nil {
		updatedAt = u.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	} else {
		updatedAt = missing
	}

	return textDisplayRecord{
		ID:               id,
		Email:            email,
		FullName:         fullName,
		PreferredName:    preferredName,
		Active:           active,
		InferredRegion:   inferredRegion,
		LocalCreatedTime: createdAt,
		LocalUpdatedTime: updatedAt,
	}
}

type getMeCmd struct {
	*cobra.Command
}

func runGetMe(kkClient helpers.MeAPI, helper cmd.Helper) (*kkComps.User, error) {
	res, err := kkClient.GetUsersMe(helper.GetContext())
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError("Failed to get current user", err, helper.GetCmd(), attrs...)
	}

	return res.GetUser(), nil
}

func (c *getMeCmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) > 0 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("the me command does not accept arguments"),
		}
	}
	return nil
}

func (c *getMeCmd) runE(cobraCmd *cobra.Command, args []string) error {
	var e error
	helper := cmd.BuildHelper(cobraCmd, args)
	if e = c.validate(helper); e != nil {
		return e
	}

	logger, e := helper.GetLogger()
	if e != nil {
		return e
	}

	outType, e := helper.GetOutputFormat()
	if e != nil {
		return e
	}

	printer, e := cli.Format(outType.String(), helper.GetStreams().Out)
	if e != nil {
		return e
	}

	defer printer.Flush()

	cfg, e := helper.GetConfig()
	if e != nil {
		return e
	}

	sdk, e := helper.GetKonnectSDK(cfg, logger)
	if e != nil {
		return e
	}

	user, e := runGetMe(sdk.GetMeAPI(), helper)
	if e != nil {
		return e
	}

	if outType == cmdCommon.TEXT {
		printer.Print(userToDisplayRecord(user))
	} else {
		printer.Print(user)
	}

	return nil
}

func newGetMeCmd(verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *getMeCmd {
	rv := getMeCmd{
		Command: baseCmd,
	}

	rv.Short = getMeShort
	rv.Long = getMeLong
	rv.Example = getMeExample
	if parentPreRun != nil {
		rv.PreRunE = parentPreRun
	}
	rv.RunE = rv.runE

	if addParentFlags != nil {
		addParentFlags(verb, rv.Command)
	}

	return &rv
}
```

**Purpose**: Core implementation of the get me command, including:
- Text display record conversion with null safety
- Validation (no arguments accepted)
- API call execution
- Output formatting for JSON, YAML, and text formats

### Phase 3: Direct Command Integration

#### Step 3.1: Create Direct Me Command (Konnect-first)
**File**: `/internal/cmd/root/verbs/get/me.go`

```go
package get

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/me"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
)

// NewDirectMeCmd creates a me command that works at the root level (Konnect-first)
func NewDirectMeCmd() (*cobra.Command, error) {
	// Define the addFlags function to add Konnect-specific flags
	addFlags := func(verb verbs.VerbValue, cmd *cobra.Command) {
		cmd.Flags().String(common.BaseURLFlagName, common.BaseURLDefault,
			fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]`,
				common.BaseURLConfigPath))

		cmd.Flags().String(common.PATFlagName, "",
			fmt.Sprintf(`Konnect Personal Access Token (PAT) used to authenticate the CLI. 
Setting this value overrides tokens obtained from the login command.
- Config path: [ %s ]`,
				common.PATConfigPath))
	}

	// Define the preRunE function to set up Konnect context
	preRunE := func(c *cobra.Command, args []string) error {
		ctx := c.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		ctx = context.WithValue(ctx, products.Product, konnect.Product)
		ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, helpers.SDKAPIFactory(common.KonnectSDKFactory))
		c.SetContext(ctx)

		// Bind flags
		return bindFlags(c, args)
	}

	// Create the me command using the me package
	meCmd, err := me.NewMeCmd(Verb, addFlags, preRunE)
	if err != nil {
		return nil, err
	}

	// Set example for direct usage
	meCmd.Example = `  # Get current user information
  kongctl get me`

	return meCmd, nil
}
```

**Purpose**: Creates the direct me command following the Konnect-first pattern, allowing `kongctl get me` usage.

#### Step 3.2: Register Me Command
**File**: `/internal/cmd/root/verbs/get/get.go`

**Location**: Add to the `NewGetCmd()` function, after the existing direct commands (like `NewDirectPortalCmd()` and `NewDirectAPICmd()`).

**Code to Add**:
```go
// Add me command directly for Konnect-first pattern
meCmd, err := NewDirectMeCmd()
if err != nil {
	return nil, err
}
cmd.AddCommand(meCmd)
```

**Exact Location**: Insert this code block after the existing direct command registrations and before the product command registrations.

### Phase 4: Quality Assurance

#### Step 4.1: Build Verification
Run the following commands in sequence:

```bash
# Fix any Go module issues
go mod tidy

# Verify build succeeds
make build

# Check for linting issues
make lint

# Run unit tests
make test
```

**Expected Results**:
- Build completes without errors
- No linting issues
- All existing tests continue to pass

#### Step 4.2: Manual Testing
Test the new command with various scenarios:

```bash
# Basic functionality (requires authentication)
./kongctl get me --pat $(cat ~/.konnect/claude.pat)

# Test output formats
./kongctl get me --pat $(cat ~/.konnect/claude.pat) --output json
./kongctl get me --pat $(cat ~/.konnect/claude.pat) --output yaml
./kongctl get me --pat $(cat ~/.konnect/claude.pat) --output text

# Test error cases
./kongctl get me invalidArg  # Should show error about no arguments accepted
./kongctl get me --pat invalid_token  # Should show authentication error

# Test help
./kongctl get me --help
```

**Expected Behaviors**:
- Command executes successfully with valid authentication
- All output formats display user information correctly
- Error handling works as expected
- Help text displays proper usage and examples

#### Step 4.3: Integration Testing
If integration tests are applicable:

```bash
# Run integration tests
make test-integration
```

**Note**: Integration tests should be run when the implementation interacts with real Konnect APIs.

## Testing Strategy

### Unit Tests
Create tests for the following components:

#### Test File: `/internal/cmd/root/products/konnect/me/getMe_test.go`

**Test Cases**:
1. **Display Record Conversion**:
   - Test `userToDisplayRecord()` with complete user data
   - Test with nil/empty fields (null safety)
   - Test timestamp formatting

2. **Validation Logic**:
   - Test `validate()` with no arguments (should pass)
   - Test `validate()` with arguments (should fail)

3. **Error Handling**:
   - Test API call failures
   - Test authentication errors

**Example Test Structure**:
```go
package me

import (
	"testing"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/assert"
)

func TestUserToDisplayRecord(t *testing.T) {
	tests := []struct {
		name     string
		user     *kkComps.User
		expected textDisplayRecord
	}{
		{
			name: "complete user data",
			user: &kkComps.User{
				ID:             stringPtr("user-123"),
				Email:          stringPtr("user@example.com"),
				FullName:       stringPtr("John Doe"),
				PreferredName:  stringPtr("John"),
				Active:         boolPtr(true),
				InferredRegion: stringPtr("us"),
				CreatedAt:      timePtr(time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)),
				UpdatedAt:      timePtr(time.Date(2023, 1, 2, 12, 0, 0, 0, time.UTC)),
			},
			expected: textDisplayRecord{
				ID:               "user-123",
				Email:            "user@example.com",
				FullName:         "John Doe",
				PreferredName:    "John",
				Active:           "true",
				InferredRegion:   "us",
				LocalCreatedTime: "2023-01-01 12:00:00",
				LocalUpdatedTime: "2023-01-02 12:00:00",
			},
		},
		{
			name: "minimal user data",
			user: &kkComps.User{},
			expected: textDisplayRecord{
				ID:               "n/a",
				Email:            "n/a",
				FullName:         "n/a",
				PreferredName:    "n/a",
				Active:           "n/a",
				InferredRegion:   "n/a",
				LocalCreatedTime: "n/a",
				LocalUpdatedTime: "n/a",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := userToDisplayRecord(tt.user)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func timePtr(t time.Time) *time.Time {
	return &t
}
```

### Integration Tests
Create integration tests that verify the complete command flow:

#### Test File: `/test/integration/get_me_test.go`

**Test Cases**:
1. Successful API call with valid authentication
2. Authentication failure scenarios
3. Output format verification

**Example Test Structure**:
```go
//go:build integration

package integration

import (
	"os"
	"testing"

	"github.com/kong/kongctl/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetMeCommand(t *testing.T) {
	pat := os.Getenv("KONNECT_PAT")
	if pat == "" {
		t.Skip("KONNECT_PAT environment variable not set")
	}

	tests := []struct {
		name           string
		args           []string
		expectedOutput string
		expectError    bool
	}{
		{
			name: "successful get me",
			args: []string{"get", "me", "--pat", pat},
			expectError: false,
		},
		{
			name: "get me with json output",
			args: []string{"get", "me", "--pat", pat, "--output", "json"},
			expectError: false,
		},
		{
			name: "get me with invalid arguments",
			args: []string{"get", "me", "invalid-arg"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := testutil.RunCommand(t, tt.args...)
			
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, output)
			}
		})
	}
}
```

## Error Handling and Edge Cases

### Authentication Errors
- **PAT Invalid**: Clear error message about invalid token
- **PAT Missing**: Clear message about required authentication
- **Token Expired**: Automatic refresh attempt if refresh token available

### API Errors
- **Network Issues**: Proper error reporting with retry suggestions
- **Rate Limiting**: Clear message about rate limit status
- **Service Unavailable**: User-friendly error message

### Input Validation
- **Extra Arguments**: Clear error that me command accepts no arguments
- **Invalid Flags**: Standard Cobra flag validation error handling

### Output Format Errors
- **Invalid Format**: Standard output format validation
- **Write Failures**: Proper error reporting for output issues

## Potential Issues and Mitigation

### Issue 1: SDK Compatibility
**Risk**: Changes to SDK interface might break existing functionality
**Mitigation**: 
- Only add new methods, don't modify existing ones
- Test with existing commands after changes
- Verify build and test success

### Issue 2: Import Path Issues
**Risk**: Incorrect import paths causing build failures
**Mitigation**:
- Use exact import paths from existing code
- Verify imports match project structure
- Test build after each file creation

### Issue 3: Authentication Flow
**Risk**: Authentication might not work correctly
**Mitigation**:
- Follow exact same patterns as portal/API commands
- Test with both PAT and login tokens
- Verify error handling for auth failures

### Issue 4: Output Formatting
**Risk**: Text display might not format correctly
**Mitigation**:
- Test all output formats (text, json, yaml)
- Handle null/empty fields safely
- Use same formatting patterns as existing commands

## Integration Guidelines

### Following Existing Patterns
1. **Command Structure**: Follows same pattern as portal and API commands
2. **Flag Handling**: Uses same Konnect flags (PAT, base-url)
3. **Authentication**: Uses same auth flow and token handling
4. **Output**: Uses same output formatting library and patterns
5. **Error Handling**: Uses same error types and reporting

### Maintaining Consistency
1. **Import Organization**: Groups imports same as existing files
2. **Variable Naming**: Follows existing naming conventions
3. **Function Structure**: Uses same patterns for command functions
4. **Documentation**: Uses same i18n and help text patterns

### Code Quality
1. **Error Handling**: Always return errors, don't log in functions
2. **Null Safety**: Handle nil pointers in SDK responses
3. **Context Handling**: Proper context passing through call chain
4. **Resource Cleanup**: Proper cleanup of resources (printer.Flush)

## Validation Steps

### Step 1: Compilation Verification
```bash
# Ensure all files compile correctly
go build ./...

# Verify specific packages
go build ./internal/cmd/root/products/konnect/me/...
go build ./internal/cmd/root/verbs/get/...
go build ./internal/konnect/helpers/...
```

### Step 2: Command Registration Verification
```bash
# Verify command appears in help
./kongctl get --help | grep -i me

# Verify command help works
./kongctl get me --help
```

### Step 3: Functionality Verification
```bash
# Test with valid authentication
./kongctl get me --pat $(cat ~/.konnect/claude.pat)

# Test all output formats
./kongctl get me --pat $(cat ~/.konnect/claude.pat) --output json
./kongctl get me --pat $(cat ~/.konnect/claude.pat) --output yaml
./kongctl get me --pat $(cat ~/.konnect/claude.pat) --output text
```

### Step 4: Error Case Verification
```bash
# Test validation error
./kongctl get me invalid-arg

# Test authentication error
./kongctl get me --pat invalid-token
```

## Implementation Checklist

### Phase 1: SDK Interface
- [ ] Create `/internal/konnect/helpers/me.go` with MeAPI interface
- [ ] Modify `/internal/konnect/helpers/sdk.go` to add GetMeAPI() method
- [ ] Verify build succeeds: `make build`

### Phase 2: Command Implementation  
- [ ] Create `/internal/cmd/root/products/konnect/me/me.go` command factory
- [ ] Create `/internal/cmd/root/products/konnect/me/getMe.go` implementation
- [ ] Verify build succeeds: `make build`

### Phase 3: Integration
- [ ] Create `/internal/cmd/root/verbs/get/me.go` direct command
- [ ] Modify `/internal/cmd/root/verbs/get/get.go` to register command
- [ ] Verify build succeeds: `make build`

### Phase 4: Quality Assurance
- [ ] Run linting: `make lint` (zero issues)
- [ ] Run tests: `make test` (all pass)
- [ ] Test manual execution with valid PAT
- [ ] Test all output formats (text, json, yaml)
- [ ] Test error cases (invalid args, bad auth)
- [ ] Verify help text displays correctly

### Phase 5: Documentation and Final Verification
- [ ] Verify command appears in `kongctl get --help`
- [ ] Test `kongctl get me --help` shows proper usage
- [ ] Run integration tests if applicable: `make test-integration`
- [ ] Final build verification: `make build`

## Conclusion

This implementation plan provides comprehensive, step-by-step instructions for adding the `kongctl get me` command. The approach follows established patterns in the codebase and maintains full compatibility with existing functionality.

Key aspects of this implementation:
- **Pattern Consistency**: Follows exact same patterns as portal and API commands
- **Authentication**: Uses established auth flow with PAT and refresh token support
- **Output Formats**: Supports all existing output formats (text, json, yaml)
- **Error Handling**: Implements consistent error handling and validation
- **Testing**: Includes comprehensive unit and integration testing approach
- **Quality Gates**: Includes all required build, lint, and test verification steps

The implementation is designed to be completed incrementally with validation at each phase, ensuring robust and maintainable code that integrates seamlessly with the existing kongctl architecture.