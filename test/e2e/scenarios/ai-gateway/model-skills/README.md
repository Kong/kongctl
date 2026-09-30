# AI Gateway 2.2: model-skills

The skills capability on an Anthropic-backed API model. The update adds
the files capability alongside skills and verifies both persist.

Each scenario creates an isolated gateway, provider, and model; verifies
list and model-detail responses; updates provider/model metadata and feature
configuration; dumps and reloads without changes; and deletes the children
and gateway. Read-back assertions use bounded polling. Mutation commands
are not retried, and plans assert exact create/update/delete counts.

These are management API tests. Provider authentication uses an empty basic
header list, and no requests are sent to an upstream provider. No provider
credentials or running data plane are required. Inference, skills execution,
and runtime conversion are not validated.

The suite's default stable classification applies: unsupported backend
features fail CI. These new scenarios are not replay-enabled.

Run with:

```sh
make test-e2e-scenarios SCENARIO=ai-gateway/model-skills
```
