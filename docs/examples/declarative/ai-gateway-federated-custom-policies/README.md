# Federated custom definitions

Requires a backend with custom-policy support. Apply `platform.yaml`, then
sync `team-a.yaml` and `team-b.yaml` independently. Team A owns an installed
definition; team B owns a streaming definition. Installed plugin binaries
remain a separate runtime deployment responsibility.

Apply `instance-team.yaml` to instantiate team A's definition from another
namespace. Sync planning with `empty-a.yaml` alone fails while that instance
remains. Sync `empty-a.yaml` and `empty-instance-team.yaml` together to delete
the instance before the definition, retaining team B's streaming definition.
`omitted-a.yaml` leaves definitions untouched. Sync `cleanup.yaml` to delete
the disposable gateway using its platform owner's cascade authority.

See [definition ownership](../../../ai-gateway-custom-policy-ownership.md).
