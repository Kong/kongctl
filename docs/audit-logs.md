# Konnect Audit Logs

This page documents the Konnect audit-log listener feature in `kongctl`,
including detached process management with `kongctl ps`.

## Overview

`kongctl` can:

- Pull Konnect organization audit logs on demand.
- Automatically retrieve every cursor page in a result set.
- Follow new organization audit logs by polling until interrupted.
- Create a Konnect audit-log destination.
- Configure the regional Konnect audit-log webhook.
- Start a local HTTP listener to receive webhook events.
- Persist events to local JSONL storage.
- Optionally stream events to STDOUT.
- Optionally run the listener detached in the background.

The feature is exposed through `listen`, `tail`, `get audit-logs`, and `ps`.

## Command Forms

Supported forms (Konnect-first):

- `kongctl listen`
- `kongctl listen audit-logs`
- `kongctl listen konnect audit-logs`
- `kongctl tail`
- `kongctl tail audit-logs`
- `kongctl tail konnect audit-logs`
- `kongctl tail audit-logs listener`
- `kongctl get audit-logs`
- `kongctl get konnect audit-logs`
- `kongctl get audit-logs destinations`
- `kongctl get audit-logs destination <id|name>`
- `kongctl get audit-logs webhook`
- `kongctl ps`
- `kongctl ps stop <pid>`
- `kongctl ps stop --all`

Important:

- Provide the endpoint from either `--endpoint` or `--public-url` + `--path`.
- `--endpoint` is the full public listener URL, including the listener path.
- `--public-url` is a public base URL; `kongctl` appends `--path` to build
  the destination endpoint.
- Listener `--jq` requires listener `--tail`.
- Listener `--detach` is not compatible with listener `--tail`.

## Choosing a Command

Use `listen audit-logs` for a local listener session. It creates a new Konnect
audit-log destination, optionally binds the regional webhook to that
destination, starts the local listener, stores events, and cleans up its
destination when the listener stops.

Use `get audit-logs` to retrieve Konnect organization audit logs through the
pull API. It follows cursor pagination automatically, so `--page-size` affects
the number of API requests but not the total number of records retrieved.

Use `tail audit-logs` to retrieve a five-minute catch-up window and then poll
for new organization audit logs until interrupted. It is equivalent to
`get audit-logs --since 5m --follow`.

Use `tail audit-logs listener` for the webhook listener setup and live STDOUT
streaming previously provided directly by `tail audit-logs`.

Use `get audit-logs destinations` to inspect existing audit-log destinations.
Use `get audit-logs destination <id|name>` when you already know the
destination ID or exact name.

Use `get audit-logs webhook` to inspect the regional webhook configuration,
including whether it is enabled and which destination it currently references.

Use `ps` to inspect and stop detached listener processes created with
`listen --detach`.

## Pull Organization Audit Logs

Retrieve the 50 most recent events:

```shell
kongctl get audit-logs
```

Retrieve the last 24 hours and write one complete event per line:

```shell
kongctl get audit-logs --since 24h --output jsonl > audit-logs.jsonl
```

Use absolute inclusive bounds and an event type filter:

```shell
kongctl get audit-logs \
  --start-time 2026-08-23T00:00:00Z \
  --end-time 2026-08-24T00:00:00Z \
  --type authorization
```

Supported event types are `authentication`, `authorization`, and
`gateway_access`. The CLI preserves complete API records, including each
record's ED25519 `signature`. Signature verification and JWKS retrieval are
not performed by kongctl.

### Pagination and Limits

`--page-size` controls the maximum records requested in each Konnect API call
and defaults to 100 for audit-log pulls. It must be between 1 and 1,000.
kongctl follows the returned cursor until the final page, regardless of page
size.

`--limit` is the separate client-side total record limit. It defaults to 50
when no time window is specified. Time-window queries are unlimited unless
`--limit` is specified. Set `--limit 0` explicitly for unlimited retrieval.

JSON and YAML output include `metadata.count` and `metadata.truncated`;
`truncated` is true when either the implicit or explicit limit stops collection
while additional records are known to exist.

For example, 2,000 matching records with the default page size of 100 require
about 20 API requests but still return all 2,000 records if every request
succeeds. Increasing `--page-size` reduces requests without changing the
result set.

### Time Filters

`--start-time` and `--end-time` accept RFC3339 timestamps and are inclusive.
`--since` accepts a Go duration such as `30m` or `24h` and cannot be combined
with either absolute bound. For finite retrieval, kongctl resolves `--since`
once at startup into a fixed start and end time. When no time flag is supplied,
the service chooses its default window.

Absolute timestamps can use UTC (`Z`) or a numeric UTC offset. kongctl
normalizes both forms to UTC:

```shell
kongctl get audit-logs \
  --start-time 2026-08-23T14:00:00Z \
  --end-time 2026-08-24T09:00:00-05:00
```

Common relative durations include `30s`, `15m`, `2h`, `24h`, `168h`, and
combined values such as `1h30m`. Go durations do not support `d` or `w` units;
use `24h` for one day and `168h` for one week.

### Output and Collection Failures

Finite pull supports `text`, `json`, `yaml`, and `jsonl` output:

- JSON and YAML are buffered and written only after every required page is
  retrieved successfully.
- JSONL writes completed pages immediately and keeps memory bounded. If a
  later page fails, STDOUT contains a partial collection and kongctl exits
  nonzero.
- Text provides a compact summary. Use repeated `--columns HEADER=.field`
  flags to select fields from the complete records.
- JSON and YAML apply `--jq` to the output envelope. JSONL applies `--jq` to
  each record independently.

Collection jobs must check the exit status rather than relying only on the
presence of an output file:

```shell
if kongctl get audit-logs --since 24h --output jsonl > audit-logs.jsonl; then
  echo "audit-log collection completed"
else
  echo "audit-log collection failed or is partial" >&2
  exit 1
fi
```

## Follow Organization Audit Logs

Start with a five-minute catch-up and continue polling:

```shell
kongctl tail audit-logs
```

Equivalent explicit forms are:

```shell
kongctl get audit-logs --since 5m --follow
kongctl get audit-logs --since 5m -F
kongctl tail konnect audit-logs
```

Follow supports `text` and `jsonl`, because JSON and YAML require a bounded
document. `--poll-interval` defaults to 10 seconds. Press Ctrl-C to stop
cleanly.

Each successful polling cycle uses a fixed end checkpoint. The next cycle
overlaps the checkpoint by one minute, deduplicates records by signature (or a
record hash when no signature is present), and emits new records in timestamp
order. Retryable network, rate-limit, and server failures retain the checkpoint
and retry with exponential backoff capped at one minute. Authentication,
authorization, and other non-retryable client failures stop the command with a
nonzero exit status.

## Tail Listener Migration

`tail audit-logs` now follows the organization pull API. The webhook-based
listener remains available under the `listener` child:

```shell
# Previous form
kongctl tail audit-logs \
  --endpoint https://example.tld/audit-logs \
  --authorization "Bearer <token>"

# Current form
kongctl tail audit-logs listener \
  --endpoint https://example.tld/audit-logs \
  --authorization "Bearer <token>"
```

`kongctl listen` and `kongctl listen audit-logs` are unchanged.

## End-to-End Flow

When you run `kongctl listen`:

1. Determines endpoint from `--endpoint` or `--public-url` + `--path`.
1. Checks a webhook does not already exist for the region (due to one
   webhook per region limitation).
1. Creates an audit-log destination in Konnect.
1. Configures and enables the regional webhook to use that destination.
1. Starts a local listener on `--listen-address` and `--path`.
1. Persists events to local storage.
1. On shutdown, attempt webhook/destination cleanup.

### Destination and Webhook Behavior

`listen` creates a new audit-log destination for each listener session. It
does not attach or reuse an existing destination. On normal shutdown, it
deletes the destination it created.

By default, `--configure-webhook=true`, so `listen` also binds the regional
webhook to the destination it just created. To create the destination and
listener without changing regional webhook configuration, pass:

```shell
kongctl listen \
  --endpoint https://example.tld/audit-logs \
  --authorization "Bearer <token>" \
  --configure-webhook=false
```

That mode does not point Konnect at the listener. Use it only when the webhook
is managed separately or when you need to test the local listener behavior
without changing regional configuration.

### Startup Guard

Before attaching a new destination, `kongctl` validates that the regional
webhook is in the unconfigured state:

- `enabled=false`
- `endpoint="unconfigured"`

If webhook state is already configured, startup fails fast.

## Event Storage and Format

Default config profile-scoped storage directory:

- `~/.config/kongctl/audit-logs/<sanitized-profile>/`
- `<sanitized-profile>` is the profile name with unsupported path
  characters replaced by `_`.

Files:

- `events.jsonl`: received event records (raw records, one per line)
- `listener.json`: listener state metadata
- `destination.json`: destination state metadata

Payload handling:

- Only `POST` requests to configured listener path are accepted.
- `gzip` request bodies are decoded when needed.
- Decoded payload is split into line-delimited records.
- Records are stored as-is in `events.jsonl`.

No additional `kongctl` event envelope is added.

## Tailing and JQ

Use the webhook listener child to stream received records to STDOUT:

```shell
kongctl tail audit-logs listener \
  --endpoint https://example.tld/audit-logs \
  --authorization "Bearer <token>"
```

Filter JSON records with `jq` expression support:

```shell
kongctl tail audit-logs listener \
  --endpoint https://example.tld/audit-logs \
  --log-format json \
  --jq '{ts:.event_ts, name, request:(.request // null)}'
```

Notes:

- For structured filtering, `--log-format json` is recommended.
- In tail mode, lifecycle text is logged to the log file, not STDOUT.

## Security

Recommended:

- Use an HTTPS destination endpoint.
- Keep TLS verification enabled (default).
- Provide `--authorization` so Konnect sends an `Authorization` header.

Listener-side authorization validation:

- If `--authorization` is provided, listener requires an exact header match.
- Validation is done in-process before accepting event payloads.

About TLS:

- The local listener is plain HTTP by default.
- HTTPS is usually terminated by your tunnel or reverse proxy.
- `--skip-ssl-verification` affects Konnect delivery to destination endpoint.

## Tailscale Example

You can use [Tailscale](https://tailscale.com/) to expose a local listener
through a public HTTPS endpoint during local development.

Example:

```shell
tailscale funnel 19090
```

If your Tailscale DNS host is `my-host.ts.net`, set the destination endpoint
to your listener path:

```shell
kongctl listen --endpoint https://my-host.ts.net/audit-logs
```

Equivalent pattern:

```text
--endpoint https://<tailscale-host>.ts.net/audit-logs
```

## Detached Listener Mode

Run listener in the background:

```shell
kongctl listen --endpoint https://example.tld/audit-logs --detach
```

Parent process prints:

- child `pid`
- child log file path
- process record file path

Child logs are written to:

- `~/.config/kongctl/logs/kongctl-listener-<pid>.log`

## Process Registry and `kongctl ps`

Detached processes are tracked in:

- `~/.config/kongctl/processes/<pid>.json`

List tracked detached processes:

```shell
kongctl ps
```

Stop one detached process:

```shell
kongctl ps stop <pid>
```

Stop all tracked detached processes:

```shell
kongctl ps stop --all
```

Behavior:

- Running tracked process: `stop` sends `SIGTERM` and removes record.
- Exited or stale record: `stop` prunes the record.
- Failed detached startup keeps process record for debugging.

## Troubleshooting

### `kongctl ps` shows no running listener

If `kongctl ps` is empty but `ps aux` shows a `kongctl listen` process, that
process is unmanaged (typically started before process registry tracking).

Use OS tools for unmanaged processes:

```shell
kill -TERM <pid>
```

Then launch a new detached listener to use managed tracking.

### Startup fails with webhook already configured

If you see an error similar to:

- `regional audit-log webhook is already configured ...`

A regional webhook is already active. Stop the active listener and clear
webhook state before launching a new one.

### No events arriving

Check:

- Destination endpoint includes listener path (for example `/audit-logs`).
- Tunnel forwards HTTPS endpoint to local listen address and port.
- Listener is running and bound to expected `--listen-address`.
- Authorization header configuration matches on both sides.

### Verify process and socket quickly

```shell
pid=<pid>
ps -p "$pid" -o pid,ppid,stat,etime,cmd
ss -ltnp | rg ':19090'
tail -n 200 ~/.config/kongctl/logs/kongctl-listener-${pid}.log
```

## Current Limitations

- Event file retention and rotation are not implemented yet.
- Replay jobs are not implemented yet.
- `kongctl ps` currently manages tracked detached processes only.
- Pull covers Konnect organization audit logs only. Dev Portal audit logs
  remain webhook-based.
- Audit-log retention is controlled by the service and is evolving toward the
  planned one-year history.
- Access to the pull API can still be controlled by the server-side
  `TPS-4185-Audit-Logs-V2-Read` rollout flag.
