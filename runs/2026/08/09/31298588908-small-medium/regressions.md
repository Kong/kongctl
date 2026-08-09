# Declarative Benchmark Regression Report

- Run: [`31298588908`](https://github.com/Kong/kongctl/actions/runs/31298588908)
- Git commit: `4c91afc`
- Suite duration: `4m29.091s`
- HTTP requests: `492`
- HTTP errors: `0`
- History samples required: `3`

Regressions detected in the latest benchmark run.

| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| `medium-multi-file` | `apply_noop` | duration | 0 | +1210 (+71.0%) | 0 | 0 |
| `medium-single-file` | `apply_noop` | duration | 0 | +1316 (+80.2%) | 0 | 0 |

Inspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.
