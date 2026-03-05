# Declarative Troubleshooting

Use this file for common failures when planning, applying, syncing, deleting,
dumping, or adopting declarative resources.

## Quick Triage

1. Confirm CLI and auth
   `kongctl version`
   `kongctl get organization -o json`
2. Validate parse and plan scope
   `kongctl plan -f <path> --mode apply -o json`
3. Inspect drift intent before destructive execution
   `kongctl diff -f <path> --mode sync -o text`

## Common Issues

### Parse Errors with `--recursive`

Symptom: `--recursive` fails with parse errors on files that are not
kongctl declarative resources (e.g. OpenAPI specs, documentation YAML).

Actions:

- Only place kongctl resource files in the resources directory.
- Keep specs, docs, and other YAML outside the resources directory.
- If non-resource files cannot be moved, use multiple `-f` flags for
  individual resource files instead of `--recursive`:
  `kongctl diff -f resources/apis.yaml -f resources/portals.yaml --mode apply`

### Unknown Field Errors

Symptom: parser rejects a field name.

Actions:

- Check field patterns in `references/resources.md`.
- If uncertain, generate live examples with `dump declarative`.
- Fix common typos like `lables` vs `labels`.
- Ensure `kongctl` metadata is only on parent resources.

### `!file` Resolution Errors

Symptom: file not found or base-dir boundary violation.

Actions:

- Verify path is relative to the declarative config file.
- Set an absolute base directory that includes existing spec paths:
  `kongctl plan -f <path> --base-dir "$(pwd)" --mode apply -o json`
- Move spec files only when the user explicitly asks to change layout.

### Unexpected Deletes in Sync

Symptom: `sync` plan includes deletes you did not expect.

Actions:

- Re-check with `diff --mode apply` to isolate create/update intent.
- Restrict scope with `--require-namespace=<ns>`.
- Use `--dry-run` before executing `sync` or `delete`.

### Namespace Label and Adopt Conflicts

Symptom: adopt fails or namespace ownership looks inconsistent.

Actions:

- Read current labels with query commands before adopting.
- If namespace label already exists, align `_defaults.kongctl.namespace`
  with the existing owner.
- Use adopt only for unmanaged resources you intend to manage declaratively.

### Dump Output Missing KONGCTL Labels

Symptom: dumped YAML does not contain `KONGCTL-namespace` labels even though
the resource was adopted.

This is expected behavior — `dump` filters out KONGCTL metadata labels by
design. Use `--default-namespace` to add a `_defaults.kongctl` block to the
output instead.

### Dump Ref Values Are UUIDs

Symptom: dumped `ref` fields contain UUIDs instead of human-friendly names.

This is expected — `dump` uses the resource UUID as the default `ref`. Replace
UUID refs with meaningful names during integration into the declarative config
repository.

### Drift After Adopt and Dump

Symptom: `diff` shows unexpected changes after integrating dumped config.

Actions:

- Ensure adopt ran before dump so the resource is labeled.
- Run `kongctl get <resource-type> <name> -o json` to compare live state
  against the dumped config.
- Check that `_defaults.kongctl.namespace` in the config matches the
  namespace used in the adopt command.

### Output or Profile Confusion

Symptom: output format or target account is unexpected.

Actions:

- Set explicit output: `-o text`, `-o json`, or `-o yaml`.
- Set explicit profile: `--profile <name>`.
- Inspect environment overrides:
  `env | grep '^KONGCTL_'`

### API/Auth/Network Failures

Symptom: plan or execution fails with auth or transport errors.

Actions:

- Re-authenticate with `kongctl login` or set `KONGCTL_DEFAULT_KONNECT_PAT`.
- Verify region/base URL configuration.
- Re-run with diagnostics:
  `--log-level debug` or `--log-level trace`
