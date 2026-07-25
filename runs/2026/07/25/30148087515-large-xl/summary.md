# Declarative Benchmark Results

- Run ID: `30148087515`
- Git commit: `0cc36c7`
- Base URL: `https://us.api.konghq.com`
- Duration: `15m36.034s`
- Cases: `12`
- Phases: `24`
- HTTP requests: `9624`
- HTTP errors: `0`

Suite duration includes fixture generation and destructive org reset. Phase rows measure only `kongctl apply` commands.

| Case | Phase | Rep | APIs | API documents | Duration | Requests | Responses | Errors |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `large-single-file` | `apply_create` | 1 | 20 | 160 | 9.961s | 221 | 221 | 0 |
| `large-single-file` | `apply_noop` | 1 | 20 | 160 | 16.508s | 181 | 181 | 0 |
| `large-single-file` | `apply_create` | 2 | 20 | 160 | 9.739s | 221 | 221 | 0 |
| `large-single-file` | `apply_noop` | 2 | 20 | 160 | 16.604s | 181 | 181 | 0 |
| `large-single-file` | `apply_create` | 3 | 20 | 160 | 9.95s | 221 | 221 | 0 |
| `large-single-file` | `apply_noop` | 3 | 20 | 160 | 16.706s | 181 | 181 | 0 |
| `large-multi-file` | `apply_create` | 1 | 20 | 160 | 9.822s | 221 | 221 | 0 |
| `large-multi-file` | `apply_noop` | 1 | 20 | 160 | 16.646s | 181 | 181 | 0 |
| `large-multi-file` | `apply_create` | 2 | 20 | 160 | 9.72s | 221 | 221 | 0 |
| `large-multi-file` | `apply_noop` | 2 | 20 | 160 | 16.759s | 181 | 181 | 0 |
| `large-multi-file` | `apply_create` | 3 | 20 | 160 | 9.86s | 221 | 221 | 0 |
| `large-multi-file` | `apply_noop` | 3 | 20 | 160 | 16.574s | 181 | 181 | 0 |
| `xl-single-file` | `apply_create` | 1 | 50 | 500 | 28.033s | 651 | 651 | 0 |
| `xl-single-file` | `apply_noop` | 1 | 50 | 500 | 52.013s | 551 | 551 | 0 |
| `xl-single-file` | `apply_create` | 2 | 50 | 500 | 27.704s | 651 | 651 | 0 |
| `xl-single-file` | `apply_noop` | 2 | 50 | 500 | 50.876s | 551 | 551 | 0 |
| `xl-single-file` | `apply_create` | 3 | 50 | 500 | 28.237s | 651 | 651 | 0 |
| `xl-single-file` | `apply_noop` | 3 | 50 | 500 | 50.378s | 551 | 551 | 0 |
| `xl-multi-file` | `apply_create` | 1 | 50 | 500 | 27.586s | 651 | 651 | 0 |
| `xl-multi-file` | `apply_noop` | 1 | 50 | 500 | 50.669s | 551 | 551 | 0 |
| `xl-multi-file` | `apply_create` | 2 | 50 | 500 | 27.958s | 651 | 651 | 0 |
| `xl-multi-file` | `apply_noop` | 2 | 50 | 500 | 51.722s | 551 | 551 | 0 |
| `xl-multi-file` | `apply_create` | 3 | 50 | 500 | 27.741s | 651 | 651 | 0 |
| `xl-multi-file` | `apply_noop` | 3 | 50 | 500 | 50.936s | 551 | 551 | 0 |
