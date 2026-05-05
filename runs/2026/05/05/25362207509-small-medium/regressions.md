# Declarative Benchmark Regression Report

- Run: [`25362207509`](https://github.com/Kong/kongctl/actions/runs/25362207509)
- Git commit: `3d5202d`
- Suite duration: `4m11.006s`
- HTTP requests: `492`
- HTTP errors: `0`
- History samples required: `3`

Regressions detected in the latest benchmark run.

| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| `medium-multi-file` | `apply_noop` | duration | 0 | +1536 (+98.5%) | 0 | 0 |
| `medium-single-file` | `apply_noop` | duration | 0 | +1526 (+98.2%) | 0 | 0 |

Inspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.
