# Declarative Benchmark Regression Report

- Run: [`25205905043`](https://github.com/Kong/kongctl/actions/runs/25205905043)
- Git commit: `d610c45`
- Suite duration: `3m47.906s`
- HTTP requests: `492`
- HTTP errors: `0`
- History samples required: `3`

Regressions detected in the latest benchmark run.

| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| `medium-multi-file` | `apply_noop` | duration | 0 | +879 (+60.0%) | 0 | 0 |
| `medium-single-file` | `apply_noop` | duration | 0 | +858 (+59.9%) | 0 | 0 |

Inspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.
