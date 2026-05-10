# Declarative Benchmark Results

- Run ID: `25622433683`
- Git commit: `9dc7aa2`
- Base URL: `https://us.api.konghq.com`
- Duration: `4m25.007s`
- Cases: `12`
- Phases: `24`
- HTTP requests: `492`
- HTTP errors: `0`

Suite duration includes fixture generation and destructive org reset. Phase rows measure only `kongctl apply` commands.

| Case | Phase | Rep | APIs | API documents | Duration | Requests | Responses | Errors |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `small-single-file` | `apply_create` | 1 | 1 | 2 | 1.153s | 6 | 6 | 0 |
| `small-single-file` | `apply_noop` | 1 | 1 | 2 | 634ms | 4 | 4 | 0 |
| `small-single-file` | `apply_create` | 2 | 1 | 2 | 906ms | 6 | 6 | 0 |
| `small-single-file` | `apply_noop` | 2 | 1 | 2 | 538ms | 4 | 4 | 0 |
| `small-single-file` | `apply_create` | 3 | 1 | 2 | 1.092s | 6 | 6 | 0 |
| `small-single-file` | `apply_noop` | 3 | 1 | 2 | 557ms | 4 | 4 | 0 |
| `small-multi-file` | `apply_create` | 1 | 1 | 2 | 959ms | 6 | 6 | 0 |
| `small-multi-file` | `apply_noop` | 1 | 1 | 2 | 638ms | 4 | 4 | 0 |
| `small-multi-file` | `apply_create` | 2 | 1 | 2 | 937ms | 6 | 6 | 0 |
| `small-multi-file` | `apply_noop` | 2 | 1 | 2 | 544ms | 4 | 4 | 0 |
| `small-multi-file` | `apply_create` | 3 | 1 | 2 | 972ms | 6 | 6 | 0 |
| `small-multi-file` | `apply_noop` | 3 | 1 | 2 | 544ms | 4 | 4 | 0 |
| `medium-single-file` | `apply_create` | 1 | 5 | 25 | 5.951s | 41 | 41 | 0 |
| `medium-single-file` | `apply_noop` | 1 | 5 | 25 | 2.973s | 31 | 31 | 0 |
| `medium-single-file` | `apply_create` | 2 | 5 | 25 | 5.903s | 41 | 41 | 0 |
| `medium-single-file` | `apply_noop` | 2 | 5 | 25 | 3.023s | 31 | 31 | 0 |
| `medium-single-file` | `apply_create` | 3 | 5 | 25 | 6.024s | 41 | 41 | 0 |
| `medium-single-file` | `apply_noop` | 3 | 5 | 25 | 3.045s | 31 | 31 | 0 |
| `medium-multi-file` | `apply_create` | 1 | 5 | 25 | 5.967s | 41 | 41 | 0 |
| `medium-multi-file` | `apply_noop` | 1 | 5 | 25 | 2.996s | 31 | 31 | 0 |
| `medium-multi-file` | `apply_create` | 2 | 5 | 25 | 6.147s | 41 | 41 | 0 |
| `medium-multi-file` | `apply_noop` | 2 | 5 | 25 | 3.026s | 31 | 31 | 0 |
| `medium-multi-file` | `apply_create` | 3 | 5 | 25 | 6.147s | 41 | 41 | 0 |
| `medium-multi-file` | `apply_noop` | 3 | 5 | 25 | 3.042s | 31 | 31 | 0 |
