# Declarative Benchmark Regression Report

- Run: [`26497568949`](https://github.com/Kong/kongctl/actions/runs/26497568949)
- Git commit: `37a1240`
- Suite duration: `4m1.503s`
- HTTP requests: `492`
- HTTP errors: `0`
- History samples required: `3`

Regressions detected in the latest benchmark run.

| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| `medium-multi-file` | `apply_noop` | duration | 0 | +1422 (+87.1%) | 0 | 0 |
| `medium-single-file` | `apply_noop` | duration | 0 | +1280 (+76.4%) | 0 | 0 |

Inspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.
