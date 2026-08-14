---
title: "AI Gateway: Route OpenAI"
summary: >-
  Create an AI Gateway and proxy an OpenAI chat request through a local data
  plane.
order: 1
related:
  - label: OpenAI LLM declarative example
    url: https://github.com/Kong/kongctl/tree/main/docs/examples/declarative/ai-gateway/openai-llm
  - label: Declarative configuration documentation
    url: https://developer.konghq.com/kongctl/declarative/
---

## Goal

You will use `kongctl` to create an AI Gateway, launch a local data plane in
Docker, and configure an OpenAI model provider. You will then send an
OpenAI-compatible chat request through the local Kong AI Gateway proxy.

The provider adds your OpenAI credential before forwarding the request. The
client calling the local proxy does not need the credential.

## Prerequisites

Before you begin, you need:

- A [Kong Konnect account](https://konghq.com/products/kong-konnect/register)
  with permission to manage AI Gateway resources.
- `kongctl` installed and authenticated. Complete
  [Konnect Authentication](../../installation/authenticate/) first.
- A running [Docker](https://docs.docker.com/get-started/get-docker/) daemon.
- `openssl` and `curl` available in your terminal.
- An [OpenAI API key](https://platform.openai.com/api-keys) with access to
  `gpt-4.1-nano` and available API quota.

Keep the same terminal open throughout the lesson. The exported variables are
used by later commands.

## Set the OpenAI API key

You need the OpenAI API key when you apply the declarative configuration.
Export it now, replacing `<openai-api-key>` with your key:

```shell label="Set the OpenAI API key"
export OPENAI_API_KEY="<openai-api-key>"
```

Keep this value in the environment. Do not write the key into the declarative
configuration.

## Create a working directory

Create an isolated directory for the configuration and certificate files:

```shell label="Create the working directory"
mkdir -p openai-llm/certs
cd openai-llm
```

## Generate a data plane certificate

The local data plane uses a certificate and private key to authenticate to
Konnect. Generate a self-signed pair:

```shell label="Generate the certificate and key"
openssl req -new -x509 -nodes -newkey rsa:2048 -days 365 \
  -subj "/CN=openai-llm-data-plane/C=US" \
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

Write the AI Gateway, certificate, OpenAI provider, and model configuration to
`ai-gateway.yaml`:

```shell label="Create ai-gateway.yaml"
cat > ai-gateway.yaml <<'YAML'
_defaults:
  kongctl:
    namespace: openai-llm-example

ai_gateways:
  - ref: openai-llm
    name: openai-llm
    display_name: OpenAI LLM Gateway
    description: Routes OpenAI-compatible chat traffic to OpenAI
    proxy_urls:
      - host: localhost
        port: 8000
        protocol: http
    labels:
      example: openai-llm
    data_plane_certificates:
      - ref: openai-llm-data-plane
        title: openai-llm-data-plane
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
    models:
      - ref: gpt-4-1-nano
        name: gpt-4.1-nano
        display_name: GPT-4.1 Nano
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
                - gpt-4.1-nano
        targets:
          - name: gpt-4.1-nano
            provider: openai
            config:
              type: openai
        policies: []
        capabilities:
          - generate
YAML
```

The `!file` tag reads the public certificate relative to this configuration
file. `!env OPENAI_API_KEY` reads the environment variable you exported at the
start of the lesson. The surrounding `!secret` tag marks the assembled bearer
token as sensitive. Together, these tags resolve the key only during
execution, so it is not stored in the configuration or generated plan.

## Create the AI Gateway

Apply the configuration and review the plan before confirming it:

```shell label="Create the Konnect resources"
kongctl apply -f ai-gateway.yaml
```

The plan creates the AI Gateway and its data plane certificate, provider, and
model resources.

## Run a local dataplane

The local dataplane needs the gateway's configuration and telemetry endpoint
hostnames to connect to Konnect. Read them into environment variables:

```shell label="Set the configuration endpoint"
export AIGW_CONTROL_PLANE="$(kongctl get ai-gateway \
  "OpenAI LLM Gateway" --output json --jq \
  '.endpoints.configuration | sub("^https://"; "") | sub(":443$"; "")' \
  --jq-raw-output)"
```

```shell label="Set the telemetry endpoint"
export AIGW_TELEMETRY="$(kongctl get ai-gateway \
  "OpenAI LLM Gateway" --output json --jq \
  '.endpoints.telemetry | sub("^https://"; "") | sub(":443$"; "")' \
  --jq-raw-output)"
```

Confirm that both variables contain hostnames:

```shell label="Show the endpoint hostnames"
echo "Configuration: ${AIGW_CONTROL_PLANE}"
echo "Telemetry:     ${AIGW_TELEMETRY}"
```

### Start the dataplane

Start Kong AI Gateway in Docker and mount the certificate pair read-only:

```shell label="Start the data plane"
docker run --detach --rm --name openai-llm-data-plane \
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
  kong/kong-ai-gateway:2.0.1
```

The container exposes the local HTTP proxy on port `8000` and the HTTPS proxy
on port `8443`.

## Verify the data plane connection

Allow the container a few seconds to connect, then list the gateway nodes:

```shell label="Verify the data plane"
kongctl get ai-gateway nodes --gateway-name "OpenAI LLM Gateway"
```

The output should include the new data plane node. If it appears, continue to
the next step.

### Troubleshoot the connection (optional)

Only inspect the container logs if the data plane node does not appear:

```shell label="Inspect the data plane logs"
docker logs openai-llm-data-plane
```

## Send an LLM request

Send an OpenAI-compatible chat completion request to the local proxy. Kong
maps the configured `gpt-4.1-nano` model to the OpenAI target:

```shell label="Send a chat request"
curl --no-progress-meter --fail-with-body \
  --request POST http://localhost:8000/v1/chat/completions \
  --header "Accept: application/json" \
  --json '{
    "model": "gpt-4.1-nano",
    "messages": [
      {"role": "user", "content": "Say this is a test!"}
    ]
  }'
```

The JSON response should contain an assistant message generated by OpenAI.
The request does not include an `Authorization` header because Kong adds the
credential configured on the provider.

## Clean up

> **Warning:** The following commands stop the local data plane and delete the
> AI Gateway resources created by this lesson.

Stop the Docker container. Because it was started with `--rm`, Docker removes
the container after it stops:

```shell label="Stop the data plane"
docker stop openai-llm-data-plane
```

Delete the AI Gateway and its child resources from Konnect:

```shell label="Delete the Konnect resources"
kongctl delete -f ai-gateway.yaml
```

Review the delete plan before confirming it. The configuration, public
certificate, and private key remain in the local `openai-llm` directory.
