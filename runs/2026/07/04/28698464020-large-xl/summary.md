# Declarative Benchmark Results

- Run ID: `28698464020`
- Git commit: `71234c9`
- Base URL: `https://us.api.konghq.com`
- Duration: `16m30.483s`
- Cases: `12`
- Phases: `24`
- HTTP requests: `9624`
- HTTP errors: `0`

Suite duration includes fixture generation and destructive org reset. Phase rows measure only `kongctl apply` commands.

| Case | Phase | Rep | APIs | API documents | Duration | Requests | Responses | Errors |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `large-single-file` | `apply_create` | 1 | 20 | 160 | 10.765s | 221 | 221 | 0 |
| `large-single-file` | `apply_noop` | 1 | 20 | 160 | 17.786s | 181 | 181 | 0 |
| `large-single-file` | `apply_create` | 2 | 20 | 160 | 10.437s | 221 | 221 | 0 |
| `large-single-file` | `apply_noop` | 2 | 20 | 160 | 18s | 181 | 181 | 0 |
| `large-single-file` | `apply_create` | 3 | 20 | 160 | 10.503s | 221 | 221 | 0 |
| `large-single-file` | `apply_noop` | 3 | 20 | 160 | 18.109s | 181 | 181 | 0 |
| `large-multi-file` | `apply_create` | 1 | 20 | 160 | 10.351s | 221 | 221 | 0 |
| `large-multi-file` | `apply_noop` | 1 | 20 | 160 | 18.108s | 181 | 181 | 0 |
| `large-multi-file` | `apply_create` | 2 | 20 | 160 | 10.757s | 221 | 221 | 0 |
| `large-multi-file` | `apply_noop` | 2 | 20 | 160 | 17.899s | 181 | 181 | 0 |
| `large-multi-file` | `apply_create` | 3 | 20 | 160 | 10.677s | 221 | 221 | 0 |
| `large-multi-file` | `apply_noop` | 3 | 20 | 160 | 17.875s | 181 | 181 | 0 |
| `xl-single-file` | `apply_create` | 1 | 50 | 500 | 30.085s | 651 | 651 | 0 |
| `xl-single-file` | `apply_noop` | 1 | 50 | 500 | 56.358s | 551 | 551 | 0 |
| `xl-single-file` | `apply_create` | 2 | 50 | 500 | 30.319s | 651 | 651 | 0 |
| `xl-single-file` | `apply_noop` | 2 | 50 | 500 | 57.229s | 551 | 551 | 0 |
| `xl-single-file` | `apply_create` | 3 | 50 | 500 | 29.797s | 651 | 651 | 0 |
| `xl-single-file` | `apply_noop` | 3 | 50 | 500 | 54.571s | 551 | 551 | 0 |
| `xl-multi-file` | `apply_create` | 1 | 50 | 500 | 30.148s | 651 | 651 | 0 |
| `xl-multi-file` | `apply_noop` | 1 | 50 | 500 | 55.19s | 551 | 551 | 0 |
| `xl-multi-file` | `apply_create` | 2 | 50 | 500 | 29.585s | 651 | 651 | 0 |
| `xl-multi-file` | `apply_noop` | 2 | 50 | 500 | 56.378s | 551 | 551 | 0 |
| `xl-multi-file` | `apply_create` | 3 | 50 | 500 | 30.216s | 651 | 651 | 0 |
| `xl-multi-file` | `apply_noop` | 3 | 50 | 500 | 55.661s | 551 | 551 | 0 |
