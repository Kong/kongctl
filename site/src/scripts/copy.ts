const languageLabels: Record<string, string> = {
  bash: "Run this...",
  console: "Run this...",
  json: "Expected output",
  sh: "Run this...",
  shell: "Run this...",
  text: "Example output",
  yaml: "YAML",
  yml: "YAML",
  zsh: "Run this...",
};
let nextCodeBlockId = 0;

export async function copyText(
  text: string,
  clipboard: Pick<Clipboard, "writeText"> | undefined = navigator.clipboard,
): Promise<void> {
  if (!clipboard) {
    throw new Error("Clipboard access is unavailable");
  }
  await clipboard.writeText(text);
}

function codeText(pre: HTMLPreElement): string {
  return pre.querySelector("code")?.textContent ?? pre.textContent ?? "";
}

function blockLabel(pre: HTMLPreElement): string | undefined {
  if (pre.dataset.codeLabelHidden === "true") {
    return undefined;
  }
  if (pre.dataset.codeLabel !== undefined) {
    return pre.dataset.codeLabel;
  }

  const language = pre.dataset.language?.toLowerCase() ?? "code";
  return languageLabels[language] ?? language.toUpperCase();
}

function ensureCodeBlockId(pre: HTMLPreElement): void {
  if (pre.id) {
    return;
  }

  let id: string;
  do {
    nextCodeBlockId += 1;
    id = `code-block-${nextCodeBlockId}`;
  } while (document.getElementById(id));
  pre.id = id;
}

function createExpandButton(pre: HTMLPreElement): HTMLButtonElement {
  const button = document.createElement("button");
  button.className = "expand-button";
  button.type = "button";
  button.dataset.codeExpandButton = "";
  button.setAttribute("aria-expanded", "false");
  button.setAttribute("aria-controls", pre.id);
  button.innerHTML =
    '<svg aria-hidden="true" viewBox="0 0 16 16"><path d="m3.25 6 4.75 4 4.75-4"></path></svg><span data-expand-label>Expand</span>';

  button.addEventListener("click", () => {
    const expanded = pre.dataset.codeExpanded !== "true";
    const label = button.querySelector<HTMLElement>("[data-expand-label]");

    pre.dataset.codeExpanded = String(expanded);
    button.setAttribute("aria-expanded", String(expanded));
    if (label) {
      label.textContent = expanded ? "Collapse" : "Expand";
    }
    if (!expanded) {
      pre.scrollTop = 0;
    }
  });

  return button;
}

export function initializeCodeBlocks(root: ParentNode = document): void {
  root.querySelectorAll<HTMLPreElement>("pre.astro-code").forEach((pre) => {
    if (pre.dataset.copyEnhanced === "true") {
      return;
    }
    pre.dataset.copyEnhanced = "true";
    if (pre.dataset.codeRows !== undefined) {
      ensureCodeBlockId(pre);
    }

    const shell = document.createElement("div");
    shell.className = "code-shell";

    const toolbar = document.createElement("div");
    toolbar.className = "code-toolbar";

    const labelText = blockLabel(pre);
    if (labelText === undefined) {
      toolbar.classList.add("code-toolbar-label-hidden");
    }

    const status = document.createElement("span");
    status.className = "sr-only";
    status.setAttribute("aria-live", "polite");

    const button = document.createElement("button");
    button.className = "copy-button";
    button.type = "button";
    button.dataset.copyButton = "";
    button.innerHTML =
      '<svg aria-hidden="true" viewBox="0 0 16 16"><rect x="5.25" y="5.25" width="8" height="8" rx="1.25"></rect><path d="M10.75 5.25v-2a1.5 1.5 0 0 0-1.5-1.5h-6a1.5 1.5 0 0 0-1.5 1.5v6a1.5 1.5 0 0 0 1.5 1.5h2"></path></svg><span data-copy-label>Copy</span>';
    button.addEventListener("click", async () => {
      const buttonLabel =
        button.querySelector<HTMLElement>("[data-copy-label]");
      try {
        await copyText(codeText(pre));
        if (buttonLabel) {
          buttonLabel.textContent = "Copied";
        }
        status.textContent = "Code copied to clipboard";
      } catch {
        if (buttonLabel) {
          buttonLabel.textContent = "Copy failed";
        }
        status.textContent =
          "Copy failed. Select the code and copy it manually.";
      }

      window.setTimeout(() => {
        if (buttonLabel) {
          buttonLabel.textContent = "Copy";
        }
      }, 1800);
    });

    const actions = document.createElement("div");
    actions.className = "code-actions";
    if (pre.dataset.codeRows !== undefined) {
      actions.append(createExpandButton(pre));
    }
    actions.append(button);

    if (labelText !== undefined) {
      const label = document.createElement("span");
      label.className = "code-label";
      label.textContent = labelText;
      toolbar.append(label);
    }
    toolbar.append(status, actions);
    pre.before(shell);
    shell.append(toolbar, pre);
  });
}
