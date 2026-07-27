import { defineConfig } from "astro/config";

export default defineConfig({
  site: "https://kong.github.io",
  base: "/kongctl",
  trailingSlash: "always",
  markdown: {
    shikiConfig: {
      theme: "github-dark",
      wrap: false,
    },
  },
});
