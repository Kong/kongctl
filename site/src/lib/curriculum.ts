import type { CollectionEntry } from "astro:content";
import { getCollection } from "astro:content";
import { z } from "astro/zod";
import { parse } from "yaml";

import chaptersSource from "../data/chapters.yaml?raw";
import { pageUrl } from "./urls";

const chaptersSchema = z.array(
  z.object({
    id: z.string().regex(/^[a-z0-9]+(?:-[a-z0-9]+)*$/),
    title: z.string().min(1),
    description: z.string().min(1),
    order: z.number().int().positive(),
  }),
);

type LessonEntry = CollectionEntry<"lessons">;

export interface Lesson {
  chapterId: string;
  entry: LessonEntry;
  key: string;
  slug: string;
  url: string;
}

export interface Chapter {
  description: string;
  id: string;
  lessons: Lesson[];
  order: number;
  title: string;
  url: string;
}

export interface Curriculum {
  chapters: Chapter[];
  lessons: Lesson[];
}

function loadChapterDefinitions() {
  const chapters = chaptersSchema.parse(parse(chaptersSource));

  const ids = new Set<string>();
  const orders = new Set<number>();
  for (const chapter of chapters) {
    if (ids.has(chapter.id)) {
      throw new Error(`Duplicate chapter id: ${chapter.id}`);
    }
    if (orders.has(chapter.order)) {
      throw new Error(`Duplicate chapter order: ${chapter.order}`);
    }
    ids.add(chapter.id);
    orders.add(chapter.order);
  }

  return chapters.toSorted((a, b) => a.order - b.order);
}

function lessonLocation(entry: LessonEntry) {
  const segments = entry.id.split("/");
  if (segments.length !== 2) {
    throw new Error(
      `Lesson "${entry.id}" must be stored as <chapter>/<lesson>.md`,
    );
  }

  return { chapterId: segments[0], slug: segments[1] };
}

export async function loadCurriculum(): Promise<Curriculum> {
  const chapterDefinitions = loadChapterDefinitions();
  const entries = await getCollection("lessons");
  const chapterIds = new Set(chapterDefinitions.map((chapter) => chapter.id));
  const lessonsByChapter = new Map<string, Lesson[]>();

  for (const entry of entries) {
    const { chapterId, slug } = lessonLocation(entry);
    if (!chapterIds.has(chapterId)) {
      throw new Error(
        `Lesson "${entry.id}" references unknown chapter "${chapterId}"`,
      );
    }

    const lesson: Lesson = {
      chapterId,
      entry,
      key: `${chapterId}/${slug}`,
      slug,
      url: pageUrl(`${chapterId}/${slug}`),
    };
    const chapterLessons = lessonsByChapter.get(chapterId) ?? [];
    chapterLessons.push(lesson);
    lessonsByChapter.set(chapterId, chapterLessons);
  }

  const chapters = chapterDefinitions.map((chapter) => {
    const lessons = (lessonsByChapter.get(chapter.id) ?? []).toSorted(
      (a, b) => a.entry.data.order - b.entry.data.order,
    );
    const orders = new Set<number>();
    for (const lesson of lessons) {
      if (orders.has(lesson.entry.data.order)) {
        throw new Error(
          `Duplicate lesson order ${lesson.entry.data.order} in chapter "${chapter.id}"`,
        );
      }
      orders.add(lesson.entry.data.order);
    }

    return {
      ...chapter,
      lessons,
      url: pageUrl(chapter.id),
    };
  });

  return {
    chapters,
    lessons: chapters.flatMap((chapter) => chapter.lessons),
  };
}

export function adjacentLessons(curriculum: Curriculum, key: string) {
  const index = curriculum.lessons.findIndex((lesson) => lesson.key === key);
  if (index === -1) {
    throw new Error(`Unknown lesson key: ${key}`);
  }

  return {
    previous: curriculum.lessons[index - 1],
    next: curriculum.lessons[index + 1],
  };
}
