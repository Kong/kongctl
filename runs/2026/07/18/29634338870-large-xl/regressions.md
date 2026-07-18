# Declarative Benchmark Regression Report

- Run: [`29634338870`](https://github.com/Kong/kongctl/actions/runs/29634338870)
- Git commit: `907f539`
- Suite duration: `17m28.688s`
- HTTP requests: `9624`
- HTTP errors: `0`
- History samples required: `3`

Regressions detected in the latest benchmark run.

| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| `large-multi-file` | `apply_create` | duration | 0 | +4136 (+58.8%) | 0 | 0 |
| `large-multi-file` | `apply_noop` | duration | 0 | +9051 (+89.8%) | 0 | 0 |
| `large-single-file` | `apply_create` | duration | 0 | +4234 (+60.8%) | 0 | 0 |
| `large-single-file` | `apply_noop` | duration | 0 | +8452 (+83.7%) | 0 | 0 |
| `xl-multi-file` | `apply_create` | duration | 0 | +11024 (+53.9%) | 0 | 0 |
| `xl-multi-file` | `apply_noop` | duration | 0 | +27820 (+88.1%) | 0 | 0 |
| `xl-single-file` | `apply_create` | duration | 0 | +11030 (+54.1%) | 0 | 0 |
| `xl-single-file` | `apply_noop` | duration | 0 | +27268 (+86.5%) | 0 | 0 |

Inspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.
