# Declarative Benchmark Regression Report

- Run: [`29183452121`](https://github.com/Kong/kongctl/actions/runs/29183452121)
- Git commit: `f73a36f`
- Suite duration: `4m31.019s`
- HTTP requests: `492`
- HTTP errors: `0`
- History samples required: `3`

Regressions detected in the latest benchmark run.

| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| `medium-single-file` | `apply_noop` | duration | 0 | +1094 (+58.7%) | 0 | 0 |

Inspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.
