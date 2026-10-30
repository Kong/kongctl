import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

// Execute the actual inline configuration code against a read-only API double.
const workflow = readFileSync(
  new URL("../../.github/workflows/release.md", import.meta.url),
  "utf8",
);
const inline = workflow
  .split("      - name: Compute release configuration\n")[1]
  .split("          script: |\n")[1]
  .split("      - name: Restrict full releases")[0]
  .split("\n")
  .map((line) => line.replace(/^ {12}/, ""))
  .join("\n");
const configure = new (Object.getPrototypeOf(async () => {}).constructor)(
  "github",
  "context",
  "core",
  "exec",
  "console",
  inline,
);
const assets = [
  "checksums.txt",
  ...["darwin", "linux", "windows"].flatMap((os) =>
    ["amd64", "arm64"].map((arch) => `kongctl_${os}_${arch}.zip`),
  ),
].map((name) => ({ name }));

async function run(overrides = {}) {
  const release = {
    id: 100,
    tag_name: "v1.15.1",
    draft: true,
    prerelease: false,
    published_at: null,
    assets,
    ...overrides.release,
  };
  const source = {
    path: ".github/workflows/release.lock.yml",
    event: "workflow_dispatch",
    head_branch: "main",
    head_sha: "tag-commit",
    status: "completed",
    ...overrides.source,
  };
  const inputs = {
    recovery_tag: "v1.15.1",
    recovery_run_id: "123",
    ...overrides.inputs,
  };
  const outputs = {};
  const errors = [];
  const repos = { listReleases: "releases" };
  const actions = {
    getWorkflowRun: async ({ run_id }) => {
      assert.equal(run_id, 123);
      return { data: source };
    },
    listJobsForWorkflowRun: "jobs",
    listWorkflowRunArtifacts: "artifacts",
  };
  const pages = {
    releases: overrides.releases ?? [release],
    jobs: overrides.jobs ?? [
      { name: "publish_release", conclusion: "success" },
    ],
    artifacts: overrides.artifacts ?? [
      { name: "homebrew-v1.15.1", expired: false },
    ],
  };
  const github = {
    rest: { repos, actions, git: { getRef: async () => ({ data: {} }) } },
    paginate: async (endpoint) => {
      assert.ok(endpoint in pages, `Unexpected API endpoint ${endpoint}`);
      return pages[endpoint];
    },
  };
  const core = {
    setOutput: (key, value) => {
      outputs[key] = value;
    },
    setFailed: (message) => errors.push(message),
  };
  const exec = {
    getExecOutput: async (command, args) => {
      assert.equal(command, "git");
      assert.deepEqual(args, ["rev-list", "-n", "1", "refs/tags/v1.15.1"]);
      return { stdout: "tag-commit\n" };
    },
    exec: async (command, args) => {
      assert.equal(command, "git");
      assert.deepEqual(args, [
        "merge-base",
        "--is-ancestor",
        "tag-commit",
        "HEAD",
      ]);
      if (overrides.foreignTag) throw new Error("Tag is not on trusted main");
    },
  };
  await configure(
    github,
    { payload: { inputs }, repo: { owner: "Kong", repo: "kongctl" } },
    core,
    exec,
    { log() {} },
  );
  return { outputs, errors };
}

test("signed draft recovery reuses the original tag and metadata run", async () => {
  const result = await run();
  assert.deepEqual(result.errors, []);
  assert.deepEqual(result.outputs, {
    build_mode: "draft-recovery",
    artifact_mode: "full",
    release_tag: "v1.15.1",
    release_version: "1.15.1",
    recovery_run_id: "123",
  });
});

test("published recovery remains verification-only", async () => {
  const result = await run({
    release: { draft: false, published_at: "2026-09-06" },
    inputs: { recovery_run_id: "" },
  });
  assert.deepEqual(result.errors, []);
  assert.equal(result.outputs.build_mode, "recovery");
  assert.equal(result.outputs.recovery_run_id, undefined);
});

for (const [name, overrides] of Object.entries({
  "missing source run": { inputs: { recovery_run_id: "" } },
  "malformed source run": { inputs: { recovery_run_id: "123;echo" } },
  "unsafe integer run": { inputs: { recovery_run_id: "9007199254740993" } },
  "missing recovery tag": { inputs: { recovery_tag: "" } },
  "invalid tag": { inputs: { recovery_tag: "v1.15.1-rc.1" } },
  "missing draft access": { releases: [] },
  "ambiguous release": {
    releases: [{ tag_name: "v1.15.1" }, { tag_name: "v1.15.1" }],
  },
  prerelease: { release: { prerelease: true } },
  "incomplete assets": { release: { assets: [] } },
  "smoke draft": {
    release: { assets: [{ name: "kongctl-1.15.1-linux-amd64-smoke.tar.gz" }] },
  },
  "foreign workflow": {
    source: { path: ".github/workflows/adhoc-prerelease.yml" },
  },
  "untrusted branch": { source: { head_branch: "feature" } },
  "wrong commit": { source: { head_sha: "other-commit" } },
  "wrong event": { source: { event: "pull_request" } },
  "unfinished source run": { source: { status: "in_progress" } },
  "failed original publisher": {
    jobs: [{ name: "publish_release", conclusion: "failure" }],
  },
  "missing metadata": { artifacts: [] },
  "expired metadata": {
    artifacts: [{ name: "homebrew-v1.15.1", expired: true }],
  },
  "wrong metadata version": {
    artifacts: [{ name: "homebrew-v1.15.0", expired: false }],
  },
  "published release with draft source run": {
    release: { draft: false, published_at: "2026-09-06" },
  },
})) {
  test(`reject ${name}`, async () => {
    const { errors, outputs } = await run(overrides);
    assert.equal(errors.length, 1);
    assert.equal(outputs.build_mode, undefined);
  });
}

test("reject a tag outside trusted main before any publication", async () => {
  await assert.rejects(run({ foreignTag: true }), /not on trusted main/);
});

test("recovery never rebuilds or recreates tags, but draft recovery finishes Homebrew", () => {
  const createTag = workflow
    .split("  create_tag:\n")[1]
    .split("  build_release:\n")[0];
  assert.match(
    createTag,
    /name: Create and push tag\n        if: env.RELEASE_BUILD_MODE == 'full' \|\| env.RELEASE_BUILD_MODE == 'smoke'/,
  );
  assert.match(
    workflow,
    /name: Build and generate without signing or publishing credentials\n        if: env.RELEASE_BUILD_MODE == 'full'\n/,
  );
  const homebrew = workflow
    .split("  publish_cask:\n")[1]
    .split("  release_complete:\n")[0];
  for (const condition of homebrew.matchAll(/        if: (.*)/g)) {
    assert.equal(
      condition[1],
      "needs.config.outputs.build_mode == 'full' || needs.config.outputs.build_mode == 'draft-recovery'",
    );
  }
  assert.match(
    workflow,
    /run-id: \$\{\{ needs.config.outputs.recovery_run_id \}\}/,
  );
});

test("release build, signing, and publication have separate credential scopes", () => {
  const build = workflow.split("  build_release:\n")[1].split("  sign_release:\n")[0];
  const sign = workflow.split("  sign_release:\n")[1].split("  publish_release:\n")[0];
  const publish = workflow.split("  publish_release:\n")[1].split("  verify_apple_release:\n")[0];
  for (const stage of [build, sign]) {
    assert.match(stage, /permissions:\n      contents: read\n/);
    assert.doesNotMatch(stage, /persist-credentials: true/);
    assert.doesNotMatch(stage, /contents: write|packages: write|TAP_GITHUB_TOKEN|DOCKER_TOKEN/);
    assert.match(stage, /ref: \$\{\{ needs.config.outputs.release_commit \}\}/);
  }
  assert.deepEqual([...build.matchAll(/secrets\.([A-Z_]+)/g)].map(match => match[1]),
    ["GH_TOKEN_PRIVATE_READ"]);
  assert.match(build, /args: build --clean/);
  assert.match(build, /KONGCTL_SKIP_NOTARIZATION: "true"/);
  assert.match(sign, /KONGCTL_SKIP_NOTARIZATION: "false"/);
  assert.doesNotMatch(sign, /GH_TOKEN_PRIVATE_READ|secrets.GITHUB_TOKEN|actions\/setup-go|docker\/login-action/);
  assert.match(sign, /args: release --clean --skip=before,publish,docker /);
  assert.match(sign, /PATH=\$GITHUB_WORKSPACE\/scripts\/release-tools:\$PATH/);
  assert.ok(sign.indexOf("release-handoff.py verify") < sign.indexOf("secrets.APPLE_"));
  assert.doesNotMatch(publish, /secrets.APPLE_|GH_TOKEN_PRIVATE_READ|actions\/setup-go|goreleaser\/goreleaser-action/);
  assert.match(publish, /needs: \["config", "build_release", "sign_release"\]/);
  assert.ok(publish.indexOf("release-handoff.py verify signed") < publish.indexOf("gh release create"));
  assert.match(publish, /cmp handoff\/handoff.json signed\/unsigned-handoff.json/);
});
