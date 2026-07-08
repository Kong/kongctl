# Declarative Benchmark Regression Report

- Run: [`28923718258`](https://github.com/Kong/kongctl/actions/runs/28923718258)
- Git commit: `1cd7ff1`
- Suite duration: `5m11.364s`
- HTTP requests: `492`
- HTTP errors: `0`
- History samples required: `3`

Regressions detected in the latest benchmark run.

| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| `medium-multi-file` | `apply_create` | duration | 0 | +826 (+53.4%) | 0 | 0 |
| `medium-multi-file` | `apply_noop` | duration | 0 | +1426 (+80.5%) | 0 | 0 |
| `medium-single-file` | `apply_create` | duration | 0 | +854 (+54.2%) | 0 | 0 |
| `medium-single-file` | `apply_noop` | duration | 0 | +1410 (+77.8%) | 0 | 0 |

Inspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.
