# GitHub Copilot Instructions for kongctl

This file provides repository-specific guidance for GitHub Copilot when working
with the kongctl codebase.

## Repository Overview

**kongctl** is a command-line interface (CLI) tool for operating Kong Konnect
and (eventually) Kong Gateway on-prem. The tool is currently in beta and under
heavy development.

- **Language**: Go
- **CLI Framework**: Cobra for command-line processing
- **Configuration**: Viper for configuration management
- **Project Size**: Medium (~14 internal packages, comprehensive test suite)
- **Target Runtime**: Go 1.23+ (see `.go-version`)

## Project Structure

### Key Directories and Files

```
main.go                               # CLI entrypoint, wires build info and IO
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

### Configuration Files

- **Makefile**: Common build, test, and lint tasks
- **.golangci.yml**: Linter configuration (revive, staticcheck, gosec, etc.)
- **.pre-commit-config.yaml**: Pre-commit hooks (YAML lint, secrets scan)
- **.secrets.baseline**: Baseline for detect-secrets tool
- **go.mod/go.sum**: Go module dependencies

## Build, Test, and Development Commands

### Essential Commands - Always Use These

**CRITICAL**: CGO must be disabled for builds. Always use `make build` or
`CGO_ENABLED=0 go build`.

#### Build Commands

```sh
# Build the main binary (REQUIRED: CGO_ENABLED=0 is handled by Makefile)
make build

# Direct build (if not using Makefile)
CGO_ENABLED=0 go build -o kongctl
```

#### Test Commands

```sh
# Run unit tests (ALWAYS run before committing)
make test

# Run all tests (unit + integration)
make test-all

# Run integration tests (requires tags)
make test-integration

# Run E2E tests (captures artifacts)
make test-e2e

# Generate coverage report
make coverage
```

#### Lint and Format Commands

```sh
# Run linter (REQUIRED before PR)
make lint

# Auto-format code (REQUIRED before commit)
make format
# or
make fmt

# Format uses:
# - gofumpt for Go formatting
# - golines -m 120 for line wrapping (max 120 chars)
```

### Quality Gates - MUST Pass Before PR

All code changes must pass these gates in order:

1. **Format check**: `make format` (must produce no changes)
2. **Build check**: `make build` (must succeed)
3. **Lint check**: `make lint` (zero issues)
4. **Unit tests**: `make test` (all pass)
5. **Integration tests**: `make test-integration` (when applicable)

### Common Build Issues and Solutions

#### Go Module Issues

```sh
# If you see "missing go.sum entry" or dependency errors
go mod tidy

# If imports are broken
goimports -w .
```

#### Build Failures

```sh
# Debug build with verbose output
go build -v ./...

# Ensure CGO is disabled (required for this project)
CGO_ENABLED=0 go build -o kongctl
```

#### Test Failures

```sh
# Run specific test with verbose output
go test -v ./path/to/package

# Run with race detection for concurrency issues
go test -race ./...
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

### Command Implementation Pattern

When creating new Cobra commands, follow this pattern:

```go
func newResourceCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "resource-name",
        Short: "Brief description",
        Long:  "Detailed description",
        RunE:  runResourceCommand,
    }
    
    // Add flags
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

## Coding Conventions

### Style and Naming

- **Packages**: lowercase, short, no underscores
- **Exported identifiers**: PascalCase
- **Internal identifiers**: camelCase
- **Error wrapping**: Use `%w` format verb
- **Line length**: Max 120 characters (enforced by golines)
- **Comments**: Only add if they match existing style or explain complexity

### Testing Guidelines

- Unit tests in `*_test.go` with `TestXxx` functions
- Integration tests use `-tags=integration` build tag
- E2E tests use `-tags=e2e` build tag
- Keep tests deterministic
- Use test helpers in `test/{cmd,config}` packages
- **Don't test external libraries or SDKs** - only test your own code

### When to Add Tests

- **Always**: New CLI commands or subcommands
- **Always**: Authentication flow changes
- **Always**: Configuration management changes
- **Often**: API client modifications
- **Unit tests sufficient**: Pure functions, validation, string manipulation
- **Skip**: Documentation-only changes (unless doc has specific tests)

## Git and Commit Conventions

### Commit Messages

Format: `<scope>: <subject>`

Scopes:
- `cmd:` - Command-line interface changes
- `declarative:` - Declarative config engine
- `konnect:` - Konnect API integration
- `docs:` - Documentation
- `test:` - Test code
- `ci:` - CI/CD changes

Examples:
```
cmd: add support for API product filtering
declarative: fix plan diff for nested resources
konnect: implement retry logic for token refresh
docs: update getting started guide
```

### Pull Requests

- Include description and rationale
- Link related issues
- Add tests for new functionality
- Update documentation when changing behavior
- Ensure all CI checks pass

## CI/CD Pipeline

### GitHub Actions Workflows

The repository runs these checks on PRs:

1. **checks.yaml**: Basic validation and PR checks
2. **test.yaml**: Unit and integration tests
   - Runs `go test -race -count=1 ./...`
   - Runs integration tests with `-tags=integration`
   - Requires golangci-lint v2.1.2

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

### Security Considerations

- Never commit secrets to source code
- Use `.secrets.baseline` for detect-secrets tool
- Redact sensitive headers in logs (Authorization, X-Api-Key, tokens)
- Use trace logging (`--log-level trace`) for debugging HTTP requests

## Debugging and Troubleshooting

### HTTP Request Debugging

Enable trace logging to see HTTP requests/responses:

```sh
# Via flag
kongctl apply --plan plan.json --log-level trace

# Via config file
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

- **CLAUDE.md**: Detailed guidance for Claude Code AI agent
- **AGENTS.md**: Repository guidelines for AI agents
- **README.md**: User-facing documentation and getting started guide
- **docs/declarative.md**: Declarative configuration guide
- **docs/e2e.md**: E2E test harness documentation
- **docs/troubleshooting.md**: Common issues and solutions

## Important Notes for Copilot

1. **Always disable CGO**: Use `make build` or `CGO_ENABLED=0 go build`
2. **Never log errors in functions**: Return them and handle at top level
3. **Run quality gates before PR**: format → build → lint → test
4. **Use correct build tags**: `-tags=integration` or `-tags=e2e` for tests
5. **Follow verb-noun command pattern**: `kongctl <verb> <resource>`
6. **Respect line length**: Max 120 characters (golines enforces)
7. **Use structured error wrapping**: `fmt.Errorf("context: %w", err)`
8. **Profile-aware configuration**: Use Viper to access config values
9. **Document markdown at 80 chars**: When writing docs, wrap at 80 chars
10. **Trust existing tests**: Don't modify working tests unless fixing a bug

## Quick Reference

```sh
# Complete development workflow
make format              # Format code
make build              # Build binary
make lint               # Check for issues
make test               # Run unit tests
make test-integration   # Run integration tests (if applicable)

# Pre-commit checks
pre-commit run -a       # Run all pre-commit hooks

# E2E testing
make test-e2e          # Run E2E tests with artifacts
```
