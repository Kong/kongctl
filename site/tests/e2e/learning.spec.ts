import { expect, test } from "@playwright/test";

test.beforeEach(async ({ page }) => {
  await page.goto("./");
  await page.evaluate(() => {
    localStorage.clear();
  });
  await page.reload();
});

test("presents the home curriculum as a chapter list", async ({ page }) => {
  const curriculum = page.locator(".curriculum-section");

  await expect(
    curriculum.getByRole("heading", { name: "Curriculum" }),
  ).toBeVisible();
  await expect(curriculum.locator(".curriculum-chapters > li")).toHaveCount(6);
  await expect(curriculum.getByRole("link", { name: /Setup/ })).toHaveAttribute(
    "href",
    "/kongctl/installation/",
  );
  const usingKongctl = curriculum.locator(
    ".curriculum-chapters > li:nth-child(2)",
  );
  await expect(
    usingKongctl.getByText("Using kongctl", { exact: true }),
  ).toBeVisible();
  await expect(usingKongctl.getByRole("link")).toHaveAttribute(
    "href",
    "/kongctl/using-kongctl/",
  );
  const configuration = curriculum.locator(
    ".curriculum-chapters > li:nth-child(3)",
  );
  await expect(
    configuration.getByText("CLI Configuration", { exact: true }),
  ).toBeVisible();
  await expect(configuration.getByRole("link")).toHaveAttribute(
    "href",
    "/kongctl/kongctl-configuration/",
  );
  await expect(
    curriculum.getByRole("link", { name: /Declarative Configuration/ }),
  ).toHaveAttribute("href", "/kongctl/declarative-configuration/");
  await expect(
    curriculum.getByRole("link", {
      name: /Federated Management/,
    }),
  ).toHaveAttribute("href", "/kongctl/federated-api-platform-management/");
  await expect(
    curriculum.getByRole("link", { name: /Extensions/ }),
  ).toHaveAttribute("href", "/kongctl/extensions/");
});

test("presents the federated management journey", async ({ page }) => {
  await page.goto("federated-api-platform-management/");
  const lessons = page.locator(".chapter-lessons");

  await expect(
    page.getByRole("heading", {
      name: "Federated Management",
    }),
  ).toBeVisible();
  await expect(lessons.locator(":scope > li")).toHaveCount(10);
  await expect(
    lessons.getByRole("link", { name: /Federated Management/ }),
  ).toBeVisible();
  await expect(
    lessons.getByRole("link", { name: /AI Gateway 2.0 Beta Steps/ }),
  ).toBeVisible();
  await expect(
    lessons.getByRole("link", { name: /Ownership Boundaries/ }),
  ).toBeVisible();
  await expect(
    lessons.getByRole("link", { name: /Initial Configuration Setup/ }),
  ).toBeVisible();
  await expect(
    lessons.getByRole("link", { name: /Run a Dataplane/ }),
  ).toBeVisible();
  await expect(
    lessons.getByRole("link", { name: /Federate Engineering/ }),
  ).toBeVisible();
  await expect(
    lessons.getByRole("link", { name: /Federate Product/ }),
  ).toBeVisible();
  await expect(
    lessons.getByRole("link", { name: /Independent Team Changes/ }),
  ).toBeVisible();
  await expect(
    lessons.getByRole("link", { name: /Route Team Requests/ }),
  ).toBeVisible();
  await expect(lessons.getByRole("link", { name: /Clean Up/ })).toBeVisible();

  await page.goto("federated-api-platform-management/ai-gateway-2-beta-steps/");
  const lesson = page.locator(".lesson-body");
  await expect(lesson).toContainText("prerelease-aigw-2");
  await expect(lesson).toContainText(
    "KONGCTL_DEFAULT_KONNECT_ENVIRONMENT=tech",
  );
  await expect(lesson).toContainText("kongctl login");
  await expect(lesson).toContainText("KONGCTL_DEFAULT_KONNECT_PAT");
});

test("walks satellite teams into independent configuration", async ({
  page,
}) => {
  await page.goto(
    "federated-api-platform-management/initial-configuration-setup/",
  );
  await expect(
    page.getByText("!ref platform-aigw#id", { exact: false }).first(),
  ).toBeVisible();
  await expect(page.locator(".lesson-body")).toContainText("--recursive");
  await expect(page.locator(".lesson-body")).toContainText("Engineering");
  await expect(page.locator(".lesson-body")).toContainText("Product");
  await expect(page.locator(".lesson-body")).toContainText("/engineering");
  await expect(page.locator(".lesson-body")).toContainText("/product");
  await expect(page.locator(".lesson-body")).toContainText("body_param: model");

  await page.goto("federated-api-platform-management/run-a-data-plane/");
  await expect(page.locator(".lesson-body")).toContainText(
    "data_plane_certificates",
  );
  await expect(page.locator(".lesson-body")).toContainText(
    "KONG_CLUSTER_CONTROL_PLANE=${AIGW_CONTROL_PLANE}:443",
  );
  await expect(page.locator(".lesson-body")).toContainText(
    "KONG_CLUSTER_CERT=/etc/kong/certs/data-plane.crt",
  );
  await expect(page.locator(".lesson-body")).toContainText(
    '--group-add "$(id -g)"',
  );
  await expect(page.locator(".lesson-body")).toContainText(
    "kongctl get ai-gateway nodes",
  );

  await page.goto("federated-api-platform-management/federate-engineering/");
  await expect(page.locator(".lesson-body")).toContainText("!lookup");
  await expect(
    page.getByText("kongctl apply -f engineering/model.yaml", {
      exact: true,
    }),
  ).toBeVisible();
  await expect(page.locator(".lesson-body")).toContainText(
    "No changes needed. Resources match configuration.",
  );

  await page.goto("federated-api-platform-management/federate-product/");
  await expect(page.locator(".lesson-body")).toContainText("!lookup");
  await expect(
    page.getByText("kongctl apply -f product/model.yaml", { exact: true }),
  ).toBeVisible();

  await page.goto("federated-api-platform-management/independent-changes/");
  await expect(page.locator(".lesson-body")).toContainText(
    "engineering-code-reviewer",
  );
  await expect(page.locator(".lesson-body")).toContainText(
    "/engineering/code-review",
  );
  await expect(page.locator(".lesson-body")).toContainText(
    "one CREATE for engineering-code-reviewer",
  );

  await page.goto("federated-api-platform-management/route-team-requests/");
  await expect(page.locator(".lesson-body")).toContainText(
    "${AIGW_PROXY_URL}/product/chat/completions",
  );
  await expect(page.locator(".lesson-body")).toContainText(
    '"model": "product-assistant"',
  );
  await expect(page.locator(".lesson-body")).toContainText(
    "${AIGW_PROXY_URL}/engineering/code-review/chat/completions",
  );
  await expect(page.locator(".lesson-body")).toContainText(
    '"model": "engineering-code-reviewer"',
  );

  await page.goto("federated-api-platform-management/clean-up/");
  await expect(page.locator(".lesson-body")).toContainText(
    "docker stop federated-aigw-dp",
  );
  await expect(page.locator(".lesson-body")).toContainText(
    "kongctl delete -f platform/ai-gateway.yaml",
  );
});

test("presents the extension developer journey", async ({ page }) => {
  await page.goto("extensions/");
  const lessons = page.locator(".chapter-lessons");

  await expect(lessons.locator(":scope > li")).toHaveCount(6);
  await expect(
    lessons.getByRole("link", { name: /Extension Fundamentals/ }),
  ).toBeVisible();
  await expect(
    lessons.getByRole("link", { name: /Extension Manifest/ }),
  ).toBeVisible();
  await expect(
    lessons.getByRole("link", { name: /Runtime Context/ }),
  ).toBeVisible();
  await expect(
    lessons.getByRole("link", { name: /Local Development/ }),
  ).toBeVisible();
  await expect(
    lessons.getByRole("link", { name: /Install and Upgrade/ }),
  ).toBeVisible();
  await expect(
    lessons.getByRole("link", { name: /Publish an Extension/ }),
  ).toBeVisible();
});

test("explains extension manifests and runtime context", async ({ page }) => {
  await page.goto("extensions/manifest/");

  await expect(
    page.getByText("schema_version: 1", { exact: false }),
  ).toBeVisible();
  await expect(
    page.getByText("kongctl get foo", { exact: true }).first(),
  ).toBeVisible();

  await page.goto("extensions/runtime-context/");
  await expect(
    page.getByText("KONGCTL_EXTENSION_CONTEXT=/path/to/context.json", {
      exact: true,
    }),
  ).toBeVisible();
  await expect(
    page.getByText("kongctl get foo -- --output raw", { exact: true }),
  ).toBeVisible();
});

test("presents focused configuration lessons", async ({ page }) => {
  await page.goto("kongctl-configuration/");
  const lessons = page.locator(".chapter-lessons");

  await expect(
    lessons.getByRole("link", { name: /Configuration Profiles/ }),
  ).toBeVisible();
  await expect(
    lessons.getByRole("link", { name: /The Configuration File/ }),
  ).toBeVisible();
  await expect(
    lessons.getByRole("link", { name: /Configuration Paths/ }),
  ).toBeVisible();
  await expect(
    lessons.getByRole("link", { name: /Environment Variables/ }),
  ).toBeVisible();
});

test("maps command help paths into profile YAML", async ({ page }) => {
  await page.goto("kongctl-configuration/configuration-paths/");

  await expect(
    page.getByText("Config path: [ output ]", { exact: false }),
  ).toBeVisible();
  await expect(
    page.getByText("Config path: [ konnect.page-size ]", { exact: false }),
  ).toBeVisible();
  await expect(
    page.getByText("default:\n  output: yaml\n  konnect:\n    page-size: 10", {
      exact: true,
    }),
  ).toBeVisible();
});

test("explains default configuration file creation", async ({ page }) => {
  await page.goto("kongctl-configuration/configuration-file/");
  const lesson = page.locator(".lesson-body");

  await expect(lesson).toContainText(
    "The first kongctl command creates it when it is missing and writes an initial default profile",
  );
  await expect(lesson).toContainText(
    "After that initial creation, kongctl reads the file but does not rewrite it",
  );
});

test("maps configuration paths into environment variable names", async ({
  page,
}) => {
  await page.goto("kongctl-configuration/environment-variables/");

  await expect(
    page.getByRole("heading", {
      name: "Environment variables and profiles",
    }),
  ).toBeVisible();
  await expect(
    page.getByText("KONGCTL_DEFAULT_KONNECT_PAGE_SIZE", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("KONGCTL_DEFAULT_KONNECT_PAT", { exact: true }).first(),
  ).toBeVisible();

  const currentCircle = page.locator(
    ".lesson-link.current .completion-mark:not(.complete)",
  );
  const currentChapter = page.locator(".chapter-link.current");
  const circleBorder = await currentCircle.evaluate(
    (element) => getComputedStyle(element).borderTopColor,
  );
  const circleFill = await currentCircle.evaluate(
    (element) => getComputedStyle(element).backgroundColor,
  );
  const chapterAccent = await currentChapter.evaluate(
    (element) => getComputedStyle(element).color,
  );

  expect(circleBorder).toBe(chapterAccent);
  expect(circleFill).toBe(chapterAccent);
});

test("introduces the kongctl command structure", async ({ page }) => {
  await page.goto("using-kongctl/command-structure/");

  await expect(
    page.getByText("kongctl <verb> [product] <resource> [name-or-id] [flags]", {
      exact: true,
    }),
  ).toBeVisible();
  await expect(
    page.getByText("kongctl get konnect apis\nkongctl get apis", {
      exact: true,
    }),
  ).toBeVisible();
  await expect(
    page.getByText("kongctl get konnect apis --help", { exact: true }),
  ).toBeVisible();
});

test("introduces imperative and declarative workflows", async ({ page }) => {
  await page.goto("using-kongctl/reading-konnect-state/");

  await expect(
    page.getByRole("heading", { name: "Imperative vs declarative" }),
  ).toBeVisible();
  await expect(
    page.getByText('kongctl get apis "My Simple API" -o text', {
      exact: true,
    }),
  ).toBeVisible();
  await expect(
    page.getByText(
      "kongctl get apis 45d79870-eb41-4c23-b51b-99123de692ea -o json",
      { exact: true },
    ),
  ).toBeVisible();
  await expect(page.getByText("kongctl view", { exact: true })).toBeVisible();
});

test("presents the declarative configuration journey", async ({ page }) => {
  await page.goto("declarative-configuration/");
  const lessons = page.locator(".chapter-lessons");

  await expect(lessons.locator(":scope > li")).toHaveCount(8);
  await expect(
    lessons.getByRole("link", { name: /Plan-Based Configuration/ }),
  ).toBeVisible();
  await expect(lessons.getByRole("link", { name: /Metadata/ })).toBeVisible();
  await expect(
    lessons.getByRole("link", { name: /Resource Identity/ }),
  ).toBeVisible();
  await expect(lessons.locator(":scope > li").nth(1)).toContainText("Metadata");
  await expect(lessons.locator(":scope > li").nth(2)).toContainText(
    "Resource Identity",
  );
  await expect(lessons.getByRole("link", { name: /YAML Tags/ })).toBeVisible();
  await expect(
    lessons.getByRole("link", { name: /Working Example/ }),
  ).toBeVisible();
  await expect(lessons.getByRole("link", { name: /Sync Scope/ })).toBeVisible();
  await expect(
    lessons.getByRole("link", { name: /Adopt Existing Resources/ }),
  ).toBeVisible();
  await expect(
    lessons.getByRole("link", { name: /Discovering Declarative Schemas/ }),
  ).toBeVisible();
});

test("explains plan modes and mode-matched execution", async ({ page }) => {
  await page.goto("declarative-configuration/plan-based-configuration/");
  const lesson = page.locator(".lesson-body");
  const planSyntax = page
    .locator(".code-shell")
    .filter({ hasText: "kongctl plan --mode <mode>" });
  const examplePlan = page
    .locator(".code-shell")
    .filter({ hasText: '"generator": "kongctl/1.8.0"' });

  await expect(
    planSyntax.getByText("Command syntax", { exact: true }),
  ).toBeVisible();
  await expect(
    examplePlan.getByText("example plan", { exact: true }),
  ).toBeVisible();
  await expect(examplePlan.locator("pre")).toHaveAttribute(
    "data-code-rows",
    "8",
  );
  const expandButton = examplePlan.locator("[data-code-expand-button]");
  await expect(expandButton).toBeVisible();
  await expect(expandButton).toHaveAccessibleName("Expand");
  await expandButton.click();
  await expect(expandButton).toHaveAttribute("aria-expanded", "true");
  await expect(expandButton).toHaveAccessibleName("Collapse");
  await expect(examplePlan.locator("pre")).toHaveCSS("max-height", "none");
  await expandButton.click();
  await expect(expandButton).toHaveAttribute("aria-expanded", "false");
  await expect(expandButton).toHaveAccessibleName("Expand");
  await expect(
    lesson.getByRole("cell", { name: "apply" }).first(),
  ).toBeVisible();
  await expect(
    lesson.getByRole("cell", { name: "sync" }).first(),
  ).toBeVisible();
  await expect(
    lesson.getByRole("cell", { name: "delete" }).first(),
  ).toBeVisible();
  await expect(
    page.getByText("kongctl apply --plan plan.json", { exact: true }),
  ).toBeVisible();
  await expect(lesson).toContainText(
    "rejects a plan when the execution command does not match its mode",
  );
});

test("explains declarative identity, metadata, and YAML tags", async ({
  page,
}) => {
  await page.goto("declarative-configuration/resource-identity/");
  await expect(
    page.getByRole("cell", { name: "ref", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("cell", { name: "id", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("cell", { name: "name", exact: true }),
  ).toBeVisible();
  await expect(page.locator(".lesson-body")).toContainText(
    "Every declarative resource requires a unique ref identifier",
  );
  await expect(
    page.getByText("ai_gateway: !ref my-aigw#id", { exact: true }),
  ).toBeVisible();

  await page.goto("declarative-configuration/metadata/");
  await expect(page.locator(".definition-card")).toHaveCount(2);
  await expect(
    page.locator(".definition-card").first().getByText("string"),
  ).toBeVisible();
  await expect(
    page.locator(".definition-card").last().getByText("boolean"),
  ).toBeVisible();
  await expect(page.locator(".lesson-body")).toContainText("aigw-learning");
  await expect(page.locator(".lesson-body")).toContainText("KONGCTL-namespace");
  await expect(page.locator(".lesson-body")).toContainText("KONGCTL-protected");

  await page.goto("declarative-configuration/yaml-tags/");
  await expect(
    page.getByText("target_field: !<tag> <tag-input>", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("title: !file ./specs/openapi.yaml#info.title", {
      exact: true,
    }),
  ).toBeVisible();
  await expect(page.locator(".lesson-body")).toContainText(
    "omitting #<field> selects id by default",
  );
  await expect(page.locator(".lesson-body")).toContainText("Full mapping form");
  await expect(page.locator(".lesson-body")).toContainText("!lookup");
  await expect(page.locator(".lesson-body")).toContainText("!file");
  await expect(page.locator(".lesson-body")).toContainText("!env");
});

test("works through the AI Gateway declarative lifecycle", async ({
  context,
  page,
}) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  await page.goto("declarative-configuration/working-example/");

  const heredoc = page
    .locator(".code-shell")
    .filter({ hasText: "cat > ai-gateway.yaml <<'YAML'" })
    .first();
  await expect(heredoc).toBeVisible();
  await heredoc.getByRole("button", { name: "Copy" }).click();
  await expect
    .poll(() => page.evaluate(() => navigator.clipboard.readText()))
    .toContain("display_name: My AI Gateway");
  await expect
    .poll(() => page.evaluate(() => navigator.clipboard.readText()))
    .toContain("\nYAML");
  await expect(
    page
      .getByText("kongctl diff --mode apply -f ai-gateway.yaml", {
        exact: true,
      })
      .first(),
  ).toBeVisible();
  await expect(
    page.getByText("kongctl delete -f ai-gateway.yaml", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("kongctl list ai-gateways", { exact: true }),
  ).toBeVisible();
});

test("explains sync scope and desired count zero", async ({ page }) => {
  await page.goto("declarative-configuration/sync-scope/");
  const lesson = page.locator(".lesson-body");

  await expect(lesson).toContainText(
    "An omitted collection and an empty collection are intentionally different",
  );
  await expect(lesson).toContainText("ai_gateways: []");
  await expect(lesson).toContainText("models: []");
  await expect(lesson).toContainText(
    "root-level ai_gateway_models: [] is rejected",
  );
  await expect(
    page.getByText("kongctl diff --plan sync-plan.json", { exact: true }),
  ).toBeVisible();
});

test("discovers declarative resource schemas", async ({ page }) => {
  await page.goto("declarative-configuration/discovering-schemas/");

  await expect(
    page.getByText("kongctl explain", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("kongctl explain ai_gateway.models", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("kongctl scaffold ai_gateway", { exact: true }),
  ).toBeVisible();
});

test("filters the chapter navigation by lesson title", async ({ page }) => {
  await page.getByPlaceholder("Find a lesson").fill("declarative");

  await expect(
    page.getByRole("link", { name: "Discovering Declarative Schemas" }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Install kongctl", exact: true }),
  ).toBeHidden();
});

test("copies terminal commands without a prompt", async ({ context, page }) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  await page.goto("installation/install-kongctl/");

  const terminal = page
    .locator(".code-shell")
    .filter({ hasText: "brew install" });
  await expect(
    terminal.getByText("Run this...", { exact: true }),
  ).toBeVisible();
  await terminal.getByRole("button", { name: "Copy" }).click();

  await expect(terminal.getByText("Copied", { exact: true })).toBeVisible();
  await expect
    .poll(() => page.evaluate(() => navigator.clipboard.readText()))
    .toBe("brew install --cask kong/kongctl/kongctl");
  await expect(page.getByText("Example output", { exact: true })).toBeVisible();
});

test("offers browser and access token authentication flows", async ({
  page,
}) => {
  await page.goto("installation/authenticate/");

  await expect(
    page.getByRole("heading", { name: "Browser-based login flow" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Access token" }),
  ).toBeVisible();
  await expect(page.getByText(/personal access token \(PAT\)/)).toBeVisible();
  await expect(page.getByText(/system account access token/)).toBeVisible();
  await expect(page.getByText(/expected to fail with a SPAT/)).toBeVisible();
  await expect(
    page.getByText('export KONGCTL_DEFAULT_KONNECT_PAT="<access-token>"', {
      exact: true,
    }),
  ).toBeVisible();
});

test("verifies Konnect access and demonstrates output formats", async ({
  page,
}) => {
  await page.goto("installation/authenticate/");

  await expect(page.getByText("kongctl get me", { exact: true })).toBeVisible();
  await expect(
    page.getByText("kongctl get organization", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText(
      "kongctl get organization -o text\nkongctl get organization -o json\nkongctl get organization -o yaml",
      { exact: true },
    ),
  ).toBeVisible();
});

test("keeps the lesson outline in a desktop right rail", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("installation/install-kongctl/");

  await expect(
    page.getByText("Install the CLI and verify that it runs.", { exact: true }),
  ).toHaveCount(0);

  const outline = page.getByRole("navigation", { name: "On this page" });
  const article = page.locator(".lesson-body");
  const outlineBox = await outline.boundingBox();
  const articleBox = await article.boundingBox();

  expect(outlineBox).not.toBeNull();
  expect(articleBox).not.toBeNull();
  expect(outlineBox!.x).toBeGreaterThan(articleBox!.x + articleBox!.width);
  await expect(outline).toHaveCSS("position", "sticky");
  await expect(outline.getByRole("link", { name: "Goal" })).toBeVisible();
  await expect(
    outline.getByRole("link", { name: "Prerequisites" }),
  ).toBeVisible();
});

test("persists explicit completion and continues to the next lesson", async ({
  page,
}) => {
  await page.goto("installation/install-kongctl/");
  await page.getByRole("link", { name: "Mark complete and continue" }).click();

  await expect(page).toHaveURL(/\/kongctl\/installation\/authenticate\/$/);
  await expect(page.getByText("1 of 32", { exact: true })).toBeVisible();

  await page.reload();
  await expect(page.getByText("1 of 32", { exact: true })).toBeVisible();
});

test("persists a chosen color theme", async ({ page }) => {
  const initial = await page.locator("html").getAttribute("data-theme");
  await page.getByRole("button", { name: /Switch to .* theme/ }).click();
  const changed = await page.locator("html").getAttribute("data-theme");

  expect(changed).not.toBe(initial);
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("data-theme", changed!);
});

test("opens the guide navigation on a small screen", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const toggle = page.getByRole("button", { name: "Open guide navigation" });

  await toggle.click();

  await expect(toggle).toHaveAttribute("aria-expanded", "true");
  await expect(
    page.getByRole("navigation", { name: "Guide chapters" }),
  ).toBeVisible();
});
