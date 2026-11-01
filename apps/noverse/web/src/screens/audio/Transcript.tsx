// Bản đọc (#/p/:p/s/:s/audio/:v) — Transcript.dc.html 06.
// Trạng thái: review · locked · drafting · stale · allDone; tab Phân vai · Cần xem · Cách đọc;
// chế độ nhiều giọng / một giọng. Nghe thử, sửa chữ, đổi người nói, chốt chương, soạn phần còn lại.
import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties } from 'react'
import { navigate, paths } from '../../router'
import {
  audioChapterURL, composeAudioScripts, getAudioScript, lockAudioChapter, previewAudioSegment,
  putAudioScript, savePronunciationResource, refreshAudioChapter, renderAudioChapter, enqueueAudioChapters, unlockAudioChapter, uploadProjectVoice,
  getPronunciationResource, ConflictError,
} from '../../api/client'
import { useSession } from '../../api/session'
import type { AudioCast, AudioNormalize, AudioPronunciation, AudioScript, AudioSegment } from '../../api/types'
import { Icon } from '@noverse/ui'
import { Button } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { EmptyState, Skeleton, Spinner } from '@noverse/ui'
import { cache, keys, setData, useAction, useResource } from '../../data'
import { ConflictBar } from '@noverse/ui'
import { ConfirmDialog } from '@noverse/ui'
import { useSaveShortcut } from '@noverse/ui'
import { AudioFeedback, AudioSkeleton } from './AudioFeedback'
import { MachineCard, useMachineCardActions, useMachineStatus } from './MachineCard'
import { PipelineBar } from './PipelineBar'
import { SegmentRow } from './SegmentRow'
import { CastPanel, DictPanel, IssuesPanel, RightPanel, type PanelTab } from './panels'
import { AddVoiceDialog, useQualityChange } from './dialogs'
import {
  allDone, allLocked, chapterAudioLine, chapterStatusLabel, clockOf, filterSegments, formatClock,
  formatDuration, issueSegments, speakerFilters, stepIssueIndex,
  type SegmentFilter,
} from './logic'
import { errText, loadAudioScript, useAudioAccess, useAudioData, usePlayer, usePreviewAudio } from './useAudio'
import { usePackVoice } from './packVoice'
import { Eq } from './ui'
import './audio.css'
import { t } from '../../i18n/vi'
import { ScrollTopButton } from '@noverse/ui'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'
import { useUnsavedGuard } from '../../ui/useUnsavedGuard'

/** Chiều cao hàng thao tác đáy: nút 36px + đệm 12px trên/dưới (xem khối footer bên dưới). */
const TRANSCRIPT_FOOTER_H = 60

export function TranscriptScreen({ project, story, volume }: { project: string; story: string; volume: number }) {
  const data = useAudioData(project, story, volume)
  // Hàng thao tác sát đáy (Chốt & tạo giọng chương) không được bị dock việc nền đè:
  // khai báo chiều cao hàng đó để dock nhấc lên trên (shell/jobs/JobsDock DOCK_BOTTOM).
  useEffect(() => {
    const root = document.documentElement
    root.style.setProperty('--dock-bottom', `${TRANSCRIPT_FOOTER_H}px`)
    return () => { root.style.removeProperty('--dock-bottom') }
  }, [])
  const { writable } = useAudioAccess(project, story)
  const machine = useMachineStatus()
  const machineActions = useMachineCardActions()
  const { user } = useSession()
  const pack = usePackVoice(project, story)
  const chapters = data.volume?.chapters ?? []
  const [chapter, setChapter] = useState<number | null>(null)
  const [filter, setFilter] = useState<SegmentFilter>('all')
  const [showDiff, setShowDiff] = useState(false)
  const [selected, setSelected] = useState<string | null>(null)
  const [editing, setEditing] = useState<{ id: string; text: string; original: string; version?: string } | null>(null)
  const [tab, setTab] = useState<PanelTab>('cast')
  const qualityChange = useQualityChange(data)
  const [addVoice, setAddVoice] = useState(false)
  const [castChange, setCastChange] = useState<AudioCast | null>(null)
  const scriptKey = keys.story(project, story, 'audio', 'script', volume, chapter)
  const scriptResource = useResource(scriptKey, ({ signal }) => loadAudioScript(project, story, volume, chapter!, signal), { enabled: chapter !== null, keepPrevious: true })
  const script = scriptResource.data ?? null
  const pronunciationKey = keys.project(project, 'pronunciations')
  const dictionary = useResource(pronunciationKey, ({ signal }) => getPronunciationResource(project, signal))
  const pronunciations = dictionary.data?.entries ?? []
  const [scriptConflict, setScriptConflict] = useState<AudioScript | null>(null)
  const [dictConflict, setDictConflict] = useState<AudioPronunciation[] | null>(null)
  const [chapterPlaying, setChapterPlaying] = useState(false)
  const listRef = useRef<HTMLDivElement>(null)
  const headingRef = useRef<HTMLHeadingElement>(null)
  const preview = usePreviewAudio()

  const chapterInfo = chapters.find((item) => item.chapter === chapter) ?? null
  const complete = !!chapterInfo?.audio && chapterInfo.audio.segmentsDone >= chapterInfo.audio.segments

  // Chọn chương mặc định: chương đang chờ duyệt đầu tiên, không có thì chương đầu.
  useEffect(() => {
    if (chapter !== null || chapters.length === 0) return
    const review = chapters.find((item) => item.status === 'review') ?? chapters[0]
    setChapter(review.chapter)
  }, [chapters, chapter])


  useEffect(() => {
    setFilter('all'); setSelected(null); setEditing(null); setChapterPlaying(false); setScriptConflict(null)
  }, [chapter])

  const segments = script?.segments ?? []
  const mode = data.cast?.mode ?? 'cast'
  const issues = useMemo(() => issueSegments(segments, mode), [segments, mode])
  const shown = useMemo(() => filterSegments(segments, filter, mode), [segments, filter, mode])
  const chapterUrl = chapter !== null && complete ? audioChapterURL(project, story, volume, chapter, chapterInfo?.audio?.version) : null
  const player = usePlayer(chapterUrl)
  useEffect(() => { if (!chapterUrl) setChapterPlaying(false) }, [chapterUrl])

  const scriptSave = useAction(async (next: AudioScript, force = false) => {
    try {
      const saved = await putAudioScript(project, story, volume, next.chapter, next.segments, force ? '' : next.version)
      setData(keys.story(project, story, 'audio', 'script', volume, next.chapter), saved)
      setScriptConflict(null)
      return saved
    } catch (cause) {
      if (cause instanceof ConflictError) setScriptConflict(next)
      throw cause
    }
  }, {
    invalidate: [keys.story(project, story, 'audio')],
    optimistic: (next, _force = false) => {
      const key = keys.story(project, story, 'audio', 'script', volume, next.chapter)
      const previous = setData(key, next)
      return () => cache.restore(key, previous)
    },
  })
  const scriptBusy = scriptSave.busy
  const saveSegments = async (next: AudioSegment[], version = script?.version) => {
    if (!script || !writable || scriptBusy || scriptConflict || chapterInfo?.status === 'locked' || chapterInfo?.status === 'draft') return
    return scriptSave.run({ ...script, segments: next, version })
  }

  const playSegment = (segment: AudioSegment) => {
    if (segment.pending || (segment.audio?.state !== 'ready' && !writable)) return
    void preview.play(segment.id, async () => {
      if (segment.audio?.state === 'ready') return segment.audio.url
      const result = await previewAudioSegment(project, story, volume, chapter ?? 1, segment.id)
      setData<AudioScript>(scriptKey, (current) => current ? {
        ...current,
        segments: current.segments.map((item) => item.id === segment.id
          ? { ...item, audio: { url: result.url, state: 'ready', durationSec: result.durationSec } }
          : item),
      } : script!)
      return result.url
    }).catch((cause) => toast(errText(cause), 'err', { label: 'Thử lại', onClick: () => playSegment(segment) }))
  }

  const playChapter = () => {
    if (!chapterUrl) return
    setChapterPlaying(true)
    player.toggle()
  }

  const changeSpeaker = (segmentId: string, speakerId: string) => {
    void saveSegments(segments.map((segment) => segment.id === segmentId ? {
      ...segment, speaker: speakerId, speakerGuess: undefined, flags: segment.flags?.filter((flag) => flag !== 'unknown_speaker'),
      changed: true, audio: segment.audio ? { ...segment.audio, state: 'stale' as const } : undefined,
    } : segment))
  }

  const commitEdit = async () => {
    if (!editing || scriptBusy) return
    const text = editing.text.trim()
    if (!text) return
    const target = segments.find((segment) => segment.id === editing.id)
    if (target && target.text !== text) {
      const changed = { ...target, text, normalized: [], changed: true, audio: target.audio ? { ...target.audio, state: 'stale' as const } : undefined }
      const saved = await saveSegments(segments.map((segment) => segment.id === editing.id ? changed : segment), editing.version)
      if (!saved) return
    }
    setEditing(null)
  }
  const editDirty = !!editing && editing.text !== editing.original
  useUnsavedGuard(editDirty || !!scriptConflict || !!dictConflict)
  useSaveShortcut(commitEdit, !!editing?.text.trim() && editDirty && writable && !scriptBusy && !scriptConflict)

  const splitSegment = (segment: AudioSegment) => {
    const words = segment.text.split(/\s+/)
    if (words.length < 6) { toast.err(t.audio.transcript.doanQuaNganDe); return }
    const cut = Math.ceil(words.length / 2)
    const first = { ...segment, id: `${segment.id}a`, text: words.slice(0, cut).join(' '), normalized: [], audio: undefined, changed: true, flags: segment.flags?.filter((flag) => flag !== 'long_sentence') }
    const second = {
      ...segment, id: `${segment.id}b`, text: words.slice(cut).join(' '), normalized: [], audio: undefined, changed: true, flags: segment.flags?.filter((flag) => flag !== 'long_sentence'),
      pauseMs: segment.pauseMs,
    }
    const next: AudioSegment[] = []
    for (const item of segments) {
      if (item.id !== segment.id) { next.push(item); continue }
      next.push(first, second)
    }
    void saveSegments(next)
  }

  const chapterAction = useAction(async (action: 'lock' | 'unlock' | 'refresh' | 'render', target: number) => {
    if (action === 'unlock') await unlockAudioChapter(project, story, volume, target)
    else if (action === 'refresh') await refreshAudioChapter(project, story, volume, target)
    else if (action === 'render') {
      const current = chapters.find(item => item.chapter === target)
      if (current?.chapterChanged || current?.scriptChanged || current?.status === 'stale') {
        await enqueueAudioChapters(project, story, volume, {
          chapters: [target], quality: data.settings?.quality ?? 'standard',
          voiceMode: data.cast?.voiceMode ?? data.cast?.mode ?? 'cast', lock: true,
        })
      } else await renderAudioChapter(project, story, volume, target)
    }
    else await lockAudioChapter(project, story, volume, target)
    return action
  }, { invalidate: [keys.story(project, story, 'audio'), keys.jobs()], success: 'Đã cập nhật bản đọc' })
  const lock = () => chapter !== null && chapterAction.run('lock', chapter)
  const unlock = () => chapter !== null && chapterAction.run('unlock', chapter)
  const composeAction = useAction(async (todo: number[]) => {
    await composeAudioScripts(project, story, volume, todo)
  }, { success: 'Đang soạn các chương còn lại · xem trong khay công việc', invalidate: [keys.story(project, story, 'audio')] })

  const jumpToSegment = (segmentId: string) => {
    setFilter('all')
    setSelected(segmentId)
    requestAnimationFrame(() => {
      const node = listRef.current?.querySelector(`[data-segment="${segmentId}"]`)
      node?.scrollIntoView({ block: 'center', behavior: 'smooth' })
    })
  }

  const nextIssue = useCallback(() => {
    const from = selected ? segments.findIndex((segment) => segment.id === selected) : -1
    const index = stepIssueIndex(segments, from, 1, mode)
    if (index >= 0) jumpToSegment(segments[index].id)
  }, [segments, selected, mode])

  // Phím tắt §5: J/K chỗ cần xem · P nghe đoạn đang chọn · Enter sửa chữ.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null
      if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.tagName === 'SELECT' || target.isContentEditable)) return
      if (event.metaKey || event.ctrlKey || event.altKey) return
      const key = event.key.toLowerCase()
      if (key === 'j' || key === 'k') {
        event.preventDefault()
        const from = selected ? segments.findIndex((segment) => segment.id === selected) : -1
        const index = stepIssueIndex(segments, from, key === 'j' ? 1 : -1, mode)
        if (index >= 0) jumpToSegment(segments[index].id)
        return
      }
      if (key === 'p') {
        const target2 = segments.find((segment) => segment.id === selected)
        if (target2) { event.preventDefault(); playSegment(target2) }
        return
      }
      if (key === 'enter' && selected && writable && !scriptBusy && !scriptConflict && chapterInfo?.status !== 'locked' && chapterInfo?.status !== 'draft') {
        const target3 = segments.find((segment) => segment.id === selected)
        if (target3) { event.preventDefault(); setEditing({ id: target3.id, text: target3.text, original: target3.text, version: script?.version }) }
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })

  const dictionarySave = useAction(async (entries: AudioPronunciation[], version: string | undefined, force = false) => {
    try {
      const result = await savePronunciationResource(project, entries, force ? '' : version)
      setData(pronunciationKey, { entries, version: result.version ?? '' })
      setDictConflict(null)
      return result
    } catch (cause) {
      if (cause instanceof ConflictError) setDictConflict(entries)
      throw cause
    }
  }, {
    success: t.audio.transcript.daLuuCachDoc,
    invalidate: [pronunciationKey, keys.story(project, story, 'audio')],
    optimistic: (entries, _version: string | undefined, _force = false) => {
      const previous = setData(pronunciationKey, { entries, version: dictionary.data?.version ?? '' })
      return () => cache.restore(pronunciationKey, previous)
    },
  })
  const dictionaryDelete = useAction(async (entry: AudioPronunciation, _index: number) => {
    const current = await getPronunciationResource(project)
    const removeAt = current.entries.findIndex((item) => item.written === entry.written && item.spoken === entry.spoken)
    const entries = current.entries.filter((_, at) => at !== removeAt)
    try {
      const result = await savePronunciationResource(project, entries, current.version)
      setData(pronunciationKey, { entries, version: result.version ?? '' })
      return entry
    } catch (cause) {
      if (cause instanceof ConflictError) setDictConflict(entries)
      throw cause
    }
  }, {
    success: 'Đã xoá cách đọc',
    invalidate: [pronunciationKey, keys.story(project, story, 'audio')],
    optimistic: (entry, index) => {
      const current = dictionary.data
      if (current) setData(pronunciationKey, { ...current, entries: current.entries.filter((item, at) => at !== index || item !== entry) })
      return () => cache.restore(pronunciationKey, current)
    },
    undo: async (entry, [, index]) => {
      const current = await getPronunciationResource(project)
      if (current.entries.some((item) => item.written === entry.written && item.spoken === entry.spoken)) return
      const entries = [...current.entries]
      entries.splice(Math.min(index, entries.length), 0, entry)
      const result = await savePronunciationResource(project, entries, current.version)
      setData(pronunciationKey, { entries, version: result.version ?? '' })
    },
  })
  const viewScript = useAction(async (target: number) => {
    const latest = await getAudioScript(project, story, volume, target)
    setData(keys.story(project, story, 'audio', 'script', volume, target), latest)
    setScriptConflict(null); setEditing(null)
  })
  const viewDictionary = useAction(async () => {
    const latest = await getPronunciationResource(project)
    setData(pronunciationKey, latest)
    setDictConflict(null)
  })
  const saveEntries = async (entries: AudioPronunciation[]) => {
    if (!dictionary.data) return false
    return !!await dictionarySave.run(entries, dictionary.data.version)
  }
  const saveNormalize = (normalize: AudioNormalize) => {
    if (data.settings) void data.saveSettings({ ...data.settings, normalize })
  }
  if (data.loading && !data.volume) return <AudioSkeleton />
  if (data.error && !data.volume) {
    return (
      <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <EmptyState title={t.audio.transcript.khongMoDuocBan} hint={data.error} action={<Button onClick={data.refresh}>{t.audio.transcript.thuLai}</Button>} />
      </div>
    )
  }
  if (!data.volume) return <AudioSkeleton />
  if (chapters.length === 0) return <EmptyState title="Chưa có chương để soạn bản đọc" hint="Nghiệm thu chương trước khi soạn bản đọc audio." />
  if (chapter === null || !chapterInfo) return <AudioSkeleton />

  const done = allDone(chapters)
  const nextChapter = chapters.find((item) => item.chapter === (chapter ?? 0) + 1) ?? null
  const filters = speakerFilters(segments, data.cast)
  const locked = chapters.filter((item) => item.status === 'locked').length
  const todo = chapters.filter((item) => item.status === 'todo')
  const status = chapterInfo.status
  const editable = writable && !scriptBusy && !scriptConflict && status !== 'locked' && status !== 'draft'
  const draftDone = segments.filter((segment) => !segment.pending).length
  const lineCounts: Record<string, number> = {}
  for (const segment of segments) lineCounts[segment.speaker] = (lineCounts[segment.speaker] ?? 0) + 1
  const issueCount = issues.length
  const chapterMeta = status === 'draft'
    ? t.audio.transcript.dangSoanMeta(draftDone, chapterInfo.segments)
    : chapterInfo.durationSec
      ? t.audio.transcript.doanNguoiNoi(chapterInfo.segments, chapterInfo.speakers, Math.max(1, Math.round(chapterInfo.durationSec / 60)))
      : t.audio.transcript.doanNguoiNoiChuaCo(chapterInfo.segments, chapterInfo.speakers)
  const lockClock = clockOf(chapterInfo.lockedAt)
  const renderingNow = Boolean(chapterInfo.audio?.rendering)
  const footTone = status === 'locked' ? 'ok' : status === 'draft' || status === 'stale' ? 'muted' : issueCount > 0 ? 'warn' : 'muted'
  const footColor = footTone === 'ok' ? '#A6EDC1' : footTone === 'warn' ? '#F6CB8C' : '#A0A0AB'
  const footIcon = status === 'locked' ? 'tick-circle-bulk' : status === 'draft' ? 'magic-star' : footTone === 'warn' ? 'warning-2' : 'info-circle'
  const footText = status === 'locked'
    ? (done ? t.audio.transcript.duChuongDaChot(chapters.length, volume) : t.audio.transcript.chuongDaChot(chapter))
    : status === 'draft' ? t.audio.transcript.choAiSoanXong
      : status === 'stale' ? t.audio.transcript.doanThayDoiChuaDuyet(chapterInfo.changedSegments)
        : issueCount > 0 ? t.audio.transcript.conChoCanXem(issueCount) : t.audio.transcript.khongConChoNao
  const primary: CSSProperties = {
    flex: '0 0 auto', whiteSpace: 'nowrap', height: 36, borderRadius: 9, border: 'none', background: '#EDEDEF',
    color: '#0B0B0D', fontSize: 13, fontWeight: 600, display: 'flex', alignItems: 'center',
  }
  const banner = (tone: 'ok' | 'warn'): CSSProperties => ({
    flex: '0 0 auto', margin: '12px 24px 0', padding: '10px 12px', borderRadius: 12,
    background: tone === 'ok' ? 'rgba(166,237,193,0.06)' : 'rgba(246,203,140,0.06)',
    border: `1px solid ${tone === 'ok' ? 'rgba(166,237,193,0.25)' : 'rgba(246,203,140,0.25)'}`,
    display: 'flex', alignItems: 'center', gap: 10, fontSize: 13,
  })
  const bannerBtn: CSSProperties = {
    flex: '0 0 auto', whiteSpace: 'nowrap', height: 28, padding: '0 10px', borderRadius: 7,
    fontSize: 12, display: 'flex', alignItems: 'center', gap: 6,
  }
  const bannerPrimaryInk = '#0B0B0D'
  const bannerPrimary: CSSProperties = { ...bannerBtn, border: 'none', background: '#EDEDEF', color: bannerPrimaryInk, fontWeight: 600 }

  return (
    <div style={{ flex: 1, minWidth: 0, minHeight: 0, display: 'flex', flexDirection: 'column' }}>
      <PipelineBar project={project} story={story} volume={volume} mode="review" chapters={chapters} jobs={data.queue}
        quality={qualityChange.quality} onQuality={qualityChange.ask}
        audioRunning={data.queue.some((job) => job.state === 'rendering')} />
      <AudioFeedback data={data} project={project} story={story} />
      <MachineCard view={machine.view} onAction={machineActions.onAction} style={{ margin: '12px 24px 0' }} />
      {(composeAction.error || chapterAction.error || data.composeError) && <div role="alert" style={{ padding: '8px 16px', color: 'var(--color-err)', fontSize: 12 }}>
        {errText(composeAction.error || chapterAction.error || data.composeError)}
        {user?.role === 'admin' && <a href={`#${paths.settings('providers')}`} style={{ marginLeft: 8, color: 'var(--color-info)' }}>{t.audio.transcript.tuyChinhNhaCungCap}</a>}
      </div>}

      <div style={{ flex: 1, minHeight: 0, display: 'flex' }}>
        <nav aria-label={t.audio.transcript.chuongNav} className="scroll" style={{
          flex: '0 0 264px', padding: '14px 10px', borderRight: '1px solid rgba(255,255,255,0.06)',
          display: 'flex', flexDirection: 'column', gap: 12,
        }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', padding: '0 8px' }}>
            <span style={{ fontSize: 13, fontWeight: 600 }}>{t.audio.transcript.banDocTheoChuong}</span>
            <span className="num" style={{ fontSize: 12, color: '#7E7E8A' }}>{t.audio.transcript.nDaChot(locked, chapters.length)}</span>
          </div>
          <div style={{ padding: '0 8px' }}>
            <div role="progressbar" aria-label={t.audio.transcript.chuongDaChotBar} aria-valuemin={0} aria-valuemax={chapters.length} aria-valuenow={locked}
              style={{ height: 3, borderRadius: 3, background: '#1E1E21', overflow: 'hidden' }}>
              <div style={{ width: `${Math.round(locked / chapters.length * 100)}%`, height: '100%', borderRadius: 3, background: '#A6EDC1' }} />
            </div>
          </div>
          <ol style={{ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: 1 }}>
            {chapters.map((item) => {
              const line = chapterAudioLine(item)
              const on = item.chapter === chapter
              const st = item.status
              const stColor = st === 'locked' ? '#A6EDC1' : st === 'review' ? '#C9A6F5' : st === 'draft' ? '#A0A0AB' : st === 'stale' ? '#F6CB8C' : '#7E7E8A'
              const lineColor = line?.tone === 'ok' ? '#A6EDC1' : line?.tone === 'accent' ? '#C9A6F5' : line?.tone === 'warn' ? '#F6CB8C' : '#A0A0AB'
              const showDur = item.durationSec !== null && st !== 'draft' && st !== 'todo' && line?.tone !== 'ok'
              return (
                <li key={item.chapter}>
                  <button type="button" className="row hov-row" onClick={() => {
                    if (editDirty && !window.confirm(t.audio.transcript.doiChuongBoThayDoi)) return
                    setChapter(item.chapter)
                  }} disabled={scriptBusy || chapterAction.busy} aria-current={on ? 'page' : undefined}
                    style={{
                      width: '100%', display: 'flex', alignItems: 'flex-start', gap: 10, padding: 8, borderRadius: 9, border: 'none',
                      background: on ? '#1E1E21' : 'transparent', color: '#EDEDEF', textAlign: 'left',
                    }}>
                    <span className="num" style={{ flex: '0 0 18px', fontSize: 12, color: '#7E7E8A', paddingTop: 1 }}>{item.chapter}</span>
                    <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 3 }}>
                      <span style={{ fontSize: 13, color: st === 'todo' ? '#7E7E8A' : '#EDEDEF', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{item.title}</span>
                      <span style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 11.5, color: stColor }}>
                        {st === 'locked' && <Icon name="lock" size={12} />}
                        {st === 'draft' && <Spinner size={10} style={{ borderWidth: 1.5 }} />}
                        {st === 'review' && <span aria-hidden="true" style={{ width: 7, height: 7, borderRadius: '50%', background: '#C9A6F5' }} />}
                        {st === 'stale' && <Icon name="warning-2" size={12} />}
                        {st === 'todo' && <span aria-hidden="true" style={{ width: 9, height: 9, borderRadius: '50%', border: '1.5px dashed rgba(255,255,255,0.3)' }} />}
                        <span className={st === 'draft' ? 'audio-shim' : undefined}>{chapterStatusLabel(item, mode)}</span>
                      </span>
                      {line && (
                        <span style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 11.5, color: lineColor }}>
                          <Icon name="headphone" size={12} />
                          <span className="num" title={line.text} style={{minWidth:0,overflow:'hidden',textOverflow:'ellipsis',whiteSpace:'nowrap'}}>{line.text}</span>
                          {line.running && (
                            <span aria-hidden="true" style={{ flex: '0 0 48px', height: 3, borderRadius: 3, background: '#1E1E21', overflow: 'hidden' }}>
                              <span className="audio-run" style={{ display: 'block', width: `${line.percent}%`, height: '100%' }} />
                            </span>
                          )}
                        </span>
                      )}
                    </span>
                    <span className="num" style={{ fontSize: 11.5, color: '#7E7E8A', paddingTop: 1 }}>{showDur ? formatClock(item.durationSec ?? 0) : ''}</span>
                  </button>
                </li>
              )
            })}
          </ol>
          <button type="button" className="hov-ghost" title={t.audio.transcript.aiSoanChoChuongChua}
            disabled={!writable || composeAction.busy || todo.length === 0}
            onClick={() => void composeAction.run(todo.map((item) => item.chapter))}
            style={{
              margin: '0 8px', height: 34, padding: '0 12px', borderRadius: 9, border: '1px solid rgba(255,255,255,0.1)',
              background: 'transparent', color: '#EDEDEF', fontSize: 12.5, display: 'flex', alignItems: 'center',
              justifyContent: 'center', gap: 6, whiteSpace: 'nowrap',
            }}>
            {composeAction.pending ? <Spinner size={14} color="#EDEDEF" /> : <Icon name="magic-star" size={15} style={{ color: '#C9A6F5' }} />}
            {todo.length > 0 ? t.audio.transcript.soanConLai(todo.length) : t.audio.transcript.daSoanDu(chapters.length)}
          </button>
          <div style={{
            margin: '0 8px', padding: '10px 12px', borderRadius: 12, border: '1px solid rgba(255,255,255,0.07)',
            display: 'flex', gap: 8, fontSize: 12, color: '#A0A0AB',
          }}>
            <Icon name="info-circle" size={14} style={{ marginTop: 2, flex: '0 0 auto' }} />
            <span>{t.audio.transcript.ngheThuDoanNaoLa}</span>
          </div>
        </nav>

        <main style={{ flex: '999 1 560px', minWidth: 0, display: 'flex', flexDirection: 'column' }}>
          <PanelRefreshBar resourceKeys={[scriptResource.key]} updatedAt={scriptResource.updatedAt} />
          <span aria-live="polite" className="sr-only">
            {scriptSave.pending ? t.audio.transcript.dangLuuBanDoc : scriptSave.success ? t.audio.transcript.daLuu : ''}
          </span>
          {!!scriptResource.error && <div role="alert" style={{ padding: '8px 16px', color: 'var(--color-err)', fontSize: 12 }}>
            {script ? t.audio.transcript.khongTaiLaiDuoc : t.audio.transcript.khongMoDuocBan} · {errText(scriptResource.error)}
            <Button size="sm" variant="ghost" onClick={() => void scriptResource.refresh()}>{t.audio.transcript.thuLai}</Button>
          </div>}
          {scriptConflict && <ConflictBar pending={scriptSave.pending || viewScript.pending}
            onView={() => viewScript.run(scriptConflict.chapter)}
            onOverwrite={() => scriptSave.run(scriptConflict, true)} />}
          <div style={{ flex: '0 0 auto', padding: '14px 24px 10px', display: 'flex', flexDirection: 'column', gap: 10, borderBottom: '1px solid rgba(255,255,255,0.05)' }}>
            <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12, flexWrap: 'wrap' }}>
              <div style={{ flex: 1, minWidth: 260 }}>
                <div style={{ fontSize: 12, color: '#7E7E8A' }}>{t.audio.transcript.chuongN(chapter)}</div>
                <h2 ref={headingRef} tabIndex={-1} style={{ margin: '2px 0 0', fontSize: 20, fontWeight: 600, letterSpacing: '-0.01em' }}>{chapterInfo.title}</h2>
                <div className="num" style={{ fontSize: 12.5, color: '#A0A0AB', marginTop: 2 }}>{chapterMeta}</div>
              </div>
              <button type="button" className="hov-ghost" onClick={() => setShowDiff((on) => !on)} aria-pressed={showDiff}
                style={{
                  height: 32, padding: '0 12px', borderRadius: 8, border: '1px solid rgba(255,255,255,0.1)',
                  background: showDiff ? '#222225' : 'transparent', color: '#EDEDEF', fontSize: 12.5, display: 'flex', alignItems: 'center', gap: 6,
                }}>
                <Icon name="document-text" size={15} style={{ color: '#A0A0AB' }} />{t.audio.transcript.soVoiChuongGoc}
              </button>
              <button type="button" className="hov-ghost" disabled={!complete} onClick={playChapter}
                title={complete ? undefined : t.audio.transcript.chuongChuaCoDu}
                style={{
                  height: 32, padding: '0 12px', borderRadius: 8, border: '1px solid rgba(255,255,255,0.1)',
                  background: 'transparent', color: '#EDEDEF', fontSize: 12.5, display: 'flex', alignItems: 'center', gap: 6,
                }}>
                {chapterPlaying && player.playing ? <Eq color="currentColor" height={12} /> : <Icon name="play" size={14} />}
                {t.audio.transcript.ngheThuCaChuong}
              </button>
            </div>
            {chapterPlaying && complete && (
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <button type="button" className="hov-ghost" onClick={() => player.toggle()} aria-label={t.audio.transcript.phatDungChuong}
                  style={{ width: 24, height: 24, borderRadius: '50%', border: '1px solid rgba(255,255,255,0.1)', color: '#EDEDEF', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                  <Icon name={player.playing ? 'pause' : 'play'} size={11} />
                </button>
                <span className="num" style={{ fontSize: 11.5, color: '#7E7E8A' }}>
                  {formatClock(player.time)} / {formatClock(player.duration || chapterInfo.durationSec || 0)}
                </span>
                <span style={{ flex: 1, height: 4, borderRadius: 4, background: '#1E1E21', overflow: 'hidden' }}>
                  <span style={{
                    display: 'block', height: '100%', borderRadius: 4, background: '#C9A6F5',
                    width: `${player.duration > 0 ? (player.time / player.duration) * 100 : 0}%`,
                  }} />
                </span>
                <button type="button" onClick={() => { player.pause(); setChapterPlaying(false) }}
                  style={{ fontSize: 12, color: '#A0A0AB' }}>{t.audio.transcript.dung}</button>
              </div>
            )}
            <div role="tablist" aria-label={t.audio.transcript.locDoan} style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
              {[
                { id: 'all' as SegmentFilter, label: t.audio.transcript.tatCaDoan, count: chapterInfo.segments, warn: false },
                { id: 'issues' as SegmentFilter, label: t.audio.transcript.canXem, count: issueCount, warn: true },
                ...filters.map((chip) => ({ id: chip.id, label: chip.label, count: chip.count, warn: false })),
              ].map((chip) => {
                const on = filter === chip.id
                return (
                  <button key={chip.id} role="tab" type="button" aria-selected={on} onClick={() => setFilter(chip.id)} style={{
                    height: 28, padding: '0 10px', borderRadius: 999, fontSize: 12, display: 'flex', alignItems: 'center', gap: 6,
                    border: `1px solid ${on ? 'rgba(255,255,255,0.3)' : 'rgba(255,255,255,0.08)'}`,
                    background: on ? '#222225' : 'transparent', color: on ? '#EDEDEF' : '#A0A0AB',
                  }}>
                    {chip.label}
                    {chip.count > 0 && <span className="num" style={{ fontSize: 11, color: chip.warn ? '#F6CB8C' : '#7E7E8A' }}>{chip.count}</span>}
                  </button>
                )
              })}
            </div>
          </div>

          {status === 'locked' && (
            <div role="status" style={banner('ok')}>
              <Icon name="lock" size={16} style={{ color: '#A6EDC1' }} />
              <span style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 4 }}>
                <span>{complete ? t.audio.transcript.daChotXongDungLuon : renderingNow ? t.audio.transcript.daChotLucDangTao(lockClock) : t.audio.transcript.daChotLuc(lockClock)}</span>
                {renderingNow && (
                  <span style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12, color: '#A0A0AB' }}>
                    <span aria-hidden="true" style={{ flex: '0 0 160px', height: 4, borderRadius: 4, background: '#1E1E21', overflow: 'hidden' }}>
                      <span className="audio-run" style={{ display: 'block', width: `${Math.round(chapterInfo.audio?.percent ?? 0)}%`, height: '100%' }} />
                    </span>
                    <span className="num" style={{ minWidth: 0, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                      {t.audio.transcript.doanCoGiong(chapterInfo.audio?.segmentsDone ?? 0, chapterInfo.segments)}</span>
                    <a href={`#${paths.audio(project, story, volume, 'make')}`} style={{ flex: '0 0 auto', color: '#C9C9D1', whiteSpace: 'nowrap' }}>{t.audio.transcript.xemTienDo}</a>
                  </span>
                )}
              </span>
              {!complete && !renderingNow && (
                <button type="button" className="primary hov-primary" disabled={!writable || chapterAction.busy || scriptBusy}
                  onClick={() => void chapterAction.run('render', chapter)}
                  style={bannerPrimary}>
                  {chapterAction.pending ? <Spinner size={14} color={bannerPrimaryInk} /> : <Icon name="refresh-2" size={14} />}
                  {t.audio.transcript.taoGiongDoanConThieu(Math.max(0, (chapterInfo.audio?.segments ?? chapterInfo.segments) - (chapterInfo.audio?.segmentsDone ?? 0)))}
                </button>
              )}
              <button type="button" className="hov-ghost" disabled={!writable || chapterAction.busy || scriptBusy} onClick={() => void unlock()}
                style={{ ...bannerBtn, border: '1px solid rgba(255,255,255,0.12)', background: 'transparent', color: '#EDEDEF' }}>
                {chapterAction.pending ? <Spinner size={14} color="#EDEDEF" /> : <Icon name="unlock" size={14} />}{t.audio.transcript.moKhoaDeSua}
              </button>
            </div>
          )}
          {(status === 'stale' || chapterInfo.chapterChanged || chapterInfo.scriptChanged) && (
            <div role="status" style={banner('warn')}>
              <Icon name="warning-2" size={16} style={{ color: '#F6CB8C' }} />
              <span style={{ flex: 1 }}>{chapterInfo.staleReason || t.audio.transcript.chuongNayDaDuocSua(chapterInfo.changedSegments)}</span>
              <button type="button" className="primary hov-primary" disabled={!writable || chapterAction.busy || scriptBusy}
                onClick={() => void chapterAction.run('render', chapter)}
                style={bannerPrimary}>
                {chapterAction.pending ? <Spinner size={14} color={bannerPrimaryInk} /> : <Icon name="refresh-2" size={14} />}{t.audio.generation.redo}
              </button>
            </div>
          )}

          <div style={{ flex: 1, minHeight: 0, position: 'relative', display: 'flex' }}>
          <div ref={listRef} className="scroll" style={{ flex: 1, minHeight: 0, padding: '12px 16px 16px' }}>
            {status === 'draft' && (
              <div role="status" style={{ margin: '4px 8px 12px', display: 'flex', alignItems: 'center', gap: 10, fontSize: 13 }}>
                <Icon name="magic-star" size={16} style={{ color: '#C9A6F5' }} />
                <span className="audio-shim">{t.audio.transcript.aiDangSoanBanDoc(chapter, draftDone, chapterInfo.segments)}</span>
              </div>
            )}
            {scriptResource.initialLoading && !script && <TranscriptSkeleton />}
            {!script && !scriptResource.initialLoading && !scriptResource.error && status === 'todo' && <EmptyState
              title={t.audio.transcript.chuaSoan}
              hint={t.audio.transcript.soanBanDocDeTach}
              action={writable ? <Button pending={composeAction.pending} disabled={composeAction.busy} onClick={() => void composeAction.run([chapter])}>{t.audio.transcript.soanBanDocChuong(chapter)}</Button> : undefined}
            />}
            {script && shown.length === 0 && (
              <div style={{ padding: '18px 8px', fontSize: 12.5, color: '#7E7E8A' }}>
                {filter === 'issues' ? t.audio.transcript.khongConChoNao2 : t.audio.transcript.khongCoDoanNao}
              </div>
            )}
            <ol style={{ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: 2 }}>
              {shown.map((segment) => (
                <SegmentRow key={segment.id} segment={segment} index={segments.indexOf(segment)}
                  cast={data.cast} writable={editable}
                  showDiff={showDiff} selected={selected === segment.id}
                  dimmed={filter === 'issues' && !issues.includes(segment)}
                  playing={preview.playingId === segment.id} loading={preview.loadingId === segment.id}
                  editing={editing?.id === segment.id} draft={editing?.id === segment.id ? editing.text : ''}
                  saving={scriptSave.pending}
                  onSelect={() => setSelected(segment.id)}
                  onPlay={() => playSegment(segment)}
                  onEdit={() => setEditing({ id: segment.id, text: segment.text, original: segment.text, version: script?.version })}
                  onDraft={(text) => setEditing((current) => current ? { ...current, text } : current)}
                  onCommit={commitEdit} onCancel={() => setEditing(null)}
                  onSpeaker={(speakerId) => changeSpeaker(segment.id, speakerId)}
                  onSplit={() => splitSegment(segment)}
                  onGuess={(who) => changeSpeaker(segment.id, who)} />
              ))}
            </ol>
          </div>
            <ScrollTopButton target={listRef} focusTarget={headingRef} label={t.scrollTop.chapter} size="md" />
          </div>

          <div style={{
            flex: '0 0 auto', display: 'flex', alignItems: 'center', gap: 12, padding: '12px 24px',
            borderTop: '1px solid rgba(255,255,255,0.06)', background: '#0B0B0D',
          }}>
            <span title={footText} style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, color: footColor }}>
              <Icon name={footIcon} size={16} />
              <span style={{ minWidth: 0, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{footText}</span>
            </span>
            {status === 'locked' ? (
              nextChapter ? (
                <button type="button" className="primary hov-primary" onClick={() => setChapter(nextChapter.chapter)}
                  style={{ ...primary, padding: '0 12px 0 14px', gap: 6 }}>
                  {t.audio.transcript.sangChuong(nextChapter.chapter)}<Icon name="arrow-right-2" size={15} />
                </button>
              ) : (
                <button type="button" className="primary hov-primary" disabled={!allLocked(chapters)}
                  onClick={() => navigate(paths.audio(project, story, volume, 'listen'))}
                  style={{ ...primary, padding: '0 14px 0 12px', gap: 8 }}>
                  <Icon name="headphone" size={15} />{t.audio.pipeline.ngheXuatTap(volume)}
                </button>
              )
            ) : status === 'draft' ? null : (
              <>
                {(
                  <button type="button" className="hov-ghost" onClick={nextIssue} title={t.audio.transcript.choCanXemTiepTheo}
                    style={{
                      flex: '0 0 auto', height: 36, padding: '0 10px', borderRadius: 9, border: 'none', background: 'transparent',
                      color: '#A0A0AB', fontSize: 13, whiteSpace: 'nowrap', display: 'flex', alignItems: 'center', gap: 8,
                    }}>
                    {t.audio.transcript.choCanXemTiep}
                    <span className="kbd" style={{
                      fontFamily: 'var(--font-mono)', fontSize: 11, color: '#A0A0AB', padding: '1px 5px', borderRadius: 4,
                      border: '1px solid rgba(255,255,255,0.1)', background: '#151517',
                    }}>J</span>
                  </button>
                )}
                <button type="button" className="primary hov-primary"
                  disabled={!writable || !script || !!scriptConflict || scriptBusy || chapterAction.busy}
                  onClick={async () => {
                    if (status === 'stale') await chapterAction.run('render', chapter)
                    else await lock()
                  }}
                  style={{ ...primary, padding: '0 14px 0 12px', gap: 8 }}>
                  {chapterAction.pending ? <Spinner size={14} color="#0B0B0D" /> : <Icon name="lock" size={15} />}
                  {status === 'stale' ? t.audio.generation.redo : t.audio.transcript.chotVaTaoGiong(chapter)}
                </button>
              </>
            )}
          </div>
        </main>

        <RightPanel tab={tab} onTab={setTab} issues={issueCount} model={data.ready?.engine.model || t.audio.panels.boDocChuaRo}>
          {tab === 'cast' && (
            <CastPanel cast={data.cast} voices={data.voices} lineCounts={lineCounts} pack={pack} styleHref={`${paths.story(project, story)}?tab=style`} writable={writable && !data.savePending && !data.conflict}
              onCast={(next) => {
                if (next.mode !== data.cast?.mode && chapters.some((item) => (item.audio?.segmentsDone ?? 0) > 0)) setCastChange(next)
                else void data.saveCast(next)
              }} onAddVoice={() => setAddVoice(true)} />
          )}
          {tab === 'issues' && (
            <IssuesPanel segments={segments} cast={data.cast} mode={mode} onJump={jumpToSegment} onCastTab={() => setTab('cast')} />
          )}
          {tab === 'dict' && (
            <>
              <PanelRefreshBar resourceKeys={[dictionary.key]} updatedAt={dictionary.updatedAt} />
              {!!dictionary.error && <div role="alert" style={{ fontSize: 12, color: 'var(--color-err)' }}>
                {dictionary.data ? t.audio.transcript.khongTaiLaiDuoc : t.audio.transcript.khongMoDuocCachDoc} · {errText(dictionary.error)}
                <Button size="sm" variant="ghost" onClick={() => void dictionary.refresh()}>{t.audio.transcript.thuLai}</Button>
              </div>}
              {dictConflict && <ConflictBar pending={dictionarySave.pending || viewDictionary.pending}
                onView={() => viewDictionary.run()}
                onOverwrite={() => dictionarySave.run(dictConflict, undefined, true)} />}
              {dictionary.initialLoading && !dictionary.data ? <TranscriptSkeleton rows={3} /> : dictionary.data && (
                <DictPanel entries={pronunciations} settings={data.settings}
                  voiceId={mode === 'single' ? data.cast?.single.voiceId : data.cast?.speakers.find((speaker) => speaker.voiceId)?.voiceId}
                  writable={writable && !dictionarySave.busy && !dictionaryDelete.busy && !dictConflict}
                  pending={dictionarySave.pending || dictionaryDelete.pending} success={dictionarySave.success || dictionaryDelete.success}
                  onDelete={(entry, index) => void dictionaryDelete.run(entry, index)}
                  onSaveEntries={saveEntries} onNormalize={saveNormalize} />
              )}
            </>
          )}
        </RightPanel>
      </div>

      {qualityChange.dialog}
      {castChange && <ConfirmDialog title={t.audio.transcript.doiCheDoGiongDoc} tone="warn"
        body={t.audio.transcript.chuyenSangTaoLai(castChange.mode === 'single' ? t.audio.panels.motGiong : t.audio.panels.phanVai2,
          formatDuration(chapters.reduce((sum, item) => sum + (item.audio?.durationSec ?? 0), 0) * (data.ready?.engine.rtf ?? 1)))}
        confirmLabel={t.audio.transcript.doiVaTaoLai} pendingLabel={t.audio.transcript.dangDoiCheDo} onClose={() => setCastChange(null)}
        onConfirm={async () => { await data.saveCast(castChange) }} />}
      {addVoice && (
        <AddVoiceDialog onClose={() => setAddVoice(false)}
          onUpload={async (name, file, gender) => {
            await uploadProjectVoice(project, name, file, gender)
            await data.reloadVoices()
            toast.ok(t.audio.transcript.daThemMauGiong)
          }} />
      )}
    </div>
  )
}

function TranscriptSkeleton({ rows = 5 }: { rows?: number }) {
  const [slow, setSlow] = useState(false)
  useEffect(() => {
    const timer = setTimeout(() => setSlow(true), 8000)
    return () => clearTimeout(timer)
  }, [])
  return <div role="status" aria-label={t.audio.transcript.dangMoBanDoc} style={{ padding: 12, display: 'grid', gap: 20 }}>
    {Array.from({ length: rows }, (_, index) => <div key={index} style={{ display: 'grid', gap: 9 }}>
      <Skeleton width="35%" height={20} /><Skeleton /><Skeleton width="80%" />
    </div>)}
    <span style={{ color: '#7E7E8A', fontSize: 12 }}>{slow ? t.audio.transcript.dangChamHon : t.audio.transcript.dangMoBanDoc}</span>
  </div>
}
