# Import a complete organization chart

An organization chart comes from the native CSV import operation. The importer
creates user principals, teams and manager relationships. It is distinct from
ordinary declarative principal/group configuration and from the broader planned
arbitrary-principal CSV importer.

## Prepare the roster and directory

The [synthetic CSV example](org-chart.csv) has these required headers:
`external_id`, `email`, `given_name`, `family_name` and `manager_external_id`.
Its root has an empty manager cell; other manager values refer to external IDs
in the file. Optional `cost_center` omission preserves existing values; a blank
cell removes the metadata key. CSV must be UTF-8 with RFC 4180 quoting and
between 1 and 50000 data rows. Other canonical-format rules are tracked in Q48.

Use a complete roster for the selected directory. An actual import can adopt
existing users by email, deactivate absent imported users and remove emptied
teams. Multiple independent files are not established as isolated import
sources. Replace the example data with your intended complete roster before
executing it.

The usual target is the directory named `default`:

```sh
./kongctl get identity directories
./kongctl get identity directory default -o json
```

If it is absent, bootstrap it separately using
[directories.yaml](directories.yaml). No directory adoption is required for
explicit import. Selection never silently creates or adopts a directory.
Explicit imports use API authorization; they do not enforce declarative
namespace/protection or stamp KongCTL ownership.

## Preview without writing

Choose an existing private directory for report files. Every invocation needs
a new report filename; existing files and symlinks are never overwritten.

```sh
./kongctl create identity org-chart-import \
  --directory-name default \
  --file docs/examples/declarative/identity/org-chart.csv \
  --dry-run \
  --report-output-file /path/to/private/org-preview.json
```

Exactly one of `--dry-run` and `--execute` is required. Local validation checks
UTF-8, CSV syntax and row count. The API validates the canonical columns and
whole hierarchy; the command sends one complete multipart file, not a loop of
individual desired-state writes.

Normal JSON/YAML/text output shows directory ID, mode, input row count, public
counts, diagnostic totals and value-free notices. Known warning codes and data
row numbers are shown; row errors show only canonical field names and row
numbers. Unknown diagnostic types/fields become `other`. Detailed messages,
employee names/emails and unknown response fields stay in the new 0600 report.

The private JSON file contains `directory_id`, `dry_run`, the exact uploaded
bytes' `request_sha256`, capture `status` and the complete native `report`.
The adoption count distinguishes matched existing users from newly created
users. Capture status `captured` means the response was saved, not that an
execution succeeded.

Inspect the full private report, especially warnings, before execution.
The preview is not a saved plan or a server-side reservation. A later execution
uploads its file again against the then-current directory state.

## Execute and inspect

```sh
./kongctl create identity org-chart-import \
  --directory-name default \
  --file docs/examples/declarative/identity/org-chart.csv \
  --execute \
  --report-output-file /path/to/private/org-execution-01.json

./kongctl get identity principals --directory-name default -o json
./kongctl get identity groups --directory-name default -o json
./kongctl get identity group-memberships \
  --directory-name default --group-id TEAM_UUID -o json
```

The API preserves IDs on identical re-import and reports unchanged users.
Use another output filename for each run. Manager changes reconcile team
memberships; dropping an imported user deactivates that record. An unrelated
manually created user is retained in the tested workflow.

A returning user keeps its UUID but remains inactive in the tested API.
Normal output includes `inactive_not_reactivated`; the full report explains
that import did not reactivate the person. The command never patches active
state to make a file appear fully applied. Q51 asks for the intended return and
reactivation lifecycle. `status: completed` means an error-free native report
was received; inspect counts and warnings to understand the resulting state.

## Errors and uncertain outcomes

HTTP 200 can contain row errors. The command captures the full report, prints
`status: reported_errors` with safe row/field references, then exits with an
error.
It does not assume all possible error reports prove rollback. Native validation
errors in the tested workflow leave the whole directory unchanged.

Transport/API failures are also captured privately when possible. The reserved
file remains if capture is incomplete; a non-JSON response is retained as
`response_body_base64`. Read errors may leave only the request context. Normal
errors withhold response text. Request/response bodies and read/transport
diagnostics are redacted in import traces.

Both SDK and transport retries are disabled for preview and execution. An
uncertain request is never automatically repeated or undone. Inspect the
private file and directory state, then generate a fresh preview before deciding
whether to retry. The local hash identifies the uploaded bytes; it is not a
remote resource identity or a transaction guarantee.

## Ownership and ordinary configuration

Imported users and teams expose `provisioned_by: imported`. A user adopted by
import retains its UUID and may retain old `managed_by` values. Those markers
do not make the principal writable through ordinary PUT: the tested API returns
409. KongCTL rejects managed principal declarations, scoped pruning and adoption
for that current provisioning state. Saved writes repeat the check; unscoped
omission leaves those records alone.

Directory dump exports imported principals as external UUID references,
including users with retained KongCTL markers. Ordinary imported teams and
memberships stay outside automatic configuration pruning. Independently owned
children keep their own ownership rules.

Explicit native deactivation changes an imported principal to `konnect_user`
and permits deletion in the tested environment. This import command does not
silently perform that release, activate a returning user, or delete principal
records for absent CSV rows. These operations need their own explicit intent.
Q49 covers transfer/re-import behavior and Q50 covers public schema gaps.

This is an operational workflow, not an import declaration with automatic
plan/apply/sync/delete/dump parity. It does not satisfy general CSV import for
arbitrary principal identities, credentials or metadata modes in #1419.
The `/org-root` read remains gated and has no command in this workflow.
See the [API question log][questions] and
[compatibility tracker][compatibility].

[questions]: ../../../kong-identity-api-questions.md
[compatibility]: ../../../contributor/kong-identity-compatibility.md
