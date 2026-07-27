import { expect, test } from "@playwright/test";

test.beforeEach(async ({ page }) => {
  await page.goto("./");
  await page.evaluate(() => {
    localStorage.clear();
  });
  await page.reload();
});

test("filters the chapter navigation by lesson title", async ({ page }) => {
  await page.getByPlaceholder("Find a lesson").fill("declarative");

  await expect(
    page.getByRole("link", { name: "Declarative configuration concepts" }),
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
  await expect(terminal.getByText("Terminal", { exact: true })).toBeVisible();
  await terminal.getByRole("button", { name: "Copy" }).click();

  await expect(terminal.getByText("Copied", { exact: true })).toBeVisible();
  await expect
    .poll(() => page.evaluate(() => navigator.clipboard.readText()))
    .toBe("brew install --cask kong/kongctl/kongctl");
  await expect(
    page.getByText("Expected output", { exact: true }),
  ).toBeVisible();
});

test("persists explicit completion and continues to the next chapter", async ({
  page,
}) => {
  await page.goto("installation/install-kongctl/");
  await page.getByRole("link", { name: "Mark complete and continue" }).click();

  await expect(page).toHaveURL(
    /\/kongctl\/declarative-configuration\/concepts\/$/,
  );
  await expect(page.getByText("1 of 2", { exact: true })).toBeVisible();

  await page.reload();
  await expect(page.getByText("1 of 2", { exact: true })).toBeVisible();
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
