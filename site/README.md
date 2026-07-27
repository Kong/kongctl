# Learn kongctl site

This directory contains the static, hands-on learning guide published at
<https://kong.github.io/kongctl/>.

## Local development

Use Node.js 24 and install the locked dependencies:

```shell
npm install
```

Start the development server:

```shell
npm run dev
```

The site is served under its production base path at
<http://localhost:4321/kongctl/>.

Before opening a pull request, run:

```shell
npm run check
npm test
npm run build
npm run test:e2e
```

The end-to-end tests require a Playwright Chromium installation:

```shell
npx playwright install chromium
```

## Add a lesson

Add one Markdown file under:

```text
src/content/lessons/<chapter>/<lesson>.md
```

The file needs this frontmatter:

```yaml
---
title: Preview a declarative change
summary: Use diff to inspect a proposed change without executing it.
order: 2
related:
  - label: Declarative configuration documentation
    url: https://developer.konghq.com/kongctl/declarative/
---
```

`order` must be unique within the chapter. The directory name must match a
chapter ID in `src/data/chapters.yaml`.

To add a chapter, add its ID, title, description, and order to
`src/data/chapters.yaml`, then add at least one lesson in the matching
directory.

## Write a hands-on lesson

Keep lessons short and use this sequence where it fits:

1. **Outcome**: State what the learner will accomplish.
2. **Before you begin**: Identify access, authentication, and local tools.
3. **Do it**: Explain the action and immediately show the command or input.
4. **Check it worked**: Give a verification command and expected result.
5. **Go deeper**: Use `related` links for the complete documentation.

Use fenced `shell`, `bash`, `sh`, or `zsh` blocks for commands:

````markdown
```shell
kongctl version --full
```
````

Do not include a `$` prompt. The copy button copies the source exactly, so the
block must contain only text that is safe and useful to paste into a terminal.
Put output in a separate `text` block:

````markdown
```text
1.x.x (<commit> : <build-date>)
```
````

Use angle-bracket placeholders such as `<path>` only when the surrounding
text tells the learner to replace them. Never include real credentials or
secrets. Explain effects before commands that create or modify resources, and
put an explicit warning before destructive commands.

The site cannot observe the learner's terminal. “Mark complete and continue”
is therefore an explicit learner confirmation, not automated command
verification.
