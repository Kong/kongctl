# Kong Mesh examples

These examples use native Kong Mesh documents with `kongctl apply mesh`.
They are separate from kongctl's Konnect declarative manifests and do not use
`plan`, `diff`, or `sync`. See the [Mesh guide](../../../mesh.md) for the
complete command and configuration reference.

| File | Purpose |
| --- | --- |
| [mesh.yaml](mesh.yaml) | Create `kongctl-example-mesh` with a user label |
| [timeout-policy.yaml](timeout-policy.yaml) | Apply mesh-wide timeouts |

## Select a control plane

Use a Kong Mesh 3-compatible control plane. For a Konnect-hosted target, your
active kongctl profile needs a login or PAT with access to that control plane.
Control-plane provisioning happens outside kongctl; these examples create
resources within an existing control plane.

From the repository root, enter the example directory and list hosted targets:

```shell
cd docs/examples/declarative/mesh
kongctl get mesh control-planes
export MESH_CP_ID="<source-control-plane-id>"
kongctl get mesh resource-types --control-plane-id "$MESH_CP_ID"
```

Replace the placeholder with an ID from the list. Resource discovery should
include `Mesh` and `MeshTimeout`. The commands below use an explicit ID so
they consistently address the selected hosted target.

For a self-managed target, replace the ID selector with
`--control-plane-url https://mesh.example.com:5681`. Configure its token and
TLS options as described in the Mesh guide.

## Apply and read resources

Apply the Mesh before its policy:

```shell
kongctl apply mesh -f mesh.yaml -f timeout-policy.yaml \
  --control-plane-id "$MESH_CP_ID"
```

The result reports both resources as `created` on the first run and `updated`
when reapplied. The policy sets an outbound connection timeout of five
seconds and an idle timeout of one hour for the example mesh.

Apply writes directly without a plan, diff, or confirmation prompt. It creates
or replaces resources and does not delete resources omitted from the input.

Read the Mesh and policy, and list dataplanes in that mesh:

```shell
kongctl get mesh meshes kongctl-example-mesh \
  --control-plane-id "$MESH_CP_ID" -o yaml
kongctl get mesh meshtimeouts kongctl-example-timeouts \
  --mesh kongctl-example-mesh --control-plane-id "$MESH_CP_ID" -o yaml
kongctl get mesh dataplanes --mesh kongctl-example-mesh \
  --control-plane-id "$MESH_CP_ID" -o json
```

The policy read returns its configured timeouts. The control plane may
normalize `1h` to `1h0m0s`. Dataplane lists use an `items` / `total` envelope;
the list is empty until dataplanes connect to this mesh through a zone.

## Dump and migrate configuration

Dump a migration stream containing policies, then apply it to a different
control plane:

```shell
(umask 077; kongctl dump mesh --export-profile federation-with-policies \
  --control-plane-id "$MESH_CP_ID" > mesh-export.yaml)
export MESH_DESTINATION_CP_ID="<destination-control-plane-id>"
kongctl apply mesh -f mesh-export.yaml \
  --control-plane-id "$MESH_DESTINATION_CP_ID"
```

Use an isolated source and destination for this walkthrough: dump covers
the entire source control plane, including every mesh, rather than only
`kongctl-example-mesh`. The migration profile includes policies and removes
control-plane-owned labels; the user label on the example Mesh remains.
The stream can contain secrets, so the example creates a private output file.

The default `federation` profile omits policies. The `all` and `no-dataplanes`
profiles produce inventories that can include read-only resources and cannot
necessarily be reapplied unchanged.

Read the migrated policy from the destination:

```shell
kongctl get mesh meshtimeouts kongctl-example-timeouts \
  --mesh kongctl-example-mesh \
  --control-plane-id "$MESH_DESTINATION_CP_ID" -o yaml
```

## Inspect computed state

Inspect mesh and dataplane overviews, then find the dataplanes the policy
matches:

```shell
kongctl get mesh inspect meshes --control-plane-id "$MESH_CP_ID"
kongctl get mesh inspect dataplanes --mesh kongctl-example-mesh \
  --control-plane-id "$MESH_CP_ID"
kongctl get mesh inspect meshtimeout kongctl-example-timeouts \
  --mesh kongctl-example-mesh --control-plane-id "$MESH_CP_ID"
```

Policy matching and dataplane overviews are empty until dataplanes connect.
These manifests configure resources; they do not deploy a zone or workloads.

With a connected dataplane, replace `<dataplane-name>` below with its name:

```shell
kongctl get mesh inspect dataplane "<dataplane-name>" \
  --mesh kongctl-example-mesh --control-plane-id "$MESH_CP_ID"
kongctl get mesh inspect dataplane "<dataplane-name>" --type stats \
  --mesh kongctl-example-mesh --control-plane-id "$MESH_CP_ID"
```

The first command shows matched policies per inbound and outbound port.
The second returns raw proxy statistics and requires the control plane to
reach the proxy. These commands depend on a running zone and dataplane and
are not exercised by applying the two example manifests alone.

## Issue tokens

Request tokens with a one-hour lifetime:

```shell
(umask 077; kongctl create mesh zone-token --zone example-zone \
  --valid-for 1h --control-plane-id "$MESH_CP_ID" > zone-token)
(umask 077; kongctl create mesh dataplane-token --name example-backend \
  --mesh kongctl-example-mesh --valid-for 1h \
  --control-plane-id "$MESH_CP_ID" > dataplane-token)
```

Each file contains the bare token, without JSON wrapping or a trailing
newline. Issuing a token does not create or connect a zone or dataplane.
Generated token and export files are ignored by Git in this directory.

## Clean up

Delete the policy before the Mesh on the source:

```shell
kongctl delete mesh meshtimeouts kongctl-example-timeouts \
  --mesh kongctl-example-mesh --control-plane-id "$MESH_CP_ID"
kongctl delete mesh meshes kongctl-example-mesh \
  --control-plane-id "$MESH_CP_ID"
```

If you migrated the examples, repeat those commands using
`--control-plane-id "$MESH_DESTINATION_CP_ID"`. They remove the named example
resources, not everything in the exported stream. For isolated control
planes provisioned for this walkthrough, delete them through Konnect when
finished. Remove the generated local export and token files when no longer
needed.
