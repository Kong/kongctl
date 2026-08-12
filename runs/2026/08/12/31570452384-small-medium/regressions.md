# Declarative Benchmark Regression Report

- Run: [`31570452384`](https://github.com/Kong/kongctl/actions/runs/31570452384)
- Git commit: `e9b8f0b`
- Suite duration: `5m20.239s`
- HTTP requests: `492`
- HTTP errors: `0`
- History samples required: `3`

Regressions detected in the latest benchmark run.

| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| `medium-multi-file` | `apply_noop` | duration | 0 | +1230 (+69.4%) | 0 | 0 |
| `medium-single-file` | `apply_noop` | duration | 0 | +1513 (+88.5%) | 0 | 0 |

Inspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.
