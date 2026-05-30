# Declarative Benchmark Results

- Run ID: `26677517977`
- Git commit: `32ff983`
- Base URL: `https://us.api.konghq.com`
- Duration: `11m58.79s`
- Cases: `12`
- Phases: `24`
- HTTP requests: `9624`
- HTTP errors: `0`

Suite duration includes fixture generation and destructive org reset. Phase rows measure only `kongctl apply` commands.

| Case | Phase | Rep | APIs | API documents | Duration | Requests | Responses | Errors |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `large-single-file` | `apply_create` | 1 | 20 | 160 | 8.158s | 221 | 221 | 0 |
| `large-single-file` | `apply_noop` | 1 | 20 | 160 | 12.698s | 181 | 181 | 0 |
| `large-single-file` | `apply_create` | 2 | 20 | 160 | 7.906s | 221 | 221 | 0 |
| `large-single-file` | `apply_noop` | 2 | 20 | 160 | 12.877s | 181 | 181 | 0 |
| `large-single-file` | `apply_create` | 3 | 20 | 160 | 7.816s | 221 | 221 | 0 |
| `large-single-file` | `apply_noop` | 3 | 20 | 160 | 12.569s | 181 | 181 | 0 |
| `large-multi-file` | `apply_create` | 1 | 20 | 160 | 8.073s | 221 | 221 | 0 |
| `large-multi-file` | `apply_noop` | 1 | 20 | 160 | 13.117s | 181 | 181 | 0 |
| `large-multi-file` | `apply_create` | 2 | 20 | 160 | 7.951s | 221 | 221 | 0 |
| `large-multi-file` | `apply_noop` | 2 | 20 | 160 | 12.435s | 181 | 181 | 0 |
| `large-multi-file` | `apply_create` | 3 | 20 | 160 | 7.999s | 221 | 221 | 0 |
| `large-multi-file` | `apply_noop` | 3 | 20 | 160 | 12.892s | 181 | 181 | 0 |
| `xl-single-file` | `apply_create` | 1 | 50 | 500 | 22.568s | 651 | 651 | 0 |
| `xl-single-file` | `apply_noop` | 1 | 50 | 500 | 38.584s | 551 | 551 | 0 |
| `xl-single-file` | `apply_create` | 2 | 50 | 500 | 23.406s | 651 | 651 | 0 |
| `xl-single-file` | `apply_noop` | 2 | 50 | 500 | 38.585s | 551 | 551 | 0 |
| `xl-single-file` | `apply_create` | 3 | 50 | 500 | 22.957s | 651 | 651 | 0 |
| `xl-single-file` | `apply_noop` | 3 | 50 | 500 | 39.77s | 551 | 551 | 0 |
| `xl-multi-file` | `apply_create` | 1 | 50 | 500 | 23.423s | 651 | 651 | 0 |
| `xl-multi-file` | `apply_noop` | 1 | 50 | 500 | 39.471s | 551 | 551 | 0 |
| `xl-multi-file` | `apply_create` | 2 | 50 | 500 | 22.946s | 651 | 651 | 0 |
| `xl-multi-file` | `apply_noop` | 2 | 50 | 500 | 38.966s | 551 | 551 | 0 |
| `xl-multi-file` | `apply_create` | 3 | 50 | 500 | 22.494s | 651 | 651 | 0 |
| `xl-multi-file` | `apply_noop` | 3 | 50 | 500 | 39.723s | 551 | 551 | 0 |
