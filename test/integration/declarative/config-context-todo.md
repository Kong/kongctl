# Integration Test Config Context TODO

## Issue Summary
Command-level integration tests fail with "no config found in context" error when executing commands via `cmd.Execute()`. This happens because the cobra initialization chain (`cobra.OnInitialize(initConfig)`) doesn't properly execute in the test environment.

## Root Cause
1. The `root.init()` function sets up `cobra.OnInitialize(initConfig)`
2. Commands expect configuration to be in context via `config.ConfigKey`
3. The `PersistentPreRun` in root command sets the config in context
4. When tests call `cmd.Execute()`, the initialization chain doesn't run properly

## Current Workaround
Tests are marked as skipped with clear explanation while we work on a proper solution.

## Potential Solutions

### Option 1: Test-Specific Command Factory
Create a test helper that properly initializes commands with config context:
```go
func NewTestCommand(t *testing.T) (*cobra.Command, func()) {
    // Set up temp config
    // Initialize viper
    // Create command with proper context
    // Return cleanup function
}
```

### Option 2: Direct Context Setup
Skip the cobra initialization and directly set up the context:
```go
ctx := context.Background()
ctx = context.WithValue(ctx, config.ConfigKey, testConfig)
ctx = context.WithValue(ctx, iostreams.StreamsKey, streams)
// ... other context values
cmd.SetContext(ctx)
```

### Option 3: Test-Mode Flag
Add a test mode flag that bypasses normal config initialization and uses test defaults.

## Files Affected
- `/test/integration/declarative/apply_test.go`
- `/test/integration/declarative/sync_command_test.go`
- Any future command-level integration tests

## Implementation Priority
Low - The SDK factory mocking infrastructure is in place and working. The tests are written and can be enabled once the config context issue is resolved. This doesn't block other development work.