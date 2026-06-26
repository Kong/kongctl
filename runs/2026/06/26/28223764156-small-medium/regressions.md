# Declarative Benchmark Regression Report

- Run: [`28223764156`](https://github.com/Kong/kongctl/actions/runs/28223764156)
- Git commit: `e7c07c0`
- Suite duration: `4m24.831s`
- HTTP requests: `492`
- HTTP errors: `0`
- History samples required: `3`

Regressions detected in the latest benchmark run.

| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| `medium-single-file` | `apply_noop` | duration | 0 | +1608 (+92.8%) | 0 | 0 |

Inspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.
