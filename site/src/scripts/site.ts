import { initializeCodeBlocks } from "./copy";
import {
  completeLesson,
  readProgress,
  resetProgress,
  visitLesson,
  type LearningProgress,
} from "./progress";

interface LearningData {
  currentLessonKey?: string;
  lessons: Array<{
    chapterId: string;
    key: string;
    url: string;
  }>;
}

function readLearningData(): LearningData | undefined {
  const element = document.querySelector("#learning-data");
  if (!element?.textContent) {
    return undefined;
  }

  try {
    return JSON.parse(element.textContent) as LearningData;
  } catch {
    return undefined;
  }
}

function initializeTheme(): void {
  const button = document.querySelector<HTMLButtonElement>(
    "[data-theme-toggle]",
  );
  if (!button) {
    return;
  }

  const updateLabel = () => {
    const current = document.documentElement.dataset.theme ?? "light";
    button.setAttribute(
      "aria-label",
      `Switch to ${current === "dark" ? "light" : "dark"} theme`,
    );
    button.title = button.getAttribute("aria-label") ?? "Switch color theme";
  };
  updateLabel();

  button.addEventListener("click", () => {
    const next =
      document.documentElement.dataset.theme === "dark" ? "light" : "dark";
    document.documentElement.dataset.theme = next;
    try {
      localStorage.setItem("kongctl-learn-theme", next);
    } catch {
      // Theme switching still works when storage is unavailable.
    }
    updateLabel();
  });
}

function initializeNavigation(): void {
  const toggle = document.querySelector<HTMLButtonElement>("[data-nav-toggle]");
  const backdrop = document.querySelector<HTMLButtonElement>(
    "[data-sidebar-backdrop]",
  );

  const close = () => {
    document.body.classList.remove("nav-open");
    toggle?.setAttribute("aria-expanded", "false");
    if (backdrop) {
      backdrop.hidden = true;
    }
  };

  toggle?.addEventListener("click", () => {
    const open = !document.body.classList.contains("nav-open");
    document.body.classList.toggle("nav-open", open);
    toggle.setAttribute("aria-expanded", String(open));
    if (backdrop) {
      backdrop.hidden = !open;
    }
  });
  backdrop?.addEventListener("click", close);
  document
    .querySelectorAll<HTMLAnchorElement>("[data-sidebar] a")
    .forEach((link) => link.addEventListener("click", close));
}

function initializeFilter(): void {
  const filter = document.querySelector<HTMLInputElement>(
    "[data-lesson-filter]",
  );
  if (!filter) {
    return;
  }

  filter.addEventListener("input", () => {
    const query = filter.value.trim().toLowerCase();
    let matches = 0;

    document
      .querySelectorAll<HTMLElement>("[data-lesson-item]")
      .forEach((item) => {
        const match = !query || (item.dataset.search ?? "").includes(query);
        item.hidden = !match;
        matches += match ? 1 : 0;
      });

    document
      .querySelectorAll<HTMLElement>("[data-nav-chapter]")
      .forEach((chapter) => {
        chapter.hidden = !chapter.querySelector(
          "[data-lesson-item]:not([hidden])",
        );
      });

    const empty = document.querySelector<HTMLElement>("[data-filter-empty]");
    if (empty) {
      empty.hidden = matches !== 0;
    }
  });
}

function updateProgressUI(
  progress: LearningProgress,
  learningData: LearningData,
): void {
  const knownKeys = new Set(learningData.lessons.map((lesson) => lesson.key));
  const completed = new Set(
    progress.completed.filter((key) => knownKeys.has(key)),
  );

  document
    .querySelectorAll<HTMLElement>("[data-progress-key]")
    .forEach((mark) => {
      mark.classList.toggle(
        "complete",
        completed.has(mark.dataset.progressKey ?? ""),
      );
    });

  document
    .querySelectorAll<HTMLProgressElement>("[data-progress-bar]")
    .forEach((bar) => {
      bar.value = completed.size;
    });
  document
    .querySelectorAll<HTMLElement>("[data-progress-text]")
    .forEach((text) => {
      text.textContent = `${completed.size} of ${learningData.lessons.length}`;
    });

  const counts = new Map<string, number>();
  for (const lesson of learningData.lessons) {
    if (completed.has(lesson.key)) {
      counts.set(lesson.chapterId, (counts.get(lesson.chapterId) ?? 0) + 1);
    }
  }
  document
    .querySelectorAll<HTMLElement>("[data-chapter-count]")
    .forEach((counter) => {
      const chapterId = counter.dataset.chapterCount ?? "";
      const total = learningData.lessons.filter(
        (lesson) => lesson.chapterId === chapterId,
      ).length;
      counter.textContent = `${counts.get(chapterId) ?? 0}/${total}`;
    });

  const resumeLesson = learningData.lessons.find(
    (lesson) => lesson.key === progress.lastVisited,
  );
  document
    .querySelectorAll<HTMLAnchorElement>("[data-resume-link]")
    .forEach((link) => {
      link.hidden = !resumeLesson;
      if (resumeLesson) {
        link.href = resumeLesson.url;
      }
    });
  document
    .querySelectorAll<HTMLAnchorElement>("[data-start-link]")
    .forEach((link) => {
      if (resumeLesson) {
        link.href = resumeLesson.url;
        link.firstChild!.textContent = "Resume learning ";
      }
    });

  document
    .querySelectorAll<HTMLButtonElement>("[data-reset-progress]")
    .forEach((button) => {
      button.hidden = completed.size === 0 && !resumeLesson;
    });
}

function initializeProgress(): void {
  const learningData = readLearningData();
  if (!learningData) {
    return;
  }

  let progress: LearningProgress;
  try {
    progress = readProgress(localStorage);
    if (learningData.currentLessonKey) {
      progress = visitLesson(
        localStorage,
        progress,
        learningData.currentLessonKey,
      );
    }
  } catch {
    progress = { completed: [] };
  }
  updateProgressUI(progress, learningData);

  document
    .querySelectorAll<HTMLAnchorElement>("[data-complete-and-continue]")
    .forEach((link) => {
      link.addEventListener("click", () => {
        const key = link.dataset.completeAndContinue;
        if (!key) {
          return;
        }
        try {
          progress = completeLesson(localStorage, progress, key);
          updateProgressUI(progress, learningData);
        } catch {
          // Navigation remains available when storage is unavailable.
        }
      });
    });

  document
    .querySelectorAll<HTMLButtonElement>("[data-reset-progress]")
    .forEach((button) => {
      button.addEventListener("click", () => {
        try {
          progress = resetProgress(localStorage);
          updateProgressUI(progress, learningData);
        } catch {
          // There is no stored progress to reset when storage is unavailable.
        }
      });
    });
}

export function initializeSite(): void {
  initializeCodeBlocks();
  initializeTheme();
  initializeNavigation();
  initializeFilter();
  initializeProgress();
}
