# Federated providers

Set `OPENAI_AUTHORIZATION` to a provider credential outside the manifest.
Create the shared gateway, then sync each team's file independently:

```sh
kongctl plan -f platform.yaml --mode apply
kongctl apply -f platform.yaml --auto-approve
kongctl sync -f team-a.yaml --auto-approve
kongctl sync -f team-b.yaml --auto-approve
kongctl plan -f team-a.yaml -f team-b.yaml --mode sync
```

Match the preview mode to the next operation: `--mode apply` before `apply`,
and `--mode sync` before `sync`. The default plan mode is `sync`.

Sync `empty-a.yaml` to remove only team A's providers. A retained model
referencing one of them will block deletion; detach its references first.
Apply `model-team.yaml` to attach a model owned by a third namespace to
team A's provider. A direct sync of `empty-a.yaml` then fails. To remove
both in dependency order across gateway aliases:

```sh
kongctl sync -f empty-a.yaml -f empty-model-team.yaml --auto-approve
```

Do not use these example names against production resources.

See [provider ownership](../../../ai-gateway-provider-ownership.md) for
explicit adoption, migration, protection, and credential constraints.
