# Flow Report: kongctl Get Commands Execution Analysis

## Executive Summary

This report provides a comprehensive analysis of the execution flow for get commands in the kongctl codebase, specifically tracing the path from CLI invocation to API response. The analysis focuses on understanding how to implement a new `kongctl get me` command that calls the Konnect `/users/me` endpoint using the SDK method `s.Me.GetUsersMe(ctx)`.

## Complete Execution Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                           CLI INVOCATION: kongctl get portals                    │
└─────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│ ENTRY POINT                                                                       │
│ ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐                │
│ │ main.go         │ -> │ registerSignal  │ -> │ root.Execute()  │                │
│ │ - Sets up signal│    │ Handler()       │    │ - Loads config  │                │
│ │   handling      │    │ - Handles       │    │ - Executes cmd  │                │
│ │ - Calls Execute │    │   SIGINT/SIGTERM│    │                 │                │
│ └─────────────────┘    └─────────────────┘    └─────────────────┘                │
└─────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│ ROOT COMMAND SETUP                                                                │
│ ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐                │
│ │ root.go         │ -> │ newRootCmd()    │ -> │ addCommands()   │                │
│ │ - Creates root  │    │ - Global flags  │    │ - Adds get cmd  │                │
│ │   command       │    │ - PersistentPre │    │ - Adds other    │                │
│ │ - Config init   │    │   Run setup     │    │   commands      │                │
│ └─────────────────┘    └─────────────────┘    └─────────────────┘                │
└─────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│ GET COMMAND REGISTRATION                                                          │
│ ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐                │
│ │ get/get.go      │ -> │ NewGetCmd()     │ -> │ Direct Commands │                │
│ │ - Creates get   │    │ - Adds konnect  │    │ - NewDirectPor  │                │
│ │   base command  │    │ - Adds on-prem  │    │   talCmd()      │                │
│ │                 │    │ - Adds profile  │    │ - NewDirectAPI  │                │
│ │                 │    │                 │    │   Cmd()         │                │
│ └─────────────────┘    └─────────────────┘    └─────────────────┘                │
└─────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│ DIRECT COMMAND PATTERN (KONNECT-FIRST)                                           │
│ ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐                │
│ │ get/portal.go   │ -> │ NewDirectPortal │ -> │ portal.NewPortal│                │
│ │ - Defines       │    │ Cmd()           │    │ Cmd()           │                │
│ │   addFlags      │    │ - Sets up flags │    │ - Creates actual│                │
│ │ - Defines       │    │ - Sets preRunE  │    │   command       │                │
│ │   preRunE       │    │ - Calls portal  │    │                 │                │
│ └─────────────────┘    └─────────────────┘    └─────────────────┘                │
└─────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│ FLAG SETUP AND BINDING                                                           │
│ ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐                │
│ │ addFlags()      │ -> │ Flag Definition │ -> │ bindFlags()     │                │
│ │ - PAT flag      │    │ - base-url      │    │ - Binds to Viper│                │
│ │ - base-url flag │    │ - pat           │    │ - Config paths  │                │
│ │ - page-size flag│    │ - page-size     │    │ - Error handling│                │
│ └─────────────────┘    └─────────────────┘    └─────────────────┘                │
└─────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│ PRE-RUN CONTEXT SETUP                                                            │
│ ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐                │
│ │ preRunE()       │ -> │ Context Setup   │ -> │ Flag Binding    │                │
│ │ - Sets Product  │    │ - Product=      │    │ - bindFlags()   │                │
│ │ - Sets SDK      │    │   konnect       │    │ - Viper config  │                │
│ │   Factory       │    │ - SDKAPIFactory │    │                 │                │
│ └─────────────────┘    └─────────────────┘    └─────────────────┘                │
└─────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│ COMMAND EXECUTION                                                                 │
│ ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐                │
│ │ getPortal.go    │ -> │ runE()          │ -> │ Validation      │                │
│ │ - newGetPortal  │    │ - Build helper  │    │ - validate()    │                │
│ │   Cmd()         │    │ - Get config    │    │ - Args check    │                │
│ │ - Command setup │    │ - Get SDK       │    │ - Page size     │                │
│ └─────────────────┘    └─────────────────┘    └─────────────────┘                │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## Authentication Flow

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│ AUTHENTICATION FLOW                                                               │
└─────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│ TOKEN ACQUISITION                                                                 │
│ ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐                │
│ │ helper.GetKonnect│ -> │ common.GetAccess│ -> │ Token Priority  │                │
│ │ SDK()           │    │ Token()         │    │ 1. PAT flag     │                │
│ │ - Calls SDK     │    │ - Check PAT     │    │ 2. Refresh token│                │
│ │   factory       │    │ - Load token    │    │    from login   │                │
│ └─────────────────┘    └─────────────────┘    └─────────────────┘                │
└─────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│ TOKEN LOADING AND REFRESH                                                        │
│ ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐                │
│ │ auth.LoadAccess │ -> │ Load from Disk  │ -> │ Refresh if      │                │
│ │ Token()         │    │ - Profile-based │    │ Expired         │                │
│ │ - Check if PAT  │    │   file path     │    │ - RefreshAccess │                │
│ │   not provided  │    │ - JSON format   │    │   Token()       │                │
│ └─────────────────┘    └─────────────────┘    └─────────────────┘                │
└─────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│ SDK CLIENT CREATION                                                               │
│ ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐                │
│ │ auth.GetAuthentic│ -> │ SDK Options     │ -> │ KonnectSDK      │                │
│ │ atedClient()    │    │ - ServerURL     │    │ - Wraps real SDK│                │
│ │ - Creates SDK   │    │ - Security      │    │ - Implements    │                │
│ │   with token    │    │ - HTTP client   │    │   SDKAPI        │                │
│ └─────────────────┘    └─────────────────┘    └─────────────────┘                │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## API Call Execution Flow

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│ API EXECUTION FLOW                                                                │
└─────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│ OPERATION DETERMINATION                                                           │
│ ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐                │
│ │ Args Analysis   │ -> │ Operation Type  │ -> │ Function Call   │                │
│ │ - 0 args: list  │    │ - List all      │    │ - runList()     │                │
│ │ - 1 arg: get    │    │ - Get by ID     │    │ - runGet()      │                │
│ │ - UUID check    │    │ - Get by name   │    │ - runListByName │                │
│ └─────────────────┘    └─────────────────┘    └─────────────────┘                │
└─────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│ SDK API CALLS                                                                     │
│ ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐                │
│ │ Get Portal API  │ -> │ SDK Method Call │ -> │ Response        │                │
│ │ sdk.GetPortal   │    │ kkClient.List   │    │ - Success: Data │                │
│ │ API()           │    │ Portals()       │    │ - Error: Attrs  │                │
│ │ - Returns impl  │    │ kkClient.Get    │    │   for logging   │                │
│ │                 │    │ Portal()        │    │                 │                │
│ └─────────────────┘    └─────────────────┘    └─────────────────┘                │
└─────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│ PAGINATION HANDLING (for list operations)                                        │
│ ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐                │
│ │ Page Loop       │ -> │ Request Build   │ -> │ Response        │                │
│ │ - Start page 1  │    │ - PageSize      │    │ Processing      │                │
│ │ - Continue until│    │ - PageNumber    │    │ - Append data   │                │
│ │   all fetched   │    │ - SDK request   │    │ - Check total   │                │
│ └─────────────────┘    └─────────────────┘    └─────────────────┘                │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## Output Formatting Flow

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│ OUTPUT FORMATTING FLOW                                                            │
└─────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│ OUTPUT FORMAT DETERMINATION                                                       │
│ ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐                │
│ │ helper.GetOutput│ -> │ Config Lookup   │ -> │ Format Types    │                │
│ │ Format()        │    │ - Viper config  │    │ - JSON          │                │
│ │ - From flag     │    │ - Flag override │    │ - YAML          │                │
│ │ - From config   │    │ - Default: text │    │ - TEXT (default)│                │
│ └─────────────────┘    └─────────────────┘    └─────────────────┘                │
└─────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│ PRINTER CREATION                                                                  │
│ ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐                │
│ │ cli.Format()    │ -> │ Segmentio CLI   │ -> │ Formatter Setup │                │
│ │ - Format string │    │ Library         │    │ - Output stream │                │
│ │ - Output stream │    │ - JSON printer  │    │ - Buffer setup  │                │
│ │ - Creates       │    │ - YAML printer  │    │ - Flush handling│                │
│ │   formatter     │    │ - Text printer  │    │                 │                │
│ └─────────────────┘    └─────────────────┘    └─────────────────┘                │
└─────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│ DATA CONVERSION AND OUTPUT                                                        │
│ ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐                │
│ │ Format Decision │ -> │ Data Conversion │ -> │ Print and Flush │                │
│ │ - if TEXT:      │    │ - TEXT: convert │    │ - printer.Print │                │
│ │   convert to    │    │   to display    │    │ - printer.Flush │                │
│ │   display record│    │   record        │    │ - Error handling│                │
│ │ - else: raw SDK │    │ - JSON/YAML:    │    │                 │                │
│ │   response      │    │   raw response  │    │                 │                │
│ └─────────────────┘    └─────────────────┘    └─────────────────┘                │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## Error Handling Paths

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│ ERROR HANDLING FLOW                                                               │
└─────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│ ERROR TYPES                                                                       │
│ ┌─────────────────┐    ┌─────────────────┐                                       │
│ │ Configuration   │    │ Execution Error │                                       │
│ │ Error           │    │ - Network issues│                                       │
│ │ - Bad flags     │    │ - Auth failures │                                       │
│ │ - Invalid config│    │ - API errors    │                                       │
│ │ - Usage issues  │    │ - SDK errors    │                                       │
│ └─────────────────┘    └─────────────────┘                                       │
└─────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│ ERROR PROCESSING                                                                  │
│ ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐                │
│ │ Error Detection │ -> │ Error Wrapping  │ -> │ Error Attributes│                │
│ │ - Validate input│    │ - PrepareExecution│  │ - TryConvertError│               │
│ │ - Check responses│   │   Error()       │    │   ToAttrs()     │                │
│ │ - SDK call fails│    │ - Context setup │    │ - JSON unmarshal│                │
│ └─────────────────┘    └─────────────────┘    └─────────────────┘                │
└─────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│ ERROR REPORTING                                                                   │
│ ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐                │
│ │ Error Bubbling  │ -> │ Root Handler    │ -> │ Logger Output   │                │
│ │ - Return errors │    │ - root.Execute()│    │ - Structured    │                │
│ │ - Don't log in  │    │ - Check error   │    │   logging       │                │
│ │   functions     │    │   type          │    │ - Exit(1)       │                │
│ └─────────────────┘    └─────────────────┘    └─────────────────┘                │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## Key File Interconnections

### Core Files and Their Roles

#### 1. Entry and Command Setup
- **`main.go`**: Application entry point, signal handling, calls root.Execute()
- **`internal/cmd/root/root.go`**: Root command setup, configuration initialization, global flags
- **`internal/cmd/root/verbs/get/get.go`**: Get command registration, adds direct commands

#### 2. Direct Command Pattern (Konnect-first)
- **`internal/cmd/root/verbs/get/portal.go`**: Direct portal command setup
  - Defines addFlags function (PAT, base-url, page-size)
  - Defines preRunE function (context setup)
  - Calls actual portal command implementation

#### 3. Command Implementation
- **`internal/cmd/root/products/konnect/portal/portal.go`**: Portal command factory
- **`internal/cmd/root/products/konnect/portal/getPortal.go`**: Get portal implementation
  - Text display record structs
  - Conversion functions
  - Run functions (runList, runGet, runListByName)
  - Validation and execution logic

#### 4. SDK and Authentication
- **`internal/cmd/helper.go`**: Command helper interface and implementation
- **`internal/cmd/root/products/konnect/common/common.go`**: Authentication logic
- **`internal/konnect/auth/auth.go`**: Token management and SDK client creation
- **`internal/konnect/helpers/sdk.go`**: SDK interface and implementation wrapper

#### 5. Configuration and Output
- **`internal/config/config.go`**: Configuration management with Viper
- **`internal/cmd/common/common.go`**: Output format definitions
- **`internal/iostreams/iostreams.go`**: I/O stream management

## Implementation Plan for `kongctl get me`

### Files to Create

#### 1. `/internal/cmd/root/verbs/get/me.go`
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

#### 2. `/internal/cmd/root/products/konnect/me/me.go`
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

#### 3. `/internal/cmd/root/products/konnect/me/getMe.go`
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

### Files to Modify

#### 1. `/internal/cmd/root/verbs/get/get.go`
Add the me command registration:
```go
// Add me command directly for Konnect-first pattern
meCmd, err := NewDirectMeCmd()
if err != nil {
    return nil, err
}
cmd.AddCommand(meCmd)
```

#### 2. `/internal/konnect/helpers/sdk.go`
Add the MeAPI interface and method:
```go
// Add to SDKAPI interface
type SDKAPI interface {
    // ... existing methods ...
    GetMeAPI() MeAPI
}

// Add to KonnectSDK struct
func (k *KonnectSDK) GetMeAPI() MeAPI {
    if k.SDK == nil {
        return nil
    }
    return k.SDK.Me
}
```

#### 3. Create `/internal/konnect/helpers/me.go`
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

## Data Flow Summary

### 1. CLI Invocation to Command Execution
```
User types: kongctl get me
→ main.go entry point
→ root.Execute() loads config and executes command
→ get.NewGetCmd() registers direct commands
→ NewDirectMeCmd() sets up Konnect-first pattern
→ me.NewMeCmd() creates actual command
→ newGetMeCmd() sets up execution logic
```

### 2. Authentication Token Flow
```
Command execution starts
→ helper.GetKonnectSDK() calls SDK factory
→ common.GetAccessToken() checks PAT flag first
→ If no PAT: auth.LoadAccessToken() loads from profile file
→ If expired: auth.RefreshAccessToken() refreshes token
→ auth.GetAuthenticatedClient() creates SDK with token
→ helpers.KonnectSDK wraps real SDK implementation
```

### 3. API Call to Response Flow
```
runGetMe() called with SDK client
→ sdk.GetMeAPI() returns Me API interface
→ kkClient.GetUsersMe(ctx) calls SDK method
→ SDK makes HTTP request to /users/me endpoint
→ Response parsed into GetUsersMeResponse struct
→ User component extracted and returned
```

### 4. Output Formatting Flow
```
Response received from API
→ helper.GetOutputFormat() determines format from config/flag
→ cli.Format() creates appropriate printer
→ if TEXT: userToDisplayRecord() converts to display format
→ if JSON/YAML: raw user object used
→ printer.Print() outputs formatted data
→ printer.Flush() ensures output is written
```

## Security Considerations

1. **Token Handling**: PAT tokens are never logged, stored securely in profile-specific files
2. **Authentication Priority**: PAT flag takes precedence over stored tokens for security
3. **Token Refresh**: Automatic refresh prevents expired token usage
4. **Logging**: Sensitive headers are redacted in trace logging
5. **Error Handling**: Authentication errors don't expose token details

## Testing Strategy

1. **Unit Tests**: Test display record conversion, validation logic
2. **Integration Tests**: Test with real Konnect API when possible
3. **Mock Tests**: Use existing SDK mock patterns for isolated testing
4. **Error Scenarios**: Test authentication failures, network issues, invalid responses

This comprehensive flow analysis provides the foundation for implementing the `kongctl get me` command following established patterns and maintaining consistency with the existing codebase architecture.