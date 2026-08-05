const base = import.meta.env.BASE_URL.endsWith("/")
  ? import.meta.env.BASE_URL
  : `${import.meta.env.BASE_URL}/`;

export function pageUrl(path = ""): string {
  const normalized = path.replace(/^\/+|\/+$/g, "");
  return normalized ? `${base}${normalized}/` : base;
}
