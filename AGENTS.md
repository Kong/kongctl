# kongctl Repository Guidelines

## Repository Overview

kongctl is a command-line interface (CLI) tool for managing Kong Konnect and (eventually) Kong Gateway on-prem. 
This tool is currently under heavy development and is released as Beta software.

Main documentation is published at:
https://developer.konghq.com/kongctl/

## Project Structure & Module Organization
- `main.go`: CLI entrypoint; wires build info and IO streams.
- `internal/cmd/...`: Cobra commands and shared CLI helpers.
- `internal/declarative/{loader,planner,executor,validator,...}`: Declarative config engine.
- `internal/konnect/{auth,httpclient,helpers}`: Konnect API integration.
- `internal/{profile,iostreams,util,log,build}`: Support packages.
- `docs/`: User docs and guides.  `test/`: `integration/`, `e2e/`, plus helpers and testdata.
- `Makefile`: common tasks; `.golangci.yml`, `.pre-commit-config.yaml`: lint/format hooks.

## Build, Test, and Development Commands
- `make build`: Compile `kongctl` (CGO disabled) into `./kongctl`.
- `make lint`: Run `golangci-lint` on `./...`.
- `make format` (alias `make fmt`): Apply `gofumpt` and `golines -m 120`.
- `make test`: Run unit tests with `-race`.
- `make test-integration`: Run `-tags=integration` tests. Pass extra flags via `GOTESTFLAGS`.
- `make test-e2e`: Run end-to-end tests (`-tags=e2e`). Set `KONGCTL_E2E_ARTIFACTS_DIR=/tmp/kongctl-e2e` to capture logs/artifacts.
- `make coverage`: Generate `coverage.out` (generated files filtered). Example: `go tool cover -html=coverage.out`.
- `make lint`: Run linters (revive, staticcheck, gosec, etc.).
- `make format`: Format code with `gofumpt` and `golines -m 120`.

## Coding Style & Naming Conventions
- All changes should pass coding standard gates (make format, make lint, make test-all).
- Coding changes are not complete until the gates pass
- Packages: lower-case, short, no underscores. Exported identifiers `PascalCase`; internal `camelCase`.
- Errors: prefer `%w` wrapping; avoid unused/unchecked errors (linters enforce).

## Testing Guidelines
- Place tests in `*_test.go` with `TestXxx` functions.
- Use `test/integration/...` for API-backed flows (`-tags=integration`).
- Use `test/e2e/...` for CLI flows; harness lives in `test/e2e/harness`. 
- e2e test scenarios are preferred `test/e2e/scenarios`
- Keep tests deterministic; use provided helpers in `test/{cmd,config}`.
- Unit tests for core business functionality but only when necessary. Don't test other libaries or SDKs. 
- Integration tests with the `-tags=integration` build tag
- Test utilities in `test/` directory

## Commit & Pull Request Guidelines
- Commits: concise subject, imperative mood; prefix scope when helpful (e.g., `cmd:`, `declarative:`, `konnect:`, `docs:`).
- PRs: include description, rationale, and linked issue; add tests and docs when applicable.
- CI hygiene: run `make test-all` (lint, unit, integration) locally; attach E2E artifact path if relevant.

## Security & Configuration Tips
- Install hooks: `pre-commit install`; run `pre-commit run -a` before pushing (YAML lint, secrets scan).
- Avoid committing secrets; `detect-secrets` uses `.secrets.baseline`.
- Local auth/config lives under `$XDG_CONFIG_HOME/kongctl/`. Use `KONGCTL_PROFILE` and `KONGCTL_*` env vars for tests.

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
