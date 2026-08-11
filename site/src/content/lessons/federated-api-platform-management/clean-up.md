---
title: Clean Up
summary: Stop the local data plane and remove the federated AI Gateway.
order: 10
related:
  - label: Declarative delete documentation
    url: https://developer.konghq.com/kongctl/declarative/
---

## Goal

You will stop the local data plane and delete the resources created throughout
this chapter.

## Stop the data plane

Stop the Docker container before removing the AI Gateway it is connected to:

```shell label="Stop the data plane"
docker stop federated-aigw-dp
```

The container was started with `--rm`, so Docker removes it after it stops. The
certificate and private key remain in `platform/certs`.

## Delete the AI Gateway

The Platform file describes the data plane certificate, model provider, and
AI Gateway. Use it as the input configuration for the delete:

```shell label="Delete the AI Gateway"
kongctl delete -f platform/ai-gateway.yaml
```

Review the plan before confirming it. Deleting `platform-aigw` also removes the
models attached to the shared AI Gateway.

The local configuration files are left in place so you can review or reuse
the example.
