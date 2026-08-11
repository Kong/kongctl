export const PROGRESS_STORAGE_KEY = "kongctl-learn-progress-v1";

export interface LearningProgress {
  completed: string[];
  lastVisited?: string;
}

interface StorageLike {
  getItem(key: string): string | null;
  removeItem(key: string): void;
  setItem(key: string, value: string): void;
}

const emptyProgress = (): LearningProgress => ({ completed: [] });

export function readProgress(storage: StorageLike): LearningProgress {
  const value = storage.getItem(PROGRESS_STORAGE_KEY);
  if (!value) {
    return emptyProgress();
  }

  try {
    const parsed = JSON.parse(value) as Partial<LearningProgress>;
    return {
      completed: Array.isArray(parsed.completed)
        ? [
            ...new Set(
              parsed.completed.filter(
                (item): item is string => typeof item === "string",
              ),
            ),
          ]
        : [],
      lastVisited:
        typeof parsed.lastVisited === "string" ? parsed.lastVisited : undefined,
    };
  } catch {
    return emptyProgress();
  }
}

export function writeProgress(
  storage: StorageLike,
  progress: LearningProgress,
): void {
  storage.setItem(PROGRESS_STORAGE_KEY, JSON.stringify(progress));
}

export function visitLesson(
  storage: StorageLike,
  progress: LearningProgress,
  lessonKey: string,
): LearningProgress {
  const updated = { ...progress, lastVisited: lessonKey };
  writeProgress(storage, updated);
  return updated;
}

export function completeLesson(
  storage: StorageLike,
  progress: LearningProgress,
  lessonKey: string,
): LearningProgress {
  const updated = {
    ...progress,
    completed: [...new Set([...progress.completed, lessonKey])],
    lastVisited: lessonKey,
  };
  writeProgress(storage, updated);
  return updated;
}

export function resetProgress(storage: StorageLike): LearningProgress {
  storage.removeItem(PROGRESS_STORAGE_KEY);
  return emptyProgress();
}
