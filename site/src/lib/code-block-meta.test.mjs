import { describe, expect, it } from "vitest";

import {
  codeBlockMetaTransformer,
  parseCodeBlockRows,
} from "./code-block-meta.mjs";

describe("code block row metadata", () => {
  it("parses a positive visible row count", () => {
    expect(parseCodeBlockRows('label="Configuration" rows=8')).toBe(8);
    expect(parseCodeBlockRows("rows='12'")).toBe(12);
  });

  it("ignores missing or invalid row counts", () => {
    expect(parseCodeBlockRows()).toBeUndefined();
    expect(parseCodeBlockRows("rows=0")).toBeUndefined();
    expect(parseCodeBlockRows("rows=many")).toBeUndefined();
  });

  it("adds scroll metadata without replacing Shiki styles", () => {
    const node = {
      properties: {
        style: "background-color:#07100a;color:#fff",
      },
    };

    codeBlockMetaTransformer.pre.call(
      { options: { meta: { __raw: "rows=6" } } },
      node,
    );

    expect(node.properties).toMatchObject({
      dataCodeRows: "6",
      style: "background-color:#07100a;color:#fff;--code-max-height:6lh;",
      tabIndex: 0,
    });
  });
});
