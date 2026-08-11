const labelPattern = /(?:^|\s)label=(?:"([^"]*)"|'([^']*)'|([^\s]+))/u;
const rowsPattern = /(?:^|\s)rows=(?:"(\d+)"|'(\d+)'|(\d+))(?=\s|$)/u;

export function parseCodeBlockLabel(meta) {
  if (!meta) {
    return undefined;
  }

  const match = labelPattern.exec(meta);
  if (!match) {
    return undefined;
  }

  const value = match[1] ?? match[2] ?? match[3];
  return value === "false" || value === "" ? false : value;
}

export function parseCodeBlockRows(meta) {
  if (!meta) {
    return undefined;
  }

  const match = rowsPattern.exec(meta);
  if (!match) {
    return undefined;
  }

  const rows = Number.parseInt(match[1] ?? match[2] ?? match[3], 10);
  return rows > 0 ? rows : undefined;
}

export const codeBlockMetaTransformer = {
  name: "kongctl-code-block-meta",
  pre(node) {
    const meta = this.options.meta?.__raw;
    const label = parseCodeBlockLabel(meta);

    if (label === false) {
      node.properties.dataCodeLabelHidden = "true";
    } else if (label !== undefined) {
      node.properties.dataCodeLabel = label;
    }

    const rows = parseCodeBlockRows(meta);
    if (rows === undefined) {
      return;
    }

    const style = node.properties.style;
    const separator =
      typeof style === "string" && !style.endsWith(";") ? ";" : "";

    node.properties.dataCodeRows = String(rows);
    node.properties.style = `${style ?? ""}${separator}--code-max-height:${rows}lh;`;
    node.properties.tabIndex = 0;
  },
};
