# Declarative Benchmark Results

- Run ID: `29312539064`
- Git commit: `944c474`
- Base URL: `https://us.api.konghq.com`
- Duration: `4m18.565s`
- Cases: `12`
- Phases: `24`
- HTTP requests: `492`
- HTTP errors: `0`

Suite duration includes fixture generation and destructive org reset. Phase rows measure only `kongctl apply` commands.

| Case | Phase | Rep | APIs | API documents | Duration | Requests | Responses | Errors |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `small-single-file` | `apply_create` | 1 | 1 | 2 | 708ms | 6 | 6 | 0 |
| `small-single-file` | `apply_noop` | 1 | 1 | 2 | 335ms | 4 | 4 | 0 |
| `small-single-file` | `apply_create` | 2 | 1 | 2 | 552ms | 6 | 6 | 0 |
| `small-single-file` | `apply_noop` | 2 | 1 | 2 | 350ms | 4 | 4 | 0 |
| `small-single-file` | `apply_create` | 3 | 1 | 2 | 505ms | 6 | 6 | 0 |
| `small-single-file` | `apply_noop` | 3 | 1 | 2 | 296ms | 4 | 4 | 0 |
| `small-multi-file` | `apply_create` | 1 | 1 | 2 | 559ms | 6 | 6 | 0 |
| `small-multi-file` | `apply_noop` | 1 | 1 | 2 | 295ms | 4 | 4 | 0 |
| `small-multi-file` | `apply_create` | 2 | 1 | 2 | 523ms | 6 | 6 | 0 |
| `small-multi-file` | `apply_noop` | 2 | 1 | 2 | 294ms | 4 | 4 | 0 |
| `small-multi-file` | `apply_create` | 3 | 1 | 2 | 529ms | 6 | 6 | 0 |
| `small-multi-file` | `apply_noop` | 3 | 1 | 2 | 267ms | 4 | 4 | 0 |
| `medium-single-file` | `apply_create` | 1 | 5 | 25 | 1.437s | 41 | 41 | 0 |
| `medium-single-file` | `apply_noop` | 1 | 5 | 25 | 1.452s | 31 | 31 | 0 |
| `medium-single-file` | `apply_create` | 2 | 5 | 25 | 1.424s | 41 | 41 | 0 |
| `medium-single-file` | `apply_noop` | 2 | 5 | 25 | 1.5s | 31 | 31 | 0 |
| `medium-single-file` | `apply_create` | 3 | 5 | 25 | 1.322s | 41 | 41 | 0 |
| `medium-single-file` | `apply_noop` | 3 | 5 | 25 | 1.45s | 31 | 31 | 0 |
| `medium-multi-file` | `apply_create` | 1 | 5 | 25 | 1.406s | 41 | 41 | 0 |
| `medium-multi-file` | `apply_noop` | 1 | 5 | 25 | 1.552s | 31 | 31 | 0 |
| `medium-multi-file` | `apply_create` | 2 | 5 | 25 | 1.385s | 41 | 41 | 0 |
| `medium-multi-file` | `apply_noop` | 2 | 5 | 25 | 1.476s | 31 | 31 | 0 |
| `medium-multi-file` | `apply_create` | 3 | 5 | 25 | 1.364s | 41 | 41 | 0 |
| `medium-multi-file` | `apply_noop` | 3 | 5 | 25 | 1.483s | 31 | 31 | 0 |
