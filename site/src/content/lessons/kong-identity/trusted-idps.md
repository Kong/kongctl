---
title: Configure a Trusted IdP
summary: >-
  Create a demonstration issuer and configure the directory to trust its
  tokens.
order: 5
---

## Goal

Configure an issuer whose tokens the directory will trust. Complete
[Enable Token Vault](../enable-token-vault/) and keep using `identity-guide`.

A **Trusted IdP** identifies a token issuer that the directory trusts. It is
not a credential provider: the issuer identifies the caller; a credential
provider identifies the upstream service whose credential will be used.

For a reproducible example we create a Kong Auth Server. A corporate OIDC
issuer, such as your organization's existing IdP, could serve this purpose
instead. Do not assume a principal API key is an OIDC token: those are separate
runtime authentication paths.

## Create the demonstration issuer

```shell label="Create issuer.yaml"
cat > issuer.yaml <<'YAML'
_defaults:
  kongctl:
    namespace: identity-guide
identity:
  auth_servers:
    - ref: guide-issuer
      name: identity-guide-issuer
      description: Demonstration issuer for the Identity guide
      audience: https://support.example.test
      signing_algorithm: RS256
YAML
```

```shell
kongctl apply -f issuer.yaml
```

Expect one Auth Server creation. `audience` is a demonstration token audience,
not a service we contact. The server returns real issuer and signing-key URLs.
Read them without manually copying JSON quotes:

```shell
export IDENTITY_GUIDE_ISSUER="$(kongctl get id auth-server \
  identity-guide-issuer -o json --jq '.issuer' -r)"
```

```shell
export IDENTITY_GUIDE_JWKS_URI="$(kongctl get id auth-server \
  identity-guide-issuer -o json --jq '.jwks_uri' -r)"
```

```shell
printf 'Issuer: %s\nJWKS: %s\n' \
  "$IDENTITY_GUIDE_ISSUER" "$IDENTITY_GUIDE_JWKS_URI"
```

Both must be absolute HTTPS URLs. They are public identifiers, not secrets.
`-r` returns raw strings; without it JSON quotes can become part of an exported
value. The JWKS contains the issuer's public token-verification keys.

## Declare trust on the directory

Use the directory's existing ownership namespace if it differs from the one
adopted in this guide:

```shell label="Create trust.yaml"
cat > trust.yaml <<'YAML'
_defaults:
  kongctl:
    namespace: identity-guide-directory
identity:
  directories:
    - ref: default-directory
      name: default
      vault:
        enabled: true
      trusted_idps:
        - ref: guide-trust
          issuer_url: !env IDENTITY_GUIDE_ISSUER
          display_name: Identity guide issuer
          jwks_uri: !env IDENTITY_GUIDE_JWKS_URI
          jit_provisioning_enabled: false
          vault_access_enabled: true
YAML
```

```shell
kongctl apply -f trust.yaml
```

```shell
kongctl get id trusted-idps -o json
```

Expect one Trusted IdP creation. Its `issuer_url` must match the Auth Server
exactly. The display name is not a unique selector. No identities or credentials
are created by this declaration. JIT provisioning is explicitly disabled to
avoid creating principals implicitly when experimenting. `vault_access_enabled`
permits the issuer's tokens to be used for vault access; it does not issue one.

## Key distinctions

There are three different kinds of keys in the full walkthrough:

| Key                                  | Purpose                                                     |
| ------------------------------------ | ----------------------------------------------------------- |
| Principal API key                    | Authenticates the caller in our API-key quickstart.         |
| Auth Server signing keys / JWKS      | Sign and verify JWTs for the alternative issuer-token path. |
| Vault public/private encryption pair | Protects upstream credentials stored in Token Vault.        |

The later quickstarts exercise the **principal API-key path**, including vault
exchange. They do not issue or validate a JWT from this Auth Server, and do
not demonstrate JIT provisioning. This trust configuration makes that separate
path available for further experimentation; it does not prove it works.
