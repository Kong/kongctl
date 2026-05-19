# Declarative Benchmark Results

- Run ID: `26082587826`
- Git commit: `f674722`
- Base URL: `https://us.api.konghq.com`
- Duration: `3m13.95s`
- Cases: `12`
- Phases: `24`
- HTTP requests: `516`
- HTTP errors: `0`

Suite duration includes fixture generation and destructive org reset. Phase rows measure only `kongctl apply` commands.

| Case | Phase | Rep | APIs | API documents | Duration | Requests | Responses | Errors |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `small-single-file` | `apply_create` | 1 | 1 | 2 | 733ms | 7 | 7 | 0 |
| `small-single-file` | `apply_noop` | 1 | 1 | 2 | 288ms | 5 | 5 | 0 |
| `small-single-file` | `apply_create` | 2 | 1 | 2 | 559ms | 7 | 7 | 0 |
| `small-single-file` | `apply_noop` | 2 | 1 | 2 | 303ms | 5 | 5 | 0 |
| `small-single-file` | `apply_create` | 3 | 1 | 2 | 496ms | 7 | 7 | 0 |
| `small-single-file` | `apply_noop` | 3 | 1 | 2 | 294ms | 5 | 5 | 0 |
| `small-multi-file` | `apply_create` | 1 | 1 | 2 | 485ms | 7 | 7 | 0 |
| `small-multi-file` | `apply_noop` | 1 | 1 | 2 | 305ms | 5 | 5 | 0 |
| `small-multi-file` | `apply_create` | 2 | 1 | 2 | 494ms | 7 | 7 | 0 |
| `small-multi-file` | `apply_noop` | 2 | 1 | 2 | 308ms | 5 | 5 | 0 |
| `small-multi-file` | `apply_create` | 3 | 1 | 2 | 464ms | 7 | 7 | 0 |
| `small-multi-file` | `apply_noop` | 3 | 1 | 2 | 301ms | 5 | 5 | 0 |
| `medium-single-file` | `apply_create` | 1 | 5 | 25 | 1.429s | 42 | 42 | 0 |
| `medium-single-file` | `apply_noop` | 1 | 5 | 25 | 1.424s | 32 | 32 | 0 |
| `medium-single-file` | `apply_create` | 2 | 5 | 25 | 1.447s | 42 | 42 | 0 |
| `medium-single-file` | `apply_noop` | 2 | 5 | 25 | 1.471s | 32 | 32 | 0 |
| `medium-single-file` | `apply_create` | 3 | 5 | 25 | 1.49s | 42 | 42 | 0 |
| `medium-single-file` | `apply_noop` | 3 | 5 | 25 | 1.424s | 32 | 32 | 0 |
| `medium-multi-file` | `apply_create` | 1 | 5 | 25 | 1.474s | 42 | 42 | 0 |
| `medium-multi-file` | `apply_noop` | 1 | 5 | 25 | 1.436s | 32 | 32 | 0 |
| `medium-multi-file` | `apply_create` | 2 | 5 | 25 | 1.412s | 42 | 42 | 0 |
| `medium-multi-file` | `apply_noop` | 2 | 5 | 25 | 1.449s | 32 | 32 | 0 |
| `medium-multi-file` | `apply_create` | 3 | 5 | 25 | 1.396s | 42 | 42 | 0 |
| `medium-multi-file` | `apply_noop` | 3 | 5 | 25 | 1.469s | 32 | 32 | 0 |
