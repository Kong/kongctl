# Federated providers

Set `OPENAI_AUTHORIZATION` to a provider credential outside the manifest.
Create the shared gateway, then sync each team's file independently:

```sh
kongctl apply -f platform.yaml --auto-approve
kongctl sync -f team-a.yaml --auto-approve
kongctl sync -f team-b.yaml --auto-approve
kongctl plan -f team-a.yaml -f team-b.yaml --mode sync
```

Sync `empty-a.yaml` to remove only team A's providers. A retained model
referencing one of them will block deletion; detach its references first.
Do not use these example names against production resources.

See [provider ownership](../../../ai-gateway-provider-ownership.md) for
explicit adoption, migration, protection, and credential constraints.
