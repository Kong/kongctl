# kongctl Repository Guidelines

## Repository Overview

kongctl is a command-line interface (CLI) tool for managing Kong Konnect and (eventually) Kong Gateway on-prem. 
This tool is currently under heavy development and is released as Beta software.

Main documentation is published at:
https://developer.konghq.com/kongctl/

This file provides repository-specific guidance for AI coding agents working
with the kongctl codebase.

## Repository Overview

**kongctl** is a command-line interface (CLI) tool for operating Kong Konnect
and (eventually) Kong Gateway on-prem. The tool is currently in beta.

- **Language**: Go 1.23+
- **CLI Framework**: Cobra for command-line processing
- **Configuration**: Viper for profile-based configuration management
- **Project Size**: Medium (~14 internal packages, comprehensive test suite)

## Project Structure & Module Organization

```
main.go                               # CLI entrypoint; wires build info and IO
internal/cmd/                         # Cobra commands and shared CLI helpers
internal/declarative/                 # Declarative config engine
  ├── loader/                         # Configuration loading and parsing
  ├── planner/                        # Plan generation and diffing
  ├── executor/                       # Execution of planned changes
  └── validator/                      # Configuration validation
internal/konnect/                     # Konnect API integration
  ├── auth/                          # Authentication and token management
  ├── httpclient/                    # HTTP client with retry logic
  └── helpers/                       # API helper functions
internal/profile/                     # Profile management
internal/iostreams/                   # I/O stream handling
internal/util/                        # Utility functions
internal/log/                         # Structured logging
internal/build/                       # Build information
docs/                                 # User documentation
test/                                 # Test code
  ├── integration/                   # Integration tests (-tags=integration)
  ├── e2e/                           # End-to-end tests (-tags=e2e)
  │   └── harness/                   # E2E test harness
  ├── cmd/                           # Test helpers for commands
  └── config/                        # Test configuration utilities
```

**Configuration Files:**
- `Makefile`: common tasks; `.golangci.yml`, `.pre-commit-config.yaml`: lint/format hooks.
- `.secrets.baseline`: Baseline for detect-secrets tool
- `go.mod/go.sum`: Go module dependencies

## Build, Test, and Development Commands

**CRITICAL**: CGO must be disabled for builds. Always use `make build` or set `CGO_ENABLED=0`.

### Essential Commands

```sh
# Build the main binary (CGO_ENABLED=0 handled by Makefile)
make build

# Run unit tests (ALWAYS run before committing)
make test

# Run all tests (unit + integration)
make test-all

# Run integration tests (requires -tags=integration)
make test-integration

# Run E2E tests (captures artifacts, requires -tags=e2e)
make test-e2e

# Auto-format code (REQUIRED before commit)
make format  # or: make fmt
# Uses gofumpt and golines -m 120 for 120-char line wrapping

# Run linter (REQUIRED before PR)
make lint

# Generate coverage report
make coverage
```

### Quality Gates - MUST Pass Before PR

All code changes must pass these gates in order:

1. **Format check**: `make format` (must produce no changes)
2. **Build check**: `make build` (must succeed)
3. **Lint check**: `make lint` (zero issues)
4. **Unit tests**: `make test` (all pass)
5. **Integration tests**: `make test-integration` (when applicable)

### Common Build Issues and Solutions

**Go Module Issues:**
```sh
go mod tidy          # Fix "missing go.sum entry" or dependency errors
goimports -w .       # Fix broken imports
```

**Build Failures:**
```sh
go build -v ./...                    # Debug with verbose output
CGO_ENABLED=0 go build -o kongctl   # Ensure CGO is disabled
```

**Test Failures:**
```sh
go test -v ./path/to/package   # Run specific test with verbose output
go test -race ./...            # Check for race conditions
```

## Architecture and Design Patterns

### Command Structure

Commands follow a **verb-noun pattern** with `konnect` as the default product:

```
kongctl <verb> [product] <resource-type> [resource-name] [flags]
```

Examples:
- `kongctl get apis` - List all APIs (konnect implied)
- `kongctl get konnect apis` - List all APIs (explicit)
- `kongctl delete api my-api` - Delete specific API

**Command Implementation Pattern:**

```go
func newResourceCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "resource-name",
        Short: "Brief description",
        Long:  "Detailed description",
        RunE:  runResourceCommand,
    }
    cmd.Flags().StringVar(&flagVar, "flag-name", "", "Description")
    return cmd
}

func runResourceCommand(cmd *cobra.Command, args []string) error {
    // Implementation - return errors, don't log
    return nil
}
```

### Error Handling

**CRITICAL**: Always return errors, never log them in functions. Bubble errors
to the highest level possible before reporting to user on STDERR.

```go
// CORRECT: Return errors
func doOperation() error {
    if err := someOperation(); err != nil {
        return fmt.Errorf("operation failed: %w", err)
    }
    return nil
}

// WRONG: Don't log errors in functions
func doOperation() error {
    if err := someOperation(); err != nil {
        log.Error(err)  // ❌ Don't do this
        return err
    }
    return nil
}
```

### Configuration Management

Uses Viper with profile-based configuration:

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

Configuration locations:
- `$XDG_CONFIG_HOME/kongctl/config.yaml`
- `$HOME/.config/kongctl/config.yaml`
- Environment variables: `KONGCTL_<PROFILE>_<PATH>`

### Output Formatting

Support multiple output formats consistently:

```go
func outputResult(data interface{}, format string) error {
    switch format {
    case "json":
        return json.NewEncoder(os.Stdout).Encode(data)
    case "yaml":
        return yaml.NewEncoder(os.Stdout).Encode(data)
    default:
        return outputAsText(data)
    }
}
```

## Coding Style & Naming Conventions

- All changes should pass coding standard gates (make format, make lint, make test-all).
- Coding changes are not complete until the gates pass
- **Packages**: lowercase, short, no underscores
- **Exported identifiers**: PascalCase; **Internal identifiers**: camelCase
- **Error wrapping**: Use `%w` format verb; avoid unused/unchecked errors (linters enforce)
- **Line length**: Max 120 characters (enforced by golines)
- **Comments**: Only add if they match existing style or explain complexity
- **Documentation markdown**: Wrap at 80 characters

## Testing Guidelines

- Place tests in `*_test.go` with `TestXxx` functions.
- Use `test/integration/...` for API-backed flows (`-tags=integration`).
- Use `test/e2e/...` for CLI flows; harness lives in `test/e2e/harness`. 
- e2e test scenarios are preferred `test/e2e/scenarios`
- Keep tests deterministic; use provided helpers in `test/{cmd,config}`.
- Unit tests for core business functionality but only when necessary. Don't test other libaries or SDKs. 
- Integration tests with the `-tags=integration` build tag
- Test utilities in `test/` directory

### When to Add Tests

- **Always**: New CLI commands or subcommands
- **Always**: Authentication flow changes
- **Always**: Configuration management changes
- **Often**: API client modifications
- **Unit tests sufficient**: Pure functions, validation, string manipulation
- **Skip**: Documentation-only changes (unless doc has specific tests)

## Commit & Pull Request Guidelines

- Commits: concise subject, imperative mood; prefix scope when helpful
  - Scopes: `cmd:`, `declarative:`, `konnect:`, `docs:`, `test:`, `ci:`
  - Examples: `cmd: add support for API product filtering`, `docs: update getting started guide`
- PRs: include description, rationale, and linked issue; add tests and docs when applicable.
- CI hygiene: run `make test-all` (lint, unit, integration) locally; attach E2E artifact path if relevant.

## CI/CD Pipeline

### GitHub Actions Workflows

The repository runs these checks on PRs:

1. **checks.yaml**: Basic validation and PR checks
2. **test.yaml**: Unit and integration tests
   - Runs `go test -race -count=1 ./...`
   - Runs integration tests with `-tags=integration`
   - Requires golangci-lint v2.10.1

### Pre-commit Hooks

Install with: `pre-commit install`

Hooks include:
- YAML linting
- Secret detection (uses `.secrets.baseline`)
- File formatting checks

Run manually: `pre-commit run -a`

## Authentication and Security

### Authentication Methods

1. **Device Flow** (Recommended): `kongctl login`
   - Tokens stored in `.<profile>-konnect-token.json`
   - Supports token refresh

2. **Personal Access Token**: `--pat` flag or `KONGCTL_DEFAULT_KONNECT_PAT` env var

## Security & Configuration Tips

- Install hooks: `pre-commit install`; run `pre-commit run -a` before pushing (YAML lint, secrets scan).
- Avoid committing secrets; `detect-secrets` uses `.secrets.baseline`.
- Redact sensitive headers in logs (Authorization, X-Api-Key, tokens)
- Use trace logging (`--log-level trace`) for debugging HTTP requests
- Local auth/config lives under `$XDG_CONFIG_HOME/kongctl/`. Use `KONGCTL_PROFILE` and `KONGCTL_*` env vars for tests.

## Error Handling Pattern

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
   - Supports both Personal Access Tokens (PAT) (--pat flag)
   - browser-based device authorization flow which handles token storage, refresh, and expiration (kongctl login)

4. **Command Organization**:
   - Root commands (verbs): get, list, create, delete, login, dump, apply, plan, diff
   - Product namespaces: konnect, gateway, mesh (defaults to konnect for "Konnect First" approach)
   - Resource types: apis, portals, control-planes, services, routes, etc.

5. **I/O Handling**:
   - Supports multiple output formats (text, json, yaml)
   - Configurable logging levels (trace, debug, info, warn, error) --log-level

## Important Patterns

1. **Profile-Based Configuration**: Commands are executed in the context of a profile, which determines which configuration values to use.
   - Users can switch profiles using the `--profile` flag or environment variable `KONGCTL_PROFILE`.

4. **Error Handling**: Structured error handling with consistent logging. In functions prefer to reeturn errors and defer 
     handling to callers. Ideally errors are bubbled as high as possible in the call stack to provide context before reporting.

5. When writing markdown documentation, use the following conventions:
    - Line width should be 80 characters or less

## Debugging and Troubleshooting

### HTTP Request Debugging

Enable trace logging to see HTTP requests/responses:

```sh
# Via flag
kongctl apply --plan plan.json --log-level trace

# Via config file (in $XDG_CONFIG_HOME/kongctl/config.yaml or $HOME/.config/kongctl/config.yaml)
log_level: trace

# Via environment variable
KONGCTL_LOG_LEVEL=trace kongctl apply --plan plan.json
```

### Common Issues

1. **Build failures**: Ensure `CGO_ENABLED=0` (use `make build`)
2. **Import errors**: Run `go mod tidy` and `goimports -w .`
3. **Test failures**: Check for race conditions with `go test -race`
4. **Lint errors**: Run `make format` before `make lint`

## Additional Resources

- **README.md**: User-facing documentation and getting started guide
- **docs/declarative.md**: Declarative configuration guide
- **docs/e2e.md**: E2E test harness documentation
- **docs/troubleshooting.md**: Common issues and solutions
