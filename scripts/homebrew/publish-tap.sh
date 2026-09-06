#!/usr/bin/env bash
# The tap publishes finished metadata, never compiles or signs an executable.
set -euo pipefail
[[ $# -eq 3 ]]
kind=$1
recipe=$(cd "$(dirname "$2")" && pwd)/$(basename "$2")
version=$3
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]
case "$kind" in
  cask) target=Casks/kongctl.rb ;;
  formula) target=Formula/kongctl.rb ;;
  *) exit 2 ;;
esac
: "${GH_TOKEN:?TAP_GITHUB_TOKEN must be supplied as GH_TOKEN}"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
gh auth setup-git --hostname github.com
gh repo clone Kong/homebrew-kongctl "$work/tap" -- --depth=1 --branch main
cd "$work/tap"
git config user.name 'kongctl Release Bot'
git config user.email kongctl@konghq.com
cp "$recipe" "$target"
if git diff --quiet -- "$target"; then
  echo "$kind already publishes kongctl $version"; exit 0
fi
if [[ "$kind" == cask ]]; then
  git add "$target"
  git commit -m "task(release): publish kongctl $version cask"
  for attempt in 1 2 3; do
    if git push origin HEAD:main; then exit 0; fi
    echo "Retrying cask metadata push ($attempt/3)"
    git pull --rebase origin main
  done
  exit 1
fi
branch="release/kongctl-$version"
if git ls-remote --exit-code origin "refs/heads/$branch" > /dev/null 2>&1; then
  git fetch origin "$branch"
  git diff --exit-code FETCH_HEAD -- "$target"
else
  git switch --create "$branch"
  git add "$target"
  git commit -m "task(release): publish kongctl $version formula"
  git push origin HEAD:"$branch"
fi
pr=$(gh pr list --repo Kong/homebrew-kongctl --head "$branch" --state open --json number --jq '.[0].number // empty')
if [[ -z "$pr" ]]; then
  url=$(gh pr create --repo Kong/homebrew-kongctl --base main --head "$branch" \
    --title "task(release): publish kongctl $version formula" \
    --body 'Publish upstream binary formula metadata. Bottles were packaged, verified, and published by Kong/kongctl. Tap CI only verifies installation; its guarded publisher automatically merges this PR.')
  pr=${url##*/}
fi
head_sha=$(gh pr view "$pr" --repo Kong/homebrew-kongctl --json headRefOid --jq .headRefOid)
for attempt in {1..30}; do
  checks=$(gh pr view "$pr" --repo Kong/homebrew-kongctl --json statusCheckRollup \
    --jq '[.statusCheckRollup[] | select(.name | startswith("test-bot ("))] | length')
  if (( checks >= 3 )); then break; fi
  if (( attempt == 30 )); then echo 'Tap installation checks did not start' >&2; exit 1; fi
  sleep 10
done
gh pr checks "$pr" --repo Kong/homebrew-kongctl --watch --fail-fast --interval 15
gh workflow run publish.yml --repo Kong/homebrew-kongctl --ref main \
  --field pull_request="$pr" --field head_sha="$head_sha"
for attempt in {1..120}; do
  state=$(gh pr view "$pr" --repo Kong/homebrew-kongctl --json state --jq .state)
  if [[ "$state" == MERGED ]]; then
    gh api 'repos/Kong/homebrew-kongctl/contents/Formula/kongctl.rb?ref=main' \
      --jq .content | base64 --decode > "$work/published.rb"
    cmp "$recipe" "$work/published.rb"
    echo "Published formula through PR $pr"; exit 0
  fi
  [[ "$state" == OPEN ]] || { echo "PR $pr closed without merging" >&2; exit 1; }
  echo "Waiting for metadata publication ($attempt/120)"
  sleep 10
done
echo "Timed out publishing formula PR $pr" >&2
exit 1
