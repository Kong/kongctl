import { defineConfig } from "astro/config";

import { codeBlockMetaTransformer } from "./src/lib/code-block-meta.mjs";

export default defineConfig({
  site: "https://kong.github.io",
  base: "/kongctl",
  trailingSlash: "always",
  markdown: {
    shikiConfig: {
      theme: "github-dark",
      transformers: [codeBlockMetaTransformer],
      wrap: false,
    },
  },
});
