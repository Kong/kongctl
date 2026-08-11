# Declarative Benchmark Regression Report

- Run: [`31464845506`](https://github.com/Kong/kongctl/actions/runs/31464845506)
- Git commit: `cb57896`
- Suite duration: `5m10.77s`
- HTTP requests: `492`
- HTTP errors: `0`
- History samples required: `3`

Regressions detected in the latest benchmark run.

| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| `medium-multi-file` | `apply_create` | duration | 0 | +768 (+51.1%) | 0 | 0 |
| `medium-multi-file` | `apply_noop` | duration | 0 | +1326 (+79.2%) | 0 | 0 |
| `medium-single-file` | `apply_noop` | duration | 0 | +1352 (+84.0%) | 0 | 0 |

Inspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.
