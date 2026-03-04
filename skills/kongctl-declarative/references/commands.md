# Declarative Command Reference

Use this file for fast command selection and execution patterns.
Use it in both modes:

- User-run mode: provide commands and explain intent/effect.
- Agent-run mode: execute commands and report outcomes.

Use command help for ground-truth syntax in environments without local docs.

## Command Roles

- `kongctl plan`: Generate a plan artifact without executing changes.
- `kongctl diff`: Preview planned changes in text, JSON, or YAML.
- `kongctl apply`: Create and update resources. No deletes.
- `kongctl sync`: Create, update, and delete to match desired state.
- `kongctl delete`: Delete resources defined in the input files.
- `kongctl dump declarative`: Export live resources into declarative YAML.
- `kongctl adopt`: Label existing resources with `KONGCTL-namespace`.

## Intent to Command

Use these intent mappings:

1. Preview create/update only
   `kongctl diff -f <path> --mode apply -o text`
2. Preview full converge including deletes
   `kongctl diff -f <path> --mode sync -o text`
3. Generate a reviewable plan file
   `kongctl plan -f <path> --mode <apply|sync|delete> --output-file <plan.json>`
4. Execute a saved plan artifact
   `kongctl apply --plan <plan.json>`
   `kongctl sync --plan <plan.json>`
5. Execute create/update now (inline plan+execute)
   `kongctl apply -f <path> --dry-run -o text`
   `kongctl apply -f <path> -o text`
6. Execute full converge now (inline plan+execute)
   `kongctl sync -f <path> --dry-run -o text`
   `kongctl sync -f <path> -o text`
7. Execute delete workflow now
   `kongctl delete -f <path> --dry-run -o text`
   `kongctl delete -f <path> -o text`
8. Dump existing resources to declarative YAML
   `kongctl dump declarative --resources=<types> -o yaml --output-file <file>`
9. Adopt unmanaged resource into a namespace
   `kongctl adopt <resource> <name-or-id> --namespace <namespace> -o json`

## Plan Modes

- `--mode apply`: Create and update only.
- `--mode sync`: Create, update, and delete missing resources.
- `--mode delete`: Plan only deletions for matching input scope.

`kongctl delete` is a convenience wrapper around declarative delete planning
and execution.

## Safety Defaults

- Prefer `--dry-run` for `apply`, `sync`, and `delete` before execution.
- Use `--require-any-namespace` or `--require-namespace` as guardrails.
- Set explicit output with `-o <text|json|yaml>`.
- Use `--profile <name>` when environment separation matters.

## Command Discovery

When syntax is uncertain, check help text:

```bash
kongctl plan --help
kongctl diff --help
kongctl apply --help
kongctl sync --help
kongctl delete --help
kongctl dump declarative --help
kongctl adopt --help
```
