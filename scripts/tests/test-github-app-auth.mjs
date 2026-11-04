import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { spawnSync } from "node:child_process";
import test from "node:test";

const workflow = readFileSync(
  new URL("../../.github/workflows/github-app-auth-check.yml", import.meta.url),
  "utf8",
);
const verification = workflow
  .split("      - name: Verify token repository scopes and private SDK read access\n")[1]
  .split("        run: |\n")[1]
  .split("\n")
  .map((line) => line.slice(10))
  .join("\n");

function verify(t, mode = "valid") {
  const directory = mkdtempSync(join(tmpdir(), "kongctl-app-auth-"));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  writeFileSync(join(directory, "gh"), `#!/usr/bin/env node
const repositories = {
  sdk: "Kong/sdk-konnect-go-internal", pr: "Kong/kongctl",
  installer: "Kong/get.konghq.com", tap: "Kong/homebrew-kongctl",
};
const token = process.env.GH_TOKEN;
const mode = process.env.AUTH_TEST_MODE;
if (process.argv[3] === "/installation/repositories") {
  const repo = mode === "wrong-repository" && token === "installer"
    ? "Kong/another-repository" : repositories[token];
  const extra = mode === "extra-repository" && token === "sdk";
  console.log(JSON.stringify({total_count: extra ? 2 : 1,
    repositories: [{full_name: repo}, ...(extra ? [{full_name: "Kong/other"}] : [])]}));
} else if (process.argv[3] === "repos/Kong/sdk-konnect-go-internal/contents/go.mod") {
  if (token !== "sdk" || mode === "sdk-read-denied") process.exit(1);
  console.log(JSON.stringify({content: "private-module-content"}));
} else process.exit(1);
`, { mode: 0o755 });
  return spawnSync("bash", ["-euo", "pipefail", "-c", verification], {
    encoding: "utf8",
    env: {
      ...process.env,
      PATH: `${directory}:${process.env.PATH}`,
      SDK_TOKEN: "sdk",
      PR_TOKEN: "pr",
      INSTALLER_TOKEN: "installer",
      TAP_TOKEN: "tap",
      AUTH_TEST_MODE: mode,
    },
  });
}

test("authentication check accepts isolated tokens without printing private contents", (t) => {
  const result = verify(t);
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /Verified private SDK contents access/);
  assert.doesNotMatch(result.stdout, /private-module-content/);
});

for (const mode of ["wrong-repository", "extra-repository", "sdk-read-denied"]) {
  test(`authentication check rejects ${mode}`, (t) => {
    const result = verify(t, mode);
    assert.notEqual(result.status, 0);
    assert.doesNotMatch(result.stdout, /Verified private SDK contents access/);
  });
}

for (const filename of ["test.yaml", "e2e.yaml", "adhoc-prerelease.yml", "declarative-benchmark.yaml", "release.md"]) {
  const source = readFileSync(new URL(`../../.github/workflows/${filename}`, import.meta.url), "utf8");
  const detectors = source.split("      - name: Detect private SDK dependency\n").slice(1);
  assert.equal(detectors.length, filename === "e2e.yaml" ? 2 : 1, filename);
  for (const [index, detector] of detectors.entries()) {
    test(`${filename} SDK detector ${index + 1} requires credentials only for private builds`, (t) => {
      const directory = mkdtempSync(join(tmpdir(), "kongctl-sdk-detect-"));
      t.after(() => rmSync(directory, { recursive: true, force: true }));
      const script = detector.split("      - name: Create private SDK read token\n")[0]
        .split("        run: |\n")[1].trimEnd().split("\n").map((line) => line.slice(10)).join("\n");
      for (const privateSDK of [false, true]) {
        writeFileSync(join(directory, "go.mod"), "module example\n" + (privateSDK
          ? "replace github.com/Kong/sdk-konnect-go => github.com/Kong/sdk-konnect-go-internal v0.1.0\n" : ""));
        const output = join(directory, "output");
        writeFileSync(output, "");
        const result = spawnSync("bash", ["-euo", "pipefail", "-c", script], {
          cwd: directory, encoding: "utf8", env: { ...process.env, GITHUB_OUTPUT: output },
        });
        assert.equal(result.status, 0, result.stderr);
        assert.equal(readFileSync(output, "utf8").includes("required=true"), privateSDK);
      }
    });
  }
}

test("App authentication checks cannot run PR-head scripts or use a label write gate", () => {
  assert.match(workflow, /on:\n  workflow_dispatch: \{\}/);
  assert.match(workflow, /if: github.repository == 'Kong\/kongctl' && github.ref == 'refs\/heads\/main'/);
  assert.doesNotMatch(workflow, /pull_request:|rehearse-homebrew|actions\/checkout|homebrew-app-rehearsal/);
});

for (const filename of ["test.yaml", "e2e.yaml", "declarative-benchmark.yaml", "release.md"]) {
  const source = readFileSync(new URL(`../../.github/workflows/${filename}`, import.meta.url), "utf8");
  const configurations = source.split(/      - name: Configure private module (?:access|reads)\n/).slice(1);
  assert.equal(configurations.length, filename === "e2e.yaml" ? 2 : 1, filename);
  for (const [index, configuration] of configurations.entries()) {
    const script = configuration.split("\n      - ")[0].split("        run: |\n")[1]
      .trimEnd().split("\n").map((line) => line.slice(10)).join("\n");
    test(`${filename} SDK configuration ${index + 1} rejects missing tokens before configuring Git`, (t) => {
      const directory = mkdtempSync(join(tmpdir(), "kongctl-sdk-config-"));
      t.after(() => rmSync(directory, { recursive: true, force: true }));
      const result = spawnSync("bash", ["-euo", "pipefail", "-c", script], {
        cwd: directory, encoding: "utf8", env: { ...process.env, SDK_TOKEN: "" },
      });
      assert.notEqual(result.status, 0);
      assert.match(result.stderr, /SDK App token is required/);
    });
    test(`${filename} SDK configuration ${index + 1} scopes headers and preserves GOPRIVATE`, (t) => {
      const directory = mkdtempSync(join(tmpdir(), "kongctl-sdk-config-"));
      t.after(() => rmSync(directory, { recursive: true, force: true }));
      writeFileSync(join(directory, "go"), '#!/bin/sh\n[ "$GOTOOLCHAIN" = local ] || exit 1\necho persisted.example/*\n', { mode: 0o755 });
      const env = {
        ...process.env, SDK_TOKEN: "test-sdk-token", PATH: `${directory}:${process.env.PATH}`,
        GIT_CONFIG_GLOBAL: join(directory, "gitconfig"), GIT_CONFIG_NOSYSTEM: "1",
        GITHUB_ENV: join(directory, "env"),
      };
      const module = "github.com/Kong/sdk-konnect-go-internal";
      for (const prior of ["example.com/*", `example.com/*,${module}`, ""]) {
        writeFileSync(env.GITHUB_ENV, "");
        const result = spawnSync("bash", ["-euo", "pipefail", "-c", script], {
          cwd: directory, encoding: "utf8", env: { ...env, GOPRIVATE: prior },
        });
        assert.equal(result.status, 0, result.stderr);
        const expected = prior.includes(module) ? prior : `${prior || "persisted.example/*"},${module}`;
        assert.equal(readFileSync(env.GITHUB_ENV, "utf8"), `GOPRIVATE=${expected}\n`);
      }
      for (const suffix of ["", ".git", "/info/refs", ".git/info/refs", "-other", ".git-other"]) {
        const result = spawnSync("git", ["config", "--get-urlmatch", "http.extraheader", `https://${module}${suffix}`], {
          cwd: directory, encoding: "utf8", env,
        });
        assert.equal(result.status === 0, !suffix.endsWith("-other"), suffix);
      }
      const hostHeader = "AUTHORIZATION: basic checkout-token";
      const configured = spawnSync("git", ["config", "--global", "http.https://github.com/.extraheader", hostHeader], {
        cwd: directory, encoding: "utf8", env,
      });
      assert.equal(configured.status, 0, configured.stderr);
      const scopedHeader = `AUTHORIZATION: basic ${Buffer.from("x-access-token:test-sdk-token").toString("base64")}`;
      for (const suffix of ["", ".git", "-other"]) {
        const result = spawnSync("git", ["config", "--get-urlmatch", "http.extraheader", `https://${module}${suffix}`], {
          cwd: directory, encoding: "utf8", env,
        });
        assert.equal(result.stdout.trim(), suffix === "-other" ? hostHeader : scopedHeader);
      }
    });
  }
}
