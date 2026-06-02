# Declarative Benchmark Regression Report

- Run: [`26806048307`](https://github.com/Kong/kongctl/actions/runs/26806048307)
- Git commit: `875e23a`
- Suite duration: `3m7.243s`
- HTTP requests: `492`
- HTTP errors: `0`
- History samples required: `3`

Regressions detected in the latest benchmark run.

| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| `medium-multi-file` | `apply_noop` | duration | 0 | +1356 (+81.5%) | 0 | 0 |
| `medium-single-file` | `apply_noop` | duration | 0 | +1226 (+66.8%) | 0 | 0 |

Inspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.
