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

## Development Process and Documentation 

### Planning Documents Structure

All planning and design decisions for Kongctl are documented in the `docs/plan/` directory using a structured, stage-based approach:

**Essential Documents for Implementation:**
- [Planning Process Overview](docs/plan/process.md) - Central documentation of planning structure and development workflow
- [Implementation Quick Start](docs/plan/claude-code-guide.md) - Specific guidance for Claude Code implementation workflow
- [Planning Index](docs/plan/index.md) - Overview of all stages and current status

**Current Stage 1 Implementation:**
- [Implementation Steps](docs/plan/001-execution-plan-steps.md) - **Primary implementation guide with progress tracking**
- [Architecture Decisions](docs/plan/001-execution-plan-adrs.md) - Technical decisions and rationale
- [Technical Overview](docs/plan/001-execution-plan-overview.md) - High-level approach and examples
- [Requirements](docs/plan/001-dec-cfg-cfg-format-basic-cli.md) - Product manager requirements

### Implementation Workflow for Claude Code

**Quick Start:**
1. Check current progress: Read Progress Summary in `docs/plan/001-execution-plan-steps.md`
2. Find next task: Look for first "Not Started" step with resolved dependencies
3. Update status: Mark step as "In Progress" before starting work
4. Implement: Follow detailed step guidance with provided code examples
5. Complete: Mark step as "Completed" and update Progress Summary table

**Status Tracking:** Each step contains Status and Dependencies fields that MUST be maintained during implementation:
- Not Started → In Progress → Completed
- Update Progress Summary table to reflect current state
- Add implementation notes to steps when making decisions

**Key Reference:** The file `docs/plan/001-execution-plan-steps.md` serves as both implementation guide and progress tracker.

### Declarative Configuration Feature Context

This project is implementing declarative configuration management for Kong Konnect resources:
- **High-level design**: [Declarative Config UX Overview](docs/declarative-config-ux.md)
- **Current stage**: Stage 1 - Configuration Format & Basic CLI
- **Goal**: YAML-based resource management with plan/apply workflow (similar to Terraform)
- **Key commands**: `kongctl plan`, `kongctl apply`, `kongctl sync`, `kongctl diff`, `kongctl export`

When implementing, always refer to the planning documents for technical decisions and context.
