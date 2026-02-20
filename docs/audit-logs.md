# Konnect Audit Logs

This page documents the Konnect audit-log listener feature in `kongctl`,
including detached process management with `kongctl ps`.

## Overview

`kongctl` can:

- Create a Konnect audit-log destination.
- Configure the regional Konnect audit-log webhook.
- Start a local HTTP listener to receive webhook events.
- Persist events to local JSONL storage.
- Optionally stream events to STDOUT.
- Optionally run the listener detached in the background.

The feature is exposed through `listen` and `tail`.

## Command Forms

Supported forms (Konnect-first):

- `kongctl listen`
- `kongctl listen audit-logs`
- `kongctl listen konnect audit-logs`
- `kongctl tail`
- `kongctl tail audit-logs`
- `kongctl tail konnect audit-logs`

Important:

- Provide the endpoint from either `--endpoint` or `--public-url` + `--path`.
- `--jq` requires `--tail`.
- `--detach` is not compatible with `--tail`.

## End-to-End Flow

When you run `kongctl listen`:

1. Determines endpoint from `--endpoint` or `--public-url` + `--path`.
1. Checks a webhook does not already exist for the region (due to one
   webhook per region limitation).
1. Create audit-log destination in Konnect.
1. Configure and enable regional webhook to use that destination.
1. Start local listener on `--listen-address` and `--path`.
1. Persist events to local storage.
1. On shutdown, attempt webhook/destination cleanup.

### Startup Guard

Before attaching a new destination, `kongctl` validates that the regional
webhook is in the unconfigured state:

- `enabled=false`
- `endpoint="unconfigured"`

If webhook state is already configured, startup fails fast.

## Event Storage and Format

Default config profile-scoped storage directory:

- `~/.config/kongctl/audit-logs/<profile>/`

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

Use `tail` to stream records to STDOUT:

```shell
kongctl tail --endpoint https://example.tld/audit-logs
```

Filter JSON records with `jq` expression support:

```shell
kongctl tail \
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
