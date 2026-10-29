# Federated AI Gateway agents

Use a disposable Konnect organization and configured profile:

```sh
kongctl sync -f platform.yaml --auto-approve
kongctl sync -f team-a.yaml --auto-approve
kongctl sync -f team-b.yaml --auto-approve
kongctl plan -f team-a.yaml -f team-b.yaml --mode sync
kongctl sync -f empty-a.yaml --auto-approve
kongctl sync -f cleanup.yaml --auto-approve
```

The two agent types belong to separate namespaces and reference platform-owned
policy/auth resources. Removing team A leaves team B and the platform intact.
The final command deletes the disposable gateway and all descendants.
See [ownership and migration](../../../ai-gateway-agent-ownership.md).
