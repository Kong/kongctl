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
    expect(document.querySelector(".code-label")?.textContent).toBe("Terminal");

    document.querySelector<HTMLButtonElement>("[data-copy-button]")?.click();
    await vi.waitFor(() => {
      expect(writeText).toHaveBeenCalledWith("echo first\necho second");
      expect(document.querySelector("[data-copy-label]")?.textContent).toBe(
        "Copied",
      );
    });
  });

  it("labels text fences as expected output", () => {
    document.body.innerHTML = `
      <pre class="astro-code" data-language="text"><code>version output</code></pre>
    `;

    initializeCodeBlocks();

    expect(document.querySelector(".code-label")?.textContent).toBe(
      "Expected output",
    );
  });

  it("reports unavailable clipboard access", async () => {
    await expect(copyText("command", undefined)).rejects.toThrow(
      "Clipboard access is unavailable",
    );
  });
});
