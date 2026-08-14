#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cert_dir="${script_dir}/certs"
cert_file="${cert_dir}/data-plane.crt"
key_file="${cert_dir}/data-plane.key"
container_name="openai-llm-data-plane"
image="${KONG_AI_GATEWAY_IMAGE:-kong/kong-ai-gateway:2.0.1}"

usage() {
  echo "Usage: $0 certs|run|stop"
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Error: $1 is required" >&2
    exit 1
  fi
}

generate_certs() {
  require_command openssl
  mkdir -p "${cert_dir}"

  if [[ -e "${cert_file}" || -e "${key_file}" ]]; then
    if [[ -f "${cert_file}" && -f "${key_file}" ]]; then
      echo "Certificate files already exist in ${cert_dir}"
      return
    fi
    echo "Error: remove the incomplete certificate pair in ${cert_dir}" >&2
    exit 1
  fi

  umask 027
  openssl req -new -x509 -nodes -newkey rsa:2048 -days 365 \
    -subj "/CN=openai-llm-data-plane/C=US" \
    -keyout "${key_file}" -out "${cert_file}"
  chgrp "$(id -g)" "${key_file}"
  chmod 640 "${key_file}"
  echo "Created ${cert_file} and ${key_file}"
}

run_data_plane() {
  require_command docker
  : "${AIGW_CONTROL_PLANE:?Set AIGW_CONTROL_PLANE as shown in README.md}"
  : "${AIGW_TELEMETRY:?Set AIGW_TELEMETRY as shown in README.md}"

  if [[ ! -f "${cert_file}" || ! -f "${key_file}" ]]; then
    echo "Error: run '$0 certs' first" >&2
    exit 1
  fi

  docker run --detach --rm --name "${container_name}" \
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
    --volume "${cert_dir}:/etc/kong/certs:ro" \
    --publish 8000:8000 \
    --publish 8443:8443 \
    "${image}"
}

case "${1:-}" in
  certs)
    generate_certs
    ;;
  run)
    run_data_plane
    ;;
  stop)
    require_command docker
    docker stop "${container_name}"
    ;;
  *)
    usage
    exit 1
    ;;
esac
