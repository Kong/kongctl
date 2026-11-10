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

By default, lint checks each input as written, including template definitions.
It does not expand declarative tags or templates, so rules cannot inspect
included or inherited values, or environment variable contents. Files selected
individually or through directory discovery are checked separately.

OpenAPI `$ref` resolution is separate from declarative expansion and may read
local files or access the network.

### Linting expanded declarative configuration

Use `--expand` to include local `!file` content and principal `!csv` inputs,
then expand `_templates` and `_extends` overrides as `plan` and `apply` do:

```shell
kongctl lint -f ./templates/ -f ./team/ -r rules/team.yaml --expand
```

Inputs share template definitions but are linted separately: pass template
files as well as the files that use them. Rules cannot detect duplicates
across files.
Empty inputs, including template-only files, are skipped; their templates are
checked where used. If nothing remains to lint, the command fails.

`!env` (including `store: true`), `!ref`, `!secret` and lookup tags stay
unresolved. Lint does not read their environment values, fetch secrets or
contact Konnect. Rules expecting concrete values, such as name patterns, must
account for placeholders.

Expansion checks YAML, tag and template syntax. It does not apply defaults,
validate resource schemas or check the types of fields containing `!env`.
Passing lint does not establish that configuration is ready to apply; use
`plan` for that validation.

- **Inputs:** use `.yaml`/`.yml` files, directories (`-R` for subdirectories),
  or stdin (`-f -`). URLs are unsupported. Each selected input must contain one
  YAML document; split documents separated by `---` into separate files.
- **Paths:** includes resolve from the file containing the tag, even when
  inherited through a template. `!file` paths must stay within the source's
  root: the parent directory for a file, the directory for a directory input,
  or the working directory for stdin (also used to resolve its includes).
  These are the loader's default boundaries.
- **Fragments:** included content is checked in the file that includes it.
  A fragment also selected as an input is checked independently, including
  lists and scalars. Vacuum may report `index-failure` for standalone scalars
  in either mode. Keep fragments outside selected inputs to avoid independent
  checks.
- **Findings:** filenames identify inputs; line/column numbers refer to
  expanded content, which is held in memory and not saved. They may not match
  original source locations. Text appends `(expanded)` after the message;
  JSON/YAML sets `expanded: true`. Text retains `file:line:column` formatting.
- **Failures:** expansion errors fail the command and identify the source;
  template errors also report a position. Existing OpenAPI reference and
  ruleset network access is unchanged.

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
