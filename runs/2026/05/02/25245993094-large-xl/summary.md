# Declarative Benchmark Results

- Run ID: `25245993094`
- Git commit: `f04e95a`
- Base URL: `https://us.api.konghq.com`
- Duration: `23m46.006s`
- Cases: `12`
- Phases: `24`
- HTTP requests: `9624`
- HTTP errors: `0`

Suite duration includes fixture generation and destructive org reset. Phase rows measure only `kongctl apply` commands.

| Case | Phase | Rep | APIs | API documents | Duration | Requests | Responses | Errors |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `large-single-file` | `apply_create` | 1 | 20 | 160 | 32.714s | 221 | 221 | 0 |
| `large-single-file` | `apply_noop` | 1 | 20 | 160 | 16.849s | 181 | 181 | 0 |
| `large-single-file` | `apply_create` | 2 | 20 | 160 | 31.853s | 221 | 221 | 0 |
| `large-single-file` | `apply_noop` | 2 | 20 | 160 | 16.986s | 181 | 181 | 0 |
| `large-single-file` | `apply_create` | 3 | 20 | 160 | 32.816s | 221 | 221 | 0 |
| `large-single-file` | `apply_noop` | 3 | 20 | 160 | 16.823s | 181 | 181 | 0 |
| `large-multi-file` | `apply_create` | 1 | 20 | 160 | 32.584s | 221 | 221 | 0 |
| `large-multi-file` | `apply_noop` | 1 | 20 | 160 | 16.63s | 181 | 181 | 0 |
| `large-multi-file` | `apply_create` | 2 | 20 | 160 | 32.907s | 221 | 221 | 0 |
| `large-multi-file` | `apply_noop` | 2 | 20 | 160 | 16.957s | 181 | 181 | 0 |
| `large-multi-file` | `apply_create` | 3 | 20 | 160 | 32.147s | 221 | 221 | 0 |
| `large-multi-file` | `apply_noop` | 3 | 20 | 160 | 16.6s | 181 | 181 | 0 |
| `xl-single-file` | `apply_create` | 1 | 50 | 500 | 1m37.635s | 651 | 651 | 0 |
| `xl-single-file` | `apply_noop` | 1 | 50 | 500 | 51.881s | 551 | 551 | 0 |
| `xl-single-file` | `apply_create` | 2 | 50 | 500 | 1m37.881s | 651 | 651 | 0 |
| `xl-single-file` | `apply_noop` | 2 | 50 | 500 | 51.64s | 551 | 551 | 0 |
| `xl-single-file` | `apply_create` | 3 | 50 | 500 | 1m39.289s | 651 | 651 | 0 |
| `xl-single-file` | `apply_noop` | 3 | 50 | 500 | 52.363s | 551 | 551 | 0 |
| `xl-multi-file` | `apply_create` | 1 | 50 | 500 | 1m38.555s | 651 | 651 | 0 |
| `xl-multi-file` | `apply_noop` | 1 | 50 | 500 | 52.403s | 551 | 551 | 0 |
| `xl-multi-file` | `apply_create` | 2 | 50 | 500 | 1m38.382s | 651 | 651 | 0 |
| `xl-multi-file` | `apply_noop` | 2 | 50 | 500 | 52.217s | 551 | 551 | 0 |
| `xl-multi-file` | `apply_create` | 3 | 50 | 500 | 1m40.759s | 651 | 651 | 0 |
| `xl-multi-file` | `apply_noop` | 3 | 50 | 500 | 52.088s | 551 | 551 | 0 |
