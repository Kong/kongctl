# Declarative Benchmark Regression Report

- Run: [`29231441143`](https://github.com/Kong/kongctl/actions/runs/29231441143)
- Git commit: `f73a36f`
- Suite duration: `4m26.259s`
- HTTP requests: `492`
- HTTP errors: `0`
- History samples required: `3`

Regressions detected in the latest benchmark run.

| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| `medium-multi-file` | `apply_create` | duration | 0 | +882 (+54.6%) | 0 | 0 |
| `medium-multi-file` | `apply_noop` | duration | 0 | +1482 (+78.4%) | 0 | 0 |
| `medium-single-file` | `apply_noop` | duration | 0 | +1328 (+69.6%) | 0 | 0 |

Inspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.
