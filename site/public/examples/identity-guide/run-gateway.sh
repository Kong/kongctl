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
