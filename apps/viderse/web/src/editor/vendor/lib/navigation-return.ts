/**
 * Contextual return links: a page that sends the owner somewhere else may append
 * `?returnTo=<internal path>` so the destination can offer a way back that is
 * deterministic for direct links too (no `router.back()`). Pure — safe for the
 * client bundle and for server pages reading user-controlled search params.
 */

const ORIGIN = "http://return.invalid";
const MAX_LENGTH = 2048;
const MAX_DECODE_ROUNDS = 3;
const CONTROL = /[\u0000-\u001f\u007f-\u009f]/;

/** Targets that are never user-facing pages: APIs, framework assets, render/measure workers. */
function isBlockedPath(path: string): boolean {
  const lower = path.toLowerCase();
  if (lower === "/api" || lower.startsWith("/api/") || lower === "/_next" || lower.startsWith("/_next/")) return true;
  return /^\/editor\/[^/]+\/(?:render|measure)(?:\/|$)/.test(lower);
}

/** The raw path with every layer of percent-encoding removed; null when it cannot be read safely. */
function decodedPath(path: string): string | null {
  let current = path;
  for (let round = 0; round < MAX_DECODE_ROUNDS; round++) {
    let next: string;
    try {
      next = decodeURIComponent(current);
    } catch {
      return null;
    }
    if (next === current) return current;
    current = next;
  }
  // Still changing after several rounds: treat as an encoding bypass attempt.
  return null;
}

/**
 * Keep a short return chain for list → detail → editor. Bound its depth so repeated
 * cross-links cannot grow the URL indefinitely; each nested destination is validated.
 */
function normalizeReturn(raw: string | null | undefined, depth: number): string | null {
  if (typeof raw !== "string" || raw.length === 0 || raw.length > MAX_LENGTH) return null;
  if (!raw.startsWith("/") || raw.startsWith("//") || raw.includes("\\") || CONTROL.test(raw)) return null;
  const rawPath = raw.split(/[?#]/, 1)[0] ?? "";
  const decoded = decodedPath(rawPath);
  if (decoded === null || decoded.startsWith("//") || decoded.includes("\\") || CONTROL.test(decoded)) return null;
  if (decoded.split("/").some((segment) => segment === ".." || segment === ".")) return null;
  let url: URL;
  try {
    url = new URL(raw, ORIGIN);
  } catch {
    return null;
  }
  if (url.origin !== ORIGIN || isBlockedPath(url.pathname) || isBlockedPath(decoded)) return null;
  const params = new URLSearchParams(url.search);
  const nested = params.get("returnTo");
  params.delete("returnTo");
  const safeNested = depth > 0 ? normalizeReturn(nested, depth - 1) : null;
  if (safeNested) params.set("returnTo", safeNested);
  const search = params.size > 0 ? `?${params.toString().replace(/\+/g, "%20")}` : "";
  return `${url.pathname}${search}${url.hash}`;
}

/** A same-origin user-facing path, retaining at most three validated return destinations. */
export function safeReturnTo(raw: string | null | undefined): string | null {
  return normalizeReturn(raw, 3);
}

/**
 * `href` with `returnTo` attached. An unsafe return or external href is unchanged;
 * an existing return on `href` is replaced and the retained return chain is bounded.
 */
export function withReturnTo(href: string, returnTo: string): string {
  const safe = safeReturnTo(returnTo);
  if (safe === null || !href.startsWith("/") || href.startsWith("//") || href.includes("\\") || CONTROL.test(href)) return href;
  let url: URL;
  try {
    url = new URL(href, ORIGIN);
  } catch {
    return href;
  }
  if (url.origin !== ORIGIN) return href;
  const params = new URLSearchParams(url.search);
  params.set("returnTo", safe);
  return `${url.pathname}?${params.toString().replace(/\+/g, "%20")}${url.hash}`;
}

/** Plain Vietnamese name of where a return link leads, for "Quay lại …" copy. */
export function returnLabel(returnTo: string): string {
  const path = returnTo.split(/[?#]/, 1)[0] ?? "";
  const [, first = "", second, third] = path.split("/");
  switch (first) {
    case "":
      return "Tổng quan";
    case "clone":
      return second ? "video clone" : "Video";
    case "review":
      return "Duyệt";
    case "queue":
      return "Hàng chờ";
    case "channels":
      return second ? "kênh đối thủ" : "Đối thủ";
    case "videos":
      return "video nguồn";
    case "my-channels":
      return second ? "kênh của tôi" : "Kênh của tôi";
    case "templates":
      if (second === "backdrop-channels") return third ? "kênh nền" : "Kênh nền";
      return second ? "template" : "Template";
    case "editor":
      return "Editor";
    case "settings":
      return "Cài đặt";
    default:
      return "trang trước";
  }
}
