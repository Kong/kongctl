---
name: kongctl-declarative
description: Set up, initialize, and manage kongctl declarative configuration
  for Kong Konnect. Use when the user wants to configure a repository with
  Konnect declarative resources, create kongctl manifests (control planes,
  portals, APIs), generate config from OpenAPI specs, run plan/diff/apply/sync
  /delete/adopt workflows, or scaffold CI/CD pipelines for Konnect APIOps.
license: Apache-2.0
metadata:
  product: kongctl
  category: declarative
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
  - `kongctl login` — preferred for interactive use (browser-based OAuth)
  - `export KONGCTL_DEFAULT_KONNECT_PAT=<token>` — for non-interactive or CI
- PAT tokens are sensitive credentials. Never echo, log, or commit them.
  Prefer `kongctl login` for interactive sessions.
- Verify authentication works: `kongctl get organization -o json`
  This works with all token types (PAT, SPAT, browser login). If it
  returns organization info, auth is confirmed. Do not guess or try other
  commands to check auth.
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
  - Use for command selection, plan based vs inline execution, and safety flags.
- `references/resources.md`
  - Use for resource skeletons, `_defaults`, `!file`, and `!ref` patterns.
- `references/troubleshooting.md`
  - Use for common failures and fast remediation steps.
- `references/cicd-github-actions.md`
  - Use for GitHub Actions workflow patterns for declarative CI/CD.
- `references/apiops-openapi.md`
  - Use for OpenAPI source-of-truth patterns for APIs and API versions.

This skill is designed to be portable across repositories. Do not assume a
local `docs/` directory exists.

If field-level uncertainty remains after reading `references/`, discover
structure from live data using:
`kongctl dump declarative --resources=<resource-type>`.

## Operating Rules

- Declarative execution is always plan-based in kongctl:
  - Explicit path: `plan` -> `diff --plan` -> `apply/sync --plan`
  - Inline path: `apply -f`, `sync -f`, or `delete -f` (plan+execute
    happens in single shot)
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
- Use `-o text` for interactive mutating commands. `-o json` and `-o yaml`
  require `--auto-approve` or `--dry-run` on `apply`, `sync`, and `delete`.
- Use `adopt` only to bring unmanaged resources into namespace management.
- `adopt` only adds the `KONGCTL-namespace` label to the target resource.
- Prefer `!ref` for cross-resource IDs and `!file` for large spec or
  doc content.
- For `apis` and `apis.versions`, treat OpenAPI files as source of truth:
  derive fields from `!file` extraction and avoid stale duplicated literals.
- Use existing OpenAPI file paths from the user repository. Do not require a
  `konnect/resources/specs` layout.
- Only place kongctl declarative resource files in the resources directory.
  Do not put OpenAPI specs, documentation, or other non-resource YAML there.
  `--recursive` loads all YAML files in the directory tree and will fail on
  files that are not valid kongctl declarative resources.
- When non-resource YAML files coexist in a directory, use multiple `-f`
  flags pointing to individual resource files instead of `--recursive`:
  `kongctl diff -f resources/apis.yaml -f resources/portals.yaml --mode apply`
- `!file` paths resolve relative to the YAML file they appear in, not the
  project root. Calculate the relative path from the config file to the
  target file (e.g. `../../openapi.yaml` from `konnect/resources/apis.yaml`).
- `--base-dir` sets the allowed boundary for `!file` resolution but does
  not change the resolution base. Keep `!file` targets within the boundary.
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
- Use Agent-run mode when the user asks to set up, create, or apply.
- If intent is ambiguous, provide a safe preview command first and state that
  you can execute the mutating command on request.
- Do not ask clarifying questions when sensible defaults can be inferred
  from context (e.g. derive namespace from project name, include standard
  resources like control plane + portal + API). Proceed with defaults and
  let the user adjust afterward.

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
```

For APIOps API modeling, load `references/apiops-openapi.md`.

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
    name: !file <existing-openapi-path>#info.title
    description: !file <existing-openapi-path>#info.description
    versions:
      - ref: payments-v1
        version: !file <existing-openapi-path>#info.version
        spec: !file <existing-openapi-path>
    publications:
      - ref: payments-publication
        portal_id: !ref dev-portal#id
        visibility: public
```

When OpenAPI `!file` paths point outside the resources directory, set
`--base-dir` to the absolute project root so paths resolve correctly.
Relative `--base-dir` values resolve from the config file directory, not
cwd, so always use an absolute path:

```bash
kongctl diff -f konnect/resources --recursive --base-dir "$(pwd)" --mode apply -o text
```

Use `--recursive` when the `-f` target is a directory.

Use `references/commands.md` for validation and execution command patterns.

## Pattern: Generate API config from an OpenAPI spec

Use for prompts like:

- Generate an API declarative config from `@path-to/openapi-spec.yaml` and
  write it to `@path-to-existing/konnect/resources`.

Steps:

1. Choose target files under the existing resources tree, such as:
   - `<resources>/apis/<api-name>.yaml`
2. Reference existing OpenAPI spec paths in the repository. Do not require
   copying specs under the declarative resources directory.
3. Preserve existing repo conventions when a layout already exists.
4. Reference spec fields with `!file` extraction.
5. If spec files are outside the resources directory, add `--base-dir` to
   all `plan`/`diff`/`apply`/`sync` commands so `!file` paths resolve.
6. Validate and execute based on requested style.
7. In User-run mode, include where files were written and why.

Load `references/apiops-openapi.md` for the canonical API YAML template and
`references/commands.md` for validation and execution commands.

## Pattern: Adopt existing resources into declarative management

Use for prompts like:

- Adopt portal `My Dev Portal` and start managing it declaratively.
- I have resources in Konnect that I created in the UI. How do I bring them
  under kongctl?
- Dump my existing control plane into declarative config.

This pattern applies to any parent resource type (portal, api, control_plane,
etc.), not just portals.

### Background

Resources created outside kongctl (e.g. via the Konnect UI) do not have a
`KONGCTL-namespace` label. The declarative engine uses this label to track
which resources it manages and which namespace they belong to. To bring an
existing resource under declarative management:

1. **Adopt** adds the `KONGCTL-namespace` label to the live resource.
2. **Dump** exports the live resource state as declarative YAML.
3. **Integrate** the dumped config into the repository's declarative files.
4. **Verify** with `diff` to confirm zero drift.

Adopt must come before dump so the resource is labeled as managed before
generating config.

### Steps

1. Identify the target resource name (or ID) and choose a namespace.
2. Adopt the resource into the namespace:
   ```bash
   kongctl adopt <resource-type> <name-or-id> \
     --namespace <namespace> -o json
   ```
   This only adds the `KONGCTL-namespace` label — it does not modify the
   resource configuration.
3. Dump the resource to declarative YAML:
   ```bash
   kongctl dump declarative \
     --resources=<type> \
     --filter-name "<name>" \
     --include-child-resources \
     --default-namespace <namespace> \
     -o yaml \
     --output-file <output-path>
   ```
4. Integrate the dumped output into existing declarative files:
   - Replace the UUID-based `ref` values with human-friendly names.
   - Replace hard-coded UUIDs with `!ref` where the referenced resource is
     also managed declaratively (e.g. `portal_id: !ref dev-portal#id`).
   - Merge into existing resource files or create new ones following the
     repository layout conventions.
   - Ensure `_defaults.kongctl.namespace` matches the adopt namespace.
5. Verify zero drift:
   ```bash
   kongctl diff -f <resources-path> --mode apply -o text
   ```
   A clean diff confirms the dumped config matches live state.
6. In User-run mode, explain that `adopt` only labels — it does not change
   any resource fields.

### Dump output behavior

- `dump` sets `ref` to the resource UUID. Replace with meaningful names.
- `dump` filters out `KONGCTL-namespace` labels from output by design.
- `--default-namespace` adds a `_defaults.kongctl` block to the output.
- `--include-child-resources` includes nested resources (pages, snippets,
  versions, etc.).
- Use `--filter-name` or `--filter-id` to scope to a specific resource.

If names are ambiguous, use `--filter-id` for both adopt and dump.

## Pattern: Build GitHub Actions workflow for declarative CI/CD

Use for prompts like:

- Create a GitHub Actions workflow that validates and syncs Konnect
  declarative resources.
- Add CI/CD automation that installs `kongctl` and `deck` and runs the
  repository sync script.

Steps:

1. Decide trigger model from user intent:
   - Pull request validation: run plan/diff only (no mutations).
   - Branch deploy workflow: run apply/sync or a repo wrapper script.
2. Use standard setup actions:
   - `actions/checkout@v4`
   - `kong/setup-kongctl@v1`
   - `kong/setup-deck@v1` (when deck is required)
3. Configure authentication using repository secrets and workflow env.
4. Restrict execution with path filters, for example `konnect/**`.
5. Upload execution artifacts with `if: always()` for debugging and audits.
6. In User-run mode, explain required secrets and expected script behavior.

Load `references/cicd-github-actions.md` for starter workflow templates,
trigger patterns, auth conventions, and validation workflow examples.

## Safety and Troubleshooting

- Use `--dry-run` for `apply`, `sync`, and `delete` before executing changes.
- If `!file` fails with boundary errors, set `--base-dir` to include spec
  paths rather than moving files.
- If `plan` includes unexpected deletes, use `--mode apply` or tighten scope
  with `--require-namespace`.
- Load `references/troubleshooting.md` for detailed remediation steps.

## Online Documentation

If this skill's references are not sufficient, consult or direct users to:

- kongctl docs: https://developer.konghq.com/kongctl/
- Declarative guide: https://github.com/Kong/kongctl/blob/main/docs/declarative.md
- Resource reference: https://github.com/Kong/kongctl/blob/main/docs/declarative-resource-reference.md
