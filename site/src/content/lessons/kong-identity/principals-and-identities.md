---
title: Principals and Identities
summary: Create a principal beneath an external directory and inspect its ownership.
order: 2
---

## Goal

Create a support automation principal with custom and OIDC identities. The
directory already exists and remains externally managed for this step.

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
          description: Support worker for the Identity guide
          metadata:
            department: support
          labels:
            application: identity-guide
          identities:
            - ref: support-worker-number
              type: custom
              key: employee_id
              value: identity-guide-worker-001
            - ref: support-worker-oidc
              type: oidc
              issuer: https://workforce.example.test
              claim:
                name: sub
                value: identity-guide-worker-001
YAML
```

`_external` resolves the existing directory by name. It does not create,
update, or adopt the directory. The nested principal and identity are managed
resources in the `identity-guide` namespace.

Review the proposed changes in the confirmation prompt before approving:

```shell
kongctl apply -f principals.yaml
```

Expect one principal and two identities to be created, with no directory update.
If the plan differs, decline it and inspect the existing resources first.

## Inspect the result

```shell
kongctl get id principals --display-name "Guide support automation" -o json
```

The principal has business `metadata`, user `labels`, and a generated
`managed_by` object. The latter contains `kongctl-namespace`,
`kongctl-protected`, `kongctl-ref`, `tool`, and `version`.

The `kongctl-ref` value stores the manifest's stable local reference remotely.
This lets subsequent runs find the same principal even if its nonunique
`display_name` changes. The identity also has its own ownership metadata.
Do not edit these ownership fields as application metadata.

Copy the principal's full `id` into this variable, replacing the placeholder:

```shell
export SUPPORT_PRINCIPAL_ID='<principal-uuid>'
kongctl get id principal identities --principal-id "$SUPPORT_PRINCIPAL_ID" -o json
```

The custom identity associates `employee_id=identity-guide-worker-001` with
this principal. Its `ref` identifies the managed record; `key` and `value`
express the external identity. Creating this association does not itself issue
a credential or configure a Gateway authentication plugin.

The OIDC identity associates the example issuer and `sub` claim with the same
principal. The `.test` issuer is illustrative: it does not issue real tokens.
Creating this record does not configure a Trusted IdP or establish runtime
trust. Use a real issuer and subject only when connecting a real identity
provider.

Other identity variants represent an Auth Server client or a Gateway Consumer
link. Those require the corresponding external system identifiers. They are
associations on the principal, rather than credentials nested beneath an
identity.

## Update and check convergence

Edit `display_name` in `principals.yaml` to `Guide support automation v2`.
Keep `ref: support-worker` unchanged, then run:

```shell
kongctl apply -f principals.yaml
kongctl diff -f principals.yaml
```

Expect an update to the same principal UUID, followed by zero changes in the
diff. The principal's `type` is set at creation; this walkthrough does not
change it.

For an optional isolated cleanup, the following command deletes the declared
principal and identities. The externally referenced directory is preserved.
Review the deletion confirmation before approving:

```shell
kongctl delete -f principals.yaml
```

You can keep these resources while continuing to Token Vault; the next lessons
do not depend on their presence.
