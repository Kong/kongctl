---
title: Use the Default Directory
summary: Find the existing directory and prepare a workspace for the chapter.
order: 1
---

## Goal

Use Kong Identity to represent a support worker and associate an external
identifier with that principal. Then configure Token Vault to hold a shared
credential for a fictional ticketing service.

A **directory** contains principals and their identity associations. A
**principal** represents a person or application. An **identity** associates
that principal with an identifier, such as an employee number or an OIDC
subject. Credentials, such as principal API keys, are separate children of a
principal.

**Token Vault** stores credentials for accessing other services. Each directory
has a Vault that can be enabled; it is not a separately created resource.
Vault public encryption keys, providers, and provider credentials live beneath
the directory.

## Prerequisites

You need:

- A [Kong Konnect account](https://konghq.com/products/kong-konnect/register)
  with access to the Kong Identity APIs and permission to manage directories.
- A `kongctl` build containing the Identity commands in this chapter.
- An authenticated profile pointing to the environment you intend to use.
- `openssl` for the Token Vault lessons.

These examples were validated against the US `.tech` environment during
Identity development. Availability can differ by environment. Use a development
organization, and check API availability with your platform team if a command
returns an unsupported route or field. Authentication and profile setup are
covered in [Konnect Authentication](../../installation/authenticate/).

This chapter configures administrative resources. It does not require a local
Gateway or demonstrate runtime authentication or credential decryption.

## Find the default directory

```shell
kongctl get id directory default -o json
```

Look for `name: "default"`, its `managed_by` object, and `vault_enabled`.
The `id` command namespace is an alias for `identity`. Directory-scoped read
commands use the directory named `default` when neither `--directory-id` nor
`--directory-name` is supplied.

If there is no default directory, stop before the next lesson. An administrator
can bootstrap one using the configuration in
[Enable Token Vault](../enable-token-vault/), or provide another directory and
adjust the selectors in these examples. Do not create a second directory just
because a permissions or connectivity error prevents reading the existing one.

## Prepare a workspace

Keep the same terminal and working directory for the remaining lessons:

```shell
mkdir -p identity-guide
cd identity-guide
```

Use one copy of this walkthrough per organization. The namespace, principal
references, and custom identity value below are fixed demonstration values.
If other people are running it in the same organization, agree on distinct
values before applying the files.

The first lesson references the directory externally. This allows you to
manage your principal without taking ownership of the directory itself.
