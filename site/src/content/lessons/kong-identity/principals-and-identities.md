---
title: Principals and Identities
summary: >-
  Create human and agent principals beneath an externally managed directory.
order: 2
---

## Goal

Represent two callers: a human support operator and an automation agent.
Give the agent a custom employee identifier. Neither has an authentication
credential yet. Complete [Use the Default Directory](../use-the-default-directory/)
and stay in `identity-guide`.

## Create the configuration

```shell label="Create principals.yaml"
cat > principals.yaml <<'YAML'
_defaults:
  kongctl:
    namespace: identity-guide

identity:
  directories:
    - ref: default-directory
      _external:
        selector:
          matchFields:
            name: default
      principals:
        - ref: support-worker
          type: agent
          display_name: Guide support automation
          description: Automation principal for the Identity guide
          metadata:
            department: support
            role: assistant
          labels:
            application: identity-guide
          identities:
            - ref: support-worker-number
              type: custom
              key: employee_id
              value: identity-guide-worker-001
        - ref: support-operator
          type: user
          display_name: Guide support operator
          description: Human principal for the Identity guide
          metadata:
            department: support
            role: operator
          labels:
            application: identity-guide
YAML
```

`_external` selects the existing directory without creating, updating or
adopting it. The principals and custom identity have their own ownership in
namespace `identity-guide`. A namespace is a KongCTL management boundary,
not a directory or an access-control permission.

The explicit `type` classifies each principal as `user` or `agent`; it does
not grant either one permissions. Type is immutable after creation. `ref` is
the stable local identifier; `display_name` is human-facing and is not unique.
The `metadata.role` values are application data that the later AI Gateway
quickstart will explicitly use in an access rule. Metadata alone grants nothing.

## Apply and inspect

```shell
kongctl apply -f principals.yaml
```

Expect two principals and one identity creation, with no directory update.
Approve those changes in the prompt. Then inspect each principal:

```shell
kongctl get id principals \
  --display-name "Guide support automation" \
  -o json
```

```shell
kongctl get id principals \
  --display-name "Guide support operator" \
  -o json
```

These principal commands use directory `default` automatically. To read
principals in a different directory, add `--directory-name NAME` or
`--directory-id UUID`. No directory flag is needed for this guide.

The principal response includes `metadata`, `labels` and generated
`managed_by`. Business metadata and public labels are distinct from KongCTL
ownership. `managed_by.kongctl-ref` stores `support-worker` or
`support-operator` remotely so a later run can find the resource even if its
display name changes. Identity records carry their own ownership metadata.
Do not edit these generated fields as application data.

Use `kongctl get` with JSON output (`-o json`) and its built-in jq filtering
(`--jq`) to extract the agent's ID directly into a variable. The `-r` flag
returns the ID as plain text without JSON quotes; no separate `jq` installation
is needed:

```shell
export SUPPORT_PRINCIPAL_ID="$(kongctl get id principals \
  --display-name "Guide support automation" -o json \
  --jq 'if length == 1 then .[0].id
        else error("Expected exactly one matching agent") end' -r)"
```

```shell
kongctl get id principal identities \
  --principal-id "$SUPPORT_PRINCIPAL_ID" \
  -o json
```

`--jq` extracts the ID from the JSON response, and `-r` returns it without
JSON quotes. The filter requires exactly one matching principal rather than
silently choosing one if display names are duplicated.

The custom identity's
`key: employee_id` and `value: identity-guide-worker-001` associate an external
identifier with this principal. Its `ref` identifies the managed association.
It is not a password, API key, or proof of authentication.

Other supported identities associate OIDC issuer/claim pairs, Auth Server
clients, or Gateway Consumers. Those identifiers need an actual matching
system to demonstrate authentication. We avoid a fictional OIDC issuer here.
A principal's API keys are direct children of the principal, separate from
these identity records.

## Check convergence

```shell
kongctl diff --mode apply -f principals.yaml
```

Expect no changes. To experiment, edit only the agent's `description`, apply
again, and confirm that its UUID stays the same. Keep the refs, display names
and metadata used by later lessons unchanged.

Keep both principals for the next lesson. Once credentials have been added,
do not try to delete a parent principal as a shortcut for credential cleanup.
