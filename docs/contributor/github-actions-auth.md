# GitHub Actions authentication

Repository-local automation uses the built-in `GITHUB_TOKEN`, with permissions
declared on each generated job. Agent jobs have read access plus the
`id-token: write` permission needed for Anthropic federation. Safe-output and
conclusion jobs receive the write permissions needed for their operations.
Claude authenticates through GitHub OIDC; it does not need a Copilot token.

## Regenerate agentic workflows

Install gh-aw v0.86.2, edit the Markdown sources in `.github/workflows`, then
run:

```sh
make compile-gh-aw
make test-gh-aw
```

`GH_AW_BIN=/absolute/path/to/gh-aw make compile-gh-aw` can select a standalone
compiler binary. The command compiles all eight workflows, applies the
repository's maintenance overlay, and removes unused credential probes and
metadata. Run this command instead of committing raw `gh aw compile` output.
The required Checks workflow rejects obsolete secret references and missing
overlays, including changes from automated updates.

The source files explicitly set `checkout.github-token`,
`tools.github.github-token`, and `safe-outputs.github-token` to the built-in
token. These supported settings remove the optional PAT fallback chains.
The compiler still unconditionally emits probes for `GH_AW_GITHUB_TOKEN`,
`GH_AW_GITHUB_MCP_SERVER_TOKEN`, and `COPILOT_GITHUB_TOKEN`. Both v0.86.2 and
v1.0.1 emit these probes; upgrading alone does not remove the references.
See the upstream [token selection helpers][tokens] and
[activation step generator][activation].

`scripts/patch-gh-aw-tokens.py` handles those compiler-injected references.
It removes the unused OAuth checks, optional policy environment variables,
reusable secret declarations, redaction entries, and manifest entries. The
daily credit guardrail receives `GITHUB_TOKEN`, which its runtime supports
directly. The overlay preserves the activation output consumed by failure
handlers and leaves Konnect secret redaction intact. Unexpected remaining
consumers or a different compiler version fail regeneration and require
review. Review this overlay when upgrading gh-aw; remove it once upstream
offers equivalent generation without the obsolete probes.

## GitHub Apps

Keep the existing scoped installation tokens for private SDK reads, SDK
dependency PRs, installer publication, and Homebrew publication. See
[Homebrew publication](homebrew.md#stable-release-sequence) for credential
setup.
The App credentials are `API_OPS_APP_CLIENT_ID` / `API_OPS_APP_SECRET` and
`KONGCTL_RELEASE_APP_CLIENT_ID` / `KONGCTL_RELEASE_APP_PRIVATE_KEY`.

GitHub suppresses most workflow events caused by `GITHUB_TOKEN`. Repository
issue creation, comments, labels, discussions, release-note updates, and
failure reporting work with the declared permissions, but must not depend on
those writes triggering another workflow. If a new automation requires such
an event chain, access to another private repository, or organization APIs,
request an appropriately scoped App installation from the organization admins.
Do not add a PAT. The existing SDK PR and publication Apps preserve their
downstream workflow triggers.

## Obsolete Actions secrets

After merging this cleanup, remove `GH_AW_GITHUB_MCP_SERVER_TOKEN` and
`COPILOT_GITHUB_TOKEN`. Remove `GH_AW_GITHUB_TOKEN` wherever configured; it
was absent from repository secret metadata during the audit.

No checked-in Actions consumer references the older `PAT`,
`GH_TOKEN_PRIVATE_READ`, `GET_KONGHQ_COM_TOKEN`, or `TAP_GITHUB_TOKEN`
secrets. The similarly named environment variable passed to
`scripts/setup-private-sdk.sh` receives an App installation token from the
prerelease workflow, so it does not need the old Actions secret. These four
secrets can also be removed from this repository after checking consumers
outside the repository. Organization secret visibility requires an admin
audit; repository metadata alone cannot establish their wider use or match
secret values to account tokens.

Keep `KONGCTL_E2E_KONNECT_PAT`, `KONGCTL_BENCHMARK_KONNECT_PAT`, and
`KONGCTL_FEATURE_USER_AGENT_KONNECT_PAT`. They authenticate to Konnect.

[tokens]: https://github.com/github/gh-aw/blob/v0.86.2/pkg/workflow/github_token.go
[activation]: https://github.com/github/gh-aw/blob/v1.0.1/pkg/workflow/compiler_activation_steps.go
