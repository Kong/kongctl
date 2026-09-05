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
      recovery_tag:
        description: Existing stable release tag to verify and finish, for example v1.15.0
        default: ""
        required: false
        type: string
permissions:
  contents: read
  id-token: write
  pull-requests: read
  actions: read
  issues: read
engine:
  id: claude
  auth:
    type: github-oidc
    provider: anthropic
    federation-rule-id: fdrl_01Y3KFTKUynh4mumc1tNKArZ
    organization-id: 4ce7a6d3-9549-4842-bd51-1def5eba611b
    service-account-id: svac_017oc62PsXm82aqHWzHYgjfM
    workspace-id: wrkspc_01G7dX83HGYMZDwLuJNPnA5T
model: claude-opus-4-6
strict: true
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
      artifact_mode: ${{ steps.compute_config.outputs.artifact_mode }}
      build_mode: ${{ steps.compute_config.outputs.build_mode }}
      release_tag: ${{ steps.compute_config.outputs.release_tag }}
      release_version: ${{ steps.compute_config.outputs.release_version }}
    steps:
      - name: Harden Runner
        uses: step-security/harden-runner@6c3c2f2c1c457b00c10c4848d6f5491db3b629df # v2.18.0
        with:
          egress-policy: audit
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
            const recoveryTag = (context.payload.inputs.recovery_tag || "").trim();
            const requestedBuildMode = context.payload.inputs.build_mode || "full";

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

            if (recoveryTag) {
              const recoveryVersion = parseSemver(recoveryTag);
              if (!recoveryTag.startsWith("v") || !recoveryVersion) {
                core.setFailed(`Recovery tag must be a stable v-prefixed semver tag: ${recoveryTag}`);
                return;
              }

              let release;
              try {
                ({ data: release } = await github.rest.repos.getReleaseByTag({
                  owner: context.repo.owner,
                  repo: context.repo.repo,
                  tag: recoveryTag,
                }));
              } catch (error) {
                if (error.status === 404) {
                  core.setFailed(`Release ${recoveryTag} does not exist.`);
                  return;
                }
                throw error;
              }

              if (release.draft || release.prerelease || !release.published_at) {
                core.setFailed(`Release ${recoveryTag} is not a published stable release.`);
                return;
              }

              const releaseVersion = recoveryTag.slice(1);
              const assetNames = new Set(release.assets.map((asset) => asset.name));
              const fullAssets = [
                "checksums.txt",
                "kongctl_darwin_amd64.zip",
                "kongctl_darwin_arm64.zip",
                "kongctl_linux_amd64.zip",
                "kongctl_linux_arm64.zip",
                "kongctl_windows_amd64.zip",
                "kongctl_windows_arm64.zip",
              ];
              let artifactMode;
              if (fullAssets.every((asset) => assetNames.has(asset))) {
                artifactMode = "full";
              } else if (assetNames.has(`kongctl-${releaseVersion}-linux-amd64-smoke.tar.gz`)) {
                artifactMode = "smoke";
              } else {
                core.setFailed(`Unable to determine the artifact mode for release ${recoveryTag}.`);
                return;
              }

              try {
                await github.rest.git.getRef({
                  owner: context.repo.owner,
                  repo: context.repo.repo,
                  ref: `tags/${recoveryTag}`,
                });
              } catch (error) {
                if (error.status === 404) {
                  core.setFailed(`Git tag ${recoveryTag} does not exist.`);
                  return;
                }
                throw error;
              }

              core.setOutput("build_mode", "recovery");
              core.setOutput("artifact_mode", artifactMode);
              core.setOutput("release_tag", recoveryTag);
              core.setOutput("release_version", releaseVersion);
              console.log(`✓ Recovering existing ${artifactMode} release ${recoveryTag}`);
              return;
            }

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
            const repository = `${context.repo.owner}/${context.repo.repo}`.toLowerCase();
            const buildMode = repository.includes("trial")
              ? "smoke"
              : requestedBuildMode;
            if (!["full", "smoke"].includes(buildMode)) {
              core.setFailed(`Unsupported build mode: ${buildMode}`);
              return;
            }
            core.setOutput("build_mode", buildMode);
            core.setOutput("artifact_mode", buildMode);
            console.log(`✓ Release tag: ${releaseTag}`);

  create_tag:
    needs: ["config"]
    runs-on: ubuntu-latest
    permissions:
      contents: write
    env:
      RELEASE_TAG: ${{ needs.config.outputs.release_tag }}
      RELEASE_BUILD_MODE: ${{ needs.config.outputs.build_mode }}
    steps:
      - name: Harden Runner
        uses: step-security/harden-runner@6c3c2f2c1c457b00c10c4848d6f5491db3b629df # v2.18.0
        with:
          egress-policy: audit
      - name: Checkout repository
        if: env.RELEASE_BUILD_MODE != 'recovery'
        uses: actions/checkout@v6
        with:
          fetch-depth: 0
          persist-credentials: true

      - name: Reuse existing tag (recovery mode)
        if: env.RELEASE_BUILD_MODE == 'recovery'
        run: |
          echo "Recovery mode will reuse $RELEASE_TAG"

      - name: Create and push tag
        if: env.RELEASE_BUILD_MODE != 'recovery'
        run: |
          set -euo pipefail

          git fetch origin --tags
          if git rev-parse "refs/tags/$RELEASE_TAG" >/dev/null 2>&1; then
            tag_commit=$(git rev-list -n 1 "refs/tags/$RELEASE_TAG")
            head_commit=$(git rev-parse HEAD)
            if [[ "$tag_commit" == "$head_commit" ]]; then
              echo "Tag $RELEASE_TAG already points to $head_commit"
              exit 0
            fi
            echo "::error::Tag $RELEASE_TAG already points to $tag_commit, not $head_commit"
            exit 1
          fi

          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git tag -a "$RELEASE_TAG" -m "Release $RELEASE_TAG"
          git push origin "$RELEASE_TAG"

  publish_release:
    needs: ["config", "create_tag"]
    runs-on: ubuntu-latest
    permissions:
      contents: write
      packages: write
    env:
      RELEASE_TAG: ${{ needs.config.outputs.release_tag }}
      RELEASE_VERSION: ${{ needs.config.outputs.release_version }}
      RELEASE_BUILD_MODE: ${{ needs.config.outputs.build_mode }}
      DOCKER_USERNAME: ${{ secrets.DOCKER_USERNAME }}
    steps:
      - name: Harden Runner
        uses: step-security/harden-runner@6c3c2f2c1c457b00c10c4848d6f5491db3b629df # v2.18.0
        with:
          egress-policy: audit
      - name: Checkout repository
        if: env.RELEASE_BUILD_MODE != 'recovery'
        uses: actions/checkout@v6
        with:
          fetch-depth: 0
          persist-credentials: false

      - name: Reuse existing release (recovery mode)
        if: env.RELEASE_BUILD_MODE == 'recovery'
        run: |
          echo "Recovery mode will reuse release $RELEASE_TAG"

      - name: Configure private git reads for GoReleaser
        if: env.RELEASE_BUILD_MODE != 'recovery'
        env:
          GH_PRIVATE_READ_TOKEN: ${{ secrets.GH_TOKEN_PRIVATE_READ }}
        run: |
          set -euo pipefail
          if [ -n "${GH_PRIVATE_READ_TOKEN:-}" ]; then
            git config --global url."https://${GH_PRIVATE_READ_TOKEN}@github.com/".insteadOf https://github.com/
          else
            echo "GH_TOKEN_PRIVATE_READ not set; using default git auth"
          fi

      - name: Setup Go (full mode)
        if: env.RELEASE_BUILD_MODE == 'full'
        uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
          cache: false

      - name: Set up QEMU (full mode)
        if: env.RELEASE_BUILD_MODE == 'full'
        uses: docker/setup-qemu-action@c7c53464625b32c7a7e944ae62b3e17d2b600130 # v3

      - name: Set up Docker Buildx (full mode)
        if: env.RELEASE_BUILD_MODE == 'full'
        uses: docker/setup-buildx-action@8d2750c68a42422c14e847fe6c8ac0403b4cbd6f # v3

      - name: Login to GHCR (full mode)
        if: env.RELEASE_BUILD_MODE == 'full'
        uses: docker/login-action@c94ce9fb468520275223c153574b00df6fe4bcc9 # v3.7.0
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Login to Docker Hub (full mode)
        if: env.RELEASE_BUILD_MODE == 'full' && env.DOCKER_USERNAME != ''
        uses: docker/login-action@c94ce9fb468520275223c153574b00df6fe4bcc9 # v3.7.0
        with:
          username: ${{ secrets.DOCKER_USERNAME }}
          password: ${{ secrets.DOCKER_TOKEN }}

      - name: Run GoReleaser (full mode)
        if: env.RELEASE_BUILD_MODE == 'full'
        id: goreleaser
        uses: goreleaser/goreleaser-action@e435ccd777264be153ace6237001ef4d979d3a7a # v6.4.0
        with:
          distribution: goreleaser
          version: v2.13.3
          args: release --clean --parallelism=1
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          TAP_GITHUB_TOKEN: ${{ secrets.TAP_GITHUB_TOKEN }}
          CGO_ENABLED: "0"

      - name: Stage Homebrew release metadata
        if: env.RELEASE_BUILD_MODE == 'full'
        env:
          GORELEASER_METADATA: ${{ steps.goreleaser.outputs.metadata }}
        run: |
          set -euo pipefail
          mkdir -p dist/homebrew
          jq -e . <<<"$GORELEASER_METADATA" > dist/homebrew/goreleaser-metadata.json

      - name: Upload generated Homebrew files
        if: env.RELEASE_BUILD_MODE == 'full'
        uses: actions/upload-artifact@v7.0.1
        with:
          name: homebrew-${{ needs.config.outputs.release_tag }}
          path: dist/homebrew/
          if-no-files-found: error
          overwrite: true
          retention-days: 7

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

  publish_homebrew:
    needs: ["config", "publish_release"]
    runs-on: ubuntu-latest
    permissions:
      actions: read
      contents: read
    env:
      RELEASE_ARTIFACT_MODE: ${{ needs.config.outputs.artifact_mode }}
      RELEASE_TAG: ${{ needs.config.outputs.release_tag }}
      RELEASE_VERSION: ${{ needs.config.outputs.release_version }}
      RELEASE_BUILD_MODE: ${{ needs.config.outputs.build_mode }}
    steps:
      - name: Harden Runner
        uses: step-security/harden-runner@6c3c2f2c1c457b00c10c4848d6f5491db3b629df # v2.18.0
        with:
          egress-policy: audit
      - name: Checkout repository
        if: env.RELEASE_BUILD_MODE == 'full'
        uses: actions/checkout@v6
        with:
          persist-credentials: false

      - name: Reuse existing Homebrew publication (recovery mode)
        if: env.RELEASE_BUILD_MODE == 'recovery' && env.RELEASE_ARTIFACT_MODE == 'full'
        run: |
          echo "Recovery mode will verify the existing Homebrew publication for $RELEASE_TAG"

      - name: Skip Homebrew publication (smoke mode)
        if: env.RELEASE_ARTIFACT_MODE == 'smoke'
        run: |
          echo "Smoke mode does not publish Homebrew files"

      - name: Download generated Homebrew files
        if: env.RELEASE_BUILD_MODE == 'full'
        uses: actions/download-artifact@v8.0.1
        with:
          name: homebrew-${{ needs.config.outputs.release_tag }}
          path: dist/homebrew

      - name: Set up Homebrew
        if: env.RELEASE_BUILD_MODE == 'full'
        uses: Homebrew/actions/setup-homebrew@cced187498280712e078aaba62dc13a3e9cd80bf

      - name: Fetch Homebrew tap
        if: env.RELEASE_BUILD_MODE == 'full'
        env:
          TAP_GITHUB_TOKEN: ${{ secrets.TAP_GITHUB_TOKEN }}
        run: |
          set -euo pipefail
          tap_path="$(brew --repository)/Library/Taps/kong/homebrew-kongctl"
          rm -rf "$tap_path"
          mkdir -p "$(dirname "$tap_path")"
          tap_url="https://x-access-token:${TAP_GITHUB_TOKEN}@github.com"
          git clone --depth 1 \
            "${tap_url}/kong/homebrew-kongctl.git" \
            "$tap_path"
          echo "HOMEBREW_KONGCTL_TAP=$tap_path" >> "$GITHUB_ENV"

      - name: Update Homebrew packages
        if: env.RELEASE_BUILD_MODE == 'full'
        run: |
          set -euo pipefail

          generated_cask=dist/homebrew/Casks/kongctl.rb
          metadata_file=dist/homebrew/goreleaser-metadata.json
          if [[ ! -f "$generated_cask" || ! -f "$metadata_file" ]]; then
            echo "::error::Homebrew artifact is missing generated release data"
            exit 1
          fi

          metadata_version=$(jq -r '.version' "$metadata_file")
          metadata_commit=$(jq -r '.commit[0:8]' "$metadata_file")
          metadata_date=$(jq -r '.date' "$metadata_file")
          if [[ "$metadata_version" != "$RELEASE_VERSION" ]]; then
            echo "::error::GoReleaser metadata version $metadata_version" \
              "does not match $RELEASE_VERSION"
            exit 1
          fi

          mkdir -p "$HOMEBREW_KONGCTL_TAP/Casks"
          cp "$generated_cask" "$HOMEBREW_KONGCTL_TAP/Casks/kongctl.rb"
          ./scripts/update-homebrew-formula.sh \
            "$HOMEBREW_KONGCTL_TAP/Formula/kongctl.rb" \
            "$metadata_version" \
            "$metadata_commit" \
            "$metadata_date"

      - name: Fix Homebrew tap style
        if: env.RELEASE_BUILD_MODE == 'full'
        run: ./scripts/fix-homebrew-tap.sh "$HOMEBREW_KONGCTL_TAP"

      - name: Validate Homebrew packages
        if: env.RELEASE_BUILD_MODE == 'full'
        run: |
          set -euo pipefail
          brew audit --new --formula kong/kongctl/kongctl
          brew install --cask kong/kongctl/kongctl
          kongctl version --full
          brew uninstall --cask kong/kongctl/kongctl
          brew install --formula --build-from-source kong/kongctl/kongctl
          brew test kong/kongctl/kongctl
          kongctl version --full

      - name: Open Homebrew release pull request
        id: homebrew_pr
        if: env.RELEASE_BUILD_MODE == 'full'
        env:
          GH_TOKEN: ${{ secrets.TAP_GITHUB_TOKEN }}
          TAP_GITHUB_TOKEN: ${{ secrets.TAP_GITHUB_TOKEN }}
        run: |
          set -euo pipefail
          cd "$HOMEBREW_KONGCTL_TAP"
          git config user.name "kongctl Release Bot"
          git config user.email "kongctl@konghq.com"
          tap_url="https://x-access-token:${TAP_GITHUB_TOKEN}@github.com"
          git remote set-url origin \
            "${tap_url}/kong/homebrew-kongctl.git"

          if [[ -z "$(git status --porcelain -- Casks/kongctl.rb Formula/kongctl.rb)" ]]; then
            echo "The Homebrew tap already publishes kongctl $RELEASE_VERSION"
            echo "number=" >> "$GITHUB_OUTPUT"
            exit 0
          fi

          branch="release/kongctl-${RELEASE_VERSION}"
          if git ls-remote --exit-code origin "refs/heads/$branch" >/dev/null 2>&1; then
            git fetch origin "$branch"
            if ! git diff --quiet FETCH_HEAD -- Casks/kongctl.rb Formula/kongctl.rb; then
              echo "::error::Existing $branch does not match this release"
              exit 1
            fi

            pr_number=$(gh pr list \
              --repo Kong/homebrew-kongctl \
              --head "$branch" \
              --state open \
              --json number \
              --jq '.[0].number // empty')
            if [[ -z "$pr_number" ]]; then
              echo "::error::Existing $branch has no open pull request"
              exit 1
            fi
          else
            git switch --create "$branch"
            git add Casks/kongctl.rb Formula/kongctl.rb
            git commit -m "homebrew: publish kongctl ${RELEASE_VERSION}"
            git push --set-upstream origin "$branch"

            pr_body="Publish the cask and source formula for kongctl ${RELEASE_VERSION}. "
            pr_body+="Native Homebrew CI builds the formula bottles before "
            pr_body+="brew pr-pull publishes this change."
            pr_url=$(gh pr create \
              --repo Kong/homebrew-kongctl \
              --base main \
              --head "$branch" \
              --title "homebrew: publish kongctl ${RELEASE_VERSION}" \
              --body "$pr_body")
            pr_number=${pr_url##*/}
          fi

          echo "number=$pr_number" >> "$GITHUB_OUTPUT"
          echo "Homebrew release PR: https://github.com/Kong/homebrew-kongctl/pull/$pr_number"

      - name: Wait for Homebrew bottle builds
        if: env.RELEASE_BUILD_MODE == 'full' && steps.homebrew_pr.outputs.number != ''
        env:
          GH_TOKEN: ${{ secrets.TAP_GITHUB_TOKEN }}
          PR_NUMBER: ${{ steps.homebrew_pr.outputs.number }}
        run: |
          set -euo pipefail

          for attempt in {1..30}; do
            bottle_check_count=$(gh pr view "$PR_NUMBER" \
              --repo Kong/homebrew-kongctl \
              --json statusCheckRollup \
              --jq '[.statusCheckRollup[] | select(.name | startswith("test-bot ("))] | length')
            if (( bottle_check_count >= 3 )); then
              break
            fi
            if (( attempt == 30 )); then
              echo "::error::Native bottle checks did not start for Homebrew PR $PR_NUMBER"
              exit 1
            fi
            sleep 10
          done

          gh pr checks "$PR_NUMBER" \
            --repo Kong/homebrew-kongctl \
            --watch \
            --fail-fast \
            --interval 15

      - name: Publish Homebrew bottles
        if: env.RELEASE_BUILD_MODE == 'full' && steps.homebrew_pr.outputs.number != ''
        env:
          GH_TOKEN: ${{ secrets.TAP_GITHUB_TOKEN }}
          PR_NUMBER: ${{ steps.homebrew_pr.outputs.number }}
        run: |
          set -euo pipefail

          gh label create pr-pull \
            --repo Kong/homebrew-kongctl \
            --color 0E8A16 \
            --description "Publish tested Homebrew bottles" \
            --force

          if gh pr view "$PR_NUMBER" \
            --repo Kong/homebrew-kongctl \
            --json labels \
            --jq '.labels[].name' | grep -Fqx pr-pull; then
            gh pr edit "$PR_NUMBER" \
              --repo Kong/homebrew-kongctl \
              --remove-label pr-pull
          fi
          gh pr edit "$PR_NUMBER" \
            --repo Kong/homebrew-kongctl \
            --add-label pr-pull

          for attempt in {1..160}; do
            pr_state=$(gh pr view "$PR_NUMBER" \
              --repo Kong/homebrew-kongctl \
              --json state,mergedAt \
              --jq 'if .mergedAt then "merged" else (.state | ascii_downcase) end')
            case "$pr_state" in
              merged)
                echo "Homebrew PR $PR_NUMBER published with bottles"
                exit 0
                ;;
              closed)
                echo "::error::Homebrew PR $PR_NUMBER closed without merging"
                exit 1
                ;;
            esac
            sleep 15
          done

          echo "::error::Timed out waiting for Homebrew PR $PR_NUMBER to publish"
          exit 1

  release_complete:
    needs: ["config", "publish_release", "publish_homebrew"]
    runs-on: ubuntu-latest
    permissions:
      contents: read
    outputs:
      release_id: ${{ steps.verify_release.outputs.release_id }}
    env:
      RELEASE_ARTIFACT_MODE: ${{ needs.config.outputs.artifact_mode }}
      RELEASE_TAG: ${{ needs.config.outputs.release_tag }}
      RELEASE_VERSION: ${{ needs.config.outputs.release_version }}
      RELEASE_BUILD_MODE: ${{ needs.config.outputs.build_mode }}
    steps:
      - name: Harden Runner
        uses: step-security/harden-runner@6c3c2f2c1c457b00c10c4848d6f5491db3b629df # v2.18.0
        with:
          egress-policy: audit
      - name: Checkout repository
        uses: actions/checkout@v6
        with:
          fetch-depth: 0
          persist-credentials: false

      - name: Verify completed release
        id: verify_release
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          set -euo pipefail

          git fetch origin --tags
          tag_commit=$(git rev-list -n 1 "$RELEASE_TAG")
          if [[ -z "$tag_commit" ]]; then
            echo "::error::Unable to resolve $RELEASE_TAG to a commit"
            exit 1
          fi
          if ! git merge-base --is-ancestor "$tag_commit" HEAD; then
            echo "::error::$RELEASE_TAG does not point to a commit on the workflow ref"
            exit 1
          fi

          release_json=$(gh api "repos/$GITHUB_REPOSITORY/releases/tags/$RELEASE_TAG")
          if [[ "$(jq -r '.draft or .prerelease or (.published_at == null)' <<< "$release_json")" == "true" ]]; then
            echo "::error::Release $RELEASE_TAG is not a published stable release"
            exit 1
          fi

          verification_dir="dist/verification"
          mkdir -p "$verification_dir"
          gh release download "$RELEASE_TAG" --dir "$verification_dir"

          if [[ "$RELEASE_ARTIFACT_MODE" == "smoke" ]]; then
            smoke_asset="kongctl-${RELEASE_TAG#v}-linux-amd64-smoke.tar.gz"
            if [[ ! -f "$verification_dir/$smoke_asset" ]]; then
              echo "::error::Release $RELEASE_TAG is missing $smoke_asset"
              exit 1
            fi
          else
            expected_assets=(
              checksums.txt
              kongctl_darwin_amd64.zip
              kongctl_darwin_arm64.zip
              kongctl_linux_amd64.zip
              kongctl_linux_arm64.zip
              kongctl_windows_amd64.zip
              kongctl_windows_arm64.zip
            )
            for asset in "${expected_assets[@]}"; do
              if [[ ! -f "$verification_dir/$asset" ]]; then
                echo "::error::Release $RELEASE_TAG is missing $asset"
                exit 1
              fi
            done

            pushd "$verification_dir" >/dev/null
            sha256sum --check checksums.txt
            popd >/dev/null

            gh api repos/Kong/homebrew-kongctl/contents/Casks/kongctl.rb \
              --jq .content | base64 --decode > "$verification_dir/kongctl.rb"
            if ! grep -Fqx "  version \"$RELEASE_VERSION\"" "$verification_dir/kongctl.rb"; then
              echo "::error::Homebrew cask is not at $RELEASE_VERSION"
              exit 1
            fi

            gh api repos/Kong/homebrew-kongctl/contents/Formula/kongctl.rb \
              --jq .content | base64 --decode > "$verification_dir/kongctl-formula.rb"
            source_url="  url \"https://github.com/Kong/kongctl/archive/refs/tags/v${RELEASE_VERSION}.tar.gz\""
            if ! grep -Fqx "$source_url" "$verification_dir/kongctl-formula.rb"; then
              echo "::error::Homebrew formula is not at $RELEASE_VERSION"
              exit 1
            fi
            if ! grep -Fqx "  bottle do" "$verification_dir/kongctl-formula.rb"; then
              echo "::error::Homebrew formula does not publish bottles for $RELEASE_VERSION"
              exit 1
            fi

            while read -r checksum archive; do
              case "$archive" in
                kongctl_darwin_*.zip|kongctl_linux_*.zip)
                  if ! grep -Fq "sha256 \"$checksum\"" "$verification_dir/kongctl.rb"; then
                    echo "::error::Homebrew cask is missing the checksum for $archive"
                    exit 1
                  fi
                  ;;
              esac
            done < "$verification_dir/checksums.txt"
          fi

          release_id=$(jq -r .id <<< "$release_json")
          echo "release_id=$release_id" >> "$GITHUB_OUTPUT"
          echo "✓ Verified completed release $RELEASE_TAG at $tag_commit"

steps:
  - name: Setup environment and fetch release data
    env:
      RELEASE_ID: ${{ needs.release_complete.outputs.release_id }}
      RELEASE_TAG: ${{ needs.config.outputs.release_tag }}
      GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
      EXPR_GITHUB_REPOSITORY: ${{ github.repository }}
    run: |
      set -euo pipefail

      mkdir -p /tmp/gh-aw/release-data
      echo "RELEASE_TAG=$RELEASE_TAG" >> "$GITHUB_ENV"

      gh api "/repos/$EXPR_GITHUB_REPOSITORY/releases/$RELEASE_ID" > /tmp/gh-aw/release-data/current_release.json

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

        gh api "/repos/$EXPR_GITHUB_REPOSITORY/compare/${PREV_RELEASE_TAG}...${RELEASE_TAG}" \
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

**Release ID**: ${{ needs.release_complete.outputs.release_id }}

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

Generate complete release notes that replace the existing release content,
so users can quickly understand what changed and why it matters.

**Important tool usage notes:**
- All required GitHub release, PR, issue, and compare data has already been
  pre-fetched into `/tmp/gh-aw/release-data/`.
- Use local shell commands only to inspect those pre-fetched files and relevant
  repository files.
- Do NOT use `gh` CLI commands or make additional GitHub API calls from the
  agent. The agent should operate only on the pre-fetched local data.
- Use the `update_release` safe-output tool exactly once for the final write.

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
- `operation`: `replace`
- `body`: full markdown for the complete release notes (highlights + all commit references)
- Mark the release as the latest

The body should begin with:

```markdown
## <img src="https://raw.githubusercontent.com/Kong/kongctl/main/brand/logo/light/Kong-Logomark.svg#gh-light-mode-only" alt="Kong logo" width="20" /> <img src="https://raw.githubusercontent.com/Kong/kongctl/main/brand/logo/dark/Kong-Logomark.svg#gh-dark-mode-only" alt="Kong logo" width="20" /> kongctl Release Highlights
```

The `<img>` tags in the opening heading are intentional and must be preserved so
the theme-aware Kong logo renders in the GitHub release body. Avoid other raw
HTML unless it is required for GitHub-flavored Markdown rendering.

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

End with a divider, for example:

```markdown
---
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

If there are no meaningful user-facing changes, still replace with a concise
maintenance summary instead of skipping the update.
