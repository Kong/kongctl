# Declarative Benchmark Results

- Run ID: `29634338870`
- Git commit: `907f539`
- Base URL: `https://us.api.konghq.com`
- Duration: `17m28.688s`
- Cases: `12`
- Phases: `24`
- HTTP requests: `9624`
- HTTP errors: `0`

Suite duration includes fixture generation and destructive org reset. Phase rows measure only `kongctl apply` commands.

| Case | Phase | Rep | APIs | API documents | Duration | Requests | Responses | Errors |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `large-single-file` | `apply_create` | 1 | 20 | 160 | 11.194s | 221 | 221 | 0 |
| `large-single-file` | `apply_noop` | 1 | 20 | 160 | 18.386s | 181 | 181 | 0 |
| `large-single-file` | `apply_create` | 2 | 20 | 160 | 11.303s | 221 | 221 | 0 |
| `large-single-file` | `apply_noop` | 2 | 20 | 160 | 18.546s | 181 | 181 | 0 |
| `large-single-file` | `apply_create` | 3 | 20 | 160 | 11.015s | 221 | 221 | 0 |
| `large-single-file` | `apply_noop` | 3 | 20 | 160 | 18.994s | 181 | 181 | 0 |
| `large-multi-file` | `apply_create` | 1 | 20 | 160 | 11.305s | 221 | 221 | 0 |
| `large-multi-file` | `apply_noop` | 1 | 20 | 160 | 19.133s | 181 | 181 | 0 |
| `large-multi-file` | `apply_create` | 2 | 20 | 160 | 11.171s | 221 | 221 | 0 |
| `large-multi-file` | `apply_noop` | 2 | 20 | 160 | 19.157s | 181 | 181 | 0 |
| `large-multi-file` | `apply_create` | 3 | 20 | 160 | 11.076s | 221 | 221 | 0 |
| `large-multi-file` | `apply_noop` | 3 | 20 | 160 | 19.109s | 181 | 181 | 0 |
| `xl-single-file` | `apply_create` | 1 | 50 | 500 | 31.412s | 651 | 651 | 0 |
| `xl-single-file` | `apply_noop` | 1 | 50 | 500 | 58.785s | 551 | 551 | 0 |
| `xl-single-file` | `apply_create` | 2 | 50 | 500 | 31.578s | 651 | 651 | 0 |
| `xl-single-file` | `apply_noop` | 2 | 50 | 500 | 56.502s | 551 | 551 | 0 |
| `xl-single-file` | `apply_create` | 3 | 50 | 500 | 30.974s | 651 | 651 | 0 |
| `xl-single-file` | `apply_noop` | 3 | 50 | 500 | 59.09s | 551 | 551 | 0 |
| `xl-multi-file` | `apply_create` | 1 | 50 | 500 | 31.283s | 651 | 651 | 0 |
| `xl-multi-file` | `apply_noop` | 1 | 50 | 500 | 58.41s | 551 | 551 | 0 |
| `xl-multi-file` | `apply_create` | 2 | 50 | 500 | 31.599s | 651 | 651 | 0 |
| `xl-multi-file` | `apply_noop` | 2 | 50 | 500 | 1m0.06s | 551 | 551 | 0 |
| `xl-multi-file` | `apply_create` | 3 | 50 | 500 | 31.479s | 651 | 651 | 0 |
| `xl-multi-file` | `apply_noop` | 3 | 50 | 500 | 59.398s | 551 | 551 | 0 |
