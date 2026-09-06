import assert from "node:assert/strict";
import {
  readFileSync,
  writeFileSync,
  mkdtempSync,
  mkdirSync,
  rmSync,
  existsSync,
  readdirSync,
} from "node:fs";
import { execFileSync, spawnSync } from "node:child_process";
import { generateKeyPairSync } from "node:crypto";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";
import {
  selectSubmissions,
  summarizeSubmission,
  validateEvidence,
} from "../diagnose-apple-notary.mjs";

const expected = { amd64: "a".repeat(40), arm64: "b".repeat(40) };
const release = { id: 100, tag_name: "v1.15.1", draft: true, assets: [] };
const hashes = Object.entries(expected).map(([arch, cdhash]) => ({
  arch,
  cdhash,
  verification_exit_code: 0,
}));
const entry = {
  id: "11111111-1111-1111-1111-111111111111",
  createdDate: "2026-09-06T04:02:21Z",
  name: "kongctl.zip",
};
const start = "2026-09-06T03:38:23Z";
const end = "2026-09-06T04:04:43Z";

test("collector cleans its key and withholds private output on API failure", () => {
  const directory = mkdtempSync(
    join(tmpdir(), "kongctl-notary-collector-test-"),
  );
  try {
    const tools = join(directory, "tools");
    const evidence = join(directory, "evidence");
    const output = join(directory, "output");
    mkdirSync(tools);
    for (const host of ["macos-15", "macos-15-intel"]) {
      const path = join(evidence, `apple-diagnostics-${host}`);
      mkdirSync(path, { recursive: true });
      writeFileSync(join(path, "release.json"), JSON.stringify(release));
      writeFileSync(join(path, "hashes.json"), JSON.stringify(hashes));
    }
    const mock = `#!/usr/bin/env node
const fs = require('node:fs');
const tool = require('node:path').basename(process.argv[1]);
const args = process.argv.slice(2);
const send = value => process.stdout.write(JSON.stringify(value));
if (process.env.APPLE_NOTARY_API_PRIVATE_KEY) throw Error('Private key leaked into child environment');
if (tool === 'git') {
  if (args[0] === 'rev-list') process.stdout.write('tag-commit\\n');
} else if (tool === 'gh') {
  if (args.at(-1).includes('/jobs')) send([{jobs:[{name:'publish_release',conclusion:'success',started_at:'${start}',completed_at:'${end}'}]}]);
  else send({path:'.github/workflows/release.lock.yml',event:'workflow_dispatch',head_branch:'main',status:'completed',head_sha:'tag-commit'});
} else if (tool === 'xcrun') {
  if (process.env.GH_TOKEN) throw Error('GitHub token leaked into Apple request');
  const key = args[args.indexOf('--key') + 1];
  if ((fs.statSync(key).mode & 0o777) !== 0o600) throw Error('Unsafe key permissions');
  if (args[1] === 'history') {
    if (process.env.FAIL_HISTORY) process.stdout.write('PRIVATE-ACCOUNT-DATA not JSON');
    else send({history:[${JSON.stringify(entry)}]});
  } else if (args[1] === 'info') send({status:'Accepted'});
  else if (args[1] === 'log') send({ticketContents:[{path:'kongctl.zip/kongctl',cdhash:'${expected.amd64}',arch:'x86_64'}],issues:null});
  else throw Error('Unexpected mutating Apple command');
} else throw Error('Unexpected tool');
`;
    for (const name of ["gh", "git", "xcrun"])
      writeFileSync(join(tools, name), mock, { mode: 0o700 });
    const { privateKey } = generateKeyPairSync("ec", {
      namedCurve: "prime256v1",
    });
    const env = {
      ...process.env,
      PATH: `${tools}:${process.env.PATH}`,
      RUNNER_TEMP: directory,
      GITHUB_REPOSITORY: "Kong/kongctl",
      GITHUB_REF: "refs/heads/main",
      GITHUB_EVENT_NAME: "workflow_dispatch",
      RELEASE_TAG: "v1.15.1",
      RELEASE_RUN_ID: "123",
      GH_TOKEN: "fixture-token",
      APPLE_NOTARY_API_PRIVATE_KEY: privateKey.export({
        type: "pkcs8",
        format: "pem",
      }),
      APPLE_NOTARY_API_KEY_ID: "fixture-key-id",
      APPLE_NOTARY_API_ISSUER_ID: "fixture-issuer",
      GITHUB_STEP_SUMMARY: join(directory, "summary"),
    };
    const script = fileURLToPath(
      new URL("../diagnose-apple-notary.mjs", import.meta.url),
    );
    const success = spawnSync(process.execPath, [script, evidence, output], {
      env,
      encoding: "utf8",
    });
    assert.equal(success.status, 0, success.stderr);
    const report = JSON.parse(
      readFileSync(join(output, "ticket-report.json"), "utf8"),
    );
    assert.deepEqual(report.submissions[0].matching_architectures, ["amd64"]);
    assert.equal(
      readdirSync(directory).some((name) =>
        name.startsWith("kongctl-notary-diagnostics-"),
      ),
      false,
    );
    const failure = spawnSync(
      process.execPath,
      [script, evidence, join(directory, "failed-output")],
      {
        env: { ...env, FAIL_HISTORY: "true" },
        encoding: "utf8",
      },
    );
    assert.equal(failure.status, 1);
    assert.doesNotMatch(
      failure.stderr + failure.stdout,
      /PRIVATE-ACCOUNT-DATA|fixture-token|BEGIN PRIVATE KEY/,
    );
    assert.equal(
      readdirSync(directory).some((name) =>
        name.startsWith("kongctl-notary-diagnostics-"),
      ),
      false,
    );
    assert.equal(existsSync(join(directory, "failed-output")), false);
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test("collect both architectures after Intel failure without executing binaries", () => {
  const directory = mkdtempSync(
    join(tmpdir(), "kongctl-native-diagnostics-test-"),
  );
  try {
    const tools = join(directory, "tools");
    const assets = join(directory, "assets");
    const output = join(directory, "output");
    mkdirSync(tools);
    mkdirSync(assets);
    const marker = join(directory, "executed");
    writeFileSync(
      join(assets, "kongctl"),
      `#!/bin/sh\ntouch '${marker}'\nexit 99\n`,
      { mode: 0o700 },
    );
    for (const arch of ["amd64", "arm64"]) {
      execFileSync("zip", ["-q", `kongctl_darwin_${arch}.zip`, "kongctl"], {
        cwd: assets,
      });
    }
    writeFileSync(join(tools, "sw_vers"), "#!/bin/sh\necho macOS-fixture\n", {
      mode: 0o700,
    });
    writeFileSync(
      join(tools, "codesign"),
      `#!/bin/sh
case "$*" in
  *--display*)
    echo 'Authority=Developer ID Application: Example Inc. (ABCDE12345)'
    echo 'TeamIdentifier=not set'
    echo 'CodeDirectory v=20500 flags=0x10000(runtime)'
    echo 'Timestamp=fixture'
    case "$*" in
      */amd64/*) echo 'CDHash=${expected.amd64}' ;;
      *) echo 'CDHash=${expected.arm64}' ;;
    esac
    ;;
  *--check-notarization*amd64*) exit 3 ;;
esac
`,
      { mode: 0o700 },
    );
    execFileSync(
      "bash",
      [
        fileURLToPath(
          new URL("../diagnose-apple-binaries.sh", import.meta.url),
        ),
        assets,
        output,
      ],
      {
        env: {
          ...process.env,
          PATH: `${tools}:${process.env.PATH}`,
          APPLE_TEAM_ID: "ABCDE12345",
          APPLE_SIGNING_IDENTITY:
            "Developer ID Application: Example Inc. (ABCDE12345)",
          GITHUB_STEP_SUMMARY: join(directory, "summary"),
        },
      },
    );
    const result = JSON.parse(
      readFileSync(join(output, "hashes.json"), "utf8"),
    );
    assert.deepEqual(
      result.map((value) => value.verification_exit_code),
      [3, 0],
    );
    assert.deepEqual(
      result.map((value) => value.cdhash),
      [expected.amd64, expected.arm64],
    );
    assert.equal(existsSync(marker), false);
    assert.ok(existsSync(join(output, "amd64-verification.txt")));
    assert.ok(existsSync(join(output, "arm64-verification.txt")));
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test("require the exact same release and binary hashes on both runners", () => {
  assert.deepEqual(
    validateEvidence(
      [
        { release, hashes },
        { release, hashes },
      ],
      release.tag_name,
    ),
    expected,
  );
  assert.throws(() =>
    validateEvidence([{ release, hashes }], release.tag_name),
  );
  assert.throws(() =>
    validateEvidence(
      [
        { release, hashes },
        { release: { ...release, id: 101 }, hashes },
      ],
      release.tag_name,
    ),
  );
  assert.throws(() =>
    validateEvidence(
      [
        { release, hashes },
        {
          release,
          hashes: [{ ...hashes[0], cdhash: "c".repeat(40) }, hashes[1]],
        },
      ],
      release.tag_name,
    ),
  );
  assert.throws(() =>
    validateEvidence(
      [
        { release, hashes },
        { release, hashes: [hashes[0], hashes[0]] },
      ],
      release.tag_name,
    ),
  );
});

test("only select submissions inside the original publisher window", () => {
  const history = {
    history: [
      entry,
      { ...entry, createdDate: "2026-09-06T02:00:00Z" },
      { ...entry, createdDate: "2026-09-06T05:00:00Z" },
    ],
  };
  assert.deepEqual(selectSubmissions(history, start, end), [entry]);
  assert.throws(() => selectSubmissions({}, start, end));
  assert.throws(() => selectSubmissions(history, end, start));
  assert.throws(() =>
    selectSubmissions(
      { history: [{ ...entry, id: "--malicious-option" }] },
      start,
      end,
    ),
  );
  assert.throws(() =>
    selectSubmissions({ history: Array(21).fill(entry) }, start, end),
  );
});

test("correlate ticket contents by exact hash, not submission success alone", () => {
  const log = {
    ticketContents: [
      { path: "archive/kongctl", cdhash: expected.arm64, arch: "arm64" },
    ],
    issues: null,
  };
  const result = summarizeSubmission(
    entry,
    { status: "Accepted" },
    log,
    expected,
  );
  assert.deepEqual(result.matching_architectures, ["arm64"]);
  assert.deepEqual(result.matching_ticket_hashes, [expected.arm64]);
  assert.equal(result.status, "Accepted");
  const mismatch = summarizeSubmission(
    entry,
    { status: "Accepted" },
    {
      ticketContents: [
        { path: "archive/kongctl", cdhash: "c".repeat(40), arch: "x86_64" },
      ],
    },
    expected,
  );
  assert.deepEqual(mismatch.matching_architectures, []);
  assert.deepEqual(mismatch.kongctl_ticket_hashes, [
    { cdhash: "c".repeat(40), arch: "x86_64" },
  ]);
});

test("missing logs and in-progress submissions are not interpreted as accepted", () => {
  const result = summarizeSubmission(
    entry,
    { status: "In Progress" },
    null,
    expected,
  );
  assert.equal(result.log_available, false);
  assert.equal(result.status, "In Progress");
  assert.deepEqual(result.matching_architectures, []);
  assert.equal(
    summarizeSubmission(entry, null, null, expected).status,
    "Unavailable",
  );
});

test("do not expose unrelated account metadata, ticket paths, URLs, or issue text", () => {
  const other = { ...entry, name: "private-project.zip" };
  assert.equal(
    summarizeSubmission(
      other,
      { status: "Accepted" },
      { ticketContents: [] },
      expected,
    ),
    null,
  );
  const result = summarizeSubmission(
    entry,
    { status: "Accepted", private: "secret" },
    {
      private: "secret",
      ticketContents: [
        {
          path: "private-project/kongctl",
          cdhash: expected.amd64,
          arch: "x86_64",
        },
      ],
      issues: [{ message: "secret", path: "private-project" }],
      logFileUrl: "secret",
    },
    expected,
  );
  assert.doesNotMatch(
    JSON.stringify(result),
    /secret|private-project|logFileUrl/,
  );
  assert.equal(result.issue_count, 1);
});

test("workflow is main-only, uses isolated credentials, and has no publisher", () => {
  const workflow = readFileSync(
    new URL(
      "../../.github/workflows/apple-notary-diagnostics.yml",
      import.meta.url,
    ),
    "utf8",
  );
  const inspect = workflow
    .split("  inspect:\n")[1]
    .split("  notary-records:\n")[0];
  assert.match(inspect, /refs\/heads\/main/);
  assert.doesNotMatch(inspect, /secrets\.APPLE_/);
  assert.doesNotMatch(
    workflow,
    /SIGNING_CERTIFICATE|goreleaser-action|release edit|publish-tap|workflow_run:/,
  );
  const collector = readFileSync(
    new URL("../diagnose-apple-notary.mjs", import.meta.url),
    "utf8",
  );
  assert.doesNotMatch(
    collector,
    /"notarytool", "(?:submit|store-credentials)"/,
  );
  assert.match(collector, /finally \{\s*rmSync\(privateDir/);
  assert.match(collector, /timeout: 60000/);
  const native = readFileSync(
    new URL("../diagnose-apple-binaries.sh", import.meta.url),
    "utf8",
  );
  assert.doesNotMatch(native, /chmod|--sign|--force|kongctl" version|xattr -w/);
});
