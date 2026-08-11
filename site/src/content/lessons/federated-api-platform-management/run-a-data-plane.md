---
title: Run a Dataplane
summary: Connect a local Docker dataplane to the Platform AI Gateway.
order: 5
related:
  - label: Dataplane reference
    url: https://developer.konghq.com/gateway/data-plane-reference/
---

## Goal

You will register a data plane certificate with the Platform AI Gateway and
connect a local Docker data plane to it.

## Generate a certificate

The data plane uses a certificate and private key to authenticate to the AI
Gateway. Generate a pair for this example:

```shell label="Generate the certificate and key"
mkdir -p platform/certs && openssl req -new -x509 -nodes \
  -newkey rsa:2048 -days 365 \
  -subj "/CN=platform-aigw-dp/C=US" \
  -keyout platform/certs/data-plane.key \
  -out platform/certs/data-plane.crt
```

Keep the key private while allowing the container to read it through your
host group:

```shell label="Set the private key permissions"
chgrp "$(id -g)" platform/certs/data-plane.key
chmod 640 platform/certs/data-plane.key
```

Only the public certificate is sent to Konnect. Keep
`platform/certs/data-plane.key` local and do not commit it.

## Register the public certificate

Open `platform/ai-gateway.yaml` in your editor. Add
`data_plane_certificates` between `labels` and `model_providers`:

```yaml label="Add this under platform-aigw"
ai_gateways:
  - ref: platform-aigw
    # Keep the existing gateway fields above this point.
    labels:
      team: platform
    data_plane_certificates:
      - ref: platform-data-plane
        title: platform-data-plane
        description: Local data plane for the federated example
        cert: !file ./certs/data-plane.crt
    model_providers:
      # Keep the existing platform-openai provider here.
```

The `!file` path is relative to `platform/ai-gateway.yaml`. Apply the Platform
configuration to register the certificate on `platform-aigw`:

```shell label="Register the certificate"
kongctl apply -f platform/ai-gateway.yaml
```

The plan should contain one `CREATE` for
`ai_gateway_data_plane_certificate: platform-data-plane`.

## Read the connection endpoints

Each AI Gateway has unique configuration and telemetry endpoints. Read them
from the gateway and store their hostnames for the Docker command:

```shell label="Set the configuration endpoint"
export AIGW_CONTROL_PLANE="$(kongctl get ai-gateway \
  "Platform AI Gateway" --output json --jq \
  '.endpoints.configuration | sub("^https://"; "") | sub(":443$"; "")' \
  --jq-raw-output)"
```

```shell label="Set the telemetry endpoint"
export AIGW_TELEMETRY="$(kongctl get ai-gateway \
  "Platform AI Gateway" --output json --jq \
  '.endpoints.telemetry | sub("^https://"; "") | sub(":443$"; "")' \
  --jq-raw-output)"
```

Review the values before using them:

```shell label="Show the endpoint hostnames"
echo "Configuration: ${AIGW_CONTROL_PLANE}"
echo "Telemetry:     ${AIGW_TELEMETRY}"
```

## Start the data plane

Mount the certificate files instead of placing their contents in the command.
The mount keeps the private key out of your shell history:

```shell label="Run the dataplane"
docker run --detach --rm --name federated-aigw-dp \
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
  --volume "$PWD/platform/certs:/etc/kong/certs:ro" \
  --publish 8000:8000 \
  --publish 8443:8443 \
  kong/kong-ai-gateway:2.0.1
```

The image runs as the non-root `kong` user. `--group-add` gives that user
read-only access to the group-readable private key without making the key
readable by every user on the host.

The data plane receives configuration from `platform-aigw` and exposes its
local HTTP and HTTPS proxy ports at `8000` and `8443`.

## Verify the connection

Allow the container a few seconds to connect, then list the AI Gateway nodes:

```shell label="Verify the data plane"
kongctl get ai-gateway nodes --gateway-name "Platform AI Gateway"
```

The output should include the new data plane node. If it does not appear,
inspect its connection logs:

```shell label="Inspect the data plane logs"
docker logs federated-aigw-dp
```

Leave the data plane running while you complete the remaining lessons.
