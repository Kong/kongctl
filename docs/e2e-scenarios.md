E2E Declarative Scenarios (MVP)

Overview

- Author end-to-end tests as text-only scenarios: a base input config, step overlays, a sequence of kongctl commands per step, and simple assertions against expected files.
- Assertions use the JSON stdout of a command (or a shorthand get) plus a selector and masking to compare against an expected file (optionally merged with small overlays).
- This document specifies the minimal schema and includes a concrete example mirrored from the existing declarative_general_test.go.

Key Concepts

- Scenario: Top-level test definition. May reset the organization before/after and sets defaults for masking and retries.
- Step: Creates a working copy of the base inputs and applies one or more overlay directories before running commands.
- Command: Any kongctl invocation (apply, sync, diff, get, …). Each command can have 0–n assertions.
- Assertion: Selects data from a JSON source (the parent command’s stdout by default) and compares it to an expected file after masking transient fields.

Names Are Optional (Indexed From 000)

- If a name is omitted, the runner auto-generates stable names starting at 000:
  - Scenario name defaults to the containing directory name.
  - Steps: step-000, step-001, …
  - Commands (per step): cmd-000, cmd-001, …
  - Assertions (per command): assert-000, assert-001, …

Schema (YAML)

- reset: one of before | after | both | never (default: before)
- baseInputsPath: path to the base declarative config directory
- env: map of environment variables to pass to kongctl
- vars: free-form variables usable in templates (e.g., for selectors or overlay files)
- defaults:
  - retry:
      attempts: int
      interval: string duration (e.g., "1500ms", "2s")
  - mask:
      dropKeys: [list of key names]        # remove keys by name at any depth
- steps: [
  {
    name?: string,
    inputOverlayDirs?: [paths],            # merge these dirs into the step workdir (omit if none)
    mask?: { dropKeys: [...] },            # override/extend defaults.mask
    retry?: { attempts, interval },        # override defaults.retry
    commands: [
      {
        name?: string,
        run: ["apply", "-f", "{{ .workdir }}/portal.yaml", …],  # arbitrary kongctl args
        mask?: { dropKeys: [...] },
        retry?: { attempts, interval },
        assertions: [
          {
            name?: string,
            # Source: by default, the parent command’s stdout is used.
            # Optionally request a fresh source:
            source?: { get?: "apis" | "portals" | "…" },
            select: "JMESPath expression",                # isolate object/array/scalar
            expect: {
              file: path,                                  # expected file (JSON or YAML)
              overlays?: [paths]                           # small merges applied to expect.file
            },
            mask?: { dropKeys: [...] },
            retry?: { attempts, interval }
          }
        ]
      }
    ]
  }
]

Masking (MVP)

- Purpose: remove dynamic fields (ids, timestamps, etags, links) so expected files can focus on business values.
- Applied to both observed and expected before compare.
- MVP keeps this simple: only dropKeys is supported.
- Recommended defaults at scenario.defaults.mask.dropKeys:
  - [id, uuid, created_at, createdAt, updated_at, updatedAt, etag, links, href, self, version]
- Precedence (lowest to highest): defaults.mask < step.mask < command.mask < assertion.mask.
- Merge rule: dropKeys is the union of all applicable levels.

Overlays

- Step input overlays: inputOverlayDirs is a list of directories whose file trees mirror the base inputs. Files are merged into the step workdir using a JSON Merge Patch–style deep merge:
  - Maps merge; scalars replace; arrays replace entirely.
  - Templating is allowed in overlay files via Go text/template + Sprig, with these variables: .vars (from scenario), .scenario, .step, .workdir.
  - When arrays need targeted edits, copy the base file into an overlay dir and modify it, or replace the whole array; we can add JSON Patch/JMESPath ops later if needed.
- Expected overlays: expect.overlays is a list of files merged into the expect.file to form the final expected payload.
- MVP scopes overlays to steps (inputs) and assertions (expected). There are no command-level overlays.

Optional And Empty Fields

- Omit fields that are empty or not needed. For example, if a step has no input overlays, you can omit inputOverlayDirs. If a command has no assertions, you can omit the assertions key entirely.

Selectors and Sources

- Use JMESPath to target the object/array/scalar you want to compare.
- Default source is the parent command’s JSON stdout. Alternatively, set `source.get: "<resource>"` to run a fresh `kongctl get <resource>` for the assertion.
- Examples: "[?name=='My First Portal'] | [0]", "[0]", "data[?title=='SMS API']".
- Tiny example (assert under an apply command using a fresh get source):

  assertions:
    - select: "[?name=='{{ .vars.portalName }}'] | [0]"
      source: { get: "portals" }
      expect:
        file: "expect/portal.json"

Comparison Semantics (Simplicity First)

- Deep-compare JSON after masking. Object key order is irrelevant; array order is significant.
- To avoid array-order flakiness in MVP, prefer selecting a specific element with stable filters (e.g., by name) rather than comparing whole arrays.
- If you must compare arrays, use JMESPath to sort or project them into a stable form before compare, e.g., `sort_by(@,&name)` or `[].[name, visibility]`.

Library Evaluation: go-cmp / cmpopts

- We can use `github.com/google/go-cmp/cmp` to produce clear diffs on failure.
- `cmpopts.IgnoreFields` is most effective with typed Go structs; our assertions compare dynamic JSON (`map[string]any`/`[]any`). For MVP, a simple pre-compare sanitizer that drops keys by name is clearer and requires no schema.
- `cmpopts.SortSlices` and other helpers are designed for typed slices; for generic JSON arrays we’d still need custom logic or selectors. Given we encourage selectors to pick a single element or to sort via JMESPath, we don’t need array sorting in the comparator for MVP.
- Proposal: MVP uses a lightweight sanitizer (dropKeys) + `go-cmp` for deep-compare and human-friendly diffs. We can revisit path-aware ignore/sort rules using `cmp.FilterPath` if we later move to typed models or need more advanced behavior.

Golden Files (Terminology)

- We avoid the term “golden”; the DSL uses expect.file and expect.overlays.
- Update mode (for bootstrapping): set KONGCTL_E2E_UPDATE_EXPECT=1 to write the sanitized observed value into expect.file. Overlays still apply afterward.

Example Scenario (Based on declarative_general_test.go)

See: test/e2e/scenarios/portal/visibility/scenario.yaml

Notes

- Assertions attach to commands so you can validate state in-between changes.
- If a needed resource isn’t supported by kongctl get yet, add that coverage to the CLI so scenarios don’t need HTTP fallbacks.
