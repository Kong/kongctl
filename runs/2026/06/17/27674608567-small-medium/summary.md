# Declarative Benchmark Results

- Run ID: `27674608567`
- Git commit: `35e06b6`
- Base URL: `https://us.api.konghq.com`
- Duration: `4m35.364s`
- Cases: `12`
- Phases: `24`
- HTTP requests: `492`
- HTTP errors: `0`

Suite duration includes fixture generation and destructive org reset. Phase rows measure only `kongctl apply` commands.

| Case | Phase | Rep | APIs | API documents | Duration | Requests | Responses | Errors |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `small-single-file` | `apply_create` | 1 | 1 | 2 | 741ms | 6 | 6 | 0 |
| `small-single-file` | `apply_noop` | 1 | 1 | 2 | 424ms | 4 | 4 | 0 |
| `small-single-file` | `apply_create` | 2 | 1 | 2 | 714ms | 6 | 6 | 0 |
| `small-single-file` | `apply_noop` | 2 | 1 | 2 | 406ms | 4 | 4 | 0 |
| `small-single-file` | `apply_create` | 3 | 1 | 2 | 694ms | 6 | 6 | 0 |
| `small-single-file` | `apply_noop` | 3 | 1 | 2 | 426ms | 4 | 4 | 0 |
| `small-multi-file` | `apply_create` | 1 | 1 | 2 | 659ms | 6 | 6 | 0 |
| `small-multi-file` | `apply_noop` | 1 | 1 | 2 | 420ms | 4 | 4 | 0 |
| `small-multi-file` | `apply_create` | 2 | 1 | 2 | 622ms | 6 | 6 | 0 |
| `small-multi-file` | `apply_noop` | 2 | 1 | 2 | 436ms | 4 | 4 | 0 |
| `small-multi-file` | `apply_create` | 3 | 1 | 2 | 614ms | 6 | 6 | 0 |
| `small-multi-file` | `apply_noop` | 3 | 1 | 2 | 412ms | 4 | 4 | 0 |
| `medium-single-file` | `apply_create` | 1 | 5 | 25 | 1.858s | 41 | 41 | 0 |
| `medium-single-file` | `apply_noop` | 1 | 5 | 25 | 2.243s | 31 | 31 | 0 |
| `medium-single-file` | `apply_create` | 2 | 5 | 25 | 1.825s | 41 | 41 | 0 |
| `medium-single-file` | `apply_noop` | 2 | 5 | 25 | 2.352s | 31 | 31 | 0 |
| `medium-single-file` | `apply_create` | 3 | 5 | 25 | 1.805s | 41 | 41 | 0 |
| `medium-single-file` | `apply_noop` | 3 | 5 | 25 | 2.232s | 31 | 31 | 0 |
| `medium-multi-file` | `apply_create` | 1 | 5 | 25 | 1.824s | 41 | 41 | 0 |
| `medium-multi-file` | `apply_noop` | 1 | 5 | 25 | 2.328s | 31 | 31 | 0 |
| `medium-multi-file` | `apply_create` | 2 | 5 | 25 | 1.854s | 41 | 41 | 0 |
| `medium-multi-file` | `apply_noop` | 2 | 5 | 25 | 2.41s | 31 | 31 | 0 |
| `medium-multi-file` | `apply_create` | 3 | 5 | 25 | 1.815s | 41 | 41 | 0 |
| `medium-multi-file` | `apply_noop` | 3 | 5 | 25 | 2.291s | 31 | 31 | 0 |
