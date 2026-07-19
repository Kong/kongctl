# Declarative Benchmark Regression Report

- Run: [`29677252251`](https://github.com/Kong/kongctl/actions/runs/29677252251)
- Git commit: `f5e065b`
- Suite duration: `5m20.487s`
- HTTP requests: `492`
- HTTP errors: `0`
- History samples required: `3`

Regressions detected in the latest benchmark run.

| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| `medium-multi-file` | `apply_create` | duration | 0 | +766 (+51.5%) | 0 | 0 |
| `medium-multi-file` | `apply_noop` | duration | 0 | +1358 (+87.5%) | 0 | 0 |
| `medium-single-file` | `apply_create` | duration | 0 | +800 (+53.9%) | 0 | 0 |
| `medium-single-file` | `apply_noop` | duration | 0 | +1488 (+93.8%) | 0 | 0 |

Inspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.
