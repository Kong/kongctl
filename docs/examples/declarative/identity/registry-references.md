# Registry references to Identity principals

A Registry Agent can reference a principal through a separate association.
The association stores the directory UUID and principal UUID. It has its own
UUID; that UUID is different from either the Agent or principal UUID.

An independently managed Registry Agent can reference your managed principal,
including a principal declared under the externally referenced `default`
directory. The principal's ownership does not authorize removing that link.
Direct native principal deletion currently leaves the association behind
(API question Q53).

For example, save this as `registry-principal.yaml`:

```yaml
_defaults:
  kongctl:
    namespace: registry-example
identity:
  directories:
  - ref: default-directory
    _external:
      selector:
        matchFields:
          name: default
    principals:
    - ref: order-agent
      display_name: Order processing agent
      type: agent
```

```sh
kongctl apply -f registry-principal.yaml
kongctl get identity principals --directory-name default -o json
```

This creates the principal and leaves the directory externally managed.
Registry Agent declarations and association creation are separate features;
this example does not create an Agent or link it automatically.

## Inspect an incoming reference

When a Registry Agent references the principal, these operations block with an
error identifying the Agent UUID and association UUID:

```sh
kongctl plan --mode delete -f registry-principal.yaml
kongctl delete -f registry-principal.yaml
```

Scoped sync that would remove this principal has the same guard. All Registry
Agents are inspected, including unowned Agents and those outside the selected
namespace. Reads and ordinary principal updates do not require this inventory.
The externally referenced default directory is retained during teardown.

Inspect the reported Agent's associations:

```sh
export AGENT_ID=REPLACE_WITH_REPORTED_AGENT_UUID
kongctl api get "/v1/agents/$AGENT_ID/principals" -o json
```

Check `directory_id` and `principal_id` before selecting the association's own
`id`. With explicit authority to unlink it:

```sh
export ASSOCIATION_ID=REPLACE_WITH_ASSOCIATION_UUID
kongctl api delete "/v1/agents/$AGENT_ID/principals/$ASSOCIATION_ID"
kongctl plan --mode delete -f registry-principal.yaml
kongctl delete -f registry-principal.yaml
```

Removing the association does not delete the Agent or principal. Principal
retirement still requires the existing identity, credential and membership
checks. KongCTL does not unlink Registry associations automatically.

## Permissions and saved plans

Principal and managed-directory retirement require permission to list Registry
Agents and each Agent's principal associations. Missing reader configuration,
HTTP errors, malformed pagination or incomplete inventories block deletion;
an unavailable API is not interpreted as an empty collection. This also means
retirement is unavailable on deployments without these Registry endpoints.

Saved plans repeat the inventory immediately before target deletion. A link
added after planning blocks execution. Directory deletion checks the stored
directory UUID even for links whose principal no longer exists. These client
observations do not make deletion atomic; Q53 asks engineering for the native
referential-integrity and conditional-deletion contract.
