# Declarative CI/CD with GitHub Actions

Use this file when asked to create or modify GitHub Actions workflows for
`kongctl` declarative operations.

## Workflow Design Checklist

1. Confirm trigger intent:
   - Pull request validation only
   - Branch deployment (for example `main`)
   - Manual execution (`workflow_dispatch`)
2. Separate non-mutating and mutating behavior:
   - Validation jobs should run `plan` or `diff`.
   - Deployment jobs can run `apply` or `sync`.
3. Install required tooling:
   - `kong/setup-kongctl@v1`
   - `kong/setup-deck@v1` when deck is required
4. Inject secrets via workflow `env` or step-level `env`.
5. Upload artifacts for audit and troubleshooting.

## Trigger Patterns

Validation on pull request:

```yaml
on:
  pull_request:
    paths:
      - konnect/**
```

Deployment on main branch:

```yaml
on:
  push:
    branches:
      - main
    paths:
      - konnect/**
```

Manual fallback:

```yaml
on:
  workflow_dispatch:
```

## Common Job Steps

```yaml
steps:
  - uses: actions/checkout@v4
  - name: Install kongctl
    uses: kong/setup-kongctl@v1
  - name: Install deck
    uses: kong/setup-deck@v1
```

Use a repository script when available (`./scripts/konnect-sync.sh`) so CI
logic and local developer flows stay aligned.

## Auth and Environment Conventions

- Keep credentials in GitHub Secrets.
- Match environment variable names to repository scripts.
- Common names:
  - `KONNECT_TOKEN`
  - `KONNECT_REGION`
- For direct `kongctl` commands, you can also map:
  - `KONGCTL_DEFAULT_KONNECT_PAT: ${{ secrets.KONNECT_TOKEN }}`

## Deployment Workflow Pattern

Use this pattern for a main-branch sync with artifact capture:

```yaml
name: Konnect Sync

on:
  workflow_dispatch:
  push:
    branches:
      - main
    paths:
      - konnect/**

jobs:
  sync:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install kongctl
        uses: kong/setup-kongctl@v1
      - name: Install deck
        uses: kong/setup-deck@v1
      - name: Sync Konnect resources
        env:
          KONNECT_TOKEN: ${{ secrets.KONNECT_TOKEN }}
          KONNECT_REGION: ${{ secrets.KONNECT_REGION }}
        run: ./scripts/konnect-sync.sh
      - name: Upload Konnect sync artifacts
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: konnect-sync
          path: .konnect/
```

## Validation Workflow Pattern

Use this pattern for pull-request safety checks:

```yaml
name: Konnect Validate

on:
  pull_request:
    paths:
      - konnect/**

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install kongctl
        uses: kong/setup-kongctl@v1
      - name: Validate declarative config
        env:
          KONGCTL_DEFAULT_KONNECT_PAT: ${{ secrets.KONNECT_TOKEN }}
        run: |
          kongctl plan -f konnect/resources --recursive --mode apply -o json \
            --output-file .konnect/plan.json
          kongctl diff -f konnect/resources --recursive --mode apply -o text
      - name: Upload validation artifacts
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: konnect-validate
          path: .konnect/
```

## APIOps Extension Point

When users ask for OpenAPI-to-gateway automation, keep the workflow structure
above and add repository-specific APIOps steps in scripts. Prefer script calls
over long inline command chains for maintainability.
