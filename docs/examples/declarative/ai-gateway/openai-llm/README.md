# OpenAI LLM Quickstart

This example creates an AI Gateway, local data plane certificate, OpenAI model
provider, and GPT-4o model. A local Docker data plane then proxies an
OpenAI-compatible chat request.

You need kongctl authenticated to Konnect, Docker, OpenSSL, and an
[OpenAI API key](https://platform.openai.com/api-keys). Run these commands from
this directory.

The `data-plane.sh` helper only manages the local data plane:

- The `certs` command generates a self-signed certificate and private key under
  `certs/`. The declarative configuration registers the public certificate
  with Konnect; the private key remains local and is ignored by Git.
- The `run` command mounts that certificate pair read-only and starts the AI
  Gateway data plane in Docker using connection endpoints you retrieve with
  kongctl.
- The `stop` command stops and removes the Docker container.

The script never invokes kongctl or changes Konnect resources.

## 1. Create the AI Gateway

Generate the data plane certificate and set your OpenAI key:

```sh
./data-plane.sh certs
export OPENAI_API_KEY='YOUR_OPENAI_API_KEY'
```

Apply all Konnect resources. The OpenAI key is resolved during execution and
is not stored in the configuration or plan:

```sh
kongctl apply -f ai-gateway.yaml
```

## 2. Run the local data plane

Read the gateway's connection endpoints:

```sh
export AIGW_CONTROL_PLANE="$(kongctl get ai-gateway \
  "OpenAI LLM Gateway" --output json --jq \
  '.endpoints.configuration | sub("^https://"; "") | sub(":443$"; "")' \
  --jq-raw-output)"

export AIGW_TELEMETRY="$(kongctl get ai-gateway \
  "OpenAI LLM Gateway" --output json --jq \
  '.endpoints.telemetry | sub("^https://"; "") | sub(":443$"; "")' \
  --jq-raw-output)"
```

Start the data plane:

```sh
./data-plane.sh run
```

Confirm that it connects to the AI Gateway:

```sh
kongctl get ai-gateway nodes --gateway-name "OpenAI LLM Gateway"
```

## 3. Send LLM traffic

The provider adds the OpenAI credential before forwarding this request:

```sh
curl --no-progress-meter --fail-with-body \
  --request POST http://localhost:8000/v1/chat/completions \
  --header "Accept: application/json" \
  --json '{
    "model": "my-gpt-4o",
    "messages": [
      {"role": "user", "content": "Say this is a test!"}
    ]
  }'
```

## Clean up

Stop and remove the local Docker data plane:

```sh
./data-plane.sh stop
```

Delete the AI Gateway and its child resources from Konnect:

```sh
kongctl delete -f ai-gateway.yaml
```

The generated certificate files remain under `certs/` and are ignored by Git.
