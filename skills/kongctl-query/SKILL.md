---
name: kongctl-query
description: Use this skill to inspect Kong Konnect resources with read-only
  kongctl commands. List and retrieve resource state, validate authenticated user,
  discover commands and usage, and format outputs as text, json, or yaml.
license: Apache-2.0
metadata:
  product: kongctl
  category: query
  scope: read-only
---

# kongctl query commands

## Goal

Use read-only `kongctl` commands to inspect Konnect resource state and return
concise, structured results.

## Preconditions

- Confirm CLI is installed and runnable: `kongctl version`
- Authenticate with one of:
  - `export KONGCTL_DEFAULT_KONNECT_PAT=<token>` # User set token, <DEFAULT> is the config profile name
  - `kongctl login` # Interactive browser based login, only guide the user to use this
- Select configuration profile when needed: `--profile <name>`
- Optionally verify identity and access when network calls are expected:
  `kongctl get me -o json`

## Config and Environment Overrides

- `kongctl` flags can be defaulted by profile config and environment variables.
- Environment variable pattern: `KONGCTL_<PROFILE>_<PATH>`.
- Example: `KONGCTL_DEFAULT_OUTPUT=yaml` sets `--output` default for the
  `default` profile.
- For this skill, pass explicit `-o json` or `-o yaml` on query commands to
  avoid unexpected profile/env defaults.
- When troubleshooting output behavior, inspect relevant env vars:
  - `env | grep '^KONGCTL_.*OUTPUT'`
  - `env | grep '^KONGCTL_PROFILE'`

## Operating Rules

- Use only read-only operations in this skill.
- Prefer `get`, `list`, and `help` commands.
- Do not run mutating commands such as `create`, `apply`, `patch`, `delete`,
  or `adopt`.
- Hand off mutation requests to a different skill/workflow.

## Workflow

1. Identify the resource type and output expected by the user.
2. Discover command shape when unsure:
   - `kongctl help`
   - `kongctl get --help`
   - `kongctl get <resource> --help`
   - Extract current `kongctl get` subcommands from help output:
     ```bash
     kongctl get --help | awk '
     /^Available Commands:/ {capture=1; next}
     capture && NF==0 {exit}
     capture && $1 ~ /^[a-z0-9-]+$/ {print $1}
     '
     ```
3. Query resource state with structured output:
   - Default to JSON output unless YAML is explicitly requested.
   - List resources: `kongctl get <resource> -o json`
   - Get one resource by name or ID: `kongctl get <resource> "<name-or-id>" -o json`
   - Query child resources:
     `kongctl get <parent> <child> --<parent>-name "<name>" -o json`
4. Filter and summarize relevant fields for the user.
5. Return findings with IDs, names, and timestamps when available.

## Common Commands

```bash
# list portals in json format
kongctl get portals -o json

# get organization details as yaml
kongctl get organization -o yaml

# inspect current identity as json
kongctl get me -o json

# get a specific resource by name or ID
kongctl get portals <portal-name> -o json
kongctl get portals <portal-id-uuid> -o json

# query child resources (portal pages)
kongctl get portals pages --portal-name <portal-name> -o json

# query api resources
kongctl get apis -o json

# query api child documents
kongctl get apis documents --api-name <api-name> -o json
```

## Example: List Portals

Use this command to list Developer Portal instances:

```bash
kongctl get portals -o json
```

Expect fields like:
- `id`
- `name`
- `display_name`
- `canonical_domain`
- `created_at`
- `updated_at`
- `labels`

To fetch one portal instead of a list, provide a name or ID:

```bash
kongctl get portals "portal-auth" -o json
kongctl get portals "35fefe98-f098-4a65-9807-d76f40b620cf" -o json
```

## Child Resources

Use parent-child `get` patterns for nested resources.

```bash
# list pages for a specific portal
kongctl get portals pages --portal-name "portal-auth" -o json
```

Discover child commands under a parent by checking parent help:

```bash
kongctl get portals --help
```

If the response is an empty array (`[]`), treat it as a valid "no resources
found" result, not an execution error.

## Output Guidance

- Prefer `-o json` for filtering and automation.
- Use `-o yaml` for human-readable structured output.
- Use `-o text` only when jq filtering is not active.
- If you see `--jq is only supported with --output json or --output yaml`,
  rerun the same command with `-o json`. This error usually means jq is active
  via command flag or profile configuration.

## Built-in jq Filtering

- Use `--jq <expression>` on `get` and `list` commands to filter response data.
- `kongctl` uses built-in `gojq` support, so external `jq` is not required.
- Use `--jq` with `-o json` or `-o yaml` output.
- Quote expressions with single quotes to avoid shell parsing issues.
- Because jq can be enabled from config, prefer explicit `-o json` for `get`
  and `list` commands to avoid output-format errors.

```bash
# select key fields from a list response
kongctl get portals -o json --jq 'map({id, name, display_name})'

# return only portal names
kongctl get portals -o json --jq '.[].name'

# pick selected fields from the current user record
kongctl get me -o json --jq '{id, email}'
```

## Failure Handling

- If `kongctl` is missing, request installation and rerun preflight checks.
- If authentication fails, have user run `kongctl login` or set
  `KONGCTL_DEFAULT_KONNECT_PAT`.
- If a command fails with `--jq is only supported with --output json or --output yaml`,
  rerun the command with `-o json`.
- If output format is unexpected, check for env overrides like
  `KONGCTL_DEFAULT_OUTPUT`.
- If access is denied, report the exact command and resource.
- If no resources are found, report an empty result without treating it as an
  execution error.
