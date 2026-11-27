# Adopting gateway entities with Kong Operator

`kongctl dump operator` exports the entities of an existing Konnect gateway
control plane as [Kong Operator][ko-adoption] custom resources. Each resource
adopts its existing entity through `spec.adopt`, so applying the output with
`kubectl` hands the entities to the operator without recreating them.

The command only reads from Konnect. The operator changes Konnect once the
resources are applied, as described in [What adoption does](#what-adoption-does).

## Quickstart

1. Install Kong Operator with the Konnect controller enabled, and create a
   `KonnectAPIAuthConfiguration` in the target namespace.
2. Export the control plane:

   ```sh
   kongctl dump operator \
     --control-plane-name gateway-control-plane \
     --namespace kong \
     --auth-ref konnect-api-auth \
     --output-file adopt.yaml
   ```

3. Read the warnings on stderr, review `adopt.yaml`, then apply it:

   ```sh
   kubectl apply -f adopt.yaml
   ```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--control-plane-id`, `--control-plane-name` | | Control plane to export. One is required. |
| `--namespace` | `default` | Namespace for the generated resources. |
| `--auth-ref` | `konnect-api-auth` | `KonnectAPIAuthConfiguration` the control plane mirror uses. |
| `--adopt-mode` | `auto` | `auto`, `match`, or `override`. See [Adoption modes](#adoption-modes). |
| `--include-secrets` | `false` | Export entities that need secret material, writing it inline. |
| `--output-file` | stdout | File to write the YAML to. |

## What is exported

| Konnect entity | Kong Operator resource |
|---|---|
| Control plane | `KonnectGatewayControlPlane` (`source: Mirror`) |
| Service | `KongService` |
| Route | `KongRoute` |
| Upstream, target | `KongUpstream`, `KongTarget` |
| Consumer, consumer group | `KongConsumer`, `KongConsumerGroup` |
| ACL, key-auth, JWT, HMAC credentials | `KongCredentialACL`, `KongCredentialAPIKey`, `KongCredentialJWT`, `KongCredentialHMAC` |
| Certificate, CA certificate, SNI | `KongCertificate`, `KongCACertificate`, `KongSNI` |
| Key, key set | `KongKey`, `KongKeySet` |
| Vault | `KongVault` (cluster scoped, named `<control-plane>-<prefix>`), plus a `KongReferenceGrant` that lets it reference the control plane |
| Data plane client certificate | `KongDataPlaneClientCertificate` (always `match`) |
| Plugin | `KongPlugin` plus a `KongPluginBinding` that adopts it |

References between entities, such as a route's service or a plugin's targets,
become references between the generated resources. Every spec carries the
entity's full configuration, including fields the operator does not compare
when it adopts.

## Adoption modes

`match` adopts an entity only when the resource matches it. `override` adopts
it and updates it to match the resource. Against Kong Operator 2.4, neither
works for every kind, so the default, `auto`, picks per kind:

| Kind | `auto` uses | Why |
|---|---|---|
| `KongPluginBinding` | `override` | `match` always fails with `NotMatch`: the operator compares Konnect's plugin config, including unset (`null`) fields, with the `KongPlugin` config, which Kubernetes stores without nulls. |
| `KongUpstream` | `override` | `match` fails with `NotMatch` because of how the operator compares health checks. |
| Everything else | `match` | `override` adoption of a `KongService` clears its client and CA certificate references in Konnect. |

With `auto`, every exported resource adopts, and Konnect is left unchanged
apart from the tags the operator adds. Passing `match` or `override` applies
that mode to every kind, and the command warns about the resources affected.

## What adoption does

- **The operator owns adopted entities.** After adoption it writes the
  resource's full spec back to Konnect. A field missing from the resource is
  reset, which is why the export reproduces every field it can.
- **Deleting a resource deletes its entity in Konnect.** The control plane is
  the exception: it is a read-only mirror and is never modified or deleted.
- **Tags change.** The operator adds its own `k8s-*` and `managed-by` tags,
  within a limit of 20 tags per entity.

## What is skipped

Skipped entities and lossy fields are reported as warnings on stderr. The
YAML on stdout is unaffected.

- **Secrets** – certificates, private keys, key-auth, JWT and HMAC
  credentials, and vaults whose config holds credentials are skipped unless
  `--include-secrets` is set. With it, secrets are written inline, so treat
  the output as a secret.
- **Basic-auth credentials** – always skipped. Konnect returns only the
  password hash, and the operator would set the password to that hash.
- **Expression routes** – the operator cannot adopt them. Plugins attached to
  them are skipped too.
- **Plugins on `ws` or `wss`** – `KongPlugin` cannot represent those
  protocols, and adopting the plugin would narrow them. Konnect enables them
  by default on some plugins, such as `key-auth`.
- **Plugins linked to partials** – `KongPlugin` cannot represent partials.
- **mTLS-auth credentials** – there is no Kong Operator resource for them.
- **Entities already managed by Kubernetes** – entities tagged `k8s-uid:` by
  another resource.
- **Control plane groups and Kong Ingress Controller control planes** – the
  command refuses them. Export a group's member control planes instead.

A resource that references a skipped entity, such as a service using a client
certificate that was not exported, keeps the reference. It adopts once you
supply the referenced resource, rather than clearing the reference in
Konnect.

[ko-adoption]: https://developer.konghq.com/operator/konnect/crd/adoption/gateway/
