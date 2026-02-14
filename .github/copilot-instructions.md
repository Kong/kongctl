# GitHub Copilot Instructions for kongctl

**For comprehensive repository guidelines, see [AGENTS.md](../AGENTS.md)** in the root directory.

The `AGENTS.md` file contains detailed information about:

- **Repository Overview**: Project purpose, technology stack, and structure
- **Build & Test Commands**: Essential commands, quality gates, and troubleshooting
- **Architecture Patterns**: Command structure, error handling, configuration management
- **Coding Conventions**: Style guide, naming conventions, line length limits
- **Testing Guidelines**: When to add tests, test types, and test helpers
- **Git & PR Conventions**: Commit message format, PR requirements
- **CI/CD Pipeline**: GitHub Actions workflows, pre-commit hooks
- **Authentication & Security**: Device flow, PAT usage, security best practices
- **Debugging**: Trace logging, common issues and solutions

## Key Reminders for Copilot

1. **Always disable CGO**: Use `make build` or `CGO_ENABLED=0 go build`
2. **Never log errors in functions**: Return them and handle at top level
3. **Run quality gates before PR**: format → build → lint → test
4. **Use correct build tags**: `-tags=integration` or `-tags=e2e` for tests
5. **Follow verb-noun command pattern**: `kongctl <verb> <resource>`
6. **Respect line length**: Max 120 characters (golines enforces)
7. **Document markdown at 80 chars**: When writing docs, wrap at 80 chars

## Quick Reference

See [AGENTS.md](../AGENTS.md) for detailed commands and examples.

