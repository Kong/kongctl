# Declarative Benchmark Results

- Run ID: `24937035314`
- Git commit: `95c89ae`
- Base URL: `https://us.api.konghq.com`
- Duration: `4m19.579s`
- Cases: `2`
- Phases: `4`
- HTTP requests: `804`
- HTTP errors: `0`

Suite duration includes fixture generation and destructive org reset. Phase rows measure only `kongctl apply` commands.

| Case | Phase | Rep | APIs | API documents | Duration | Requests | Responses | Errors |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `large-single-file` | `apply_create` | 1 | 20 | 160 | 30.709s | 221 | 221 | 0 |
| `large-single-file` | `apply_noop` | 1 | 20 | 160 | 16.228s | 181 | 181 | 0 |
| `large-multi-file` | `apply_create` | 1 | 20 | 160 | 31.827s | 221 | 221 | 0 |
| `large-multi-file` | `apply_noop` | 1 | 20 | 160 | 16.21s | 181 | 181 | 0 |
