# Lint configuration files

`kongctl lint` uses [vacuum](https://quobix.com/vacuum/) to check configuration
files against a YAML or JSON ruleset with partial Spectral compatibility.

```shell
kongctl lint -f config.yaml -r ruleset.yaml
```

A rule selects values with `given` (a JSONPath expression), then checks them
with a built-in function named in `then.function`. For example:

```yaml
rules:
  api-name-required:
    description: APIs must have a name
    severity: error
    given: $.apis[*]
    then:
      field: name
      function: truthy
```

Use `kongctl lint --help` for input, output, and severity options. The default
failure threshold is `error`; violations at or above the selected threshold
produce a nonzero exit code.

## Spectral compatibility limits

### Custom functions

Only functions provided by vacuum are available. Custom JavaScript or native
functions cannot be loaded. Spectral `functions` and `functionsDir`
declarations do not load functions, and there is no `--functions` flag.
Referencing an unavailable function produces an `Unknown function` violation.

For rules that depend on central metadata, prepare that metadata outside
`kongctl lint` and use a built-in check such as `enumeration` in the ruleset.
See the [vacuum function reference](https://quobix.com/vacuum/functions/).

### Declarative tags and templates

Each input is read and linted as written, independently of the declarative
loader used by `plan` and `apply`. Lint does not resolve `!file`, `!ref`, or
`!env`, or expand `_templates` and `_extends`.

Rules can inspect fields present in the raw file, including template
definitions, but cannot inspect values that would appear only after expansion.
For example, selecting fields inside a resource referenced by `!file` does not
load or check the referenced content. Selecting an `!env` value does not check
the environment variable's contents.

Passing a fragment as another `-f` input checks it separately. Directory input
also checks matching files separately; it does not assemble a combined
configuration. A successful lint run does not establish that the expanded
configuration satisfies the same rules or passes declarative validation.

### Ruleset paths and inheritance

`-r`/`--ruleset` must point to a local YAML or JSON file. A URL cannot be passed
directly to this flag. A ruleset's `extends` entries can reference URLs.

Local paths inside `extends` resolve relative to the process's working
directory, rather than the ruleset's directory. For example, when running from
the repository root:

```yaml
# rules/governance.yaml
extends:
  - ./rules/base.yaml
```

```shell
kongctl lint -f config.yaml -r rules/governance.yaml
```

Run from a consistent working directory or use absolute local paths for
reproducible local inheritance. URL-based inheritance may access the network;
these path constraints do not make lint an offline command.

## Planned evaluation

Custom-function support is under evaluation. The
[custom-function proposal](contributor/lint-custom-functions.md) records the
implementation approach, security concerns, and acceptance criteria for a
possible follow-up. It describes proposed behavior, not available flags.
