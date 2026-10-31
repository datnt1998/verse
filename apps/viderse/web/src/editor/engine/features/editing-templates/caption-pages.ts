import type { CaptionAnimationPreset } from '@editor/types/timeline'

/**
 * Shared caption page normaliser — the single source of truth for "one phrase
 * on screen at a time". DOM preview, canvas export and the native MP4 (ASS)
 * overlay all consume the pages this module produces, so every surface agrees
 * on page boundaries, word timing and the karaoke-vs-fade rule.
 *
 * Stored project data can be dirtier than freshly ingested cues: cues overlap
 * (YouTube rolling windows), one word piece can span several words, and pages
 * can grow past two lines. `resolveCaptionPages` repairs all of that on read —
 * it never mutates the item — and is memoised per (words, cues, fps, duration)
 * identity, so per-frame callers can use it directly.
 */

/** Colour spoken words turn under the karaoke highlight — shared by DOM, canvas and ASS. */
export const CAPTION_HIGHLIGHT_COLOR = '#facc15'

/** Page limits: one short phrase at a time, at most two lines at 1080p. */
export const CAPTION_PAGE_MAX_WORDS = 7
export const CAPTION_PAGE_MAX_CHARS = 42
/** A measured silence at least this long blanks the screen; anything shorter keeps the previous page up. */
const CAPTION_PAUSE_MS = 350

/** One displayable word: a single token inside its own measured/estimated window. */
export interface CaptionPageWord {
  text: string
  startFrame: number
  endFrame: number
  /** True when the source flagged the timing as estimated rather than measured. */
  estimated?: boolean
}

/**
 * One caption page: the only phrase shown inside its `[startFrame, endFrame)`
 * window. Pages never overlap and never exceed the word/char limits.
 */
export interface CaptionPage {
  id: string
  /** Plain text of the page — the word texts joined (or the cue text for a text-only page). */
  text: string
  /** Item-local frames. A word page starts at its first word and ends where the next page (or the cue window) ends. */
  startFrame: number
  endFrame: number
  /** Single-token words of this page; empty for text-only pages (static text). */
  words: CaptionPageWord[]
  /** Source-flagged estimated timing (or synthesised words) → render the page fading in, never karaoke. */
  estimated: boolean
}

/** Minimal word shape the normaliser accepts (stored `CaptionWord` fits, `estimated` is tolerated). */
export interface CaptionPageWordLike {
  text: string
  startFrame: number
  endFrame: number
  estimated?: boolean
}

/** Minimal item shape — both `TextItem` and `SubtitleSegmentItem` satisfy it. */
export interface CaptionPageSource {
  cues?: readonly { id: string; text: string; startSeconds: number; endSeconds: number }[]
  words?: readonly CaptionPageWordLike[]
  /** Item-level flag recorded when stored word timings are known to be estimated. */
  wordsEstimated?: boolean
  captionPreset?: CaptionAnimationPreset
  durationInFrames: number
  /** Subtitle items only — used to recognise clone-pipeline captions. */
  source?: { type: string; fileName?: string }
}

interface PageWindow {
  id: string
  text: string
  startFrame: number
  endFrame: number
}

interface Piece {
  text: string
  startFrame: number
  endFrame: number
  estimated?: boolean
}

/** Cue text carrying markup keeps it; plain text may be word-synthesised. */
const hasMarkup = (text: string) => /<|\{|\}/.test(text)

/** Spread `parts` across `[start, end)` by character length — deterministic, strictly in order. */
function distributeTokens(parts: readonly string[], start: number, end: number, estimated: boolean): CaptionPageWord[] {
  const total = parts.reduce((sum, part) => sum + part.length, 0)
  if (total === 0 || end <= start) return []
  const out: CaptionPageWord[] = []
  let consumed = 0
  for (const [index, part] of parts.entries()) {
    const left = start + Math.floor(((end - start) * consumed) / total)
    consumed += part.length
    const right = index === parts.length - 1 ? end : start + Math.floor(((end - start) * consumed) / total)
    if (right > left) out.push({ text: part, startFrame: left, endFrame: right, estimated })
  }
  return out
}

/**
 * Split a stored word piece into single-token words over `[start, end)`.
 * A single token keeps the piece timing verbatim; a multi-word piece is
 * distributed by character length. Every produced token carries the piece's
 * own `estimated` claim — the split itself adds none.
 */
function splitPiece(piece: Piece, start: number, end: number): CaptionPageWord[] {
  const parts = piece.text.trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return []
  if (parts.length === 1) return [{ text: parts[0]!, startFrame: start, endFrame: end, estimated: piece.estimated }]
  return distributeTokens(parts, start, end, piece.estimated === true)
}

/**
 * Non-overlapping caption windows from the item's cue list: sorted, dropped
 * when empty, each end clipped to the next start and to the item duration.
 * Without cues the words form one window, so text items page the same way.
 */
function captionWindows(item: CaptionPageSource, fps: number): PageWindow[] {
  const duration = Math.max(0, item.durationInFrames)
  const cues = item.cues
  if (cues?.length) {
    const sorted = [...cues]
      .filter((cue) => cue.endSeconds > cue.startSeconds && cue.endSeconds > 0)
      .sort((a, b) => a.startSeconds - b.startSeconds)
    const windows: PageWindow[] = []
    for (const [index, cue] of sorted.entries()) {
      const startFrame = Math.max(0, Math.round(cue.startSeconds * fps))
      const nextStart = sorted[index + 1] ? Math.round(sorted[index + 1]!.startSeconds * fps) : duration
      const endFrame = Math.min(Math.round(cue.endSeconds * fps), nextStart, duration)
      if (endFrame > startFrame) windows.push({ id: cue.id, text: cue.text, startFrame, endFrame })
    }
    return windows
  }
  const words = item.words ?? []
  if (words.length === 0) return []
  const first = Math.min(...words.map((word) => word.startFrame))
  return [{ id: 'words', text: words.map((word) => word.text.trim()).filter(Boolean).join(' '), startFrame: Math.max(0, first), endFrame: duration }]
}

/**
 * Core normaliser. Also reports `degraded` — true when timing was synthesised
 * for a wordless cue or a word piece was dropped (it never displays) — which
 * the ingest path folds into the item-level `wordsEstimated` flag. Clipping a
 * piece to its page window and splitting a multi-word piece are plain
 * normalisation, not degradation: whether such timing is trustworthy is the
 * source's own `estimated` claim, carried on the words.
 */
export function buildCaptionPages(item: CaptionPageSource, fps: number): { pages: CaptionPage[]; degraded: boolean } {
  const windows = captionWindows(item, fps)
  const pieces = [...(item.words ?? [])].sort((a, b) => a.startFrame - b.startFrame || a.endFrame - b.endFrame)
  const pages: CaptionPage[] = []
  let degraded = item.wordsEstimated === true
  let index = 0
  for (const win of windows) {
    // Pieces are claimed by the window containing their start; a piece whose
    // start precedes every remaining window never displays.
    while (index < pieces.length && pieces[index]!.startFrame < win.startFrame) {
      degraded = true
      index++
    }
    const cuePieces: Piece[] = []
    while (index < pieces.length && pieces[index]!.startFrame < win.endFrame) cuePieces.push(pieces[index++]!)

    const tokens: CaptionPageWord[] = []
    if (cuePieces.length === 0) {
      // A cue with no words still needs displayable timing: plain text gets
      // evenly distributed estimated words; marked-up text stays a static page.
      if (win.text.trim() && !hasMarkup(win.text)) {
        tokens.push(...distributeTokens(win.text.trim().split(/\s+/).filter(Boolean), win.startFrame, win.endFrame, true))
        if (tokens.length) degraded = true
      }
    } else {
      for (const piece of cuePieces) {
        const start = Math.min(Math.max(piece.startFrame, win.startFrame), win.endFrame)
        const end = Math.min(Math.max(piece.endFrame, win.startFrame), win.endFrame)
        if (end <= start) {
          degraded = true
          continue
        }
        const produced = splitPiece(piece, start, end)
        if (produced.length === 0) {
          degraded = true
          continue
        }
        tokens.push(...produced)
      }
    }

    // Pack short phrases, but measured pauses leave a genuinely empty window.
    const groups: CaptionPageWord[][] = []
    let group: CaptionPageWord[] = []
    let chars = 0
    for (const token of tokens) {
      const added = token.text.length + (group.length > 0 ? 1 : 0)
      const previous = group.at(-1)
      const pause = previous && !item.wordsEstimated && !previous.estimated && !token.estimated &&
        (token.startFrame - previous.endFrame) * 1000 / fps >= CAPTION_PAUSE_MS
      if (group.length > 0 && (pause || group.length >= CAPTION_PAGE_MAX_WORDS || chars + added > CAPTION_PAGE_MAX_CHARS)) {
        groups.push(group)
        group = []
        chars = 0
      }
      group.push(token)
      chars += token.text.length + (group.length > 1 ? 1 : 0)
    }
    if (group.length > 0) groups.push(group)

    if (groups.length === 0) {
      if (!win.text.trim()) continue
      pages.push({ id: win.id, text: win.text, startFrame: win.startFrame, endFrame: win.endFrame, words: [], estimated: item.wordsEstimated === true })
      continue
    }
    for (const [pageIndex, words] of groups.entries()) {
      const startFrame = words[0]!.startFrame
      const next = groups[pageIndex + 1]?.[0]
      const last = words.at(-1)!
      const pause = next && !item.wordsEstimated && !last.estimated && !next.estimated &&
        (next.startFrame - last.endFrame) * 1000 / fps >= CAPTION_PAUSE_MS
      const endFrame = pause ? last.endFrame : next?.startFrame ?? win.endFrame
      pages.push({
        id: pageIndex === 0 ? win.id : `${win.id}.${pageIndex + 1}`,
        text: words.map((word) => word.text).join(' '),
        startFrame,
        endFrame,
        words,
        estimated: item.wordsEstimated === true || words.some((word) => word.estimated === true),
      })
    }
  }
  // Word pieces whose start never fell inside a displayable window are dropped.
  if (index < pieces.length) degraded = true
  // Degenerate zero-length windows fold their words into the neighbouring page.
  for (let i = pages.length - 1; i >= 0; i--) {
    const page = pages[i]!
    if (page.endFrame > page.startFrame) continue
    const neighbour = pages[i - 1] ?? pages[i + 1]
    if (neighbour && neighbour.words.length > 0 && page.words.length > 0) {
      neighbour.words.push(...page.words)
      neighbour.text = neighbour.words.map((word) => word.text).join(' ')
      neighbour.estimated = neighbour.estimated || page.estimated
    }
    pages.splice(i, 1)
  }
  // Across cues too, a blank shorter than a pause would only flash: hold the page until the next one.
  for (let i = 0; i + 1 < pages.length; i++) {
    const gap = pages[i + 1]!.startFrame - pages[i]!.endFrame
    if (gap > 0 && gap * 1000 / fps < CAPTION_PAUSE_MS) pages[i]!.endFrame = pages[i + 1]!.startFrame
  }
  return { pages, degraded }
}

const NO_WORDS: readonly CaptionPageWordLike[] = []
const NO_CUES: NonNullable<CaptionPageSource['cues']> = []
type PageCache = { fps: number; duration: number; wordsEstimated: boolean | undefined; pages: CaptionPage[] }
const pagesCache = new WeakMap<object, WeakMap<object, PageCache>>()

/**
 * Memoised normaliser keyed on the `words`/`cues` array identities — safe for
 * per-frame callers: a store update produces new arrays, so stale entries can
 * never survive an edit.
 */
export function resolveCaptionPages(item: CaptionPageSource, fps: number): CaptionPage[] {
  const wordsKey: object = item.words ?? NO_WORDS
  const cuesKey: object = item.cues ?? NO_CUES
  let inner = pagesCache.get(wordsKey)
  const hit = inner?.get(cuesKey)
  if (hit && hit.fps === fps && hit.duration === item.durationInFrames && hit.wordsEstimated === item.wordsEstimated) return hit.pages
  const pages = buildCaptionPages(item, fps).pages
  if (!inner) {
    inner = new WeakMap()
    pagesCache.set(wordsKey, inner)
  }
  inner.set(cuesKey, { fps, duration: item.durationInFrames, wordsEstimated: item.wordsEstimated, pages })
  return pages
}

/** Binary search for the page whose `[startFrame, endFrame)` window contains `frame`. */
export function activeCaptionPage<T extends { startFrame: number; endFrame: number }>(pages: readonly T[], frame: number): T | null {
  let lo = 0
  let hi = pages.length - 1
  while (lo <= hi) {
    const mid = (lo + hi) >> 1
    const page = pages[mid]!
    if (frame < page.startFrame) hi = mid - 1
    else if (frame >= page.endFrame) lo = mid + 1
    else return page
  }
  return null
}


/**
 * Honour the stored preset in preview and export. The explicit estimated
 * timing flag only demotes karaoke to fade; other presets stay owner-chosen.
 */
export function effectiveCaptionPreset(item: CaptionPageSource, page: Pick<CaptionPage, 'estimated'> | null | undefined): CaptionAnimationPreset {
  const estimated = item.wordsEstimated === true || page?.estimated === true
  const preset = item.captionPreset ?? 'none'
  return preset === 'karaoke' && estimated ? 'fade' : preset
}
