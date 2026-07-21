# Declarative Benchmark Regression Report

- Run: [`29808686075`](https://github.com/Kong/kongctl/actions/runs/29808686075)
- Git commit: `f5e065b`
- Suite duration: `5m31.781s`
- HTTP requests: `492`
- HTTP errors: `0`
- History samples required: `3`

Regressions detected in the latest benchmark run.

| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| `medium-multi-file` | `apply_create` | duration | 0 | +1150 (+73.1%) | 0 | 0 |
| `medium-multi-file` | `apply_noop` | duration | 0 | +1988 (+111.6%) | 0 | 0 |
| `medium-single-file` | `apply_create` | duration | 0 | +1142 (+70.6%) | 0 | 0 |
| `medium-single-file` | `apply_noop` | duration | 0 | +2025 (+116.1%) | 0 | 0 |

Inspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.
