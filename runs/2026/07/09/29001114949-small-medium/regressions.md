# Declarative Benchmark Regression Report

- Run: [`29001114949`](https://github.com/Kong/kongctl/actions/runs/29001114949)
- Git commit: `bc22fd6`
- Suite duration: `4m59.175s`
- HTTP requests: `492`
- HTTP errors: `0`
- History samples required: `3`

Regressions detected in the latest benchmark run.

| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| `medium-multi-file` | `apply_noop` | duration | 0 | +1306 (+73.2%) | 0 | 0 |
| `medium-single-file` | `apply_noop` | duration | 0 | +1176 (+64.0%) | 0 | 0 |

Inspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.
