# Declarative Benchmark Regression Report

- Run: [`28698464020`](https://github.com/Kong/kongctl/actions/runs/28698464020)
- Git commit: `71234c9`
- Suite duration: `16m30.483s`
- HTTP requests: `9624`
- HTTP errors: `0`
- History samples required: `3`

Regressions detected in the latest benchmark run.

| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| `large-multi-file` | `apply_noop` | duration | 0 | +7422 (+70.8%) | 0 | 0 |
| `large-single-file` | `apply_noop` | duration | 0 | +7658 (+74.1%) | 0 | 0 |
| `xl-multi-file` | `apply_noop` | duration | 0 | +23480 (+73.0%) | 0 | 0 |
| `xl-single-file` | `apply_noop` | duration | 0 | +24186 (+75.2%) | 0 | 0 |

Inspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.
