#!/usr/bin/env bash
# Linux-only, fail-closed offline test execution. Build/download outside this
# namespace; the proxy and kongctl children run inside it with loopback only.
set -euo pipefail

if [[ "${1:-}" != --inside ]]; then
  exec sudo unshare --net -- bash "$0" --inside "$(id -u)" "$(id -g)" "$@"
fi
shift
replay_uid="$1"
replay_gid="$2"
shift 2
ip link set lo up
# Drop root and every capability before running any repository Python/Go code.
exec setpriv --reuid "$replay_uid" --regid "$replay_gid" --clear-groups \
  --bounding-set=-all --inh-caps=-all --ambient-caps=-all --no-new-privs \
  python3 scripts/e2e-replay.py replay --require-isolated \
    --test-binary .e2e-artifacts/replay-bin/e2e.test "$@"
