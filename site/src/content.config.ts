import { defineCollection } from "astro:content";
import { glob } from "astro/loaders";
import { z } from "astro/zod";

const lessons = defineCollection({
  loader: glob({
    pattern: "**/*.md",
    base: "./src/content/lessons",
  }),
  schema: z.object({
    title: z.string().min(1),
    summary: z.string().min(1),
    order: z.number().int().positive(),
    related: z
      .array(
        z.object({
          label: z.string().min(1),
          url: z.url(),
        }),
      )
      .optional(),
  }),
});

export const collections = { lessons };
