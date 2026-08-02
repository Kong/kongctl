# Declarative Benchmark Regression Report

- Run: [`30736881642`](https://github.com/Kong/kongctl/actions/runs/30736881642)
- Git commit: `a6d3238`
- Suite duration: `5m28.445s`
- HTTP requests: `492`
- HTTP errors: `0`
- History samples required: `3`

Regressions detected in the latest benchmark run.

| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| `medium-multi-file` | `apply_noop` | duration | 0 | +1180 (+60.5%) | 0 | 0 |
| `medium-single-file` | `apply_noop` | duration | 0 | +1442 (+81.5%) | 0 | 0 |

Inspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.
