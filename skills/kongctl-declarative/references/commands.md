# Declarative Command Reference

Use this file for fast command selection and execution patterns.
Use it in both modes:

- User-run mode: provide commands and explain intent/effect.
- Agent-run mode: execute commands and report outcomes.

Use command help for ground-truth syntax in environments without local docs.

## Command Roles

- `kongctl explain`: Inspect declarative schema for a resource path or field.
- `kongctl scaffold`: Generate commented starter YAML for a resource path.
- `kongctl plan`: Generate a plan artifact without executing changes.
- `kongctl diff`: Preview planned changes in text, JSON, or YAML.
- `kongctl apply`: Create and update resources. No deletes.
- `kongctl sync`: Create, update, and delete to match desired state.
- `kongctl delete`: Delete resources defined in the input files.
- `kongctl dump declarative`: Export live resources into declarative YAML.
- `kongctl adopt`: Label existing resources with `KONGCTL-namespace`.
- `deck file openapi2kong`: Convert OpenAPI to decK Gateway state.
- `deck file patch`: Apply JSONPath-based patches to decK files.
- `deck file add-plugins`: Add plugins to selected decK entities.
- `deck file add-tags`: Add tags to selected decK entities.

## Intent to Command

Use these intent mappings:

1. Discover fields, required values, nesting, or YAML placement
   `kongctl explain <resource-path> --extended -o text`
   `kongctl explain <resource-path> -o json`
2. Generate starter YAML for a resource or child resource
   `kongctl scaffold <resource-path>`
3. Preview create/update only
   `kongctl diff -f <path> --mode apply -o text`
4. Preview full converge including deletes
   `kongctl diff -f <path> --mode sync -o text`
5. Generate a reviewable plan file
   `kongctl plan -f <path> --mode <apply|sync|delete> --output-file <plan.json>`
6. Execute a saved plan artifact
   `kongctl apply --plan <plan.json>`
   `kongctl sync --plan <plan.json>`
7. Execute create/update now (inline plan+execute)
   `kongctl apply -f <path> --dry-run -o text`
   `kongctl apply -f <path> -o text`
8. Execute full converge now (inline plan+execute)
   `kongctl sync -f <path> --dry-run -o text`
   `kongctl sync -f <path> -o text`
9. Execute delete workflow now
   `kongctl delete -f <path> --dry-run -o text`
   `kongctl delete -f <path> -o text`
10. Dump existing resources to declarative YAML
   `kongctl dump declarative --resources=<types> -o yaml --output-file <file>`
11. Adopt unmanaged resource into a namespace
   `kongctl adopt <resource> <name-or-id> --namespace <namespace> -o json`
12. Generate Gateway runtime config from OpenAPI
   ```bash
   deck file openapi2kong \
     --spec <openapi.yaml> \
     --select-tag <tag> \
     --output-file <gateway.yaml>
   ```
13. Patch or enrich generated decK state
   ```bash
   deck file patch --state <gateway.yaml> --output-file <out.yaml>
   deck file add-plugins --state <gateway.yaml> --output-file <out.yaml>
   deck file add-tags --state <gateway.yaml> --output-file <out.yaml> <tag>
   ```

## Explain Command Detail

`kongctl explain` is the first choice for schema questions. It works without
Konnect authentication and accepts resource, child-resource, and field paths.

```bash
kongctl explain <resource-path> --extended -o text
kongctl explain <resource-path> -o json
kongctl explain <resource-path> -o yaml
```

Resource path examples:

- `api`
- `api.versions`
- `api.publications.portal_id`
- `portal.pages`
- `control_plane.data_plane_certificates`

Use text output for human-readable summaries. Use `--extended` with text
output when field details are needed. Use `-o json` or `-o yaml` when an
agent needs machine-readable JSON Schema, required fields, root keys,
resource class, or nesting metadata.

For programmatic authoring, treat JSON Schema as the source of truth. When a
schema contains `oneOf`, choose exactly one branch. Branches commonly use a
discriminator field with `const`, such as `strategy_type: key_auth` or
`type: tls_server`.

## Scaffold Command Detail

`kongctl scaffold` prints commented YAML starter configuration for a resource
path. Use it before hand-writing new resource shapes.

```bash
kongctl scaffold api
kongctl scaffold api.versions
kongctl scaffold control_plane.data_plane_certificates
```

Important behavior:

- It writes YAML to stdout.
- It does not support `-o` or `--output`.
- To save output in User-run mode, tell the user to redirect stdout:
  `kongctl scaffold api > konnect/resources/apis.yaml`
- Replace placeholder refs and values such as `my-resource` before planning.
- Un-comment optional fields only when the user needs them.
- `# oneOf option: ...` marks mutually exclusive alternatives.
- One `oneOf` branch is active and the other branches are commented.
- To switch variants, uncomment one whole branch and comment or remove the
  previously active branch.
- Common fields outside `# oneOf option: ...` blocks apply to all variants.
- Do not merge branch-specific `config` or `configs` blocks from multiple
  `oneOf` options.

## decK APIOps Command Detail

Use decK file commands when generating or modifying Kong Gateway runtime
configuration files. These commands work on local files and are separate from
the `kongctl apply/sync` execution that later runs decK through `_deck`.

```bash
deck file openapi2kong --spec <openapi.yaml> --select-tag <tag> \
  --output-file <gateway.yaml>
deck file patch --state <gateway.yaml> --selector '$..services[*]' \
  --value 'read_timeout:30000' --output-file <patched.yaml>
deck file add-plugins --state <gateway.yaml> --selector '$..services[*]' \
  --config '{"name":"key-auth"}' --output-file <plugins.yaml>
deck file add-tags --state <gateway.yaml> --selector 'services[*]' \
  --output-file <tagged.yaml> <tag>
```

Load `references/deck-gateway.md` for `_deck`, select-tag, external
`gateway_services`, and API implementation wiring patterns.

## Adopt Command Detail

`kongctl adopt` labels an existing Konnect resource with
`KONGCTL-namespace: <namespace>` so the declarative engine recognizes it as
managed. Adopt does not modify any resource fields — it only sets the label.

```bash
kongctl adopt <resource-type> <name-or-id> --namespace <namespace> -o json
```

- `<resource-type>`: parent resource type (e.g. `portal`, `api`,
  `control-plane`)
- `<name-or-id>`: resource name or UUID
- `--namespace`: the declarative namespace to assign

Adopt must run before dump when bringing an existing resource under
declarative management.

## Dump Command Detail

`kongctl dump declarative` exports live Konnect resource state as declarative
YAML suitable for use with `plan`, `apply`, and `sync`.

```bash
kongctl dump declarative \
  --resources=<type> \
  --filter-name "<name>" \
  --include-child-resources \
  --default-namespace <namespace> \
  -o yaml \
  --output-file <path>
```

Key flags:

- `--resources=<type>`: resource type to dump (e.g. `portal`, `api`,
  `control_planes`)
- `--filter-name "<name>"`: scope to a single resource by name
- `--filter-id "<uuid>"`: scope to a single resource by ID
- `--include-child-resources`: include nested child resources
- `--default-namespace <ns>`: add `_defaults.kongctl.namespace` block to
  output
- `--output-file <path>`: write output to a file instead of stdout
- `-o yaml`: output format (yaml is standard for declarative config)

Output behavior:

- `ref` values default to the resource UUID. Replace with human-friendly
  names during integration.
- `KONGCTL-namespace` labels are filtered from output by design.
- `--default-namespace` adds a `_defaults.kongctl` block rather than
  per-resource `kongctl` metadata.

## Plan Modes

- `--mode apply`: Create and update only.
- `--mode sync`: Create, update, and delete missing resources.
- `--mode delete`: Plan only deletions for matching input scope.

`kongctl delete` is a convenience wrapper around declarative delete planning
and execution.

## Common Flags

- `--recursive`: load all YAML files in a directory tree. Use when `-f`
  points to a directory containing only kongctl declarative resource files.
  Non-resource YAML (specs, docs, etc.) in the tree will cause parse errors.
  When non-resource files coexist, use multiple `-f` flags instead:
  `kongctl diff -f resources/apis.yaml -f resources/portals.yaml --mode apply`
- `--base-dir <path>`: set the root for `!file` path resolution. Required
  when `!file` tags reference files outside the `-f` directory. Use an
  absolute path — relative values resolve from the config file directory,
  not cwd (e.g. `--base-dir "$(pwd)"`).

## Output and Approval

- Mutating commands (`apply`, `sync`, `delete`) with `-o json` or `-o yaml`
  require `--auto-approve` or `--dry-run` because interactive confirmation
  is not available with structured output.
- Use `-o text` for interactive runs that prompt for confirmation.
- Use `-o json --auto-approve` for non-interactive or scripted execution.

## Safety Defaults

- Prefer `--dry-run` for `apply`, `sync`, and `delete` before execution.
- Use `--require-any-namespace` or `--require-namespace` as guardrails.
- Set explicit output with `-o <text|json|yaml>`.
- Use `--profile <name>` when environment separation matters.

## Command Discovery

When syntax is uncertain, check help text:

```bash
kongctl explain --help
kongctl scaffold --help
kongctl plan --help
kongctl diff --help
kongctl apply --help
kongctl sync --help
kongctl delete --help
kongctl dump declarative --help
kongctl adopt --help
deck file openapi2kong --help
deck file patch --help
deck file add-plugins --help
deck file add-tags --help
```
