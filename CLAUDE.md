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

### Common Development Workflow

```sh
# Build and run the binary
make build
./kongctl <command>

# Example: Check version
./kongctl version --full
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

## Testing Approach

- Unit tests for core budiness functionality but only when necessary. Don't test other libaries or SDKs. 
- Integration tests with the `-tags=integration` build tag
- Test utilities in `test/` directory

## Developement Process and Documentation 

All planning and design decisions for Kongctl are documented in the `docs/` directory and the `docs/plan` subdirectory.
ADRs and execution plans are provided and named based on an ordered sequence both the the file names and in the
contents of the files themselves.  ADR-001-001 is the first ADR for the first stage, which is tied to the other documents named 001-other-doc.md.

- [Declarative Config High Level](docs/declarative-config-ux.md)
- [Planning](docs/plan/*.md)

Documents named "docs/plan/*-plan-steps.md" provide step-by-step implementation plans for each stage of development. Inside those documents
are Status values. During coding the status should be maintained to track progress of the plan and implementation.
