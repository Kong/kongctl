---
title: Vault Providers and Credentials
summary: >-
  Register an encryption key, store a shared credential, and rotate it
  explicitly.
order: 6
---

## Goal

Configure a fictional support ticketing integration. Its provider uses the
`static_secret` template and holds one shared credential. No real ticketing
service is required; use the harmless demonstration value below.

Complete [Configure a Trusted IdP](../trusted-idps/) first and stay in the
same working directory.

## Generate an encryption key

Vault encrypts credentials using registered public keys. These are different
from principal API keys used for authentication. Generate a private key in a
private local directory and export only its public counterpart into the
workspace:

```shell
mkdir -p private
```

```shell
chmod 700 private
```

```shell
(umask 077; openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 \
  -out private/vault-private.pem)
```

```shell
openssl pkey -in private/vault-private.pem -pubout -out vault-public.pem
```

Keep `private/` outside version control and retain the private key securely.
Only `vault-public.pem` is uploaded. Run key generation once; generating new
material changes the fingerprint and creates a different Vault key.

## Configure the provider and credential

```shell
export SUPPORT_TICKETING_TOKEN='identity-guide-demo-token-v1'
```

Use the directory namespace selected in the previous lesson:

```shell label="Create vault.yaml"
cat > vault.yaml <<'YAML'
_defaults:
  kongctl:
    namespace: identity-guide-directory

identity:
  directories:
    - ref: default-directory
      name: default
      vault:
        enabled: true
        keys:
          - ref: support-encryption-key
            name: identity-guide-encryption
            public_key: !file ./vault-public.pem
        providers:
          - ref: support-ticketing
            name: identity-guide-ticketing
            template_name: static_secret
            credential_type: shared
            enabled: true
            shared_credentials:
              - ref: support-ticketing-token
                secret: !secret {source: !env SUPPORT_TICKETING_TOKEN}
YAML
```

The provider identifies the service whose credential should be retrieved.
It does not configure the service's URL. AI Gateway chooses the URL separately
and selects this provider using its directory-scoped `name`,
`identity-guide-ticketing`. The local `ref: support-ticketing` is for KongCTL.
`shared` means this credential is shared rather than enrolled separately for
each user. Omitting `selector` and `priority` selects the provider's single
default credential slot.

```shell
kongctl apply -f vault.yaml
```

Review the key, provider, and credential creations in the prompt. A directory
update should not be needed after the previous lesson. If the directory
already contains five encryption keys, stop and coordinate key retirement
with its owner; do not delete someone else's key to make room.

## Inspect and repeat

```shell
kongctl get id vault keys -o json
```

```shell
kongctl get id vault provider \
  --provider-name identity-guide-ticketing \
  -o json
```

```shell
kongctl get id vault provider credentials \
  --provider-name identity-guide-ticketing -o json
```

```shell
kongctl diff --mode apply -f vault.yaml
```

The key response includes its `kid`. The provider reports
`template_name: static_secret` and `credential_type: shared`. The credential
read returns metadata rather than the plaintext secret. Expect zero changes
from the unchanged manifest.

Vault children have no remote `kongctl-ref`. Their local refs still connect
the manifest and plan, but matching uses the key fingerprint, provider name,
and credential slot described in the previous lesson.

## Rotate the secret explicitly

This is an optional exercise. Keep the latest value in
`SUPPORT_TICKETING_TOKEN`: the later upstream verifier needs that same value.
Changing the environment variable alone does not request a credential update.
The remote plaintext cannot be read back for comparison. Explicitly request
the write using the credential's local ref and secret field:

```shell
export SUPPORT_TICKETING_TOKEN='identity-guide-demo-token-v2'
```

```shell
kongctl apply -f vault.yaml \
  --write-secret 'support-ticketing-token#secret'
```

`!secret` defers reading the value until execution and keeps it out of plans
and diffs. For a real credential, use your organization's secure secret
injection mechanism instead of placing the value in terminal history.

## Export public configuration

```shell
kongctl dump declarative --resources identity.directories \
  --include-child-resources --output-file identity-export.yaml
```

```shell
kongctl diff --mode apply -f identity-export.yaml
```

The export can include other managed Identity resources in your organization;
review it before sharing. It contains public configuration and ownership
information, but not a reusable copy of the shared plaintext secret. Reloading
an unchanged export should produce zero changes. It cannot restore a deleted
credential without a secret source.

## Next steps

You have configured the administrative side of Token Vault. Only its public
key was uploaded; the matching private key stays in `private/vault-private.pem`.
The [AI Gateway authentication quickstart](../../quickstarts/identity-authentication/)
uses the principal API key to authenticate requests to a mocked model. Then
[Token Vault with AI Gateway](../../quickstarts/identity-token-vault/) uses the
private key in the runtime to demonstrate actual upstream credential use.
Neither administrative readback nor a mocked response proves vault decryption.

Leave the directory and key material in place until its owner has reviewed
credential retirement and key retention. Avoid blanket deletion or collection
sync on a shared default directory.
