# Declarative Benchmark Regression Report

- Run: [`27492256576`](https://github.com/Kong/kongctl/actions/runs/27492256576)
- Git commit: `d137e01`
- Suite duration: `4m29.769s`
- HTTP requests: `492`
- HTTP errors: `0`
- History samples required: `3`

Regressions detected in the latest benchmark run.

| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| `medium-single-file` | `apply_noop` | duration | 0 | +1084 (+59.5%) | 0 | 0 |

Inspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.
