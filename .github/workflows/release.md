---
name: Release
description: Build and release kongctl, then prepend agent-generated release highlights
on:
  roles:
    - admin
    - maintainer
  workflow_dispatch:
    inputs:
      release_type:
        description: Release type (patch, minor, or major)
        required: true
        type: choice
        options:
          - patch
          - minor
          - major
permissions:
  contents: read
  pull-requests: read
  actions: read
  issues: read
engine: copilot
strict: false
timeout-minutes: 30
network:
  allowed:
    - defaults
sandbox:
  agent: awf
tools:
  bash:
    - "*"
  edit:
safe-outputs:
  update-release:
jobs:
  config:
    needs: ["pre_activation", "activation"]
    runs-on: ubuntu-latest
    outputs:
      release_tag: ${{ steps.compute_config.outputs.release_tag }}
      release_version: ${{ steps.compute_config.outputs.release_version }}
    steps:
      - name: Checkout repository
        uses: actions/checkout@v6
        with:
          fetch-depth: 0
          persist-credentials: false

      - name: Compute release configuration
        id: compute_config
        uses: actions/github-script@v8
        with:
          script: |
            const releaseType = context.payload.inputs.release_type;

            console.log(`Computing next version for release type: ${releaseType}`);

            const { data: releases } = await github.rest.repos.listReleases({
              owner: context.repo.owner,
              repo: context.repo.repo,
              per_page: 100,
            });

            // Parse stable semver tags only, e.g. v1.2.3
            const parseSemver = (tag) => {
              const match = tag.match(/^v?(\d+)\.(\d+)\.(\d+)$/);
              if (!match) return null;
              return {
                tag,
                major: parseInt(match[1], 10),
                minor: parseInt(match[2], 10),
                patch: parseInt(match[3], 10),
              };
            };

            const sortedReleases = releases
              .filter((r) => !r.draft)
              .map((r) => parseSemver(r.tag_name))
              .filter((v) => v !== null)
              .sort((a, b) => {
                if (a.major !== b.major) return b.major - a.major;
                if (a.minor !== b.minor) return b.minor - a.minor;
                return b.patch - a.patch;
              });

            // Default to 0.0.0 when no prior stable release exists.
            let major = 0;
            let minor = 0;
            let patch = 0;

            if (sortedReleases.length > 0) {
              const latestTag = sortedReleases[0].tag;
              const version = latestTag.replace(/^v/, "");
              [major, minor, patch] = version.split(".").map(Number);
              console.log(`Latest stable release tag: ${latestTag}`);
            } else {
              console.log("No prior stable release found, using base 0.0.0");
            }

            switch (releaseType) {
              case "major":
                major += 1;
                minor = 0;
                patch = 0;
                break;
              case "minor":
                minor += 1;
                patch = 0;
                break;
              case "patch":
                patch += 1;
                break;
              default:
                core.setFailed(`Unsupported release_type: ${releaseType}`);
                return;
            }

            const releaseVersion = `${major}.${minor}.${patch}`;
            const releaseTag = `v${releaseVersion}`;
            console.log(`Computed release tag: ${releaseTag}`);

            const existingRelease = releases.find((r) => r.tag_name === releaseTag);
            if (existingRelease) {
              core.setFailed(
                `Release tag ${releaseTag} already exists (created ${existingRelease.created_at}).`,
              );
              return;
            }

            try {
              await github.rest.git.getRef({
                owner: context.repo.owner,
                repo: context.repo.repo,
                ref: `tags/${releaseTag}`,
              });
              core.setFailed(`Git tag ${releaseTag} already exists.`);
              return;
            } catch (error) {
              if (error.status !== 404) {
                throw error;
              }
            }

            core.setOutput("release_tag", releaseTag);
            core.setOutput("release_version", releaseVersion);
            console.log(`✓ Release tag: ${releaseTag}`);

  release:
    needs: ["pre_activation", "activation", "config"]
    runs-on: ubuntu-latest
    permissions:
      contents: write
    outputs:
      release_id: ${{ steps.get_release.outputs.release_id }}
    env:
      RELEASE_TAG: ${{ needs.config.outputs.release_tag }}
      RELEASE_VERSION: ${{ needs.config.outputs.release_version }}
    steps:
      - name: Checkout repository
        uses: actions/checkout@v6
        with:
          fetch-depth: 0
          persist-credentials: true

      - name: Create and push tag
        run: |
          set -euo pipefail

          git fetch origin --tags
          if git rev-parse "$RELEASE_TAG" >/dev/null 2>&1; then
            echo "::error::Tag $RELEASE_TAG already exists."
            exit 1
          fi

          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git tag -a "$RELEASE_TAG" -m "Release $RELEASE_TAG"
          git push origin "$RELEASE_TAG"

      - name: Fetch tags
        run: git fetch origin --tags

      - name: Configure private git reads for GoReleaser
        env:
          GH_PRIVATE_READ_TOKEN: ${{ secrets.GH_TOKEN_PRIVATE_READ }}
        run: |
          set -euo pipefail
          if [ -n "${GH_PRIVATE_READ_TOKEN:-}" ]; then
            git config --global url."https://${GH_PRIVATE_READ_TOKEN}@github.com/".insteadOf https://github.com/
          else
            echo "GH_TOKEN_PRIVATE_READ not set; using default git auth"
          fi

      - name: Run GoReleaser (goreleaser-cross)
        run: |
          set -euo pipefail

          HOST_UID="$(id -u)"
          HOST_GID="$(id -g)"
          docker run --rm \
            -v "${PWD}:/work" \
            -w /work \
            -e GITHUB_TOKEN="${{ secrets.GITHUB_TOKEN }}" \
            -e TAP_GITHUB_TOKEN="${{ secrets.TAP_GITHUB_TOKEN }}" \
            -e CGO_ENABLED=0 \
            -e DEBIAN_FRONTEND=noninteractive \
            -e HOST_UID="${HOST_UID}" \
            -e HOST_GID="${HOST_GID}" \
            --entrypoint /bin/sh \
            goreleaser/goreleaser-cross:v1.25.3 \
            -c "set -e; git config --global --add safe.directory /work; goreleaser release --clean; if [ -d /work/dist ]; then chown -R \${HOST_UID:-0}:\${HOST_GID:-0} /work/dist; fi"

      - name: Set up Homebrew
        uses: Homebrew/actions/setup-homebrew@master

      - name: Fetch Homebrew tap
        env:
          TAP_GITHUB_TOKEN: ${{ secrets.TAP_GITHUB_TOKEN }}
        run: |
          set -euo pipefail
          rm -rf dist/homebrew-tap
          mkdir -p dist/homebrew-tap
          git clone --depth 1 "https://x-access-token:${TAP_GITHUB_TOKEN}@github.com/kong/homebrew-kongctl.git" dist/homebrew-tap/homebrew-kongctl

      - name: Sync generated Homebrew files
        run: |
          set -euo pipefail
          mkdir -p dist/homebrew-tap/homebrew-kongctl/Casks
          rm -f dist/homebrew-tap/homebrew-kongctl/kongctl.rb
          if [[ -f dist/homebrew/Casks/kongctl.rb ]]; then
            cp dist/homebrew/Casks/kongctl.rb dist/homebrew-tap/homebrew-kongctl/Casks/kongctl.rb
          fi

      - name: Fix Homebrew tap style
        run: ./scripts/fix-homebrew-tap.sh dist/homebrew-tap/homebrew-kongctl

      - name: Commit and push Homebrew tap updates
        working-directory: dist/homebrew-tap/homebrew-kongctl
        env:
          TAP_GITHUB_TOKEN: ${{ secrets.TAP_GITHUB_TOKEN }}
        run: |
          set -euo pipefail
          git config user.name "kongctl Release Bot"
          git config user.email "kongctl@konghq.com"
          git remote set-url origin "https://x-access-token:${TAP_GITHUB_TOKEN}@github.com/kong/homebrew-kongctl.git"
          if [[ -n "$(git status --porcelain)" ]]; then
            git add .
            git commit -m "chore: normalize tap files"
            git push origin HEAD:main
          else
            echo "No tap changes to commit"
          fi

      - name: Capture release id
        id: get_release
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          set -euo pipefail
          RELEASE_ID=$(gh release view "$RELEASE_TAG" --json databaseId --jq '.databaseId')
          echo "release_id=$RELEASE_ID" >> "$GITHUB_OUTPUT"

steps:
  - name: Setup environment and fetch release data
    env:
      RELEASE_ID: ${{ needs.release.outputs.release_id }}
      RELEASE_TAG: ${{ needs.config.outputs.release_tag }}
      GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
    run: |
      set -euo pipefail

      mkdir -p /tmp/gh-aw/release-data
      echo "RELEASE_TAG=$RELEASE_TAG" >> "$GITHUB_ENV"

      gh api "/repos/${{ github.repository }}/releases/$RELEASE_ID" > /tmp/gh-aw/release-data/current_release.json

      PREV_RELEASE_TAG=$(gh release list \
        --exclude-drafts \
        --limit 20 \
        --json tagName \
        --jq '.[1].tagName // empty')

      if [ -z "$PREV_RELEASE_TAG" ]; then
        echo "PREV_RELEASE_TAG=" >> "$GITHUB_ENV"
        echo "[]" > /tmp/gh-aw/release-data/pull_requests.json
        echo "{}" > /tmp/gh-aw/release-data/compare.json
      else
        echo "PREV_RELEASE_TAG=$PREV_RELEASE_TAG" >> "$GITHUB_ENV"

        PREV_PUBLISHED_AT=$(gh release view "$PREV_RELEASE_TAG" --json publishedAt --jq .publishedAt)
        CURR_PUBLISHED_AT=$(gh release view "$RELEASE_TAG" --json publishedAt --jq .publishedAt)

        gh pr list \
          --state merged \
          --limit 1000 \
          --json number,title,author,labels,mergedAt,url,body \
          --jq "[.[] | select(.mergedAt >= \"$PREV_PUBLISHED_AT\" and .mergedAt <= \"$CURR_PUBLISHED_AT\")]" \
          > /tmp/gh-aw/release-data/pull_requests.json

        gh api "/repos/${{ github.repository }}/compare/${PREV_RELEASE_TAG}...${RELEASE_TAG}" \
          > /tmp/gh-aw/release-data/compare.json
      fi

      if ! gh issue list \
        --state all \
        --limit 300 \
        --json number,title,labels,closedAt,url,author \
        > /tmp/gh-aw/release-data/issues.json; then
        echo "[]" > /tmp/gh-aw/release-data/issues.json
      fi

      if [ -f "CHANGELOG.md" ]; then
        cp CHANGELOG.md /tmp/gh-aw/release-data/CHANGELOG.md
      fi

      find docs -type f -name "*.md" 2>/dev/null > /tmp/gh-aw/release-data/docs_files.txt || true
---

# Release Highlights Generator

Generate an engaging release highlights summary for **${{ github.repository }}**
release `${RELEASE_TAG}`.

**Release ID**: ${{ needs.release.outputs.release_id }}

## Data Available

All data is pre-fetched in `/tmp/gh-aw/release-data/`:
- `current_release.json` - Release metadata and existing generated notes
- `pull_requests.json` - PRs merged between `${PREV_RELEASE_TAG}` and
  `${RELEASE_TAG}`
- `compare.json` - Commit comparison between previous and current tags
- `issues.json` - Repository issues for optional cross-reference
- `CHANGELOG.md` - Changelog context (if present)
- `docs_files.txt` - Markdown documentation files in this repository

## Objective

Create a **"🌟 Release Highlights"** section to prepend to the existing release
notes so users can quickly understand what changed and why it matters.

The highlights should be:
- User-impact focused, not a raw changelog dump
- Concise and scannable in under one minute
- Accurate and linked (PRs/issues/docs) where useful

## Workflow

### 1. Load and Inspect Inputs

Use shell commands to inspect the pre-fetched files before writing any output.

### 2. Determine What Actually Matters to Users

Prioritize:
- New CLI capabilities, resources, flags, or behaviors
- Bug fixes that unblock common workflows
- Breaking or behavior-changing updates
- DX/documentation improvements that materially help users

De-prioritize or omit:
- Internal-only refactors with no user impact
- CI-only or maintenance-only noise unless significant

### 3. Categorize Changes

Use only relevant sections:
- `⚠️ Breaking Changes` (first when present)
- `✨ What's New`
- `🐛 Fixes & Improvements`
- `📚 Docs & DX`

When helpful, include short command examples in fenced `bash` blocks.

### 4. Community Acknowledgements

If contributor PRs are present, include a short thanks section with links.
Only include this section when there is meaningful community activity.

### 5. First-Release or Low-Change Cases

If this appears to be the first release or has very small surface area,
produce a short, accurate summary rather than forcing all sections.

## Output Requirements

You MUST call the `safeoutputs/update_release` MCP tool exactly once:
- `tag`: `${RELEASE_TAG}`
- `operation`: `prepend`
- `body`: full markdown for the highlights section

The body should begin with:

```markdown
## 🌟 Release Highlights
```

End with a divider and a short pointer to full release notes, for example:

```markdown
---
For full details, review the generated changelog entries below.
```

If there are no meaningful user-facing changes, still prepend a concise
maintenance summary instead of skipping the update.
