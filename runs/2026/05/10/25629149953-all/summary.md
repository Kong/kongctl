# Declarative Benchmark Results

- Run ID: `25629149953`
- Git commit: `87e66e4`
- Base URL: `https://us.api.konghq.com`
- Duration: `6m49.558s`
- Cases: `8`
- Phases: `16`
- HTTP requests: `3372`
- HTTP errors: `0`

Suite duration includes fixture generation and destructive org reset. Phase rows measure only `kongctl apply` commands.

| Case | Phase | Rep | APIs | API documents | Duration | Requests | Responses | Errors |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `small-single-file` | `apply_create` | 1 | 1 | 2 | 892ms | 6 | 6 | 0 |
| `small-single-file` | `apply_noop` | 1 | 1 | 2 | 505ms | 4 | 4 | 0 |
| `small-multi-file` | `apply_create` | 1 | 1 | 2 | 790ms | 6 | 6 | 0 |
| `small-multi-file` | `apply_noop` | 1 | 1 | 2 | 506ms | 4 | 4 | 0 |
| `medium-single-file` | `apply_create` | 1 | 5 | 25 | 2.234s | 41 | 41 | 0 |
| `medium-single-file` | `apply_noop` | 1 | 5 | 25 | 2.883s | 31 | 31 | 0 |
| `medium-multi-file` | `apply_create` | 1 | 5 | 25 | 2.102s | 41 | 41 | 0 |
| `medium-multi-file` | `apply_noop` | 1 | 5 | 25 | 2.84s | 31 | 31 | 0 |
| `large-single-file` | `apply_create` | 1 | 20 | 160 | 9.677s | 221 | 221 | 0 |
| `large-single-file` | `apply_noop` | 1 | 20 | 160 | 16.018s | 181 | 181 | 0 |
| `large-multi-file` | `apply_create` | 1 | 20 | 160 | 9.615s | 221 | 221 | 0 |
| `large-multi-file` | `apply_noop` | 1 | 20 | 160 | 16.15s | 181 | 181 | 0 |
| `xl-single-file` | `apply_create` | 1 | 50 | 500 | 27.938s | 651 | 651 | 0 |
| `xl-single-file` | `apply_noop` | 1 | 50 | 500 | 51.019s | 551 | 551 | 0 |
| `xl-multi-file` | `apply_create` | 1 | 50 | 500 | 27.644s | 651 | 651 | 0 |
| `xl-multi-file` | `apply_noop` | 1 | 50 | 500 | 48.916s | 551 | 551 | 0 |
