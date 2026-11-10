---
title: "Kong Identity: Authenticate AI Gateway"
summary: >-
  Use principal API keys and metadata rules with a mocked model, without a
  paid LLM provider.
order: 1
related:
  - label: AI Gateway mocking guide
    url: https://developer.konghq.com/ai-gateway/mock-ai-model-responses/
---

## What you will demonstrate

A support agent calls an AI Gateway model using its Kong Identity API key.
You will see **401** without a key, **200** with a valid key, and **403** when
an access rule rejects the principal's role. A Mocking Policy supplies the
response; there is no real model call, model-provider account or API charge.
This exercise does not retrieve a Token Vault credential.

## Prerequisites and working directory

Complete the first three lessons in [Kong Identity](../../kong-identity/):
Use the Default Directory, Principals and Identities, and Issue a Principal
API Key. You need the agent with `metadata.role: assistant`, its captured
API key, and the `identity-guide` workspace. Directory adoption, Trusted IdP
configuration and Token Vault are **not** required for this quickstart.

Use Bash, Docker, OpenSSL and curl. The configuration targets AI Gateway
**2.3.0** in the US Konnect region. Your Konnect account needs permission to create
AI Gateways. Use the same organization as the Identity lessons. In a new
terminal, restore the key from its private capture as shown
in [Issue a Principal API Key](../../kong-identity/principal-api-keys/).

From your existing workspace (do not create another nested directory):

```shell
pwd
```

```shell
kongctl version
```

```shell
docker version
```

```shell
test -n "$SUPPORT_API_KEY" && echo 'Agent key loaded'
```

If the last command prints nothing, load the key before continuing. Port
8000 must be free. Stop a previous demo runtime occupying that port first.
All commands below run in this same workspace, using `kongctl` from PATH.
`-f` means the local configuration file; apply shows a confirmation prompt.

## Generate the runtime connection certificate

This pair authenticates the runtime to Konnect. It is **not** the Token Vault
encryption pair or the caller's principal API key. Generate it once:

```shell
mkdir certs
```

```shell
(umask 027; openssl req -new -x509 -nodes -newkey rsa:2048 -days 365 \
  -subj "/CN=identity-guide-runtime/C=US" \
  -keyout certs/data-plane.key -out certs/data-plane.crt)
```

```shell
chgrp "$(id -g)" certs/data-plane.key
```

```shell
chmod 640 certs/data-plane.key
```

Only the public certificate is uploaded. The private key is mounted read-only
into Docker; the container joins your host group so it can read it. If these
files already exist from this quickstart, reuse them instead of regenerating.

## Create the AI Gateway configuration

The provider credential is a placeholder. The mock answers before contacting
the provider. Use `!secret` even for this placeholder because the field is a
write-only credential field:

```shell
export IDENTITY_GUIDE_PROVIDER_AUTH='Bearer not-a-real-provider-key'
```

```shell label="Create ai-gateway.yaml"
cat > ai-gateway.yaml <<'YAML'
_defaults:
  kongctl:
    namespace: identity-guide

ai_gateways:
  - ref: guide-gateway
    name: identity-guide-gateway
    display_name: Identity Guide Gateway
    description: AI Gateway using principals from the default Identity directory
    deployment_type: hybrid
    min_runtime_version: "2.3"
    runtime_auto_upgrade: false
    proxy_urls:
      - host: localhost
        port: 8000
        protocol: http
    labels:
      application: identity-guide
    auth_strategies:
      - ref: directory-key-auth
        name: directory-key-auth
        display_name: Default directory API keys
        type: key-auth
        config:
          key_names:
            - apikey
          hide_credentials: true
          identity_realms: []
          principals:
            enabled: true
            directory: default
            error_on_miss: true
    policies:
      - ref: guide-mock-response
        name: guide-mock-response
        display_name: Guide mock response
        type: mocking
        enabled: true
        global: false
        config:
          api_specification: |
            openapi: 3.0.0
            info:
              title: Support demo
              version: 1.0.0
            paths:
              /v1/chat/completions:
                post:
                  responses:
                    '200':
                      description: Synthetic support response
                      content:
                        application/json:
                          example:
                            id: chatcmpl-identity-guide
                            object: chat.completion
                            model: identity-guide
                            choices:
                              - index: 0
                                finish_reason: stop
                                message:
                                  role: assistant
                                  content: "Hello from the authenticated support demo."
    model_providers:
      - ref: guide-provider
        name: guide-provider
        display_name: Guide model provider
        type: openai
        config:
          auth:
            type: basic
            headers:
              - name: Authorization
                value: !secret {source: !env IDENTITY_GUIDE_PROVIDER_AUTH}
    models:
      - ref: identity-guide-model
        name: identity-guide-model
        display_name: Guide model
        type: model
        formats:
          - type: openai
        config:
          route:
            paths:
              - /v1
            model:
              body_param: model
              values:
                - identity-guide
        targets:
          - name: gpt-4o
            provider: !ref guide-provider#name
            config:
              type: openai
        policies:
          - !ref guide-mock-response#name
        access:
          auth_strategies:
            - !ref directory-key-auth#name
          acls:
            allow_when:
              - "has(principal.metadata.role) && principal.metadata.role == 'assistant'"
        capabilities:
          - generate
    data_plane_certificates:
      - ref: identity-guide-data-plane
        title: identity-guide-data-plane
        description: Local guide runtime
        cert: !file ./certs/data-plane.crt
YAML
```

The key-auth strategy looks up principals in directory `default`.
`identity_realms: []` is deliberate: the observed API rejects
`principals.enabled: true` if omitted realms are populated by default.
`error_on_miss: true` rejects unknown keys. The model references that strategy
and explicitly allows only principals with `metadata.role: assistant`.
No group membership is involved and no separate AI Gateway Consumer is needed.

```shell
kongctl apply -f ai-gateway.yaml
```

Expect six creations: gateway, public data-plane certificate, auth strategy,
model provider, mocking policy and model. Approve only these resources.

## Start a local runtime

Create this reusable helper. It reads the connection endpoints from Konnect;
it does not create resources or put credentials in the manifest:

```shell label="Create run-gateway.sh"
cat > run-gateway.sh <<'BASH'
#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
command -v kongctl >/dev/null
command -v docker >/dev/null
if [[ ! -f certs/data-plane.crt || ! -f certs/data-plane.key ]]; then
  echo "Generate the data-plane certificate and key before starting." >&2
  exit 1
fi
cp_host="$(kongctl get ai-gateway 'Identity Guide Gateway' -o json \
  --jq '.endpoints.configuration | sub("^https://"; "") | sub(":443$"; "")' -r)"
tp_host="$(kongctl get ai-gateway 'Identity Guide Gateway' -o json \
  --jq '.endpoints.telemetry | sub("^https://"; "") | sub(":443$"; "")' -r)"
if [[ -z "$cp_host" || "$cp_host" == null || -z "$tp_host" || "$tp_host" == null ]]; then
  echo "Could not resolve gateway connection endpoints." >&2
  exit 1
fi
docker run --detach --rm --name identity-guide-runtime \
  --group-add "$(id -g)" \
  --env KONG_ROLE=data_plane \
  --env KONG_DATABASE=off \
  --env KONG_VITALS=off \
  --env KONG_CLUSTER_MTLS=pki \
  --env "KONG_CLUSTER_CONTROL_PLANE=${cp_host}:443" \
  --env "KONG_CLUSTER_SERVER_NAME=${cp_host}" \
  --env "KONG_CLUSTER_TELEMETRY_ENDPOINT=${tp_host}:443" \
  --env "KONG_CLUSTER_TELEMETRY_SERVER_NAME=${tp_host}" \
  --env KONG_CLUSTER_CERT=/etc/kong/certs/data-plane.crt \
  --env KONG_CLUSTER_CERT_KEY=/etc/kong/certs/data-plane.key \
  --env KONG_LUA_SSL_TRUSTED_CERTIFICATE=system \
  --env KONG_KONNECT_MODE=on \
  --env IDENTITY_GUIDE_VAULT_PRIVATE_KEY \
  --volume "$PWD/certs:/etc/kong/certs:ro" \
  --publish 127.0.0.1:8000:8000 \
  kong/kong-ai-gateway:2.3.0
BASH
```

```shell
bash run-gateway.sh
```

```shell
kongctl get ai-gateway nodes \
  --gateway-name "Identity Guide Gateway"
```

Docker may first download the image. Repeat the node command after a few
seconds until `config_sync.state` is `STATE_IN_SYNC` and compatibility is
`COMPATIBILITY_STATE_FULLY_COMPATIBLE`. A node merely appearing is not enough.
If startup fails, inspect
`docker logs --tail 30 identity-guide-runtime`; do not enable request-header
logging or paste secrets into a support report.

The helper publishes only `127.0.0.1:8000`, so this demonstration proxy is
reachable from your machine. It accepts an optional vault-key environment
variable for the next quickstart; that variable is not needed yet.

## Observe authentication

Send a request without a key:

```shell
curl -i --max-time 20 http://localhost:8000/v1/chat/completions \
  --json '{
    "model": "identity-guide",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

Expect **401**. Repeat with the agent key; do not paste the token into a file:

```shell
curl -i --max-time 20 http://localhost:8000/v1/chat/completions \
  --header "apikey: $SUPPORT_API_KEY" \
  --json '{
    "model": "identity-guide",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

Expect **200**, an assistant message and `X-Kong-Mocking-Plugin: true`.
This confirms principal authentication and the allowed role. It does not
confirm upstream provider authentication, vault retrieval or decryption.

## Observe authorization

In `ai-gateway.yaml`, change only the model's `access.acls.allow_when` rule
from `principal.metadata.role == 'assistant'` to
`principal.metadata.role == 'operator'`. Do not change principal metadata.

```shell
kongctl apply -f ai-gateway.yaml
```

```shell
kongctl get ai-gateway nodes \
  --gateway-name "Identity Guide Gateway"
```

Expect one model update. After synchronization, repeat the authenticated curl
request: expect **403**, because authentication succeeds but the principal
fails the role rule. Change the rule back to `'assistant'`, apply, wait for
synchronization, and repeat: expect **200** again.

## Continue or clean up

Keep the runtime and files for [Token Vault with AI Gateway](../identity-token-vault/).
The next quickstart replaces the mock response with a local upstream verifier.
If stopping here, stop the runtime and delete only this quickstart's gateway:

```shell
docker stop identity-guide-runtime
```

```shell
kongctl delete -f ai-gateway.yaml
```

Review the deletion prompt. The Identity principals, their credentials and
`default` directory are outside this gateway file and are preserved. Your
local files and private key capture remain; retain them only as appropriate
for your test organization.
