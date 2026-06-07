# Declarative Benchmark Regression Report

- Run: [`27086204844`](https://github.com/Kong/kongctl/actions/runs/27086204844)
- Git commit: `cb915fa`
- Suite duration: `4m2.071s`
- HTTP requests: `492`
- HTTP errors: `0`
- History samples required: `3`

Regressions detected in the latest benchmark run.

| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| `medium-multi-file` | `apply_create` | duration | 0 | +768 (+50.1%) | 0 | 0 |
| `medium-multi-file` | `apply_noop` | duration | 0 | +1307 (+70.9%) | 0 | 0 |
| `medium-single-file` | `apply_noop` | duration | 0 | +1177 (+60.4%) | 0 | 0 |

Inspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.
