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
        default: patch
        required: false
        type: choice
        options:
          - patch
          - minor
          - major
      build_mode:
        description: Build mode (full release or smoke test)
        default: full
        required: false
        type: choice
        options:
          - full
          - smoke
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
    permissions:
      contents: read
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
            const releaseType = context.payload.inputs.release_type || "patch";

            console.log(`Computing next version for release type: ${releaseType}`);

            const { data: releases } = await github.rest.repos.listReleases({
              owner: context.repo.owner,
              repo: context.repo.repo,
              per_page: 100,
            });

            const { data: tags } = await github.rest.repos.listTags({
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

            const sortedVersions = [...new Set(
              [
                ...releases.filter((r) => !r.draft).map((r) => r.tag_name),
                ...tags.map((t) => t.name),
              ],
            )]
              .map((tag) => parseSemver(tag))
              .filter((v) => v !== null)
              .sort((a, b) => {
                if (a.major !== b.major) return b.major - a.major;
                if (a.minor !== b.minor) return b.minor - a.minor;
                return b.patch - a.patch;
              });

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

            if (sortedVersions.length > 0) {
              const latestTag = sortedVersions[0].tag;
              const version = latestTag.replace(/^v/, "");
              [major, minor, patch] = version.split(".").map(Number);
              console.log(`Latest stable version from releases/tags: ${latestTag}`);
            } else {
              console.log("No prior stable release or tag found, using base 0.0.0");
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

      - name: Export release variables
        run: |
          set -euo pipefail
          echo "RELEASE_TAG=${{ needs.config.outputs.release_tag }}" >> "$GITHUB_ENV"
          echo "RELEASE_VERSION=${{ needs.config.outputs.release_version }}" >> "$GITHUB_ENV"

      - name: Determine build mode
        env:
          REQUESTED_BUILD_MODE: ${{ github.event.inputs.build_mode || 'full' }}
        run: |
          set -euo pipefail

          RELEASE_BUILD_MODE="${REQUESTED_BUILD_MODE}"
          if [[ "${GITHUB_REPOSITORY,,}" == *"trial"* ]]; then
            RELEASE_BUILD_MODE="smoke"
          fi

          case "$RELEASE_BUILD_MODE" in
            full|smoke) ;;
            *)
              echo "::error::Unsupported build mode: $RELEASE_BUILD_MODE"
              exit 1
              ;;
          esac

          echo "RELEASE_BUILD_MODE=$RELEASE_BUILD_MODE" >> "$GITHUB_ENV"

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
        if: env.RELEASE_BUILD_MODE == 'full'
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
            goreleaser/goreleaser-cross:v1.25.3@sha256:f37a98cb1f3543c595e6d70808044f0f221ae5522911357d40cbde81d05e3786 \
            -c "set -e; git config --global --add safe.directory /work; goreleaser release --clean; if [ -d /work/dist ]; then chown -R \${HOST_UID:-0}:\${HOST_GID:-0} /work/dist; fi"

      - name: Set up Homebrew
        if: env.RELEASE_BUILD_MODE == 'full'
        uses: Homebrew/actions/setup-homebrew@cced187498280712e078aaba62dc13a3e9cd80bf

      - name: Fetch Homebrew tap
        if: env.RELEASE_BUILD_MODE == 'full'
        env:
          TAP_GITHUB_TOKEN: ${{ secrets.TAP_GITHUB_TOKEN }}
        run: |
          set -euo pipefail
          rm -rf dist/homebrew-tap
          mkdir -p dist/homebrew-tap
          git clone --depth 1 "https://x-access-token:${TAP_GITHUB_TOKEN}@github.com/kong/homebrew-kongctl.git" dist/homebrew-tap/homebrew-kongctl

      - name: Sync generated Homebrew files
        if: env.RELEASE_BUILD_MODE == 'full'
        run: |
          set -euo pipefail
          mkdir -p dist/homebrew-tap/homebrew-kongctl/Casks
          rm -f dist/homebrew-tap/homebrew-kongctl/kongctl.rb
          if [[ -f dist/homebrew/Casks/kongctl.rb ]]; then
            cp dist/homebrew/Casks/kongctl.rb dist/homebrew-tap/homebrew-kongctl/Casks/kongctl.rb
          fi

      - name: Fix Homebrew tap style
        if: env.RELEASE_BUILD_MODE == 'full'
        run: ./scripts/fix-homebrew-tap.sh dist/homebrew-tap/homebrew-kongctl

      - name: Commit and push Homebrew tap updates
        if: env.RELEASE_BUILD_MODE == 'full'
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

      - name: Setup Go (smoke mode)
        if: env.RELEASE_BUILD_MODE == 'smoke'
        uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
          cache: false

      - name: Build smoke artifact
        if: env.RELEASE_BUILD_MODE == 'smoke'
        run: |
          set -euo pipefail

          mkdir -p dist
          COMMIT="$(git rev-parse --short HEAD)"
          BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
          SMOKE_BIN="kongctl-linux-amd64"
          SMOKE_TAR="kongctl-${RELEASE_TAG#v}-linux-amd64-smoke.tar.gz"

          CGO_ENABLED=0 go build \
            -trimpath \
            -ldflags="-s -w -X main.version=${RELEASE_VERSION} -X main.commit=${COMMIT} -X main.date=${BUILD_DATE}" \
            -o "dist/${SMOKE_BIN}" \
            .

          tar -czf "dist/${SMOKE_TAR}" -C dist "${SMOKE_BIN}"

      - name: Create GitHub release (smoke mode)
        if: env.RELEASE_BUILD_MODE == 'smoke'
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          set -euo pipefail

          SMOKE_TAR="dist/kongctl-${RELEASE_TAG#v}-linux-amd64-smoke.tar.gz"
          if gh release view "$RELEASE_TAG" >/dev/null 2>&1; then
            gh release upload "$RELEASE_TAG" "$SMOKE_TAR" --clobber
          else
            gh release create "$RELEASE_TAG" "$SMOKE_TAR" \
              --title "$RELEASE_TAG" \
              --generate-notes
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

Create a **"kongctl Release Highlights"** section to prepend to the existing release
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

### 4. Build Commit Reference List

Use `compare.json` to generate a complete commit reference section with direct
links, sorted in this order:
1. Features
2. Fixes
3. Other changes

```bash
cat /tmp/gh-aw/release-data/compare.json | jq -r '
  def subject: ((.commit.message // "") | split("\n")[0]);
  def category:
    (subject | ascii_downcase) as $s |
    if ($s | test("^(feat|feature)(\\(|:|\\b)")) or ($s | test("^(add|introduce)\\b")) then "feature"
    elif ($s | test("^(fix|bugfix|hotfix)(\\(|:|\\b)")) or ($s | test("\\bfix(es|ed)?\\b")) then "fix"
    else "other" end;
  [(.commits // [])[] | {
    sha,
    html_url,
    date: (.commit.author.date // ""),
    subject: subject,
    category: category
  }]
  | sort_by(.date)
  | reverse
  | ([.[] | select(.category == "feature")]
     + [.[] | select(.category == "fix")]
     + [.[] | select(.category == "other")])
  | .[]
  | "- [`\(.sha[0:7])`](\(.html_url)) \(.subject)"'
```

Requirements for this section:
- Include all commits from `compare.json` when present.
- Keep each line to one commit with a clickable short SHA link.
- Use only the first line of the commit message.
- Order the final list by category first: Features, then Fixes, then Other.
- If there are no commits, omit this section.

### 5. Community Acknowledgements

If contributor PRs are present, include a short thanks section with links.
Only include this section when there is meaningful community activity.

### 6. First-Release or Low-Change Cases

If this appears to be the first release or has very small surface area,
produce a short, accurate summary rather than forcing all sections.

## Output Requirements

You MUST call the `safeoutputs/update_release` MCP tool exactly once:
- `tag`: `${RELEASE_TAG}`
- `operation`: `prepend`
- `body`: full markdown for the highlights section
- Mark the release as the latest

The body should begin with:

```markdown
## <img src="https://raw.githubusercontent.com/Kong/kongctl/main/brand/logo/dark/Kong-Logomark.svg" alt="Kong logo" width="20" /> kongctl Release Highlights
```

If HTML image rendering is unavailable in the release markdown renderer, fall
back to:

```markdown
## kongctl Release Highlights
```

When commits are available, include this section near the end:

```markdown
### 🔗 Commit References
#### Features
- [`abc1234`](https://github.com/OWNER/REPO/commit/abc1234...) Add support for X

#### Fixes
- [`def5678`](https://github.com/OWNER/REPO/commit/def5678...) Fix Y in Z flow

#### Other Changes
- [`0123abc`](https://github.com/OWNER/REPO/commit/0123abc...) Chore/doc update
```

End with a divider and a short pointer to full release notes, for example:

```markdown
---
For full details, review the generated changelog entries below.
```

When `PREV_RELEASE_TAG` is present, you MUST also include this exact style line
at the end of the generated highlights:

```markdown
Full Changelog: https://github.com/${{ github.repository }}/compare/${PREV_RELEASE_TAG}...${RELEASE_TAG}
```

When there is no previous release/tag (first release), include:

```markdown
Full Changelog: Initial release
```

If there are no meaningful user-facing changes, still prepend a concise
maintenance summary instead of skipping the update.
