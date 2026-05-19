# Declarative Benchmark Regression Report

- Run: [`26082587826`](https://github.com/Kong/kongctl/actions/runs/26082587826)
- Git commit: `f674722`
- Suite duration: `3m13.95s`
- HTTP requests: `516`
- HTTP errors: `0`
- History samples required: `3`

Regressions detected in the latest benchmark run.

| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| `small-multi-file` | `apply_create` | requests | +1 (+16.7%) | -76 (-13.5%) | 0 | 0 |
| `small-multi-file` | `apply_noop` | requests | +1 (+25.0%) | -9 (-2.9%) | 0 | 0 |
| `small-single-file` | `apply_create` | requests | +1 (+16.7%) | -127 (-18.5%) | 0 | 0 |
| `small-single-file` | `apply_noop` | requests | +1 (+25.0%) | -93 (-24.0%) | 0 | 0 |

Inspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.
