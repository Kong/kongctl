# Building E2E Scenarios: Complete Getting Started Guide

This guide walks you through building and running your first end-to-end
scenario for `kongctl`.

## What is a Scenario?

A scenario is a declarative YAML-based test that executes `kongctl` commands
against a live Konnect organization and validates the results. Scenarios
replace imperative Go test code with a simple structure:

- **Base inputs**: Declarative config files (portals, control planes, etc.)
- **Steps**: Sequential actions (reset org, apply config, verify state) with overlay files to test differences in inputs
- **Commands**: Per step commands to run, including `kongctl` (apply, plan, get, sync, etc.), ad-hoc shell commands, or resource creation commands
- **Assertions**: Validate command outputs using JMESPath selectors

Scenarios are simple yaml files only, making them easy to write, review, and maintain
without deep Go or `kongctl` knowledge.

## Prerequisites

### Required Tools

- **Go 1.24.3 or later**: The e2e harness builds kongctl using `go build`
  ```sh
  go version  # Should show go1.24.3 or later
  ```

### Test Organization Credentials

Set up a PAT for a test organization (this var name is special):
```sh
export KONGCTL_E2E_KONNECT_PAT="your-personal-access-token"
```

**WARNING**: Use a PAT for a test organization only. E2E scenarios reset the
org by deleting all resources (portals, control planes, etc.) before each test
run.

## Scenario Structure

A scenario is a YAML file in the `kongctl` repo at
`test/e2e/scenarios/<category>/<name>/scenario.yaml` that defines:

- **baseInputsPath**: Path to declarative config files to use as input
- **steps**: Sequential test steps (reset org, apply config, verify state)
- **commands**: kongctl commands to run in each step (apply, plan, get, etc.)
- **assertions**: Validations against command output using JMESPath selectors

## Building Your First Scenario

Let's build a simple scenario that creates a portal and verifies it exists.
In your terminal start from the `kongctl` project root directory.

### Step 1: Create the scenario directory structure

```sh
mkdir -p test/e2e/scenarios/portal/simple-portal-test/config/base
```

This creates:
- `simple-portal-test/` - The scenario root
- `simple-portal-test/config/base/` - Base declarative config files

### Step 2: Create base input files

Create a simple portal config at
`test/e2e/scenarios/portal/simple-portal-test/config/base/portal.yaml`:

```yaml
portals:
  - ref: test-portal
    name: "My Test Portal"
    auto_approve_applications: false
```

### Step 3: Write the scenario

Create `test/e2e/scenarios/portal/simple-portal-test/scenario.yaml`:

```yaml
baseInputsPath: ./config/base

steps:
  - name: 000-reset-org
    skipInputs: true
    commands:
      - resetOrg: true

  - name: 001-create-portal
    commands:
      - name: apply
        run:
          - apply
          - -f
          - "{{ .workdir }}/portal.yaml"
          - --auto-approve
        assertions:
          - select: "plan.metadata"
            expect:
              fields:
                mode: apply
          - select: >-
              plan.changes[?resource_type=='portal' &&
                            resource_ref=='test-portal'] | [0]
            expect:
              fields:
                action: CREATE

  - name: 002-verify-portal
    skipInputs: true
    commands:
      - name: get-portals
        run:
          - get
          - portals
          - -o
          - json
        assertions:
          - select: "[?name=='My Test Portal'] | [0]"
            expect:
              fields:
                name: "My Test Portal"
                auto_approve_applications: false
```

### Understanding the scenario

**baseInputsPath**:
This is a folder containing all the base data files needed for the scenario test.
Each step ran, this folder will be copied into the `.workdir` of that step and
the files are available to the step with `{{ .workdir }}` template

**Step 000-reset-org**:
- `skipInputs: true` - Don't copy base files, they aren't needed for org reset
- `resetOrg: true` - A helper command to delete all resources in the org (based on the `pat` provided)

**Step 001-create-portal**:
- Runs `kongctl apply` with the portal.yaml file
- `{{ .workdir }}` is the step's working directory where base inputs are
  copied
- First assertion checks the plan metadata shows mode is `apply`
- Second assertion verifies a `CREATE` action for the test-portal resource

**Step 002-verify-portal**:
- `skipInputs: true` - No input files needed
- Runs `kongctl get portals -o json`
- Assertion selects the portal by name using JMESPath
- Verifies the portal has expected field values

## Running Your Scenario

### Run your specific scenario

```sh
make scenario SCENARIO=portal/simple-portal-test
```

### Understanding the output

When the test completes, you'll see output like:

```
=== RUN   Test_Scenarios
=== RUN   Test_Scenarios/scenarios/portal/simple-portal-test/scenario.yaml
--- PASS: Test_Scenarios (1.91s)
    --- PASS: Test_Scenarios/scenarios/portal/simple-portal-test/scenario.yaml (1.91s)
PASS
ok  	github.com/kong/kongctl/test/e2e	3.019s
E2E artifacts: /home/<user>/go/e2e-artifacts/20251217-105734
```

This artifacts directory contains the complete test execution artifacts. After each test run,
a sym link `.latest-e2e` will be written in the project dir pointing to the latest test run. This
will allow you to easily navigate or open the latest test artifacts dir.

## Exploring Test Artifacts

```sh
pushd .latest-e2e
```

### Artifacts structure

```
20251217-105734/
├── bin/
│   └── kongctl                      # Built binary used for this test run
├── run.log                          # High level test output
└── tests/                           # Each scenario gets a folder under here
    └── Test_Scenarios_scenarios_portal_simple-portal-test_scenario.yaml/
        ├── config/                  # this is kongctl CLI configuration 
        │   ├── kongctl/
        │   │   └── config.yaml      # Generated kongctl config for test
        │   └── logs/                # Test-level logs
        └── steps/
            ├── 000-reset-org/       # Each step gets a folder
            │   ├── inputs/          # Empty (skipInputs: true)
            │   └── commands/
            │       └── 000-reset_org/
            │           ├── command.txt
            │           └── observation.json
            ├── 001-create-portal/
            │   ├── inputs/          # Copy of base inputs for this step
            │   │   └── portal.yaml
            │   └── commands/
            │       └── apply/
            │           ├── command.txt      # Exact command executed
            │           ├── stdout.txt       # Captured STDOUT 
            │           ├── stderr.txt       # Captured STDERR
            │           ├── kongctl.log      # Kongctl's log output
            │           ├── env.json         # Command environment 
            │           ├── meta.json        # Command result metadata
            │           └── assertions/
            │               ├── assert-000/
            │               │   ├── select.txt      # JMESPath selector used
            │               │   ├── observed.json   # Selected data
            │               │   ├── expected.json   # Expected data
            │               │   └── result.txt      # Pass/fail + diff
            │               └── assert-001/
            │                   └── ...
            └── 002-verify-portal/
                ├── inputs/          # Empty (skipInputs: true)
                └── commands/
                    └── get-portals/
                        ├── command.txt
                        ├── stdout.txt
                        ├── stderr.txt
                        ├── kongctl.log
                        ├── env.json
                        ├── meta.json
                        └── assertions/
                            └── assert-000/
                                ├── select.txt
                                ├── observed.json
                                ├── expected.json
                                └── result.txt
```

### Key files to examine

- **bin/kongctl**: The built binary used for this test run
- **tests/.../steps/NNN-step-name/inputs/**: Copy of base inputs for the step
- **command.txt**: The exact command line executed
- **stdout.txt**: Raw command output (useful for debugging)
- **stderr.txt**: Error output from the command
- **kongctl.log**: Kongctl's internal logs for the command
- **env.json**: Environment variables used for the command
- **observed.json**: The data selected by your JMESPath expression
- **expected.json**: What you expected to see
- **result.txt**: Shows "pass" or "fail" with a diff if they don't match

## Debugging Failed Scenarios

When an assertion fails:

1. Navigate to the test directory:
   `tests/Test_Scenarios_scenarios_portal_simple-portal-test_scenario.yaml/`
2. Find the failing step in `steps/NNN-step-name/`
3. Check `commands/<cmd-name>/stderr.txt` for command errors
4. Review `commands/<cmd-name>/kongctl.log` for internal kongctl logs
5. Look at `assertions/assert-NNN/result.txt` for the diff
6. Compare `observed.json` vs `expected.json` to see what's different
7. Verify your JMESPath selector in `select.txt` is correct
8. Check `command.txt` to see the exact command that was executed

## Next Steps

- Review `docs/e2e-scenarios.md` for advanced features (overlays, masking,
  retries)
- Look at existing scenarios in `test/e2e/scenarios/` for patterns
- Run scenarios during development to catch regressions early

## Common Patterns

### Using field assertions vs file assertions

**Field assertions** (shown above) are simpler for basic checks:
```yaml
expect:
  fields:
    name: "My Test Portal"
    auto_approve_applications: false
```

### Reusing base inputs across steps

By default, each step gets a fresh copy of `baseInputsPath` into its `inputs/`
directory. Use `skipInputs: true` when you don't need input files (like for
`get` commands or the `resetOrg` helper).

### Using overlays to test updates

Create an overlay file to modify the base config for subsequent steps:

1. Create `config/overlays/update-description/portal.yaml`:
   ```yaml
   portals:
     - ref: test-portal
       description: "Updated portal description"
   ```

2. Add a step using the overlay:
   ```yaml
   - name: 003-update-portal
     inputOverlayDirs:
       - ./config/overlays/update-description
     commands:
       - name: apply-update
         run:
           - apply
           - -f
           - "{{ .workdir }}/portal.yaml"
           - --auto-approve
         assertions:
           - select: >-
               plan.changes[?resource_type=='portal' &&
                             resource_ref=='test-portal'] | [0]
             expect:
               fields:
                 action: UPDATE

   - name: 004-verify-update
     skipInputs: true
     commands:
       - name: get-portals
         run: [get, portals, -o, json]
         assertions:
           - select: "[?name=='My Test Portal'] | [0]"
             expect:
               fields:
                 description: "Updated portal description"
   ```

The overlay merges with the base config, allowing you to test incremental
changes without duplicating the entire configuration file.
