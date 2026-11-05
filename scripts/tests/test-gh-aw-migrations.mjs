import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { spawnSync } from "node:child_process";
import test from "node:test";

const readWorkflow = (name) => readFileSync(new URL(`../../.github/workflows/${name}.md`, import.meta.url), "utf8");

function actionScript(source, name) {
  const lines = source.split("\n");
  const start = lines.findIndex((line) => line.trim() === `- name: ${name}`);
  assert.notEqual(start, -1, name);
  const run = lines.findIndex((line, index) => index > start && line.trim() === "script: |");
  assert.notEqual(run, -1, name);
  const indent = lines[run].match(/^ */)[0].length + 2;
  const body = [];
  for (const line of lines.slice(run + 1)) {
    if (line.trim() && !line.startsWith(" ".repeat(indent))) break;
    body.push(line.slice(indent));
  }
  return body.join("\n");
}

function execute(script, env) {
  const harness = `
    const context = { repo: { owner: "Kong", repo: "kongctl" } };
    const core = { warning: () => {} };
    const github = { rest: { search: { issuesAndPullRequests: async () => {
      if (process.env.LOOKUP_FAIL) throw new Error("lookup failed");
      return { data: { items: [{ number: 42 }] } };
    } } } };
    (async () => { ${script} })().catch(error => {
      console.error(error.message);
      process.exitCode = 1;
    });
  `;
  return spawnSync("node", ["-e", harness], { encoding: "utf8", env: { ...process.env, ...env } });
}

function workspace(t) {
  const directory = mkdtempSync(join(tmpdir(), "kongctl-gh-aw-migration-"));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  return directory;
}

test("safe-output mutation hooks run in safe-outputs, outside agent post-steps", () => {
  for (const [workflow, step] of [
    ["benchmark-regression-analysis", "Validate benchmark analysis comment target"],
    ["kongctl-feature-user-agent", "Hydrate replay prompt into final report safe output"],
  ]) {
    const source = readWorkflow(workflow).split("---", 3)[1];
    assert.match(source.split("safe-outputs:\n")[1].split(/^\S/m)[0], new RegExp(step));
    assert.doesNotMatch(source.split("\npost-steps:\n")[1] || "", /agent_output\.json|safeoutputs\/outputs\.jsonl/);
  }
});

for (const issue of ["42", ""]) {
  test(`benchmark preserves only the resolved issue target (input ${issue || "scheduled"})`, (t) => {
    const directory = workspace(t);
    const output = join(directory, "agent.json");
    writeFileSync(output, JSON.stringify({ items: [
      { type: "add_comment", item_number: 42, body: "correct" },
      { type: "add_comment", item_number: 99, body: "incorrect" },
      { type: "noop", message: "preserved" },
    ] }));
    const script = actionScript(readWorkflow("benchmark-regression-analysis"), "Validate benchmark analysis comment target");
    const result = execute(script, { ISSUE_NUMBER: issue, GH_AW_AGENT_OUTPUT: output });
    assert.equal(result.status, 0, result.stderr);
    assert.deepEqual(JSON.parse(readFileSync(output, "utf8")).items, [
      { type: "add_comment", item_number: 42, body: "correct" },
      { type: "noop", message: "preserved" },
    ]);
  });
}

test("benchmark emits noop when no allowed comments remain", (t) => {
  const directory = workspace(t);
  const output = join(directory, "agent.json");
  writeFileSync(output, JSON.stringify({ items: [{ type: "add_comment", item_number: 99, body: "wrong" }] }));
  const result = execute(
    actionScript(readWorkflow("benchmark-regression-analysis"), "Validate benchmark analysis comment target"),
    { ISSUE_NUMBER: "42", GH_AW_AGENT_OUTPUT: output },
  );
  assert.equal(result.status, 0, result.stderr);
  assert.equal(JSON.parse(readFileSync(output, "utf8")).items[0].type, "noop");
});

test("benchmark skips comments when scheduled issue lookup fails", (t) => {
  const directory = workspace(t);
  const output = join(directory, "agent.json");
  writeFileSync(output, JSON.stringify({ items: [{ type: "add_comment", item_number: 42, body: "Report" }] }));
  const result = execute(
    actionScript(readWorkflow("benchmark-regression-analysis"), "Validate benchmark analysis comment target"),
    { ISSUE_NUMBER: "", LOOKUP_FAIL: "1", GH_AW_AGENT_OUTPUT: output },
  );
  assert.equal(result.status, 0, result.stderr);
  assert.equal(JSON.parse(readFileSync(output, "utf8")).items[0].type, "noop");
});

for (const type of ["create_issue", "create_discussion"]) {
  test(`feature-user ${type} retains the sanitized replay prompt`, (t) => {
    const directory = workspace(t);
    const output = join(directory, "agent.json");
    const prompt = join(directory, "prompt.md");
    writeFileSync(output, JSON.stringify({ items: [{ type, body: "## Summary\nWorked" }] }));
    writeFileSync(prompt, "Exercise a safe example.\nUse ``` fences literally.");
    const script = actionScript(readWorkflow("kongctl-feature-user-agent"), "Hydrate replay prompt into final report safe output");
    const result = execute(script, { GH_AW_AGENT_OUTPUT: output, REPLAY_PROMPT: prompt });
    assert.equal(result.status, 0, result.stderr);
    const body = JSON.parse(readFileSync(output, "utf8")).items[0].body;
    assert.match(body, /## Replay Prompt/);
    assert.match(body, /Exercise a safe example/);
    assert.match(body, /````markdown/);
  });
}

test("feature-user rejects unsafe replay evidence before publishing", (t) => {
  const directory = workspace(t);
  const output = join(directory, "agent.json");
  const prompt = join(directory, "prompt.md");
  const original = JSON.stringify({ items: [{ type: "create_issue", body: "Report" }] });
  writeFileSync(output, original);
  writeFileSync(prompt, "Contact test-user@example.invalid for details.");
  const script = actionScript(readWorkflow("kongctl-feature-user-agent"), "Hydrate replay prompt into final report safe output");
  const result = execute(script, { GH_AW_AGENT_OUTPUT: output, REPLAY_PROMPT: prompt });
  assert.notEqual(result.status, 0);
  assert.match(result.stderr + result.stdout, /Replay prompt contains values that look unsafe/);
  assert.equal(readFileSync(output, "utf8"), original);
});
