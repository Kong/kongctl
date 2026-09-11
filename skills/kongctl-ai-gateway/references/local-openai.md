# Local OpenAI deployment

Use the installed schema as the field contract. The bundled manifest and
helper adapt the native [OpenAI quickstart][quickstart] and use the
`kong/kong-ai-gateway:2.0.3` image from the AI Gateway 2.0 examples. Select and
record a compatible image for the user's environment; pin its digest for a
rehearsed deployment.

## Prepare the project before authentication

Copy `assets/openai/ai-gateway.yaml` and `assets/openai/data-plane.sh` from
this skill into the project root. Adapt their names to the user's project.
The helper is invoked with `bash`; it need not have executable permissions.
Provide a README with the chosen profile/region, inputs and commands below.
Create `.gitignore` before generating keys:

```gitignore
.env
.env.*
!.env.example
certs/*.key
.plans/
.artifacts/
```

An `.env.example` may contain empty values and descriptions. Do not assume
kongctl automatically loads `.env`; document how the shell supplies inputs.

Required inputs and tools:

| Input | When needed | Purpose |
| --- | --- | --- |
| `OPENAI_MODEL` | Before loading/planning | Available upstream model ID |
| `OPENAI_API_KEY` | At apply | OpenAI provider credential |
| Konnect login or PAT | Plan, apply, discovery | Intended org and region |
| Docker and OpenSSL | Local data plane | Container and mTLS key pair |

`demo-chat` is the client-facing alias; `OPENAI_MODEL` is its upstream
target. Keep the alias stable when changing provider models. For CI, prefer
committing the selected, non-secret model IDs to YAML so review shows them.

Generate the local certificate pair once:

```sh
bash data-plane.sh certs
```

Only `certs/data-plane.crt` goes into the declarative manifest. The private
key stays on the data plane host. This certificate authenticates the data
plane to Konnect; it is distinct from a proxy's public HTTPS certificate.
The helper checks and reuses an existing pair. It refuses mismatched or
incomplete pairs without replacing either file. Run
`bash data-plane.sh check` to check a pair without starting Docker.

By default the key is `certs/data-plane.key`. For an existing key outside
the checkout, set `AIGW_DATA_PLANE_KEY` to its absolute path on the laptop.
The helper mounts that file separately from the public certificate. Keep
the key readable by its runtime group (the generated key uses mode 0640);
the container receives that file's group ID. Preserve this path setting
when handing the project over to CI/CD.

For a files-only request, generating a local pair is optional; leaving the
explicit generation command is sufficient. Explain that resolving the
manifest's `!file` requires that certificate. The helper's Docker group and
permissions follow the Linux example; adapt them if the target host differs.

## Plan and provision when authorized

Select the user's Konnect profile and region and use them consistently.
Authenticate with `kongctl login` or supply the profile's PAT environment
variable. For the default profile it is `KONGCTL_DEFAULT_KONNECT_PAT`.
Keep the token out of command arguments and logs.

Use the saved apply-plan sequence in `SKILL.md`, including the project's
namespace guard. `OPENAI_MODEL` must already be set; `OPENAI_API_KEY` is
deferred until execution. Inspect the plan's resource scope before applying.
Do not add `--write-secrets` to routine runs: new resources write their
configured secrets once; subsequent secret rotation is an explicit change.

## Discover endpoints and run locally

After apply, retrieve the gateway using its actual display name. Add the
same profile and region/base URL flags used during provisioning:

```sh
AIGW_CONTROL_PLANE="$(kongctl get ai-gateway 'AI Demo' -o json \
  --jq '.endpoints.configuration | sub("^https://"; "") |
    sub(":443$"; "")' --jq-raw-output)"
AIGW_TELEMETRY="$(kongctl get ai-gateway 'AI Demo' -o json \
  --jq '.endpoints.telemetry | sub("^https://"; "") |
    sub(":443$"; "")' --jq-raw-output)"
export AIGW_CONTROL_PLANE AIGW_TELEMETRY
bash data-plane.sh run
kongctl get ai-gateway nodes --gateway-name 'AI Demo'
```

Check that both results are nonempty real hostnames, not `null`, before
starting Docker. Do not synthesize endpoints from a gateway ID or region.
The helper mounts the pair read-only, configures both Konnect channels,
and binds the unauthenticated demo proxy to loopback. The provider key is
sent through declarative apply; it is not a client request header or a
required Docker environment variable in this example.

Allow a bounded interval for the node to connect and configuration to
propagate. Inspect its status and `docker logs ai-demo-data-plane` if it
does not. Do not use repeated paid chat requests as a readiness loop.

## Prove inference

Send an OpenAI-format chat request through the local gateway using the
configured alias and path:

```sh
curl --fail-with-body --silent --show-error --max-time 60 \
  http://127.0.0.1:8000/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"demo-chat","messages":[
    {"role":"user","content":"Reply with a short greeting."}]}'
```

Verify a successful HTTP response with a nonempty assistant completion
(for this text example, `choices[0].message.content`). Report the request
path, alias and observed result without logging credentials. A non-404
response or a running container is not this check. Use one bounded request
per smoke test unless the user authorizes a larger traffic test.

| Symptom | Check before changing configuration |
| --- | --- |
| No node connection | Discovered endpoints, cert/key match, read access |
| Connection fails | Docker logs, DNS and outbound TLS to both channels |
| Proxy 404 | Applied config, `/v1` route, unique matching body alias |
| Proxy 401 | Caller key/auth strategy; distinguish upstream auth error |
| Proxy 403 after auth | Model ACL names and caller/group membership |
| Provider error | Model availability, key, upstream quota and response |
| Success without content | Response format and actual completion body |

Keep diagnostic logs local and inspect them for credentials before sharing.
If the provider rejects the request, stop and fix that cause; switching
models or weakening access controls silently is not a valid verification.

## Cleanup and repeatability

Stop only this project's container with `bash data-plane.sh stop`. This
does not remove Konnect resources. For requested teardown, generate a
`kongctl plan --mode delete -f ai-gateway.yaml` with the same namespace
guard and target, save and inspect it using `diff --plan`, then execute
`kongctl delete --plan` after the intended removals are authorized.
Deletion of a gateway also affects its children; inspect that scope.

Keep the certificate pair for reuse until reset is deliberate. For an
unchanged deployment, generate another apply-mode plan and inspect its
actions. Do not claim idempotence merely because apply exited successfully.

[quickstart]:
  https://github.com/Kong/kongctl/tree/main/docs/examples/declarative/ai-gateway/openai-llm
