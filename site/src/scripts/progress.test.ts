import { beforeEach, describe, expect, it } from "vitest";

import {
  completeLesson,
  PROGRESS_STORAGE_KEY,
  readProgress,
  resetProgress,
  visitLesson,
} from "./progress";

class MemoryStorage {
  private values = new Map<string, string>();

  getItem(key: string): string | null {
    return this.values.get(key) ?? null;
  }

  removeItem(key: string): void {
    this.values.delete(key);
  }

  setItem(key: string, value: string): void {
    this.values.set(key, value);
  }
}

describe("learning progress", () => {
  let storage: MemoryStorage;

  beforeEach(() => {
    storage = new MemoryStorage();
  });

  it("recovers safely from malformed storage", () => {
    storage.setItem(PROGRESS_STORAGE_KEY, "{not-json");

    expect(readProgress(storage)).toEqual({ completed: [] });
  });

  it("deduplicates completed lessons and ignores invalid values", () => {
    storage.setItem(
      PROGRESS_STORAGE_KEY,
      JSON.stringify({
        completed: [
          "installation/install-kongctl",
          7,
          "installation/install-kongctl",
        ],
        lastVisited: "installation/install-kongctl",
      }),
    );

    expect(readProgress(storage)).toEqual({
      completed: ["installation/install-kongctl"],
      lastVisited: "installation/install-kongctl",
    });
  });

  it("tracks visits separately from explicit completion", () => {
    let progress = visitLesson(
      storage,
      { completed: [] },
      "installation/install-kongctl",
    );
    expect(progress).toEqual({
      completed: [],
      lastVisited: "installation/install-kongctl",
    });

    progress = completeLesson(
      storage,
      progress,
      "installation/install-kongctl",
    );
    expect(progress.completed).toEqual(["installation/install-kongctl"]);
    expect(readProgress(storage)).toEqual(progress);
  });

  it("clears all progress", () => {
    completeLesson(storage, { completed: [] }, "installation/install-kongctl");

    expect(resetProgress(storage)).toEqual({ completed: [] });
    expect(storage.getItem(PROGRESS_STORAGE_KEY)).toBeNull();
  });
});
