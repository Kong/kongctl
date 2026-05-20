# Declarative Benchmark Regression Report

- Run: [`26147784648`](https://github.com/Kong/kongctl/actions/runs/26147784648)
- Git commit: `95a6b20`
- Suite duration: `3m18.667s`
- HTTP requests: `516`
- HTTP errors: `0`
- History samples required: `3`

Regressions detected in the latest benchmark run.

| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| `small-multi-file` | `apply_create` | requests | +1 (+16.7%) | +57 (+11.2%) | 0 | 0 |
| `small-multi-file` | `apply_noop` | requests | +1 (+25.0%) | +44 (+14.6%) | 0 | 0 |
| `small-single-file` | `apply_create` | requests | +1 (+16.7%) | -18 (-2.9%) | 0 | 0 |
| `small-single-file` | `apply_noop` | requests | +1 (+25.0%) | +87 (+27.0%) | 0 | 0 |

Inspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.
