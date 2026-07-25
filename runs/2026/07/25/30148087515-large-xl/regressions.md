# Declarative Benchmark Regression Report

- Run: [`30148087515`](https://github.com/Kong/kongctl/actions/runs/30148087515)
- Git commit: `0cc36c7`
- Suite duration: `15m36.034s`
- HTTP requests: `9624`
- HTTP errors: `0`
- History samples required: `3`

Regressions detected in the latest benchmark run.

| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| `large-multi-file` | `apply_noop` | duration | 0 | +6444 (+63.2%) | 0 | 0 |
| `large-single-file` | `apply_noop` | duration | 0 | +6408 (+62.9%) | 0 | 0 |
| `xl-multi-file` | `apply_noop` | duration | 0 | +19358 (+61.3%) | 0 | 0 |
| `xl-single-file` | `apply_noop` | duration | 0 | +19359 (+61.4%) | 0 | 0 |

Inspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.
