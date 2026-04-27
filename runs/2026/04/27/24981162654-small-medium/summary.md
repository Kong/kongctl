# Declarative Benchmark Results

- Run ID: `24981162654`
- Git commit: `84e1b40`
- Base URL: `https://us.api.konghq.com`
- Duration: `3m31.531s`
- Cases: `12`
- Phases: `24`
- HTTP requests: `492`
- HTTP errors: `0`

Suite duration includes fixture generation and destructive org reset. Phase rows measure only `kongctl apply` commands.

| Case | Phase | Rep | APIs | API documents | Duration | Requests | Responses | Errors |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `small-single-file` | `apply_create` | 1 | 1 | 2 | 628ms | 6 | 6 | 0 |
| `small-single-file` | `apply_noop` | 1 | 1 | 2 | 270ms | 4 | 4 | 0 |
| `small-single-file` | `apply_create` | 2 | 1 | 2 | 651ms | 6 | 6 | 0 |
| `small-single-file` | `apply_noop` | 2 | 1 | 2 | 299ms | 4 | 4 | 0 |
| `small-single-file` | `apply_create` | 3 | 1 | 2 | 610ms | 6 | 6 | 0 |
| `small-single-file` | `apply_noop` | 3 | 1 | 2 | 251ms | 4 | 4 | 0 |
| `small-multi-file` | `apply_create` | 1 | 1 | 2 | 523ms | 6 | 6 | 0 |
| `small-multi-file` | `apply_noop` | 1 | 1 | 2 | 281ms | 4 | 4 | 0 |
| `small-multi-file` | `apply_create` | 2 | 1 | 2 | 647ms | 6 | 6 | 0 |
| `small-multi-file` | `apply_noop` | 2 | 1 | 2 | 241ms | 4 | 4 | 0 |
| `small-multi-file` | `apply_create` | 3 | 1 | 2 | 630ms | 6 | 6 | 0 |
| `small-multi-file` | `apply_noop` | 3 | 1 | 2 | 250ms | 4 | 4 | 0 |
| `medium-single-file` | `apply_create` | 1 | 5 | 25 | 3.964s | 41 | 41 | 0 |
| `medium-single-file` | `apply_noop` | 1 | 5 | 25 | 1.596s | 31 | 31 | 0 |
| `medium-single-file` | `apply_create` | 2 | 5 | 25 | 4.019s | 41 | 41 | 0 |
| `medium-single-file` | `apply_noop` | 2 | 5 | 25 | 1.57s | 31 | 31 | 0 |
| `medium-single-file` | `apply_create` | 3 | 5 | 25 | 3.974s | 41 | 41 | 0 |
| `medium-single-file` | `apply_noop` | 3 | 5 | 25 | 1.537s | 31 | 31 | 0 |
| `medium-multi-file` | `apply_create` | 1 | 5 | 25 | 3.934s | 41 | 41 | 0 |
| `medium-multi-file` | `apply_noop` | 1 | 5 | 25 | 1.501s | 31 | 31 | 0 |
| `medium-multi-file` | `apply_create` | 2 | 5 | 25 | 4.165s | 41 | 41 | 0 |
| `medium-multi-file` | `apply_noop` | 2 | 5 | 25 | 1.505s | 31 | 31 | 0 |
| `medium-multi-file` | `apply_create` | 3 | 5 | 25 | 4.027s | 41 | 41 | 0 |
| `medium-multi-file` | `apply_noop` | 3 | 5 | 25 | 1.624s | 31 | 31 | 0 |
