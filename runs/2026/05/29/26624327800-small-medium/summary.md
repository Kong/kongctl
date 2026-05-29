# Declarative Benchmark Results

- Run ID: `26624327800`
- Git commit: `b67c5b6`
- Base URL: `https://us.api.konghq.com`
- Duration: `3m24.793s`
- Cases: `12`
- Phases: `24`
- HTTP requests: `492`
- HTTP errors: `0`

Suite duration includes fixture generation and destructive org reset. Phase rows measure only `kongctl apply` commands.

| Case | Phase | Rep | APIs | API documents | Duration | Requests | Responses | Errors |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `small-single-file` | `apply_create` | 1 | 1 | 2 | 650ms | 6 | 6 | 0 |
| `small-single-file` | `apply_noop` | 1 | 1 | 2 | 325ms | 4 | 4 | 0 |
| `small-single-file` | `apply_create` | 2 | 1 | 2 | 607ms | 6 | 6 | 0 |
| `small-single-file` | `apply_noop` | 2 | 1 | 2 | 344ms | 4 | 4 | 0 |
| `small-single-file` | `apply_create` | 3 | 1 | 2 | 558ms | 6 | 6 | 0 |
| `small-single-file` | `apply_noop` | 3 | 1 | 2 | 330ms | 4 | 4 | 0 |
| `small-multi-file` | `apply_create` | 1 | 1 | 2 | 583ms | 6 | 6 | 0 |
| `small-multi-file` | `apply_noop` | 1 | 1 | 2 | 350ms | 4 | 4 | 0 |
| `small-multi-file` | `apply_create` | 2 | 1 | 2 | 524ms | 6 | 6 | 0 |
| `small-multi-file` | `apply_noop` | 2 | 1 | 2 | 322ms | 4 | 4 | 0 |
| `small-multi-file` | `apply_create` | 3 | 1 | 2 | 589ms | 6 | 6 | 0 |
| `small-multi-file` | `apply_noop` | 3 | 1 | 2 | 346ms | 4 | 4 | 0 |
| `medium-single-file` | `apply_create` | 1 | 5 | 25 | 1.473s | 41 | 41 | 0 |
| `medium-single-file` | `apply_noop` | 1 | 5 | 25 | 1.889s | 31 | 31 | 0 |
| `medium-single-file` | `apply_create` | 2 | 5 | 25 | 1.473s | 41 | 41 | 0 |
| `medium-single-file` | `apply_noop` | 2 | 5 | 25 | 1.827s | 31 | 31 | 0 |
| `medium-single-file` | `apply_create` | 3 | 5 | 25 | 1.521s | 41 | 41 | 0 |
| `medium-single-file` | `apply_noop` | 3 | 5 | 25 | 1.883s | 31 | 31 | 0 |
| `medium-multi-file` | `apply_create` | 1 | 5 | 25 | 1.563s | 41 | 41 | 0 |
| `medium-multi-file` | `apply_noop` | 1 | 5 | 25 | 1.859s | 31 | 31 | 0 |
| `medium-multi-file` | `apply_create` | 2 | 5 | 25 | 1.487s | 41 | 41 | 0 |
| `medium-multi-file` | `apply_noop` | 2 | 5 | 25 | 1.829s | 31 | 31 | 0 |
| `medium-multi-file` | `apply_create` | 3 | 5 | 25 | 1.52s | 41 | 41 | 0 |
| `medium-multi-file` | `apply_noop` | 3 | 5 | 25 | 1.857s | 31 | 31 | 0 |
