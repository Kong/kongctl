# Declarative Benchmark Regression Report

- Run: [`30611607783`](https://github.com/Kong/kongctl/actions/runs/30611607783)
- Git commit: `e17979e`
- Suite duration: `5m27.621s`
- HTTP requests: `492`
- HTTP errors: `0`
- History samples required: `3`

Regressions detected in the latest benchmark run.

| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| `medium-multi-file` | `apply_noop` | duration | 0 | +1310 (+67.2%) | 0 | 0 |
| `medium-single-file` | `apply_noop` | duration | 0 | +1590 (+89.8%) | 0 | 0 |

Inspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.
