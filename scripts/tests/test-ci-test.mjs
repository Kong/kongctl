import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";
import test from "node:test";

const workflow = readFileSync(new URL("../../.github/workflows/test.yaml", import.meta.url), "utf8");
function stepScript(name) {
  return workflow.split(`      - name: ${name}\n`)[1].split("        run: |\n")[1]
    .split("\n      - name:")[0].trimEnd().split("\n").map(line => line.slice(10)).join("\n");
}

const gate = stepScript("Require lint and test suites");
test("required check waits for both parallel jobs even if one fails or is skipped", () => {
  const job = workflow.split("\n  test:\n")[1];
  assert.match(job, /^    if: always\(\)$/m);
  assert.match(job, /^    needs: \[lint, suites\]$/m);
});

for (const lint of ["success", "failure", "cancelled", "skipped", ""]) {
  for (const suites of ["success", "failure", "cancelled", "skipped", ""]) {
    test(`required check with lint=${lint || "missing"}, suites=${suites || "missing"}`, () => {
      const result = spawnSync("bash", ["-euo", "pipefail", "-c", gate], {
        encoding: "utf8", env: { ...process.env, LINT_RESULT: lint, SUITES_RESULT: suites },
      });
      assert.equal(result.status === 0, lint === "success" && suites === "success", result.stderr);
    });
  }
}

const installer = stepScript("Install installer test tools");
for (const missing of [[], ["bsdtar"], ["shellcheck", "zip", "unzip", "bsdtar"]]) {
  test(`installer tools with missing=${missing.join(",") || "none"}`, () => {
    const mocks = `
command() {
  if [ "$1" = -v ]; then
    case "$2" in ${missing.join("|") || "never-missing"}) return 1;; *) return 0;; esac
  else
    builtin command "$@"
  fi
}
sudo() { echo "$*"; }
`;
    const result = spawnSync("bash", ["-euo", "pipefail", "-c", mocks + installer], { encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
    const packages = missing.map(tool => tool === "bsdtar" ? "libarchive-tools" : tool);
    assert.equal(result.stdout, packages.length
      ? `apt-get update\napt-get install -y ${packages.join(" ")}\n` : "");
  });
}
