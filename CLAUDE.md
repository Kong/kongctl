# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

Kongctl is a command-line interface (CLI) tool for operating Kong Konnect and (eventually) Kong Gateway on-prem. 
This tool is currently under heavy development and not recommended for production use.

## Development Commands

### Building

```sh
# Build the main binary
make build

# Alternatively, build directly with Go
CGO_ENABLED=0 go build -o kongctl
```

### Testing

```sh
# Run all tests
make test

# Run all tests with race detection
go test -race -count=1 ./...

# Run integration tests
make test-integration
# Or with specific flags:
go test -v -count=1 -tags=integration -race ./test/integration/...

# Generate test coverage report
make coverage
```

### Linting

```sh
# Run linter
make lint
# Or directly:
golangci-lint run -v ./...
```

## Quality Verification Workflow

After each implementation step, verify quality with these commands in order:

### Required Quality Gates
1. **Build check**: `make build` (must succeed)
2. **Lint check**: `make lint` (zero issues)
3. **Test check**: `make test` (all pass)
4. **Integration test**: `make test-integration` (when applicable)

### Error Recovery Commands
When builds fail:
```sh
# Fix Go module issues
go mod tidy

# Verify and fix imports
goimports -w .

# Debug build with verbose output
go build -v ./...
```

When tests fail:
```sh
# Run specific test with verbose output
go test -v ./path/to/package

# Run with race detection for concurrency issues
go test -race ./...
```

### Integration Testing Strategy

**When to run integration tests:**
- New CLI commands or subcommands
- Authentication flow changes
- Configuration management changes
- API client modifications
- Before completing any stage

**When unit tests are sufficient:**
- Pure functions and utilities
- Configuration parsing logic
- Input validation functions
- String manipulation and formatting

## Architecture Overview

Kongctl is a Go-based CLI built with the following key components:

1. **Command Structure**: Uses Cobra for command-line processing with a verb-noun command pattern (e.g., `get konnect gateway control-planes`).
    - Ideally we will build these verb-noun commands following a "Konnect first" approach, meaning that the `konnect` product will be implied in the command structure where possible.

2. **Configuration Management**:
   - Uses Viper for configuration handling
   - Supports profiles (default, dev, prod, etc.)
   - Config file at `$XDG_CONFIG_HOME/kongctl/config.yaml` or `$HOME/.config/kongctl/config.yaml`
   - Configuration can be overridden via environment variables or flags

3. **Authentication**:
   - Supports both Personal Access Tokens (PAT) and browser-based device authorization flow
   - Handles token storage, refresh, and expiration

4. **Command Organization**:
   - Root commands (verbs): get, list, create, delete, login, dump
   - Product namespaces: konnect, gateway, mesh
   - Resource types: control-planes, services, routes, etc.

5. **I/O Handling**:
   - Supports multiple output formats (text, json, yaml)
   - Configurable logging levels (debug, info, warn, error)

## Important Patterns

1. **Profile-Based Configuration**: Commands are executed in the context of a profile, which determines which configuration values to use.
   - Users can switch profiles using the `--profile` flag or environment variable `KONGCTL_PROFILE`.

2. **Konnect Authentication Flow**:
   - For login (kongctl konnect login) , the device code authorization flow is used
   - PATs can be provided via flag or environment variable
   - Auth tokens are stored in profile-specific files (not the config file)

3. **Command Hierarchy**: Commands follow a hierarchical structure:
   - Verb (get, list, create, delete, login)
   - Product (konnect, gateway)
   - Resource type (control-planes, services, routes)

4. **Error Handling**: Structured error handling with consistent logging. In functions prefer to reeturn errors and defer 
     handling to callers. Ideally errors are bubbled as high as possible in the call stack to provide context before reporting.

5. When writing markdown documentation, use the following conventions:
    - Line width should be 80 characters or less

## Code Patterns and Examples

### Command Structure Pattern
Follow this pattern for new Cobra commands:

```go
func newResourceCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "resource-name",
        Short: "Brief description of the command",
        Long:  "Longer description explaining the command purpose",
        RunE:  runResourceCommand,
    }
    
    // Add flags
    cmd.Flags().StringVar(&flagVar, "flag-name", "", "Flag description")
    
    return cmd
}

func runResourceCommand(cmd *cobra.Command, args []string) error {
    // Implementation logic
    return nil
}
```

### Error Handling Pattern

*Always* return errors, don't log or capture within functions. Bubble errors to the highest level possible and report to user on STDERR.

```go
func doOperation() error {
    if err := someOperation(); err != nil {
        return fmt.Errorf("operation failed: %w", err)
    }
    return nil
}

// In command functions, handle errors at the top level:
func runCommand(cmd *cobra.Command, args []string) error {
    if err := doOperation(); err != nil {
        return fmt.Errorf("command failed: %w", err)
    }
    return nil
}
```

### Configuration Access Pattern
Use profile-aware configuration access:

```go
// Get configuration values
config := viper.GetViper()
token := config.GetString("konnect.token")
baseURL := config.GetString("konnect.base_url") 

// Check for required configuration
if token == "" {
    return fmt.Errorf("konnect token not configured")
}
```

### HTTP Client Pattern
For API calls, follow consistent client creation:

```go
func createKonnectClient() (*http.Client, error) {
    client := &http.Client{
        Timeout: 30 * time.Second,
    }
    
    // Add authentication, retry logic, etc.
    return client, nil
}
```

### Output Formatting Pattern
Support multiple output formats consistently:

```go
func outputResult(data interface{}, format string) error {
    switch format {
    case "json":
        return json.NewEncoder(os.Stdout).Encode(data)
    case "yaml":
        return yaml.NewEncoder(os.Stdout).Encode(data)
    default:
        // Text/table format
        return outputAsText(data)
    }
}
``` 

## Testing Approach

- Unit tests for core budiness functionality but only when necessary. Don't test other libaries or SDKs. 
- Integration tests with the `-tags=integration` build tag
- Test utilities in `test/` directory

## Development Process and Documentation 

### Planning Documents Structure

All planning and design decisions for Kongctl are documented in the `docs/plan/` directory using a structured, stage-based approach:

**Essential Documents for Implementation:**
- [Planning Process Overview](docs/plan/process.md) - Central documentation of planning structure and development workflow
- [Implementation Quick Start](docs/plan/claude-code-guide.md) - Specific guidance for Claude Code implementation workflow
- [Planning Index](docs/plan/index.md) - Overview of all stages and current status

**Current Implementation:**
- [Planning Index](docs/plan/index.md) - **Master dashboard showing current active stage and implementation guide**
- All stage-specific documents linked from the index

### Session Start Checklist

For Claude Code sessions, start every session with these steps to establish context:

1. **Check current state**: `git status` and `git log --oneline -5`
2. **Verify build health**: `make build` (must succeed before starting work)
3. **Check current stage**: Read "Current Active Stage" in `docs/plan/index.md`
4. **Review recent progress**: Check Progress Summary in current stage's execution-plan-steps.md
5. **Run baseline tests**: `make test` (verify starting state)
6. **Check for lint issues**: `make lint` (resolve any existing issues first)

This establishes a clean baseline and provides necessary context for productive development.

### Implementation Workflow for Claude Code

**Quick Start:**
1. Check current stage: Read "Current Active Stage" section in `docs/plan/index.md`
2. Use implementation guide: Follow the linked execution-plan-steps.md for current stage
3. Find next task: Look for first "Not Started" step in current stage
4. Update status: Mark step as "In Progress" before starting work
5. Implement: Follow detailed step guidance with provided code examples
6. Verify: Run quality gates (build, lint, tests) before marking complete
7. Complete: Mark step as "Completed" and update Progress Summary table

**Status Tracking:** Each step contains Status fields that MUST be maintained during implementation:
- Not Started → In Progress → Completed
- Update Progress Summary table to reflect current state
- Add implementation notes to steps when making decisions

**Step Implementation Template:** Each step should follow this verification pattern:
1. Mark step as "In Progress"
2. Implement the functionality following provided code examples
3. Run quality gates: `make build && make lint && make test`
4. Add implementation notes to the step for any decisions made
5. Mark step as "Completed" only after all quality gates pass

**Key Reference:** Always start with `docs/plan/index.md` to find the current active stage and implementation guide.

### Declarative Configuration Feature Context

This project is implementing declarative configuration management for Kong Konnect resources:
- **High-level design**: [Declarative Config UX Overview](docs/declarative-config-ux.md)
- **Current stage**: Stage 1 - Configuration Format & Basic CLI
- **Goal**: YAML-based resource management with plan/apply workflow (similar to Terraform)
- **Key commands**: `kongctl plan`, `kongctl apply`, `kongctl sync`, `kongctl diff`, `kongctl export`

When implementing, always refer to the planning documents for technical decisions and context.
