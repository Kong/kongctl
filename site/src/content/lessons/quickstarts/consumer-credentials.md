---
title: "AI Gateway: Consumer Credentials"
summary: >-
  Protect an OpenAI model with an environment-backed API key for an AI Gateway
  Consumer.
order: 2
related:
  - label: Declarative configuration documentation
    url: https://developer.konghq.com/kongctl/declarative/
  - label: Kong Gateway authentication
    url: https://developer.konghq.com/gateway/authentication/
---

## Goal

You will use `kongctl` to create an AI Gateway that routes requests to OpenAI
and requires clients to authenticate as an AI Gateway Consumer. You will launch
a local data plane, verify that requests without a valid Consumer credential
are rejected, and confirm that the exact API key read from an environment
variable is accepted.

This lesson uses two separate credentials:

- `OPENAI_API_KEY` authenticates the AI Gateway to OpenAI.
- `CONSUMER_API_KEY` authenticates your client to the AI Gateway.

The data plane consumes the client credential before routing the request. It
does not forward that credential to OpenAI. The model provider independently
adds the OpenAI credential to the upstream request.

## Prerequisites

Before you begin, you need:

- A [Kong Konnect account](https://konghq.com/products/kong-konnect/register)
  with permission to manage AI Gateway resources.
- `kongctl` installed and authenticated. Complete
  [Konnect Authentication](../../installation/authenticate/) first.
- A running [Docker](https://docs.docker.com/get-started/get-docker/) daemon.
- `openssl` and `curl` available in your terminal.
- An [OpenAI API key](https://platform.openai.com/api-keys) with access to
  `gpt-5.4-nano` and available API quota.

Keep the same terminal open throughout the lesson. The exported variables are
used by later commands.

## Set the API keys

Export your OpenAI API key, replacing `<openai-api-key>` with the real value:

```shell label="Set the OpenAI API key"
export OPENAI_API_KEY="<openai-api-key>"
```

Set a harmless, known value for the Consumer credential. This value only
authenticates requests to your local data plane:

```shell label="Set the Consumer API key"
export CONSUMER_API_KEY="consumer-credential-quickstart"
```

Keep both values in the environment. Do not write them directly into the
declarative configuration.

## Create a working directory

Create an isolated directory for the configuration and certificate files:

```shell label="Create the working directory"
mkdir -p consumer-credentials/certs
cd consumer-credentials
```

## Generate a data plane certificate

The local data plane uses a certificate and private key to authenticate to
Konnect. Generate a self-signed pair:

```shell label="Generate the certificate and key"
openssl req -new -x509 -nodes -newkey rsa:2048 -days 365 \
  -subj "/CN=consumer-credentials-data-plane/C=US" \
  -keyout certs/data-plane.key \
  -out certs/data-plane.crt
```

Keep the private key readable only by its owner and group. The Docker
container joins that group in a later step:

```shell label="Protect the private key"
chgrp "$(id -g)" certs/data-plane.key
chmod 640 certs/data-plane.key
```

Only the public certificate is registered with Konnect. Keep
`certs/data-plane.key` local and do not commit it.

## Create the declarative configuration

Write the AI Gateway, certificate, OpenAI provider, model, auth strategy,
Consumer, and Consumer credential to `ai-gateway.yaml`:

```shell label="Create ai-gateway.yaml"
cat > ai-gateway.yaml <<'YAML'
_defaults:
  kongctl:
    namespace: consumer-credential-quickstart

ai_gateways:
  - ref: consumer-credential-gateway
    name: consumer-credential-gateway
    display_name: Consumer Credential Gateway
    description: Protects an OpenAI model with a Consumer API key
    deployment_type: hybrid
    proxy_urls:
      - host: localhost
        port: 8000
        protocol: http
    labels:
      example: consumer-credentials
    data_plane_certificates:
      - ref: consumer-credential-data-plane
        title: consumer-credential-data-plane
        description: Local Docker data plane
        cert: !file ./certs/data-plane.crt
    model_providers:
      - ref: openai
        name: openai
        display_name: OpenAI
        type: openai
        config:
          auth:
            type: basic
            headers:
              - name: Authorization
                value: !secret
                  parts:
                    - "Bearer "
                    - !env OPENAI_API_KEY
    auth_strategies:
      - ref: consumer-key-auth
        name: consumer-key-auth
        display_name: Consumer Key Authentication
        type: key-auth
        config:
          key_names:
            - apikey
          hide_credentials: true
    consumers:
      - ref: quickstart-consumer
        name: quickstart-consumer
        display_name: Quickstart Consumer
        custom_id: quickstart-consumer
        type: api-key
        credentials:
          - ref: quickstart-consumer-key
            name: quickstart-consumer-key
            display_name: Quickstart Consumer Key
            type: api-key
            ttl: 0
            api_key: !secret {source: !env CONSUMER_API_KEY}
    models:
      - ref: cheap-openai-model
        name: cheap-openai-model
        display_name: Cheap OpenAI Model
        type: model
        enabled: true
        access:
          auth_strategies:
            - !ref consumer-key-auth
        formats:
          - type: openai
        config:
          route:
            paths:
              - /v1
            model:
              body_param: model
              values:
                - cheap-openai-model
        targets:
          - name: gpt-5.4-nano
            provider: openai
            config:
              type: openai
        policies: []
        capabilities:
          - generate
YAML
```

The key-auth strategy protects the model and reads the Consumer key
from the `apikey` request header. The Consumer credential uses `!secret` to
mark its `api_key` as write-only and `!env` to resolve its value from
`CONSUMER_API_KEY` during execution.

## Create the AI Gateway

Apply the configuration:

```shell label="Create the Konnect resources"
kongctl apply -f ai-gateway.yaml
```

Review the proposed changes displayed by kongctl. Confirm the apply when the
resources and actions match the configuration you created. The apply creates
the AI Gateway and its certificate, model provider, auth strategy, Consumer,
Consumer credential, and model.

## Run a local data plane

The local data plane needs the gateway's configuration and telemetry endpoint
hostnames to connect to Konnect. Read them into environment variables:

```shell label="Set the configuration endpoint"
export AIGW_CONTROL_PLANE="$(kongctl get ai-gateway \
  "Consumer Credential Gateway" --output json --jq \
  '.endpoints.configuration | sub("^https://"; "") | sub(":443$"; "")' \
  --jq-raw-output)"
```

```shell label="Set the telemetry endpoint"
export AIGW_TELEMETRY="$(kongctl get ai-gateway \
  "Consumer Credential Gateway" --output json --jq \
  '.endpoints.telemetry | sub("^https://"; "") | sub(":443$"; "")' \
  --jq-raw-output)"
```

Confirm that both variables contain hostnames without a URL scheme or port:

```shell label="Show the endpoint hostnames"
echo "Configuration: ${AIGW_CONTROL_PLANE}"
echo "Telemetry:     ${AIGW_TELEMETRY}"
```

### Start the data plane

Start Kong AI Gateway in Docker and mount the certificate pair read-only:

```shell label="Start the data plane"
docker run --detach --rm --name consumer-credential-data-plane \
  --group-add "$(id -g)" \
  --env "KONG_ROLE=data_plane" \
  --env "KONG_DATABASE=off" \
  --env "KONG_VITALS=off" \
  --env "KONG_CLUSTER_MTLS=pki" \
  --env "KONG_CLUSTER_CONTROL_PLANE=${AIGW_CONTROL_PLANE}:443" \
  --env "KONG_CLUSTER_SERVER_NAME=${AIGW_CONTROL_PLANE}" \
  --env "KONG_CLUSTER_TELEMETRY_ENDPOINT=${AIGW_TELEMETRY}:443" \
  --env "KONG_CLUSTER_TELEMETRY_SERVER_NAME=${AIGW_TELEMETRY}" \
  --env "KONG_CLUSTER_CERT=/etc/kong/certs/data-plane.crt" \
  --env "KONG_CLUSTER_CERT_KEY=/etc/kong/certs/data-plane.key" \
  --env "KONG_LUA_SSL_TRUSTED_CERTIFICATE=system" \
  --env "KONG_KONNECT_MODE=on" \
  --volume "$PWD/certs:/etc/kong/certs:ro" \
  --publish 8000:8000 \
  --publish 8443:8443 \
  kong/kong-ai-gateway:2.0.2
```

The container exposes the local HTTP proxy on port `8000` and the HTTPS proxy
on port `8443`.

## Verify the data plane connection

Allow the container a few seconds to connect, then list the gateway nodes:

```shell label="Verify the data plane"
kongctl get ai-gateway nodes \
  --gateway-name "Consumer Credential Gateway"
```

The output should include the new data plane node. If it appears, continue to
the credential checks.

### Troubleshoot the connection (optional)

Only inspect the container logs if the data plane node does not appear:

```shell label="Inspect the data plane logs"
docker logs consumer-credential-data-plane
```

## Verify Consumer authentication

First, send a request without a Consumer key:

```shell label="Send a request without a Consumer key"
curl -i --no-progress-meter \
  --request POST http://localhost:8000/v1/chat/completions \
  --header 'Content-Type: application/json' \
  --json '{
    "model": "cheap-openai-model",
    "messages": [
      {"role": "user", "content": "Reply with only OK."}
    ]
  }'
```

The request should return `HTTP/1.1 401 Unauthorized`.

Next, send a request with an incorrect key:

```shell label="Send an incorrect Consumer key"
curl -i --no-progress-meter \
  --request POST http://localhost:8000/v1/chat/completions \
  --header 'apikey: incorrect-consumer-key' \
  --header 'Content-Type: application/json' \
  --json '{
    "model": "cheap-openai-model",
    "messages": [
      {"role": "user", "content": "Reply with only OK."}
    ]
  }'
```

This request should also return `HTTP/1.1 401 Unauthorized`.

Finally, send the exact value that kongctl read from `CONSUMER_API_KEY`:

```shell label="Send the configured Consumer key"
curl -i --no-progress-meter --fail-with-body \
  --request POST http://localhost:8000/v1/chat/completions \
  --header "apikey: ${CONSUMER_API_KEY}" \
  --header 'Content-Type: application/json' \
  --json '{
    "model": "cheap-openai-model",
    "messages": [
      {"role": "user", "content": "Reply with only OK."}
    ]
  }'
```

The request should return `HTTP/1.1 200 OK` and an assistant message from
OpenAI. Together, the three results prove that the environment value was sent
to Konnect and enforced by the data plane.

## Clean up

> **Warning:** The following commands stop the local data plane and delete the
> AI Gateway resources created by this lesson.

Stop the Docker container. Because it was started with `--rm`, Docker removes
the container after it stops:

```shell label="Stop the data plane"
docker stop consumer-credential-data-plane
```

Delete the AI Gateway and its child resources from Konnect:

```shell label="Delete the Konnect resources"
kongctl delete -f ai-gateway.yaml
```

Review the delete plan before confirming it. The configuration, public
certificate, and private key remain in the local `consumer-credentials`
directory.
