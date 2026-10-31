const compact = new Intl.NumberFormat("vi", { notation: "compact", maximumFractionDigits: 1 });

export function formatCount(n: number | null | undefined): string {
  return n === null || n === undefined ? "—" : compact.format(n);
}

/** 308 → "5:08", 3723 → "1:02:03". */
export function formatDuration(sec: number | null): string | null {
  if (sec === null) return null;
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = String(sec % 60).padStart(2, "0");
  return h > 0 ? `${h}:${String(m).padStart(2, "0")}:${s}` : `${m}:${s}`;
}

const AGO_STEPS: [number, string][] = [
  [365 * 86_400_000, "năm"],
  [30 * 86_400_000, "tháng"],
  [7 * 86_400_000, "tuần"],
  [86_400_000, "ngày"],
  [3_600_000, "giờ"],
  [60_000, "phút"],
];

/** Elapsed time in plain Vietnamese: "3 ngày trước", "2 tháng trước"; "vừa xong" under a minute. */
export function formatAgo(date: Date | null, now = Date.now()): string {
  if (!date) return "—";
  const elapsed = now - date.getTime();
  const step = AGO_STEPS.find(([ms]) => elapsed >= ms);
  return step ? `${Math.floor(elapsed / step[0])} ${step[1]} trước` : "vừa xong";
}

const languageNames = new Intl.DisplayNames(["vi"], { type: "language" });

/** Subtitle/language code → Vietnamese name ("en", "en-orig" → "Tiếng Anh"); unknown codes are returned as given. */
export function languageName(code: string): string {
  try {
    return languageNames.of(code.split("-")[0] ?? code) ?? code;
  } catch {
    return code;
  }
}
