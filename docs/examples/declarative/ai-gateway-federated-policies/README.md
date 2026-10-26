# Federated policies

Apply `platform.yaml`, then sync `team-a.yaml` and `team-b.yaml` separately.
Team A owns a reusable sanitizer; team B owns a gateway-wide rate limit.
Global policies affect all teams at runtime while ownership scopes writes.

Apply `model-team.yaml` to attach team A's policy from another namespace.
Sync planning with `empty-a.yaml` fails while that model remains. Sync
`empty-a.yaml` and `empty-model-team.yaml` together to delete the model before
the policy, retaining team B's global policy. `omitted-a.yaml` leaves policies
untouched. Sync `cleanup.yaml` to remove the disposable gateway using its
platform owner's cascade authority.

See [policy ownership](../../../ai-gateway-policy-ownership.md).
