# Declarative Benchmark Regression Report

- Run: [`25272519446`](https://github.com/Kong/kongctl/actions/runs/25272519446)
- Git commit: `9382bb3`
- Suite duration: `4m21.018s`
- HTTP requests: `492`
- HTTP errors: `0`
- History samples required: `3`

Regressions detected in the latest benchmark run.

| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| `medium-multi-file` | `apply_noop` | duration | 0 | +1485 (+98.8%) | 0 | 0 |
| `medium-single-file` | `apply_noop` | duration | 0 | +1488 (+99.4%) | 0 | 0 |

Inspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.
