# Federated remaining AI Gateway children

This example shares a gateway between two namespaces for MCP servers, vaults,
consumers, groups, CA certificates, and SNIs. The platform owns the gateway
and an ordinary runtime certificate; runtime certificate federation is deferred.
See the [ownership and API limits][ownership].

Omit `certificates` from team manifests; an empty collection is not team-scoped.
See the [runtime certificate limitation][tls].

[ownership]: ../../../ai-gateway-remaining-child-ownership.md
[tls]: ../../../ai-gateway-federation.md#deferred-operations-and-api-limitations

Run from this directory with an ephemeral Konnect profile. Generate disposable
certificate material locally (never commit the generated keys):

```sh
mkdir -p certs
openssl req -x509 -nodes -newkey rsa:2048 -days 1 \
  -subj /CN=federated-remaining.example.test \
  -keyout certs/runtime.key -out certs/runtime.pem
openssl req -x509 -nodes -newkey ec -pkeyopt ec_paramgen_curve:P-256 -days 1 \
  -subj /CN=federated-remaining-alt.example.test \
  -keyout certs/runtime-alt.key -out certs/runtime-alt.pem
kongctl sync -f platform.yaml --auto-approve
kongctl sync -f initial-a.yaml --auto-approve
kongctl sync -f team-b.yaml --auto-approve
kongctl sync -f team-a.yaml --auto-approve
kongctl plan -f team-a.yaml -f team-b.yaml --mode sync
```

The last plan has no changes. Team A's group includes Team B's consumer and
Team B's group includes Team A's consumer. This changes membership only, never
consumer ownership. Create Team A's consumers before Team B's group.

```sh
kongctl sync -f empty-a.yaml --auto-approve
kongctl plan -f team-b.yaml --mode sync
```

Only Team A's six children are deleted. The surviving group's declaration
still requests the deleted consumer, so restore that consumer or remove the
membership reference before syncing Team B again. Omitting a collection
instead of explicitly clearing it preserves that collection.

Delete the disposable gateway through its platform owner when finished. The
cascade includes Team B's children; read the warnings:

```sh
kongctl sync -f cleanup.yaml --auto-approve
```

Remove generated local certificate files after cleanup.
