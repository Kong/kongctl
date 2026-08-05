import { beforeEach, describe, expect, it, vi } from "vitest";

import { copyText, initializeCodeBlocks } from "./copy";

describe("code block copy controls", () => {
  beforeEach(() => {
    document.body.innerHTML = "";
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: undefined,
    });
  });

  it("labels terminal blocks and copies multiline commands without prompts", async () => {
    document.body.innerHTML = `
      <pre class="astro-code" data-language="shell"><code>echo first
echo second</code></pre>
    `;
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });

    initializeCodeBlocks();
    expect(document.querySelector(".code-label")?.textContent).toBe(
      "Run this...",
    );
    expect(document.querySelector("[data-code-expand-button]")).toBeNull();

    document.querySelector<HTMLButtonElement>("[data-copy-button]")?.click();
    await vi.waitFor(() => {
      expect(writeText).toHaveBeenCalledWith("echo first\necho second");
      expect(document.querySelector("[data-copy-label]")?.textContent).toBe(
        "Copied",
      );
    });
  });

  it("labels text fences as example output", () => {
    document.body.innerHTML = `
      <pre class="astro-code" data-language="text"><code>version output</code></pre>
    `;

    initializeCodeBlocks();

    expect(document.querySelector(".code-label")?.textContent).toBe(
      "Example output",
    );
  });

  it("labels JSON fences as expected output", () => {
    document.body.innerHTML = `
      <pre class="astro-code" data-language="json"><code>{"name":"example"}</code></pre>
    `;

    initializeCodeBlocks();

    expect(document.querySelector(".code-label")?.textContent).toBe(
      "Expected output",
    );
  });

  it("uses a per-block label override", () => {
    document.body.innerHTML = `
      <pre class="astro-code" data-language="shell" data-code-label="Command syntax"><code>kongctl plan --mode &lt;mode&gt;</code></pre>
    `;

    initializeCodeBlocks();

    expect(document.querySelector(".code-label")?.textContent).toBe(
      "Command syntax",
    );
  });

  it("can hide a block label without hiding Copy", () => {
    document.body.innerHTML = `
      <pre class="astro-code" data-language="shell" data-code-label-hidden="true"><code>kongctl apply --plan &lt;plan&gt;</code></pre>
    `;

    initializeCodeBlocks();

    expect(document.querySelector(".code-label")).toBeNull();
    expect(document.querySelector("[data-copy-button]")).not.toBeNull();
    expect(document.querySelector(".code-toolbar-label-hidden")).not.toBeNull();
  });

  it("expands and collapses a row-limited block", () => {
    document.body.innerHTML = `
      <pre class="astro-code" data-language="json" data-code-rows="8"><code>{"name":"example"}</code></pre>
    `;

    initializeCodeBlocks();

    const pre = document.querySelector<HTMLPreElement>("pre");
    const button = document.querySelector<HTMLButtonElement>(
      "[data-code-expand-button]",
    );

    expect(button?.textContent).toBe("Expand");
    expect(button?.getAttribute("aria-expanded")).toBe("false");
    expect(button?.getAttribute("aria-controls")).toBe(pre?.id);

    button?.click();
    expect(pre?.dataset.codeExpanded).toBe("true");
    expect(button?.textContent).toBe("Collapse");
    expect(button?.getAttribute("aria-expanded")).toBe("true");

    button?.click();
    expect(pre?.dataset.codeExpanded).toBe("false");
    expect(button?.textContent).toBe("Expand");
    expect(button?.getAttribute("aria-expanded")).toBe("false");
  });

  it("reports unavailable clipboard access", async () => {
    await expect(copyText("command", undefined)).rejects.toThrow(
      "Clipboard access is unavailable",
    );
  });
});
