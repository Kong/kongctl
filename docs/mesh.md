# Kong Mesh

Kong Mesh commands require a Kong Mesh 3.0 or later control plane. Resource
types, aliases, scope, and write permissions come from the control plane's
discovery API. Kong Mesh resources use their native document format, separate
from kongctl's Konnect declarative manifests.

The [Mesh examples](examples/declarative/mesh/README.md) include runnable Mesh
and MeshTimeout documents with an apply/read/delete, dump, inspection, and
token walkthrough.

## Select a control plane

For a Konnect-hosted control plane, use the active profile's login or PAT:

```shell
kongctl get mesh control-planes
kongctl get mesh resource-types --control-plane-name production
kongctl get mesh meshes --control-plane-id <id>
```

For a self-managed control plane, use `--control-plane-url`. It supports an
optional `--control-plane-token`, `--ca-cert-file`, and paired
`--client-cert-file` / `--client-key-file`. Konnect credentials are used only
for hosted targets. Remote resource-file downloads use their own TLS trust
and receive no control-plane credentials or client certificate.

Provide only one ID, name, or URL selector flag. An explicit selector overrides
configured selectors. With no selector flag, configured values are consulted
in this order: URL, ID, name. Ambiguous hosted names produce an error listing
the matching IDs.

Shared options support configuration and environment variables. For example:

```yaml
default:
  konnect:
    mesh:
      control-plane:
        name: production
      mesh: default
      export-profile: federation-with-policies
      inspect:
        type: policies
      token:
        valid-for: 1h
        scope: [cp]
```

`KONGCTL_DEFAULT_KONNECT_MESH_MESH=production` sets the mesh for the `default`
profile. Environment variables use the active profile's name, with dots and
hyphens in configuration paths replaced by underscores.

## Read and write resources

```shell
kongctl get mesh resource-types
kongctl get mesh dataplanes --mesh production
kongctl get mesh meshtimeouts --all-meshes -o json
kongctl get mesh meshtimeouts slow --mesh production -o yaml
kongctl apply mesh -f policy.yaml
kongctl delete mesh meshtimeouts slow --mesh production
```

Types can be addressed by their discovered URL path, type name, or short name.
Mesh scoped resources default to the `default` mesh. `--all-meshes` applies
to resource listings and dataplane overviews. Resource lists include all pages
and use an `items` / `total` envelope in JSON and YAML. Named resource reads
return the resource itself. Control-plane and resource-type lists return arrays.

`apply mesh` sends resources directly, creating or replacing each resource
by type and name. It does not generate a plan or diff, require confirmation,
or delete resources omitted from the input. Input can be files, directories,
URLs, or stdin (`-f -`), including multi-document YAML. Documents are applied
in input order. A failed write is reported alongside successful writes;
remaining documents are attempted and the command exits unsuccessfully.
Server warnings go to stderr and are included in structured result output.

## Export resources

```shell
kongctl dump mesh --export-profile federation-with-policies > mesh.yaml
kongctl apply mesh -f mesh.yaml --control-plane-id <destination-id>
```

The export always covers the whole control plane and every mesh. Its output
is a YAML stream; `-o` is unsupported. `--profile` continues to select kongctl
configuration and authentication, while `--export-profile` selects content.

| Export profile | Content and intended use |
| --- | --- |
| `federation` (default) | Migration resources without policies |
| `federation-with-policies` | Migration resources and policies |
| `all` | All discovered types, including read-only resources |
| `no-dataplanes` | All types except dataplanes and dataplane insights |

Migration profiles remove control-plane-owned labels except `kuma.io/mesh`.
The destination supplies its own labels. Resources are ordered with meshes
first and user-token signing keys last. Control-plane TLS CA secrets are
excluded. Exports may contain other secrets and need credential-appropriate
storage. `all` and `no-dataplanes` retain source labels and can contain
read-only resources; they cannot necessarily be reapplied unchanged.

## Inspect computed state

```shell
kongctl get mesh inspect dataplanes --mesh production
kongctl get mesh inspect meshes
kongctl get mesh inspect zones
kongctl get mesh inspect meshtimeout slow --mesh production
kongctl get mesh inspect dataplane backend --mesh production
kongctl get mesh inspect dataplane backend --type stats --mesh production
```

Policy inspection lists matching dataplanes. Dataplane policy inspection
shows matched policies per inbound and outbound port. Overview and policy
collections include every page and support text, JSON, and YAML output.

The dataplane `--type` values are `policies` (default), `xds`, `stats`,
`clusters`, and `config`. Proxy admin responses are returned verbatim,
independent of the requested output format, and depend on proxy connectivity
and the selected control plane's ability to reach the proxy.

## Issue tokens

```shell
kongctl create mesh zone-token --zone zone1 --valid-for 1h > zone-token
kongctl create mesh dataplane-token --name backend --valid-for 1h > dp-token
```

A positive lifetime is required, supplied by flag, environment, or profile.
Zone tokens default to the `cp` scope. Dataplane tokens can bind to a name,
workload, tags, or proxy type. Token subjects are invocation-only inputs.
Tokens are written as bare text without a trailing newline, including when
`-o json` or `-o yaml` is selected. Trace logs redact tokens and secret bodies.

The direct forms above work for hosted and self-managed targets. Explicit
`get konnect mesh` and `create konnect mesh` forms are also supported.
Resource apply/delete use the direct form because their `konnect` subtree is
owned by the existing declarative commands. Mesh has no plan, diff, sync,
interactive view, or control-plane creation command.
