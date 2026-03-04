---
name: kongctl-declarative
description: Use this skill to author, organize, and operate kongctl declarative
  configuration for Konnect resources. Generate new declarative manifests from
  user requests or OpenAPI specs, teach users how to manage Konnect resources
  declaratively, and use plan/diff/apply/sync/delete/adopt workflows with
  namespace guardrails.
license: Apache-2.0
metadata:
  product: kongctl
  category: declarative
  scope: declarative-config-management
---

# kongctl declarative workflows

## Goal

Generate and maintain `kongctl` declarative configuration in the repository,
and teach users how to manage Konnect resources declaratively.

Choose the execution approach from user intent:

- User-run mode: provide commands and explain what each command does.
- Agent-run mode: execute commands directly and report outcomes.

## Preconditions

- Confirm CLI is installed and runnable: `kongctl version`
- Authenticate with one of:
  - `export KONGCTL_DEFAULT_KONNECT_PAT=<token>`
  - `kongctl login` # interactive browser based login flow
- Select configuration profile when needed: `--profile <name>`
- Verify command syntax when unsure:
  - `kongctl plan --help`
  - `kongctl dump declarative --help`
  - `kongctl adopt --help`

## Config and Environment Overrides

- `kongctl` flags can be defaulted via profile config or environment.
- Environment variable pattern: `KONGCTL_<PROFILE>_<PATH>`.
- Example: `KONGCTL_DEFAULT_OUTPUT=yaml` changes default output format.
- Pass explicit `-o yaml`, `-o json`, or `-o text` on command lines to avoid
  unexpected output behavior.

## Skill References

Load only the reference file needed for the active task:

- `references/commands.md`
  - Use for command selection, plan vs inline execution, and safety flags.
- `references/resources.md`
  - Use for resource skeletons, `_defaults`, `!file`, and `!ref` patterns.
- `references/troubleshooting.md`
  - Use for common failures and fast remediation steps.

This skill is designed to be portable across repositories. Do not assume a
local `docs/` directory exists.

If field-level uncertainty remains after reading `references/`, discover
structure from live data using `kongctl dump declarative`.

## Operating Rules

- Declarative execution is always plan-based in kongctl:
  - Explicit path: `plan` -> `diff --plan` -> `apply/sync --plan`
  - Inline path: `apply -f`, `sync -f`, or `delete -f` (plan+execute inline)
- Prefer instructional guidance when the user asks how to do a task:
  provide a concise command sequence and decision notes.
- Execute commands directly when the user asks the agent to run them.
- Before any mutating run, state the intended effect in plain language.
- Choose path from user intent:
  - Preview/review/audit/CI request: use explicit plan artifacts.
  - "Do it now" execution request: use inline commands.
- Treat `sync` as destructive because it can delete missing resources.
- Treat `delete` as destructive because it deletes input configuration
  resources.
- Use `apply` for create/update workflows when the user asks to execute.
- Use `adopt` only to bring unmanaged resources into namespace management.
- `adopt` only adds the `KONGCTL-namespace` label to the target resource.
- Prefer `!ref` for cross-resource IDs and `!file` for large spec or
  doc content.
- Keep `!file` paths within the configured base directory boundary.
- Put `kongctl` metadata only on parent resources.
- Use `_defaults.kongctl.namespace` for consistent file-level ownership.
- Do not require `docs/declarative*.md` to complete tasks.

## Workflow

1. Locate the Konnect declarative configuration directory and existing files.
2. Generate or update YAML manifests and related files.
3. Choose collaboration mode based on intent:
   - User-run mode (teach + provide commands)
   - Agent-run mode (execute commands directly)
4. Load the relevant `references/*.md` file for the task.
5. If execution is needed, pick execution style:
   - Explicit plan artifact workflow (generate plan, then pass it to apply or
     sync)
   - Inline command workflow
6. Validate and/or execute `diff`, `apply`, `sync`, `delete`, or `adopt` per
   user ask.
7. Report created files, commands run, and resulting plan or execution
   summary.

### Execution Style Selection

Use this quick decision rule:

- Use explicit plan artifacts when the user asks to preview, review, export a
  plan file, or run a CI/CD style workflow.
- Use inline commands when the user asks to execute immediately and does not
  ask for a saved plan file.
- For destructive requests (`sync`, `delete`), prefer `--dry-run` first unless
  the user explicitly requests direct execution.

### Collaboration Mode Selection

Use this quick decision rule:

- Use User-run mode when the user asks "how do I do this" or asks for commands.
- Use Agent-run mode when the user asks the agent to run changes now.
- If intent is ambiguous, provide a safe preview command first and state that
  you can execute the mutating command on request.

### Konnect Configuration Directory Discovery

Prefer a user-provided path. If none is provided, discover likely roots:

```bash
rg -n --glob '*.y*ml' '^(_defaults|apis|portals|control_planes):' .
rg -n --glob '*.y*ml' \
  '^(application_auth_strategies|event_gateways|organization):' .
```

If creating new structure, prefer:

```text
konnect/resources/
  control-planes.yaml
  portals.yaml
  apis.yaml
  specs/
```

## Pattern: Bootstrap control plane, portal, and APIs

Use for prompts like:

- Generate declarative configs for a new control plane, portal, and API set.

Steps:

1. Create or update resource files under the selected resources path.
2. Add `_defaults.kongctl.namespace` and stable `ref` values.
3. Use `!file` tags to load OpenAPI metadata and version content.
4. Link API publications to portal using `!ref <portal-ref>#id`.
5. Validate and execute based on requested style.
6. In User-run mode, explain each command's purpose before listing it.

Starter manifest pattern:

```yaml
_defaults:
  kongctl:
    namespace: platform-dev
    protected: false

control_planes:
  - ref: cp-main
    name: "my-control-plane"
    cluster_type: "CLUSTER_TYPE_CONTROL_PLANE"

portals:
  - ref: dev-portal
    name: "my-dev-portal"
    display_name: "My Dev Portal"
    default_api_visibility: "private"
    default_page_visibility: "private"

apis:
  - ref: payments-api
    name: !file ./specs/payments-openapi.yaml#info.title
    description: !file ./specs/payments-openapi.yaml#info.description
    versions:
      - ref: payments-v1
        version: !file ./specs/payments-openapi.yaml#info.version
        spec: !file ./specs/payments-openapi.yaml
    publications:
      - ref: payments-publication
        portal_id: !ref dev-portal#id
        visibility: public
```

Verification commands:

```bash
kongctl plan -f <konnect-resources-path> --recursive --mode apply -o json
kongctl diff -f <konnect-resources-path> --recursive --mode apply -o text
```

Inline execution commands:

```bash
kongctl apply -f <konnect-resources-path> --recursive --dry-run -o text
kongctl apply -f <konnect-resources-path> --recursive -o text
```

## Pattern: Generate API config from an OpenAPI spec

Use for prompts like:

- Generate an API declarative config from `@path-to/openapi-spec.yaml` and
  write it to `@path-to-existing/konnect/resources`.

Steps:

1. Choose target files under the existing resources tree, such as:
   - `<resources>/apis/<api-name>.yaml`
   - `<resources>/specs/<api-name>/openapi.yaml`
2. Preserve existing repo conventions when a layout already exists.
3. Reference spec fields with `!file` extraction.
4. Keep spec files inside the base-dir boundary or set `--base-dir`.
5. Validate and execute based on requested style.
6. In User-run mode, include where files were written and why.

Starter API block:

```yaml
apis:
  - ref: my-api
    name: !file ./specs/my-api/openapi.yaml#info.title
    description: !file ./specs/my-api/openapi.yaml#info.description
    version: !file ./specs/my-api/openapi.yaml#info.version
    versions:
      - ref: my-api-v1
        version: !file ./specs/my-api/openapi.yaml#info.version
        spec: !file ./specs/my-api/openapi.yaml
```

Validation command:

```bash
kongctl plan -f <api-config-file-or-dir> --mode apply -o json
```

Inline execution command:

```bash
kongctl apply -f <api-config-file-or-dir> --dry-run -o text
```

## Pattern: Dump a portal and adopt it

Use for prompts like:

- Dump portal `My Dev Portal` into `@path/to/new/portalfile.yaml` and adopt it
  into declarative management.

Steps:

1. Dump the target portal config to YAML.
2. Adopt the live portal into the same namespace.
3. Add dumped file into the repo's declarative resource set.
4. Run `diff` or `plan` to confirm no unexpected drift.
5. In User-run mode, call out that `adopt` labels ownership only.

Commands:

```bash
kongctl dump declarative \
  --resources=portal \
  --filter-name "My Dev Portal" \
  --include-child-resources \
  --default-namespace <namespace> \
  -o yaml \
  --output-file <path/to/new/portalfile.yaml>

kongctl adopt portal "My Dev Portal" --namespace <namespace> -o json

kongctl diff -f <path/to/new/portalfile.yaml> --mode apply -o text
```

If names are ambiguous, use `--filter-id` for dump and adopt by ID.

## Safety and Troubleshooting

- If `!file` fails with boundary errors, move files under the Konnect
  configuration directory or set `--base-dir`.
- If validation fails for unknown fields, verify names in
  `references/resources.md`.
- If adoption fails because namespace already exists, align
  `_defaults.kongctl.namespace` with the existing label.
- If `plan` includes unexpected deletes, use `--mode apply` or tighten
  scope with `--require-namespace`.
- Use `--dry-run` for `apply`, `sync`, and `delete` before executing changes.
