#!/usr/bin/env bash
# Load the verified build-job archives. Never build or run a container here.
set -euo pipefail
image_dir=${1:?image archive directory required}
: "${RELEASE_VERSION:?}"
repositories=(ghcr.io/kong/kongctl)
[[ -z ${DOCKER_USERNAME:-} ]] || repositories+=(kong/kongctl)
for arch in amd64 arm64; do
  docker load --input "$image_dir/$arch.tar"
  for repository in "${repositories[@]}"; do
    docker tag "kongctl-release:$arch" "$repository:$RELEASE_VERSION-$arch"
    docker push "$repository:$RELEASE_VERSION-$arch"
  done
done
for repository in "${repositories[@]}"; do
  for tag in "v$RELEASE_VERSION" "$RELEASE_VERSION" "${RELEASE_VERSION%.*}" latest; do
    docker manifest create "$repository:$tag" \
      "$repository:$RELEASE_VERSION-amd64" "$repository:$RELEASE_VERSION-arm64"
    docker manifest push "$repository:$tag"
  done
done
