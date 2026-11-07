---
title: Use the Default Directory
summary: Find the existing directory and prepare a workspace for the chapter.
order: 1
---

## Goal

Create a human and an automation principal, associate an external identifier,
and issue an API key. Then configure Token Vault to hold a shared credential
for a fictional ticketing service. Each lesson includes CLI verification. The later [Quickstarts](../../quickstarts/) connect these resources
to a running AI Gateway.

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
- `kongctl` installed and available on your PATH.
- `kongctl` authenticated to the Konnect organization you intend to use.
- `openssl` for the Token Vault lessons, and Python 3 for API-key capture
  extraction in the API-key lesson.

These examples use the US Konnect region. Use a test organization with Kong
Identity enabled and configure KongCTL to use that organization and region.
Authentication is covered in [Konnect Authentication](../../installation/authenticate/).

Confirm that the Identity commands are available:

```shell
kongctl version
```

```shell
kongctl get identity --help
```

`identity` has the shorter alias `id`; later lessons use that alias.
The `-o json` flag selects JSON output so the fields in these examples are
visible. Use
`kongctl COMMAND --help` for command-specific flags. If a command fails, stop
and address the error before continuing.

This chapter configures administrative resources. It does not require a local
Gateway or demonstrate runtime authentication or credential decryption.

## Find the default directory

```shell
kongctl get id directory
```

Konnect normally provides a single directory named `default`. This command
should show that existing directory, which we will use throughout the guide.

If there is no default directory, stop before the next lesson. An administrator
can bootstrap one using the configuration in
[Enable Token Vault](../enable-token-vault/), or provide another directory and
adjust the selectors in these examples. Do not create a second directory just
because a permissions or connectivity error prevents reading the existing one.

## Prepare a workspace

The commands use Bash syntax. Start Bash if your usual shell is different.
Keep the same terminal and working directory for the remaining lessons:

```shell
mkdir -p identity-guide && cd identity-guide
```

```shell
printf 'private/\ncerts/\n' > .gitignore
```

Use one copy of this walkthrough per organization. The namespace, principal
references, and custom identity value below are fixed demonstration values.
If other people are running it in the same organization, agree on distinct
values before applying the files.

In the next lesson, the configuration references this existing directory and
creates principals inside it. The directory itself stays unchanged.

## How to apply a lesson

Each `cat > FILE <<'YAML'` block writes a local configuration file. Copy the
whole block, including its last `YAML` line. Quotes around `YAML` preserve
special YAML tags and prevent the shell from expanding the file contents.

`kongctl apply -f FILE` reads that file, compares it with Konnect, and prompts
with the proposed changes. Confirm only the changes described in the lesson.
`apply` creates or updates resources; it does not prune omitted collections.
We do not use `--auto-approve` in this guide. A failed apply can leave earlier
successful changes in place; inspect the error and rerun after correcting it.
