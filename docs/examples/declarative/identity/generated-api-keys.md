# Generated principal and group API keys

[generated-api-keys.yaml](generated-api-keys.yaml) creates a worker principal,
an ordinary group and one generated API key under each. It looks up the
existing directory named `default`; the directory stays unmanaged. If that
directory is absent, first use the [default-directory bootstrap](README.md).
The group key belongs to the group. This example does not attach the worker
as a group member or demonstrate runtime authentication.

Run from the repository root with your configured profile:

```sh
kongctl plan --mode apply \
  -f docs/examples/declarative/identity/generated-api-keys.yaml \
  --output-file generated-keys-plan.json
kongctl apply --plan generated-keys-plan.json --auto-approve \
  --secret-output-dir ~/kongctl-private/generated-keys
kongctl diff \
  -f docs/examples/declarative/identity/generated-api-keys.yaml
```

Review the four planned creates before executing. Creation requires a new
private capture file for each key. If recreating a previously deleted ref, use
a new `--secret-output-dir` and preserve the earlier capture; output paths
are stable for the same declared resource and are never overwritten. Kongctl
reserves every output before any planned write, captures the issued material before reading public metadata,
and prints file locations in the result. Files have mode `0600`. Keep them
outside source control and give the material to its intended consumer through
your normal secret distribution process. Never blindly repeat an uncertain
create; inspect its recovery file and the current resources first.

Inspect public metadata without printing key material:

```sh
kongctl get identity principals --directory-name default
kongctl get identity groups --directory-name default
kongctl get identity principal api-keys \
  --directory-name default --principal-id PRINCIPAL_UUID
kongctl get identity group api-keys \
  --directory-name default --group-id GROUP_UUID
```

Each generated key has four `managed_by` entries: `tool`, creator `version`,
`kongctl-ref` and `kongctl-create-type: v1`. Names are display values and need
not be unique. Matching uses the parent route and stored ref; duplicate refs
fail before changes. Client-side collection scanning is intentional.

Namespace and update/delete protection come from the current managed parent.
The key has no independent `kongctl` block. Its immutable map contains no
namespace or protection copies. A managed parent may use an external
directory. External principals use a separate namespace-based ownership
convention; see [keys for imported principals](external-principal-api-keys.md).
External groups still cannot authorize key writes.
Adopting or transferring a parent does not require rewriting its keys.

Required `type: v1` records generated creation intent. Imported keys and
existing keys without these markers are managed through the
[explicit key commands](principal-api-keys.md), with no declarative adoption.
The API currently ignores key management-map PATCH changes.

For updates, a timestamp sets expiration and `expires_at: null` removes it.
Omitting expiration preserves its current value, including during a name
update. Principal keys support scalar name updates. Group key names are
create-only; explicit null name clearing is unsupported for both kinds.
Creator-version differences never trigger a metadata update.

To rotate a generated key, change its ref to a new value, review a sync plan,
and execute with `--secret-output-dir`. The replacement is issued and captured
before the old managed key is retired. Application rollout and runtime
validation remain your responsibility; use staged manifests when a consumer
needs an overlap period.

Omitting `api_keys` during sync preserves the collection. Explicit
`api_keys: []` retires only recognized generated keys beneath that managed
parent. Foreign, imported and legacy keys are preserved, and block parent
deletion until explicitly retired. Declaring the managed keys in a delete
plan orders their retirement before fresh parent-empty checks.

```sh
kongctl dump declarative --resources identity.directories \
  --include-child-resources > identity-dump.yaml
kongctl plan --mode apply -f identity-dump.yaml
kongctl delete -f docs/examples/declarative/identity/generated-api-keys.yaml \
  --auto-approve
```

Dump includes recognized generated keys beneath managed parents and preserves
explicit null expiration. It omits material, private capture paths and the
management map. The external default directory is preserved by deletion.
