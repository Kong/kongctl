# Repository Guidelines

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

## Coding Style & Naming Conventions
- Go 1.24; format with `gofumpt` + `golines` (120 cols). Run `make format` before committing.
- Lint with `golangci-lint` (e.g., revive, staticcheck, gosec). Keep imports tidy and avoid blocked deps per `gomodguard`.
- Packages: lower-case, short, no underscores. Exported identifiers `PascalCase`; internal `camelCase`.
- Errors: prefer `%w` wrapping; avoid unused/unchecked errors (linters enforce).

## Testing Guidelines
- Place tests in `*_test.go` with `TestXxx` functions.
- Use `test/integration/...` for API-backed flows (`-tags=integration`).
- Use `test/e2e/...` for CLI flows; harness lives in `test/e2e/harness`.
- Keep tests deterministic; use provided helpers in `test/{cmd,config}`.

## Commit & Pull Request Guidelines
- Commits: concise subject, imperative mood; prefix scope when helpful (e.g., `cmd:`, `declarative:`, `konnect:`, `docs:`).
- PRs: include description, rationale, and linked issue; add tests and docs when applicable.
- CI hygiene: run `make test-all` (lint, unit, integration) locally; attach E2E artifact path if relevant.

## Security & Configuration Tips
- Install hooks: `pre-commit install`; run `pre-commit run -a` before pushing (YAML lint, secrets scan).
- Avoid committing secrets; `detect-secrets` uses `.secrets.baseline`.
- Local auth/config lives under `$XDG_CONFIG_HOME/kongctl/`. Use `KONGCTL_PROFILE` and `KONGCTL_*` env vars for tests.
