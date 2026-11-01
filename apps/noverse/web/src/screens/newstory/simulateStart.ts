// Gói AR (việc 6) — Mô phỏng văn phong khi tạo truyện: dán văn mẫu trước, rồi mới tạo truyện.
// 06-data-contract.md › Bổ sung §AR: createStory → POST S/style-samples từng đoạn (≤ 20) → POST S/simulate.
// Trước đây QuickMode tạo truyện rồi gọi simulate khi truyện chưa có văn mẫu nào, nên máy chủ trả
// “Chưa có văn mẫu nào để phân tích”. Dùng chung cho hộp thoại máy tính và ngăn M11 trên điện thoại.
import { useEffect, useRef, useState } from 'react'
import { createStory, createStyleSample, startSimulate } from '../../api/client'
import { invalidate, keys } from '../../data'

/** 18-mobile.md › M11: dán 1–20 đoạn; máy chủ cũng chặn ở 20 văn mẫu (409). */
export const SAMPLE_LIMIT = 20

/**
 * Một “đoạn” là một khối văn cách nhau bởi dòng trống (dán cả tệp .txt/.md).
 * Ngăn M11 vẽ mỗi đoạn một ô riêng; ô riêng giữ nguyên nội dung của nó.
 */
export function splitSampleText(text: string): string[] {
  return text.split(/\r?\n[ \t]*\r?\n/).map((part) => part.trim()).filter(Boolean)
}

/** Đoạn dùng được = đoạn không rỗng, tối đa 20 (đúng số ô ngăn M11 cho thêm). */
export function normalizeSamples(rows: readonly string[]): string[] {
  return rows.map((row) => row.trim()).filter(Boolean).slice(0, SAMPLE_LIMIT)
}

/** Số chữ như artboard (“Văn mẫu 1 · 42 chữ”) và như máy chủ đếm văn mẫu. */
export function sampleWords(text: string): number {
  const trimmed = text.trim()
  return trimmed ? trimmed.split(/\s+/).length : 0
}

export type SimulatePhase = 'idle' | 'creating' | 'samples' | 'simulating' | 'done'

/** Tiến trình để UI vẽ lý do khoá của M11 (“Đang tạo truyện để học văn phong…”). */
export type SimulateProgress = { phase: SimulatePhase; done: number; total: number; story: string | null }

export type SimulateStartResult = { story: string; samples: number }

const IDLE: SimulateProgress = { phase: 'idle', done: 0, total: 0, story: null }

/**
 * Bộ chạy dùng chung: giữ truyện đã tạo và các đoạn đã gửi, nên lần thử lại chỉ chạy
 * phần còn thiếu — không tạo truyện thứ hai (M11: “dùng lại truyện đã tạo”).
 */
export function simulateStarter() {
  let created: { project: string; story: string } | null = null
  // Số lần mỗi đoạn đã tạo thành công — đoạn trùng vẫn đếm riêng.
  const posted = new Map<string, number>()
  let lastRows: string[] = []

  const run = async (project: string, rows: readonly string[], report?: (progress: SimulateProgress) => void): Promise<SimulateStartResult> => {
    const samples = normalizeSamples(rows)
    lastRows = samples
    if (!samples.length) throw new Error('Chưa có đoạn văn mẫu nào')

    if (created?.project !== project) {
      report?.({ phase: 'creating', done: 0, total: samples.length, story: null })
      const summary = await createStory(project, {})
      created = { project, story: summary.story }
      posted.clear()
      void invalidate(keys.project(project))
      void invalidate(keys.library())
    }

    const story = created.story
    const seen = new Map<string, number>()
    let done = 0
    report?.({ phase: 'samples', done, total: samples.length, story })
    for (const text of samples) {
      const nth = (seen.get(text) ?? 0) + 1
      seen.set(text, nth)
      if ((posted.get(text) ?? 0) < nth) {
        await createStyleSample(project, story, { text })
        posted.set(text, nth)
      }
      done += 1
      report?.({ phase: 'samples', done, total: samples.length, story })
    }

    report?.({ phase: 'simulating', done, total: samples.length, story })
    await startSimulate(project, story)
    void invalidate(keys.jobs())
    void invalidate(keys.story(project, story))
    report?.({ phase: 'done', done, total: samples.length, story })
    // Xong việc: lần “Mô phỏng văn phong” sau phải tạo truyện mới, không dán văn mẫu vào truyện cũ
    // (màn Truyện mới được Activity giữ lại nên bộ chạy sống qua các lần mở ngăn).
    created = null
    posted.clear()
    lastRows = []
    return { story, samples: samples.length }
  }

  return {
    start: run,
    /** Chạy lại đúng danh sách vừa dán, tiếp tục từ bước hỏng. */
    retry: (project: string, report?: (progress: SimulateProgress) => void) => run(project, lastRows, report),
    /** Truyện đã tạo cho dự án này (nếu lần trước hỏng giữa đường). */
    story: (project: string) => created?.project === project ? created.story : null,
    reset: () => { created = null; posted.clear(); lastRows = [] },
  }
}

/** Hook cho UI: tiến trình, lỗi, và `start` trả về mã truyện khi xong (null khi lỗi). */
export function useSimulateStart(project: string) {
  const [starter] = useState(simulateStarter)
  const [progress, setProgress] = useState<SimulateProgress>(IDLE)
  const [error, setError] = useState<string | null>(null)
  const alive = useRef(true)
  useEffect(() => { alive.current = true; return () => { alive.current = false } }, [])

  const report = (next: SimulateProgress) => { if (alive.current) setProgress(next) }
  const attempt = async (call: () => Promise<SimulateStartResult>): Promise<string | null> => {
    setError(null)
    try {
      return (await call()).story
    } catch (cause) {
      if (alive.current) {
        // Truyện đã tạo vẫn nằm trong `starter` — lần thử lại chỉ chạy phần còn thiếu.
        setProgress((current) => ({ ...current, phase: 'idle' }))
        setError(cause instanceof Error ? cause.message : String(cause))
      }
      return null
    }
  }

  return {
    phase: progress.phase,
    done: progress.done,
    total: progress.total,
    story: progress.story ?? starter.story(project),
    error,
    busy: progress.phase === 'creating' || progress.phase === 'samples' || progress.phase === 'simulating',
    start: (rows: readonly string[]) => attempt(() => starter.start(project, rows, report)),
    retry: () => attempt(() => starter.retry(project, report)),
    /** Đóng ngăn rồi mở lại: bỏ hộp lỗi cũ, vẫn giữ truyện đã tạo để thử lại không tạo truyện thứ hai. */
    clearError: () => setError(null),
    reset: () => { starter.reset(); setProgress(IDLE); setError(null) },
  }
}
