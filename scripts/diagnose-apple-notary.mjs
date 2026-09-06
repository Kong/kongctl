// Diagnostic only. The only Apple operations are history, info, and log.
import { execFileSync } from "node:child_process";
import { createPrivateKey } from "node:crypto";
import {
  appendFileSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const arches = ["amd64", "arm64"];
const statuses = ["Accepted", "Invalid", "In Progress", "Rejected"];

export function validateEvidence(reports, tag) {
  if (reports.length !== 2) throw new Error("Expected both runner reports");
  const expected = {};
  for (const { release, hashes } of reports) {
    if (
      release.tag_name !== tag ||
      !Number.isSafeInteger(release.id) ||
      JSON.stringify(release) !== JSON.stringify(reports[0].release)
    ) {
      throw new Error("Runner release snapshots differ");
    }
    if (!Array.isArray(hashes) || hashes.length !== 2)
      throw new Error("Missing binary hashes");
    for (const arch of arches) {
      const entries = hashes.filter((entry) => entry.arch === arch);
      if (entries.length !== 1 || !/^[0-9a-f]{40}$/.test(entries[0].cdhash)) {
        throw new Error("Invalid or ambiguous binary hash");
      }
      if (expected[arch] && expected[arch] !== entries[0].cdhash) {
        throw new Error("Binary hashes differ across runners");
      }
      expected[arch] = entries[0].cdhash;
    }
  }
  return expected;
}

export function selectSubmissions(history, start, end) {
  const from = Date.parse(start);
  const to = Date.parse(end);
  if (
    !Number.isFinite(from) ||
    !Number.isFinite(to) ||
    from > to ||
    !Array.isArray(history.history)
  ) {
    throw new Error("Invalid submission history or signing time window");
  }
  const selected = history.history.filter((entry) => {
    const time = Date.parse(entry.createdDate);
    return time >= from && time <= to;
  });
  // Avoid fetching an unbounded account-wide history in a diagnostic workflow.
  if (selected.length > 20 || selected.some((entry) => !uuid.test(entry.id))) {
    throw new Error("Unexpected submission count or ID in signing window");
  }
  return selected;
}

export function summarizeSubmission(entry, info, log, expected) {
  const tickets = Array.isArray(log?.ticketContents) ? log.ticketContents : [];
  const hashes = new Set(Object.values(expected));
  const matches = tickets.filter((ticket) => hashes.has(ticket.cdhash));
  const relevant =
    matches.length > 0 ||
    tickets.some((ticket) => /(?:^|\/)kongctl$/.test(ticket.path ?? "")) ||
    /kongctl/i.test(entry.name ?? "");
  if (!relevant) return null;
  // Never persist raw logs, unrelated paths/names, account history, or download URLs.
  return {
    submission_id: entry.id,
    created_date: entry.createdDate,
    status: statuses.includes(info?.status) ? info.status : "Unavailable",
    log_available: log !== null,
    ticket_entry_count: tickets.length,
    matching_architectures: arches.filter((arch) =>
      matches.some((ticket) => ticket.cdhash === expected[arch]),
    ),
    matching_ticket_hashes: [
      ...new Set(matches.map((ticket) => ticket.cdhash)),
    ],
    kongctl_ticket_hashes: tickets
      .filter(
        (ticket) =>
          /(?:^|\/)kongctl$/.test(ticket.path ?? "") &&
          /^[0-9a-f]{40}$/.test(ticket.cdhash ?? ""),
      )
      .map((ticket) => ({
        cdhash: ticket.cdhash,
        arch: ["arm64", "x86_64"].includes(ticket.arch)
          ? ticket.arch
          : "unknown",
      })),
    issue_count: Array.isArray(log?.issues) ? log.issues.length : 0,
  };
}

function readJSON(path) {
  return JSON.parse(readFileSync(path, "utf8"));
}

async function main() {
  const [evidence, output] = process.argv.slice(2);
  const tag = process.env.RELEASE_TAG;
  const runID = process.env.RELEASE_RUN_ID;
  if (
    !evidence ||
    !output ||
    !/^v\d+\.\d+\.\d+$/.test(tag ?? "") ||
    !/^[1-9][0-9]*$/.test(runID ?? "") ||
    !Number.isSafeInteger(Number(runID))
  ) {
    throw new Error(
      "Expected evidence/output directories and valid release tag/run ID",
    );
  }
  if (
    process.env.GITHUB_REPOSITORY !== "Kong/kongctl" ||
    process.env.GITHUB_REF !== "refs/heads/main" ||
    process.env.GITHUB_EVENT_NAME !== "workflow_dispatch"
  ) {
    throw new Error(
      "Apple records may only be fetched by a manual Kong/kongctl main workflow",
    );
  }
  const env = { ...process.env };
  for (const name of Object.keys(env))
    if (name.startsWith("APPLE_")) delete env[name];
  const run = (command, args) => {
    try {
      return execFileSync(command, args, {
        encoding: "utf8",
        timeout: 60000,
        maxBuffer: 16 * 1024 * 1024,
        env:
          command === "xcrun"
            ? { ...env, GH_TOKEN: "", GITHUB_TOKEN: "" }
            : env,
        stdio: ["ignore", "pipe", "pipe"],
      });
    } catch {
      // Child error objects can contain command arguments and private API output.
      throw new Error(
        `Read-only ${command} request failed or timed out; private output withheld`,
      );
    }
  };
  const api = (path) => JSON.parse(run("gh", ["api", path]));
  const reports = ["macos-15", "macos-15-intel"].map((host) => ({
    host,
    release: readJSON(
      join(evidence, `apple-diagnostics-${host}`, "release.json"),
    ),
    hashes: readJSON(
      join(evidence, `apple-diagnostics-${host}`, "hashes.json"),
    ),
  }));
  const expected = validateEvidence(reports, tag);
  const source = api(`repos/Kong/kongctl/actions/runs/${runID}`);
  const tagCommit = run("git", [
    "rev-list",
    "-n",
    "1",
    `refs/tags/${tag}`,
  ]).trim();
  run("git", ["merge-base", "--is-ancestor", tagCommit, "HEAD"]);
  if (
    source.path !== ".github/workflows/release.lock.yml" ||
    source.event !== "workflow_dispatch" ||
    source.head_branch !== "main" ||
    source.status !== "completed" ||
    source.head_sha !== tagCommit
  ) {
    throw new Error(
      "Source must be the original completed main Release run at the tag commit",
    );
  }
  const pages = JSON.parse(
    run("gh", [
      "api",
      "--paginate",
      "--slurp",
      `repos/Kong/kongctl/actions/runs/${runID}/jobs?per_page=100`,
    ]),
  );
  const publishers = pages
    .flatMap((page) => page.jobs)
    .filter(
      (job) => job.name === "publish_release" && job.conclusion === "success",
    );
  if (publishers.length !== 1)
    throw new Error("Expected one successful original publisher");
  const { started_at: start, completed_at: end } = publishers[0];
  for (const name of [
    "APPLE_NOTARY_API_PRIVATE_KEY",
    "APPLE_NOTARY_API_KEY_ID",
    "APPLE_NOTARY_API_ISSUER_ID",
    "RUNNER_TEMP",
  ]) {
    if (!process.env[name]) throw new Error(`Missing ${name}`);
  }
  // Validate without importing a certificate/key into any persistent keychain.
  try {
    createPrivateKey(process.env.APPLE_NOTARY_API_PRIVATE_KEY);
  } catch {
    throw new Error("Invalid Apple API private key");
  }
  const privateDir = mkdtempSync(
    join(process.env.RUNNER_TEMP, "kongctl-notary-diagnostics-"),
  );
  let report;
  try {
    const key = join(privateDir, "AuthKey.p8");
    writeFileSync(key, process.env.APPLE_NOTARY_API_PRIVATE_KEY, {
      mode: 0o600,
      flag: "wx",
    });
    const auth = [
      "--key",
      key,
      "--key-id",
      process.env.APPLE_NOTARY_API_KEY_ID,
      "--issuer",
      process.env.APPLE_NOTARY_API_ISSUER_ID,
    ];
    let history;
    try {
      history = JSON.parse(
        run("xcrun", [
          "notarytool",
          "history",
          ...auth,
          "--output-format",
          "json",
        ]),
      );
    } catch {
      throw new Error(
        "Unable to retrieve or parse Apple history; private output withheld",
      );
    }
    const candidates = selectSubmissions(history, start, end);
    const submissions = [];
    let unavailable = 0;
    for (const entry of candidates) {
      let info = null;
      let log = null;
      try {
        info = JSON.parse(
          run("xcrun", [
            "notarytool",
            "info",
            entry.id,
            ...auth,
            "--output-format",
            "json",
          ]),
        );
      } catch {
        unavailable++;
      }
      try {
        log = JSON.parse(
          run("xcrun", ["notarytool", "log", entry.id, ...auth]),
        );
      } catch {
        unavailable++;
      }
      const summary = summarizeSubmission(entry, info, log, expected);
      if (summary) submissions.push(summary);
    }
    report = {
      diagnostic_only: true,
      release_tag: tag,
      release_id: reports[0].release.id,
      original_run_id: runID,
      signing_window: { start, end },
      expected_cdhashes: expected,
      runner_results: reports.map(({ host, hashes }) => ({ host, hashes })),
      candidate_count: candidates.length,
      unavailable_requests: unavailable,
      submissions,
      note: "Missing matches are inconclusive if history is incomplete or requests failed. This is not release approval.",
    };
  } finally {
    rmSync(privateDir, { recursive: true, force: true });
  }
  mkdirSync(output, { recursive: true });
  writeFileSync(
    join(output, "ticket-report.json"),
    `${JSON.stringify(report, null, 2)}\n`,
  );
  console.log(JSON.stringify(report, null, 2));
  if (process.env.GITHUB_STEP_SUMMARY) {
    const lines = [
      "### Apple ticket diagnostics",
      "",
      "Evidence collection only; no release is approved or published.",
      "",
    ];
    for (const arch of arches) {
      const matched = report.submissions.some(
        (s) =>
          s.status === "Accepted" && s.matching_architectures.includes(arch),
      );
      lines.push(
        `- ${arch}: ${matched ? "Accepted ticket contains the exact code hash" : "No accepted matching ticket found in retrieved logs (inconclusive)"}`,
      );
    }
    appendFileSync(process.env.GITHUB_STEP_SUMMARY, `${lines.join("\n")}\n`);
  }
}

if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(resolve(process.argv[1])).href
) {
  main().catch((error) => {
    console.error(error.message);
    process.exitCode = 1;
  });
}
