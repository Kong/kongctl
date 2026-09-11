#!/usr/bin/env bash
set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cert_dir="${project_dir}/certs"
cert_file="${cert_dir}/data-plane.crt"
key_file="${AIGW_DATA_PLANE_KEY:-${cert_dir}/data-plane.key}"
container_name="${AIGW_CONTAINER_NAME:-ai-demo-data-plane}"
image="${KONG_AI_GATEWAY_IMAGE:-kong/kong-ai-gateway:2.0.3}"

fail() {
  echo "Error: $*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "$1 is required"
}

check_certificates() {
  require_command openssl
  [[ "${key_file}" == /* ]] || fail "AIGW_DATA_PLANE_KEY must be absolute"
  [[ -f "${cert_file}" && -f "${key_file}" ]] ||
    fail "Missing certificate pair; supply the existing key or generate a new pair explicitly"
  local cert_public_key key_public_key
  cert_public_key="$(openssl x509 -in "${cert_file}" -pubkey -noout |
    openssl pkey -pubin -outform DER | openssl dgst -sha256)"
  key_public_key="$(openssl pkey -in "${key_file}" -passin pass: \
    -pubout -outform DER | openssl dgst -sha256)"
  [[ "${cert_public_key}" == "${key_public_key}" ]] ||
    fail "The private key does not match certs/data-plane.crt"
  openssl x509 -in "${cert_file}" -noout -checkend 0 >/dev/null ||
    fail "The data plane certificate has expired"
}

generate_certs() {
  require_command openssl
  [[ "${key_file}" == /* ]] || fail "AIGW_DATA_PLANE_KEY must be absolute"
  if [[ -e "${cert_file}" || -e "${key_file}" ]]; then
    check_certificates
    echo "Using the existing certificate pair"
    return
  fi
  umask 027
  mkdir -p "${cert_dir}" "$(dirname "${key_file}")"
  openssl req -config /dev/null -new -x509 -nodes -newkey rsa:2048 -days 365 \
    -subj "/CN=ai-demo-data-plane/C=US" \
    -keyout "${key_file}" -out "${cert_file}"
  chgrp "$(id -g)" "${key_file}"
  chmod 640 "${key_file}"
}

run_data_plane() {
  require_command docker
  check_certificates
  local key_group
  key_group="$(stat -c '%g' "${key_file}" 2>/dev/null || \
    stat -f '%g' "${key_file}")"
  : "${AIGW_CONTROL_PLANE:?Set the discovered configuration hostname}"
  : "${AIGW_TELEMETRY:?Set the discovered telemetry hostname}"
  for endpoint in "${AIGW_CONTROL_PLANE}" "${AIGW_TELEMETRY}"; do
    [[ "${endpoint}" != null &&
       "${endpoint}" =~ ^[a-zA-Z0-9][a-zA-Z0-9.-]*$ ]] ||
      fail "Endpoints must be hostnames, without a scheme, port or path"
  done
  docker run --detach --rm --name "${container_name}" \
    --group-add "${key_group}" \
    --env KONG_ROLE=data_plane --env KONG_DATABASE=off \
    --env KONG_VITALS=off --env KONG_CLUSTER_MTLS=pki \
    --env "KONG_CLUSTER_CONTROL_PLANE=${AIGW_CONTROL_PLANE}:443" \
    --env "KONG_CLUSTER_SERVER_NAME=${AIGW_CONTROL_PLANE}" \
    --env "KONG_CLUSTER_TELEMETRY_ENDPOINT=${AIGW_TELEMETRY}:443" \
    --env "KONG_CLUSTER_TELEMETRY_SERVER_NAME=${AIGW_TELEMETRY}" \
    --env KONG_CLUSTER_CERT=/etc/kong/certs/data-plane.crt \
    --env KONG_CLUSTER_CERT_KEY=/etc/kong/certs/data-plane.key \
    --env KONG_LUA_SSL_TRUSTED_CERTIFICATE=system \
    --env KONG_KONNECT_MODE=on \
    --volume "${cert_file}:/etc/kong/certs/data-plane.crt:ro" \
    --volume "${key_file}:/etc/kong/certs/data-plane.key:ro" \
    --publish 127.0.0.1:8000:8000 \
    --publish 127.0.0.1:8443:8443 \
    "${image}"
}

case "${1:-}" in
  certs) generate_certs ;;
  check) check_certificates ;;
  run) run_data_plane ;;
  stop)
    require_command docker
    docker stop "${container_name}"
    ;;
  *) fail "Usage: bash data-plane.sh certs|check|run|stop" ;;
esac
