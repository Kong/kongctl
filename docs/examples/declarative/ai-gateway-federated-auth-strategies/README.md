# Federated auth strategies

Apply `platform.yaml`, then sync `team-a.yaml` and `team-b.yaml` separately.
Set the four `FEDERATED_OIDC_*` environment variables referenced by team B
before creating its OIDC strategy. These values are deferred secret sources;
explicit later rotation requires `--write-secret` or `--write-secrets`.

Apply `model-team.yaml` to attach team A's strategy from another namespace.
Planning sync with `empty-a.yaml` alone fails while that model is retained.
Sync `empty-a.yaml` and `empty-model-team.yaml` together to delete the model
before the strategy, while retaining team B. `omitted-a.yaml` leaves the
strategy collection untouched. Sync `cleanup.yaml` to delete the disposable
gateway using the platform owner's cascade authority.

These manifests create disposable resources. See the
[ownership guide](../../../ai-gateway-auth-strategy-ownership.md).
