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

If the lint engine cannot execute a rule, the command fails and reports the
input and engine diagnostic on stderr. For example, an invalid JSONPath
selector fails even when the rule's severity is below `--fail-severity` or
`--display-only-failures` is enabled. Engine failures produce no lint result
on stdout, including with JSON or YAML output. A valid selector that finds
no matching values still succeeds.

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

Each input is read independently of the declarative loader used by `plan` and
`apply`. Lint does not resolve `!file`, `!ref`, or `!env`, or expand
`_templates` and `_extends`. OpenAPI `$ref` resolution is separate and may
read and check referenced content, including through network requests.
This is not declarative tag expansion.

Rules can inspect fields present in the raw file, including template
definitions, but cannot inspect values that would appear only after expansion.
For example, selecting fields inside a resource referenced by `!file` does not
load or check the referenced content. Selecting an `!env` value does not check
the environment variable's contents.

Passing a fragment as another `-f` input checks it separately. Directory input
also checks matching files separately; it does not assemble a combined
configuration. A successful lint run does not establish that the expanded
configuration satisfies the same rules or passes declarative validation.
To lint the expanded configuration, use `--expand`.

### Linting expanded declarative configuration

Use `--expand` to lint configuration after declarative preprocessing:

```shell
kongctl lint -f ./templates/ -f ./team/ -r rules/team.yaml --expand
```

With `--expand`:

- All inputs share template definitions, so a template defined in one file can
  be used by `_extends` in another. Pass the template files as well as the files
  that use them. Resources are not combined into one lint document.
- Local `!file` content is included, and `_templates` and `_extends` overrides
  are expanded with the same rules as `plan` and `apply`.
- `!env`, `!ref`, `!secret` and lookup tags stay unresolved placeholders. Lint
  does not read environment values, fetch secrets or contact Konnect, so rules
  cannot check the values behind those tags. This includes `!env` declarations
  with `store: true`. Rules expecting concrete values, such as a name pattern,
  need to account for unresolved values.
- Defaults are not applied and `plan` validation does not run. A passing lint
  means the selected rules passed, not that the configuration is ready to
  apply. YAML syntax, tag declarations and template syntax are still checked;
  resource schemas and the types of fields containing `!env` are not validated.
- Each input is linted on its own after expansion. Inputs that are empty
  after expansion, such as files that only define `_templates`, are skipped;
  their templates are checked through the resources that extend them. Rules
  do not see resources from other inputs, so a rule cannot detect a
  duplicate across files.
- A file included with `!file` is checked inside the file that includes it.
  If the same file is also selected as an input, for example in the same
  directory, it is also linted on its own. List and scalar fragments are passed
  to the lint engine as separate inputs too. The engine may report an
  `index-failure` for a standalone scalar, just as in raw lint mode. Keep
  fragments outside selected inputs to check them only through their consumers.
- Line and column numbers refer to the expanded document, not the original
  file. Text output keeps `file:line:column` and appends `(expanded)` to each
  finding; JSON and YAML output set `expanded: true`. Expansion happens in
  memory; the expanded document is not saved. These coordinates cannot be
  opened directly in the original source file.
- `!file` paths must stay inside each `-f` source root (file: its parent
  directory; directory: the directory itself), the default boundary of `plan`.
- Inputs can be individual YAML files, directories, or stdin (`-f -`). Use
  `-R` to include subdirectories. URL inputs are not supported. Explicit file
  inputs must use `.yaml` or `.yml`.
  Stdin includes resolve from the working directory, which is also their
  allowed boundary. Relative paths inside templates use the file that defines
  the template, even when another file extends it.
- Each input must contain one YAML document. Multiple documents separated by
  `---` are rejected rather than silently linting only the first document.
  Split them into separate files. Expansion errors, such as an unknown template
  or a missing include, fail the command and name the source; template errors
  also provide a position.
- Like the loader, preprocessing also expands local principal `!csv` inputs.
  It does not fetch URL sources. Existing OpenAPI reference resolution and
  ruleset inheritance may still access the network.

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

## Feature follow-ups

Proposed extensions are tracked separately from current behavior:

- [Custom JavaScript functions](https://github.com/Kong/kongctl/issues/2585)
- [Lint expanded declarative input](https://github.com/Kong/kongctl/issues/2584)
