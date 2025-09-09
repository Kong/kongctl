# Federated Examples: External Portal With Child Resources

This directory provides two minimal examples showing how to manage portal child resources (portal_page and portal_snippet) when the parent portal is externally managed using the `_external` block.

The two supported layouts are demonstrated:

1) Inline children under an external `portal` (single file)
2) Children defined at the root level in separate files, referencing an external `portal`

These examples are intentionally small and suitable as starting points for tests.

## 1) Inline Children With External Portal

File: `inline.yaml`

Highlights:
- Defines a `portal` with an `_external` selector by name
- Declares one page and one snippet inline under the portal

## 2) Root-Level Children With External Portal

Files:
- `root/portal.yaml` — external portal definition (no children)
- `root/pages.yaml` — `portal_pages` list at root referencing the portal by `ref`
- `root/snippets.yaml` — `portal_snippets` list at root referencing the portal by `ref`

This layout demonstrates distributing child resources into their own files while still referencing an externally managed portal.

