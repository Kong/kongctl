import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { spawnSync } from "node:child_process";
import test from "node:test";

// Execute the actual inline bash against a mocked `gh` so a regression in
// baseline selection or PR collection fails this test, not just a release.
const workflow = readFileSync(
  new URL("../../.github/workflows/release.md", import.meta.url),
  "utf8",
);
const script = workflow
  .split("  - name: Setup environment and fetch release data\n")[1]
  .split("    run: |\n")[1]
  .split("\n---")[0]
  .split("\n")
  .map((line) => line.slice(6))
  .join("\n");

const GH_MOCK = `#!/usr/bin/env node
const fs = require("fs");
const path = require("path");
const dir = __dirname;
const read = (name) => fs.readFileSync(path.join(dir, name));
const args = process.argv.slice(2);

if (args[0] === "api") {
  const p = args[1];
  if (/\\/releases\\/\\d+$/.test(p)) {
    process.stdout.write(read("current_release.json"));
  } else if (p.includes("/compare/")) {
    process.stdout.write(read("compare.json"));
  } else {
    process.stderr.write(\`unexpected api call: \${p}\\n\`);
    process.exit(1);
  }
} else if (args[0] === "release" && args[1] === "list") {
  process.stdout.write(read("releases.json"));
} else if (args[0] === "release" && args[1] === "view") {
  const map = JSON.parse(read("published_at_by_tag.json"));
  const tag = args[2];
  if (!(tag in map)) {
    process.stderr.write(\`unexpected release view: \${tag}\\n\`);
    process.exit(1);
  }
  process.stdout.write(map[tag] + "\\n");
} else if (args[0] === "pr" && args[1] === "list") {
  process.stdout.write(read("prs.json"));
} else if (args[0] === "issue" && args[1] === "list") {
  process.stdout.write(read("issues.json"));
} else {
  process.stderr.write(\`unexpected gh invocation: \${args.join(" ")}\\n\`);
  process.exit(1);
}
`;

function run(t, fixtures) {
  const directory = mkdtempSync(join(tmpdir(), "kongctl-release-notes-"));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  writeFileSync(join(directory, "gh"), GH_MOCK, { mode: 0o755 });
  for (const [name, content] of Object.entries(fixtures)) {
    writeFileSync(join(directory, name), typeof content === "string" ? content : JSON.stringify(content));
  }
  writeFileSync(join(directory, "issues.json"), fixtures["issues.json"] ?? "[]");

  const githubEnv = join(directory, "github-env");
  writeFileSync(githubEnv, "");

  rmSync("/tmp/gh-aw/release-data", { recursive: true, force: true });

  const result = spawnSync("bash", ["-euo", "pipefail", "-c", script], {
    cwd: directory,
    encoding: "utf8",
    env: {
      ...process.env,
      PATH: `${directory}:${process.env.PATH}`,
      GH_TOKEN: "test-token",
      EXPR_GITHUB_REPOSITORY: "Kong/kongctl",
      RELEASE_ID: fixtures.releaseId,
      RELEASE_TAG: fixtures.releaseTag,
      GITHUB_ENV: githubEnv,
    },
  });

  const releaseData = (name) => readFileSync(`/tmp/gh-aw/release-data/${name}`, "utf8");
  return {
    result,
    env: readFileSync(githubEnv, "utf8"),
    pullRequests: JSON.parse(releaseData("pull_requests.json")),
    compare: JSON.parse(releaseData("compare.json")),
  };
}

test("skips an intervening prerelease and keeps a PR present only via its commit reference", (t) => {
  const { result, env, pullRequests, compare } = run(t, {
    releaseId: "500",
    releaseTag: "v1.28.4",
    "current_release.json": { id: 500, tag_name: "v1.28.4", published_at: "2026-11-18T10:00:00Z" },
    "releases.json": [
      { tagName: "v1.28.4", isPrerelease: false, publishedAt: "2026-11-18T10:00:00Z" },
      { tagName: "prerelease-datastores", isPrerelease: true, publishedAt: "2026-11-10T00:00:00Z" },
      { tagName: "v1.28.3", isPrerelease: false, publishedAt: "2026-11-01T00:00:00Z" },
      { tagName: "v1.28.2", isPrerelease: false, publishedAt: "2026-10-01T00:00:00Z" },
    ],
    "published_at_by_tag.json": { "v1.28.3": "2026-11-01T00:00:00Z" },
    "compare.json": {
      commits: [
        {
          sha: "aaa1111",
          html_url: "https://github.com/Kong/kongctl/commit/aaa1111",
          commit: {
            message: "feat(declarative): support datastores on AI Gateway policies (#2798)\n\nbody",
            author: { date: "2026-11-05T00:00:00Z" },
          },
        },
      ],
    },
    "prs.json": [
      // Merged before the v1.28.3 baseline publish, so only the commit
      // reference in compare.json keeps it in scope.
      { number: 2798, title: "feat: datastores", mergedAt: "2026-10-20T00:00:00Z", url: "x" },
      // Merged inside the baseline window with no commit reference.
      { number: 9000, title: "docs: unrelated", mergedAt: "2026-11-06T00:00:00Z", url: "x" },
      // Merged before the baseline window with no commit reference: excluded.
      { number: 8000, title: "old: unrelated", mergedAt: "2026-09-01T00:00:00Z", url: "x" },
    ],
  });

  assert.equal(result.status, 0, result.stderr);
  assert.match(env, /PREV_RELEASE_TAG=v1\.28\.3\n/);
  assert.deepEqual(
    pullRequests.map((pr) => pr.number).sort(),
    [2798, 9000],
  );
  assert.equal(compare.commits[0].sha, "aaa1111");
});

test("does not let a rerun for an older stable release select a newer release as the baseline", (t) => {
  const { result, env } = run(t, {
    releaseId: "300",
    releaseTag: "v1.28.3",
    "current_release.json": { id: 300, tag_name: "v1.28.3", published_at: "2026-11-01T00:00:00Z" },
    "releases.json": [
      { tagName: "v1.28.4", isPrerelease: false, publishedAt: "2026-11-18T10:00:00Z" },
      { tagName: "v1.28.3", isPrerelease: false, publishedAt: "2026-11-01T00:00:00Z" },
      { tagName: "prerelease-datastores", isPrerelease: true, publishedAt: "2026-11-10T00:00:00Z" },
      { tagName: "v1.28.2", isPrerelease: false, publishedAt: "2026-10-01T00:00:00Z" },
    ],
    "published_at_by_tag.json": { "v1.28.2": "2026-10-01T00:00:00Z" },
    "compare.json": { commits: [] },
    "prs.json": [],
  });

  assert.equal(result.status, 0, result.stderr);
  assert.match(env, /PREV_RELEASE_TAG=v1\.28\.2\n/);
});

test("the first release has no baseline and never calls compare or PR collection", (t) => {
  const directory = mkdtempSync(join(tmpdir(), "kongctl-release-notes-"));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  writeFileSync(join(directory, "gh"), GH_MOCK, { mode: 0o755 });
  writeFileSync(join(directory, "issues.json"), "[]");
  writeFileSync(
    join(directory, "current_release.json"),
    JSON.stringify({ id: 1, tag_name: "v0.1.0", published_at: "2026-01-01T00:00:00Z" }),
  );
  writeFileSync(
    join(directory, "releases.json"),
    JSON.stringify([{ tagName: "v0.1.0", isPrerelease: false, publishedAt: "2026-01-01T00:00:00Z" }]),
  );
  // No published_at_by_tag.json, compare.json, or prs.json: an unexpected
  // call to `gh release view`, `gh api .../compare/...`, or `gh pr list`
  // fails the mock and this test, instead of silently passing.
  const githubEnv = join(directory, "github-env");
  writeFileSync(githubEnv, "");
  rmSync("/tmp/gh-aw/release-data", { recursive: true, force: true });

  const result = spawnSync("bash", ["-euo", "pipefail", "-c", script], {
    cwd: directory,
    encoding: "utf8",
    env: {
      ...process.env,
      PATH: `${directory}:${process.env.PATH}`,
      GH_TOKEN: "test-token",
      EXPR_GITHUB_REPOSITORY: "Kong/kongctl",
      RELEASE_ID: "1",
      RELEASE_TAG: "v0.1.0",
      GITHUB_ENV: githubEnv,
    },
  });

  assert.equal(result.status, 0, result.stderr);
  assert.match(readFileSync(githubEnv, "utf8"), /PREV_RELEASE_TAG=\n/);
  assert.equal(readFileSync("/tmp/gh-aw/release-data/pull_requests.json", "utf8"), "[]\n");
  assert.equal(readFileSync("/tmp/gh-aw/release-data/compare.json", "utf8"), "{}\n");
});
