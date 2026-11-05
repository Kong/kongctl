# GitHub Actions authentication

Repository-local automation uses the built-in `GITHUB_TOKEN`, with permissions
declared on each generated job. Agent jobs have read access plus the
`id-token: write` permission needed for Anthropic federation. Safe-output and
conclusion jobs receive the write permissions needed for their operations.
Claude authenticates through GitHub OIDC; it does not need a Copilot token.

## Regenerate agentic workflows

The checked-in workflows use gh-aw v1.0.1. Edit the Markdown sources in
`.github/workflows`, then run the standard compiler and local checks:

```sh
gh aw compile
make test-gh-aw
```

Commit the compiler output unchanged. There are no post-compilation patches,
custom compilation wrappers, or repository-specific compiler version guards.
The Checks workflow tests the selected authentication settings, job permissions,
and the behavior of the benchmark and feature-user publishing hooks.

The source files explicitly set `checkout.github-token`,
`tools.github.github-token`, and `safe-outputs.github-token` to the built-in
`GITHUB_TOKEN`. Claude authenticates to Anthropic through GitHub OIDC.

The v1.0.1 migration moves benchmark comment-target validation and feature-user
replay-prompt hydration into `safe-outputs.steps`. Agent `post-steps` no longer
rewrite compiler-owned output files. Both hooks use `actions/github-script`
so JavaScript templates are not parsed as shell command substitutions by the
safe-output shell harness. The benchmark publishing hook resolves
the scheduled target using the GitHub API and drops comments for any other
issue. The feature-user publishing hook downloads the sanitized artifact and
rechecks the replay prompt before embedding it in the final report. Both use
`GH_AW_AGENT_OUTPUT`, the downloaded output path provided by the compiler.

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

## Credential policy

Enforce the absence of GitHub PATs through repository and organization
configuration. Keep `GH_AW_GITHUB_TOKEN`, `GH_AW_GITHUB_MCP_SERVER_TOKEN`, and
`COPILOT_GITHUB_TOKEN` unset in repository, organization, and environment
Actions secrets. Use the built-in token or scoped App installation tokens for
GitHub operations that need authentication.

Upstream gh-aw still emits optional references to these names in checks,
manifests, reusable declarations, and redaction settings. Missing secrets are
empty, and the checks accept them as unset. The references alone do not create
a PAT dependency. Preserve them as part of the compiler's natural output.
See the upstream [token selection helpers][tokens] and
[activation step generator][activation].

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

[tokens]: https://github.com/github/gh-aw/blob/v1.0.1/pkg/workflow/github_token.go
[activation]: https://github.com/github/gh-aw/blob/v1.0.1/pkg/workflow/compiler_activation_steps.go
