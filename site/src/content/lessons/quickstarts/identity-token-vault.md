---
title: "Kong Identity: Token Vault with AI Gateway"
summary: >-
  Retrieve an encrypted shared credential and verify its use at a local
  upstream.
order: 2
related:
  - label: Kong Identity Token Vault
    url: https://developer.konghq.com/identity/token-vault/
---

## What you will demonstrate

Your agent supplies its principal API key. AI Gateway authenticates it,
applies the model's role rule, retrieves an encrypted shared ticketing token,
decrypts it with your private vault key, and sends that different credential
to an upstream service. The agent never needs to receive the upstream token.

This is the optional runtime-verification lab. A mocked model cannot prove
this behavior because it answers before the upstream call. We use a tiny
**local upstream verifier** that accepts only the stored test credential.
Its OpenAI-shaped response makes it compatible with our existing model route;
it is not a real LLM or ticketing product. No paid provider is required.

## Prerequisites and current state

Complete all six [Kong Identity lessons](../../kong-identity/), including
Trusted IdP and Vault Providers and Credentials, and complete
[Authenticate AI Gateway](../identity-authentication/). Keep the model's
role rule set to `'assistant'`. Leave the same terminal open in `identity-guide`.

You should have these files and variables:

| Item                                                        | Source                                                                        |
| ----------------------------------------------------------- | ----------------------------------------------------------------------------- |
| `principals.yaml`, `SUPPORT_API_KEY`                        | Agent, human, custom identity and captured agent key.                         |
| `directory.yaml`, `trust.yaml`, `vault.yaml`, `issuer.yaml` | Adopted directory, trusted issuer, vault public key, provider and credential. |
| `private/vault-private.pem`, `vault-public.pem`             | Matching vault encryption pair.                                               |
| `ai-gateway.yaml`, `certs/`, `run-gateway.sh`               | Mock gateway and connected runtime.                                           |
| `IDENTITY_GUIDE_ISSUER`, `IDENTITY_GUIDE_JWKS_URI`          | Public issuer URLs from the Trusted IdP lesson.                               |

If using a new terminal, reload the principal key as shown in the
API-key lesson, and read the issuer URLs again from the Trusted IdP lesson.
Do not generate replacement encryption keys: the existing credential is
protected with the public key you already registered.

```shell
kongctl get id directory default -o json
```

```shell
kongctl get id vault provider \
  --provider-name identity-guide-ticketing \
  -o json
```

```shell
kongctl get id vault provider credentials \
  --provider-name identity-guide-ticketing \
  -o json
```

Check that Vault and the provider are enabled, the directory is owned in the
namespace you chose, and the shared credential exists. If you skipped
adoption, return to
[Enable Token Vault](../../kong-identity/enable-token-vault/): KongCTL requires
explicit parent ownership because these children cannot store their own
management markers. This is a client management rule, not an API enablement
prerequisite.

This lab uses the **API-key exchange path**. The Trusted IdP from the Identity
chapter is configured for later JWT experiments, but it is
not used to verify this request's API key. We do not demonstrate OAuth consent,
JIT provisioning or per-user enrollment here.

## Set a known demonstration credential

Make the stored test value unambiguous, even if you tried the optional rotation
exercise earlier. This explicitly replaces only the declared shared secret:

```shell
export SUPPORT_TICKETING_TOKEN='identity-guide-demo-token-v2'
```

```shell
kongctl apply -f vault.yaml \
  --write-secret 'support-ticketing-token#secret'
```

Review the selected credential write. This changes the stored upstream token,
not the principal API key. We will give the same synthetic value to the verifier
so it can check what the gateway sends. In a real deployment the upstream
service already knows its issued credential; the agent would not be given it.

## Give the runtime its private encryption key

The Identity directory holds only `vault-public.pem`. The matching private
key is needed **inside the runtime** to decrypt the stored credential:

```shell
export IDENTITY_GUIDE_VAULT_PRIVATE_KEY="$(<private/vault-private.pem)"
```

```shell
docker stop identity-guide-runtime
```

```shell
bash run-gateway.sh
```

This restarts only the local demo runtime, passing the key by environment. It
is not uploaded to Konnect or written into a manifest. The helper already
passes this variable; no helper edit is needed. Environment-backed resolution
is convenient for this local exercise; use your chosen secret-management
integration for a deployed runtime.

## Start the local upstream verifier

Create this small Python server in the workspace. It logs only a Boolean
result, never the received header or credential:

```shell label="Create ticketing-upstream.py"
cat > ticketing-upstream.py <<'PYTHON'
"""Local demo upstream; never logs or returns received credentials."""

import hmac
import json
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


expected = os.environ.get("SUPPORT_TICKETING_TOKEN")
if not expected:
    raise SystemExit("Set SUPPORT_TICKETING_TOKEN to the value stored in Token Vault.")


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def do_POST(self):
        if self.path != "/v1/chat/completions":
            self.send_error(404)
            return
        provided = self.headers.get("Authorization", "")
        verified = hmac.compare_digest(
            provided.encode(), ("Bearer " + expected).encode()
        )
        if verified:
            payload = {
                "id": "chatcmpl-identity-guide",
                "object": "chat.completion",
                "created": 0,
                "model": "gpt-4o",
                "choices": [{
                    "index": 0,
                    "finish_reason": "stop",
                    "message": {
                        "role": "assistant",
                        "content": "The upstream verified the ticketing credential.",
                    },
                }],
                "usage": {
                    "prompt_tokens": 0,
                    "completion_tokens": 0,
                    "total_tokens": 0,
                },
            }
        else:
            payload = {"error": {"message": "Upstream credential did not match."}}
        body = json.dumps(payload).encode()
        self.send_response(200 if verified else 401)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)
        self.close_connection = True
        print(f"upstream_credential_verified={verified}", flush=True)


address = (os.environ.get("SUPPORT_TICKETING_BIND", "127.0.0.1"), 18080)
print(f"Demo upstream listening on http://{address[0]}:{address[1]}", flush=True)
ThreadingHTTPServer(address, Handler).serve_forever()
PYTHON
```

Run it in a second container sharing the runtime's network namespace:

```shell
docker run --detach --rm --name identity-guide-upstream \
  --network container:identity-guide-runtime \
  --env SUPPORT_TICKETING_TOKEN \
  --volume "$PWD/ticketing-upstream.py:/demo.py:ro" \
  python:3.13-alpine python -u /demo.py
```

```shell
docker logs --tail 5 identity-guide-upstream
```

Expect `Demo upstream listening on http://127.0.0.1:18080`. Sharing the runtime's
network namespace makes loopback reach the verifier without exposing port
18080 or depending on host firewall rules. The token variable must be set in
the shell starting this container. If you restart the gateway again, stop and
recreate the verifier too so it shares the new runtime's namespace.

## Connect the model provider to Token Vault

Create a second complete gateway manifest. It retains the existing gateway
and role rule, disables the mock, points the model at the verifier and adds
the vault integration:

```shell label="Create ai-gateway-vault.yaml"
cat > ai-gateway-vault.yaml <<'YAML'
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
        enabled: false
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
            headers: []
        datastores:
          - name: !ref guide-token-storage#name
        token_vault:
          provider: identity-guide-ticketing
          subject_token_type: api_key
          credential:
            inbound:
              location: header
              name: Authorization
              prefix: "Bearer "
            outbound:
              location: header
              name: Authorization
              prefix: "Bearer "
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
              upstream_url: http://127.0.0.1:18080/v1/chat/completions
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
    vaults:
      - ref: guide-private-keys
        name: guide-private-keys
        type: env
        description: Resolve the local runtime encryption key
        config:
          prefix: IDENTITY_GUIDE_VAULT_
    datastores:
      - ref: guide-token-storage
        name: guide-token-storage
        display_name: Guide Token Vault storage
        type: token-vault
        config:
          directory: default
          current_version: v1
          keys:
            - version: v1
              key: "{vault://guide-private-keys/private-key}"
YAML
```

There are two different uses of the word vault here:

| Configuration                                                        | Purpose                                                                                                           |
| -------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| Identity directory's `vault.providers`                               | Holds `identity-guide-ticketing` and its encrypted shared credential.                                             |
| Gateway `vaults`, name `guide-private-keys`, type `env`              | Resolves `{vault://guide-private-keys/private-key}` from the runtime variable `IDENTITY_GUIDE_VAULT_PRIVATE_KEY`. |
| Gateway `datastores`, name `guide-token-storage`, type `token-vault` | Selects directory `default` and the encryption-key version/reference. It does not create another Identity vault.  |
| Model provider's `token_vault.provider`                              | Selects the Identity provider by its name, `identity-guide-ticketing`.                                            |

```shell
kongctl apply -f ai-gateway-vault.yaml
```

Expect **two creations** (environment vault and datastore) and **three updates**
(disable the mock policy, configure provider credential exchange, change the
model target URL). Nothing creates a second principal or ticketing provider.
Use this new gateway file going forward; reapplying the earlier mock file can
restore its old behavior.

```shell
kongctl get ai-gateway nodes \
  --gateway-name "Identity Guide Gateway"
```

Wait for `STATE_IN_SYNC` and full compatibility. A synchronized configuration
still does not prove credential retrieval; the next request does.

## Verify the full request path

```shell
curl -i --max-time 20 http://localhost:8000/v1/chat/completions \
  --header "apikey: $SUPPORT_API_KEY" \
  --header "Authorization: Bearer $SUPPORT_API_KEY" \
  --json '{
    "model": "identity-guide",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

For this deliberate configuration the same principal key is sent twice:
`apikey` is consumed by inbound key-auth, while `Authorization` is the configured
vault-exchange input. The latter is replaced with the retrieved ticketing token
before the upstream call. Never put `SUPPORT_TICKETING_TOKEN` in this client
request. The upstream does not receive the principal API key as its bearer token.

Expect **200** and `The upstream verified the ticketing credential.` The
response should no longer carry the mocking header. Check the verifier:

```shell
docker logs --tail 5 identity-guide-upstream
```

Expect `upstream_credential_verified=True`. A 200 alone would not prove this;
the verifier only returns success after comparing the supplied upstream token.
We tested shared static credentials, not a per-user OAuth authorization.

## If the request fails

- **401 from the gateway:** check the agent key is loaded and both headers are
  supplied. Do not print or share their values.
- **403:** confirm the model allows role `assistant` and the principal has it.
- **401 from the verifier / `False`:** the runtime reached the upstream, but
  its supplied token did not match the verifier's expectation. Confirm the
  explicit test-token write completed and recreate the verifier with the
  same test value. An exchange cache can delay the effect of a later rotation.
- **Timeout or 502/504:** check both containers are running and the node is in
  sync. Check the verifier's listening message and runtime error logs. A
  timeout alone does not establish an Identity authentication failure.
- **Decryption or vault errors:** verify the runtime has the matching private
  key, the datastore references directory `default`, and the selected provider
  is `identity-guide-ticketing`. Do not regenerate the key as a troubleshooting
  shortcut.

## Keep one current configuration

The staged files make each ownership boundary visible. For ongoing use,
[download the consolidated manifest](../../examples/identity-guide/complete.yaml)
and save it as `complete.yaml` in this same workspace. It combines both
principals, their custom identity and agent key, the managed directory and
vault children, the Auth Server/Trusted IdP and the final gateway configuration.
It preserves separate directory and principal/gateway namespaces.

If you used a different directory ownership namespace, update only the
explicit directory `kongctl.namespace` override to match your existing one.
The consolidated file expects the original principal descriptions; if you
edited one earlier, retain that edit here to avoid reverting it.
Keep the public issuer variables set. It still references local public files
and deferred secrets; Docker and private material remain outside YAML.

```shell
kongctl diff --mode apply -f complete.yaml
```

Expect no changes if you followed the examples unchanged. Review any difference
before using `kongctl apply -f complete.yaml`. Do not also load older staged
manifests defining the same resources. This file is intended for the completed
walkthrough state; enabling an existing disabled vault still requires its
separate enablement step first.

## Stop the lab without deleting the shared directory

Stop the verifier before the gateway:

```shell
docker stop identity-guide-upstream
```

```shell
docker stop identity-guide-runtime
```

To remove the gateway only:

```shell
kongctl delete -f ai-gateway-vault.yaml
```

Review its plan. **Do not run `delete -f complete.yaml`, `delete -f vault.yaml`,
or `delete -f directory.yaml` for blanket cleanup**: they declare `default`
and would select the directory and its resources. Keep the directory, issuer,
principals and vault resources until their owner reviews resource-specific
retirement. Retain the encryption private key while any stored credential
still depends on it. Stop the verifier too if it is no longer needed; restarting
the gateway alone does not move it to the new network namespace.
