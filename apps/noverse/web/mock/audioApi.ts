// Mock API Audio (GAP-17…20) — router con của /api/projects/:p/stories/:s/audio/*.
// Giữ đúng tên endpoint/trường của 10-audio.md §4 + các bổ sung A2Audio đã chốt:
//   segments/:id → `<chapter>-<segId>` · preview trả {url,durationSec,cached} · accept/reject
//   GET exports/:v · GET ready · DELETE flags/:id · POST demo {scenario}
// Export: m4b|mp3|wav, optional chapterMarks/subtitles/chapters, deferred whenReady;
// deferred responses are {files:[],chapterMarks:[],durationSec:0,pending:true}.
// onExists: replace (default)|keep|fail. Keep collisions append local -YYYYMMDD-HHmmss
// to main/companion filenames and the chapter directory; same-second collisions →409.
import type { IncomingMessage, ServerResponse } from 'node:http'
import { setTimeout as delay } from 'node:timers/promises'
import type { AudioSegment, AudioExportPrefs } from '../src/api/types'
import { defaultExportPrefs } from '../src/screens/audio/export'
import {
  activateAudioScenario, addFlag, audioChanged, audioVersion, audioVersionKey, cancelJob,
  chapterDuration, chapterRows, composeChapters, defaultExportOptions, lockChapter, markStale,
  MockAudioExportConflict,
  peaksFor, queueOf, queueTimelineOf, rangeOf, readyToExport, refreshChapter, requestAudioExport,
  scriptFor, setJobState, startRender, unlockChapter, volumeOf, wavFor,
  type MockAudioExportOptions, type MockAudioState,
} from './audio'
import { ttsStatus } from './tts'
import { takeExportFailure } from './exportDemo'
import { mockConflict } from './events'
import { mockProjectVoices, mockUploadVoice, mockVoiceSample } from './audioVoices'

const demoEngineDefaults = { ...ttsStatus() }

export type AudioApiContext = {
  req: IncomingMessage
  res: ServerResponse
  p: string
  s: string
  /** path sau 'audio' (đã decode) */
  rest: string[]
  state: MockAudioState
  canWrite: boolean
  readBody: () => Promise<Record<string, unknown>>
}

const json = (res: ServerResponse, status: number, data: unknown) => {
  const body = JSON.stringify(data)
  res.writeHead(status, { 'content-type': 'application/json; charset=utf-8', 'content-length': Buffer.byteLength(body) })
  res.end(body)
}
const fail = (res: ServerResponse, status: number, error: string) => json(res, status, { error })

const sendWav = (req: IncomingMessage, res: ServerResponse, buffer: Buffer) => {
  const range = rangeOf(buffer, req.headers.range)
  if (!range) {
    res.writeHead(416, { 'content-range': `bytes */${buffer.length}` })
    res.end()
    return
  }
  res.writeHead(req.headers.range ? 206 : 200, {
    'content-type': 'audio/wav',
    'accept-ranges': 'bytes',
    'content-length': range.body.length,
    'cache-control': 'no-store',
    ...(req.headers.range ? { 'content-range': `bytes ${range.start}-${range.end}/${range.total}` } : {}),
  })
  res.end(range.body)
}


const exportPrefs = new Map<string, AudioExportPrefs>()
export async function handleAudioApi(ctx: AudioApiContext): Promise<boolean> {
  const { req, res, rest, state } = ctx
  const method = req.method ?? 'GET'
  const volume = Number.parseInt(rest[1] ?? '', 10) || state.volume
  const locked = () => fail(res, 404, 'Không tìm thấy truyện')
  const conflict = (resource: string, current: unknown) => mockConflict(req, res, audioVersionKey(state, resource), current)
  const engineStatus = state.engineMissing
    ? { ...ttsStatus(), ready: false, state: 'missing' as const, rtf: null, installedBytes: 0 }
    : ttsStatus()

  // Mock-only deterministic seeds for desktop and mobile artboard states.
  if (rest[0] === 'demo' && method === 'POST') {
    if (!ctx.canWrite) return locked(), true
    const body = await ctx.readBody()
    const scenario = String(body.scenario ?? '')
    if (!activateAudioScenario(state, scenario, body.live === true, typeof body.len === 'number' ? body.len : undefined, typeof body.doneChapters === 'number' ? body.doneChapters : undefined)) return fail(res, 400, 'Không có kịch bản audio này'), true
    Object.assign(ttsStatus(), demoEngineDefaults)
    if (scenario === 'noEngine' || scenario === 'mobileNoEngine') {
      Object.assign(ttsStatus(), { ready: false, state: 'missing', rtf: null, installedBytes: 0 })
    } else if (scenario === 'ffmpegMissing') ttsStatus().ffmpeg = false
    return json(res, 200, { ok: true, volume: volumeOf(state), jobs: queueOf(state) }), true
  }
  if (rest[0] === 'export-prefs') {
    const key = `${state.project}/${state.story}`
    if (method === 'GET') return json(res, 200, exportPrefs.get(key) ?? defaultExportPrefs), true
    if (method === 'PUT') {
      if (!ctx.canWrite) return locked(), true
      const body = await ctx.readBody()
      if (!['mp3', 'm4b', 'wav'].includes(String(body.format)) || !['files', 'zip'].includes(String(body.pack)) || typeof body.gapSec !== 'number' || body.gapSec < 0 || body.gapSec > 10) return fail(res, 400, 'Tuỳ chọn xuất không hợp lệ.'), true
      const prefs: AudioExportPrefs = { format: body.format as AudioExportPrefs['format'], pack: body.pack as AudioExportPrefs['pack'], gapSec: body.gapSec,
        intro: body.intro === true, chapterTitles: body.chapterTitles === true, loudness: body.loudness === true, chapterMarks: body.chapterMarks === true, subtitles: body.subtitles === true }
      exportPrefs.set(key, prefs)
      return json(res, 200, prefs), true
    }
  }

  // ─── S/audio/events — SSE riêng của audio ───
  if (rest[0] === 'events') {
    res.writeHead(200, { 'content-type': 'text/event-stream', 'cache-control': 'no-cache', connection: 'keep-alive' })
    const client = { res }
    state.clients.add(client)
    res.write(`event: audio.queue\ndata: ${JSON.stringify({ type: 'audio.queue', category: 'AUDIO', data: { jobs: queueOf(state) } })}\n\n`)
    const keepAlive = setInterval(() => { try { res.write(':ka\n\n') } catch { /* đóng */ } }, 15000)
    req.on('close', () => { clearInterval(keepAlive); state.clients.delete(client) })
    return true
  }

  // ─── thiếu ffmpeg / bộ đọc chưa cài ───
  if (rest[0] === 'ready') {
    const engine = engineStatus
    const message = !engine.ffmpeg
      ? 'Máy chưa có ffmpeg nên chưa tạo hay ghép audio được. Vẫn soạn và duyệt bản đọc bình thường.'
      : engine.state === 'missing'
        ? 'Bộ đọc trên máy chưa được cài. Cài một lần để tạo giọng.'
        : engine.state === 'error'
          ? engine.error || 'Bộ đọc trên máy đang lỗi.'
          : ''
    return json(res, 200, {
      ffmpeg: engine.ffmpeg,
      engine,
      ready: engine.ready && engine.ffmpeg,
      message,
    }), true
  }

  // ─── volumes/:v ───
  if (rest[0] === 'volumes' && rest[1]) {
    const tail = rest[2]
    if (!tail && method === 'GET') return json(res, 200, volumeOf(state)), true

    if (tail === 'scripts' && !rest[3]) {
      if (method !== 'POST') return fail(res, 405, 'Method not allowed'), true
      if (!ctx.canWrite) return locked(), true
      const body = await ctx.readBody()
      const chapters = Array.isArray(body.chapters) ? (body.chapters as number[]) : []
      if (chapters.length === 0) return fail(res, 400, 'Chọn ít nhất một chương để soạn bản đọc'), true
      const lockedChapter = chapters.find((chapter) => state.chapters[chapter]?.status === 'locked')
      if (lockedChapter) return fail(res, 409, `Chương ${lockedChapter} đã chốt — mở khoá để soạn lại`), true
      composeChapters(state, chapters)
      return json(res, 200, { ok: true }), true
    }

    if (tail === 'scripts' && rest[3]) {
      const chapter = Number.parseInt(rest[3], 10)
      const seed = state.chapters[chapter]
      if (!seed) return fail(res, 404, 'Không tìm thấy chương'), true
      const action = rest[4]
      if (!action && method === 'GET') {
        const script = scriptFor(state, chapter)
        return script ? (json(res, 200, script), true) : (fail(res, 404, 'Chương chưa có bản đọc.'), true)
      }
      if (!action && method === 'PUT') {
        if (!ctx.canWrite) return locked(), true
        if (conflict(`script:${chapter}`, scriptFor(state, chapter))) return true
        if (seed.status === 'locked') return fail(res, 409, 'Chương đã chốt — mở khoá để sửa'), true
        const body = await ctx.readBody()
        const segments = Array.isArray(body.segments) ? (body.segments as AudioSegment[]) : []
        const previous = state.scripts.get(chapter) ?? []
        const changed = segments.filter((segment, index) => segment.text !== previous[index]?.text)
        const hadAudio = previous.some((segment) => segment.audio && segment.audio.state !== 'none')
        state.scripts.set(chapter, segments.map((segment) => ({
          ...segment,
          changed: changed.some((item) => item.id === segment.id) && hadAudio,
          audio: segment.audio ?? previous.find((item) => item.id === segment.id)?.audio ?? null,
        })))
        seed.totalSegments = segments.length
        if (changed.length === 0 || !hadAudio) audioChanged(state, `script:${chapter}`)
        if (changed.length > 0 && hadAudio) markStale(state, chapter, changed.length)
        return json(res, 200, scriptFor(state, chapter)), true
      }
      if (action === 'lock' && method === 'POST') {
        if (!ctx.canWrite) return locked(), true
        lockChapter(state, chapter, state.settings.autoOnLock)
        return json(res, 200, { ok: true }), true
      }
      if (action === 'unlock' && method === 'POST') {
        if (!ctx.canWrite) return locked(), true
        unlockChapter(state, chapter)
        return json(res, 200, { ok: true }), true
      }
      if (action === 'refresh' && method === 'POST') {
        if (!ctx.canWrite) return locked(), true
        refreshChapter(state, chapter)
        return json(res, 200, { ok: true }), true
      }
      return fail(res, 404, 'Not found'), true
    }

    if (tail === 'chapters' && rest[3]) {
      const file = rest[3]
      if (file.endsWith('.mp3') || file.endsWith('.wav')) {
        const chapter = Number.parseInt(file.replace(/\.(mp3|wav)$/, ''), 10)
        if (!Number.isFinite(chapter)) return fail(res, 404, 'Không tìm thấy chương'), true
        const row = chapterRows(state).find((item) => item.chapter === chapter)
        if (!row?.audio || row.audio.segmentsDone < row.audio.segments) {
          return fail(res, 409, 'Chương này chưa có đủ audio'), true
        }
        sendWav(req, res, wavFor(chapterDuration(chapter, state)))
        return true
      }
      const chapter = Number.parseInt(file, 10)
      if (!Number.isFinite(chapter)) return fail(res, 404, 'Không tìm thấy chương'), true
      if ((rest[4] === 'prioritize' || rest[4] === 'rerender') && method === 'POST') {
        if(!ctx.canWrite)return locked(),true
        const seed=state.chapters[chapter]
        if(!seed)return fail(res,404,'Không tìm thấy chương.'),true
        if(rest[4]==='prioritize'){
          const job=state.jobs.get(chapter)
          if(job?.state!=='queued')return fail(res,409,'Chỉ làm trước được chương đang chờ.'),true
          state.jobs=new Map([[chapter,job],...Array.from(state.jobs).filter(([n])=>n!==chapter)])
        }else{
          const body=await ctx.readBody()
          if(seed.status!=='locked')return fail(res,409,'Chương chưa chốt bản đọc.'),true
          const job=startRender(state,chapter);job.quality=(body.quality??state.settings.quality) as MockAudioState['settings']['quality']
        }
        audioChanged(state);return json(res,200,{ok:true}),true
      }
      if (rest[4] === 'render' && method === 'POST') {
        if (!ctx.canWrite) return locked(), true
        if (!engineStatus.ready) return fail(res, 503, 'Bộ đọc trên máy chưa sẵn sàng'), true
        if (!engineStatus.ffmpeg) return fail(res, 503, 'Máy chưa có ffmpeg để ghép audio'), true
        const seed = state.chapters[chapter]
        if (!seed) return fail(res, 404, 'Không tìm thấy chương'), true
        if (seed.status !== 'locked') return fail(res, 409, 'Chương chưa nghiệm thu'), true
        startRender(state, chapter, seed.totalSegments)
        return json(res, 200, { ok: true }), true
      }
      if (rest[4] === 'peaks' && method === 'GET') {
        const row = chapterRows(state).find((item) => item.chapter === chapter)
        if (!row?.audio || row.audio.segmentsDone < row.audio.segments) {
          return fail(res, 409, 'Chương này chưa có đủ audio'), true
        }
        const durationSec = chapterDuration(chapter, state)
        const peaks = state.scenario?.startsWith('mobile') && chapter === 4
          ? Array.from({ length: 48 }, (_, index) => Math.min(100, 18 + Math.round(Math.abs(Math.sin(index * 1.7) * 60 + Math.cos(index * 0.6) * 22))) / 100)
          : peaksFor(durationSec, chapter)
        return json(res, 200, { peaks, durationSec }), true
      }
      return fail(res, 404, 'Not found'), true
    }

    if (tail === 'exports') {
      if (method === 'GET') {
        return json(res, 200, state.exported ?? {
          volume: state.volume, files: [], chapterMarks: [], exportedAt: null,
          durationSec: 0, voices: 0, chapterCount: 0, dir: 'xuat-ban › tap-01', totalBytes: 0,
        }), true
      }
      if (method === 'POST') {
        if (!ctx.canWrite) return locked(), true
        const body = await ctx.readBody()
        if (!engineStatus.ffmpeg) return fail(res, 503, 'Máy chưa có ffmpeg để ghép audio'), true
        const format = body.format ?? defaultExportOptions.format
        if (format !== 'm4b' && format !== 'mp3' && format !== 'wav') return fail(res, 400, 'Định dạng audio không được hỗ trợ'), true
        let chapters: number[] | undefined
        if (body.chapters !== undefined && body.chapters !== null) {
          if (!Array.isArray(body.chapters) || body.chapters.some((chapter) => typeof chapter !== 'number' || !Number.isInteger(chapter))) {
            return fail(res, 400, 'Danh sách chương không hợp lệ'), true
          }
          chapters = [...new Set(body.chapters as number[])].sort((a, b) => a - b)
          if (chapters.length === 0) return fail(res, 409, 'Chọn ít nhất một chương để xuất.'), true
          const unknown = chapters.find((chapter) => chapter < 1 || !state.chapters[chapter])
          if (unknown !== undefined) return fail(res, 409, `Chương ${unknown} không thuộc tập này`), true
        }
        const onExists = body.onExists === '' ? 'replace' : body.onExists ?? 'replace'
        if (typeof onExists !== 'string') return fail(res, 400, 'Cách xử lý tệp đã có không hợp lệ'), true
        if (onExists !== 'replace' && onExists !== 'keep' && onExists !== 'fail' && onExists !== 'warn') return fail(res, 409, 'onExists phải là replace, keep, warn hoặc fail'), true
        const options: MockAudioExportOptions = {
          format,
          chapterMarks: body.chapterMarks === true,
          subtitles: body.subtitles === true,
          whenReady: body.whenReady === true,
          chapters,
          split: body.split === true,
          pack: body.pack === 'zip' ? 'zip' : 'files',
          trial: body.trial === true,
          onExists: onExists === 'warn' ? 'fail' : onExists,
          intro: body.intro === true,
          chapterTitles: body.chapterTitles === true,
          loudness: body.loudness === true,
          gapSec: typeof body.gapSec === 'number' && Number.isFinite(body.gapSec) ? Math.min(10, Math.max(0, body.gapSec)) : 0,
        }
        if (options.split && !chapters?.length) return fail(res, 409, 'Chọn ít nhất một chương để xuất.'), true
        if (options.trial && (chapters?.length !== 1 || options.whenReady)) return fail(res,409,'Bản thử chỉ xuất một chương đã có giọng.'),true
        if (!options.whenReady && !readyToExport(state, chapters)) {
          const missing = (chapters ?? chapterRows(state).map((row) => row.chapter)).find((chapter) => !readyToExport(state, [chapter]))
          return fail(res, 409, `Chương ${missing} chưa có đủ giọng`), true
        }
        try {
          // §AE — ghép chạy nền: POST trả {jobId} ngay; tiến độ/xong/lỗi qua SSE job.*.
          const demoFail = takeExportFailure()
          const job = requestAudioExport(state, {
            ...options,
            ...(body.demoFail === 'intro' ? { demoFail: 'intro' as const } : demoFail ? { demoFail } : {}),
          })
          // §AE: 202 Accepted — {jobId} ngay, kết quả qua SSE job.* (khớp máy chủ thật).
          return json(res, 202, job), true
        } catch (error) {
          if (error instanceof MockAudioExportConflict) return json(res, 409, { error: error.message, jobId: error.jobId }), true
          throw error
        }
      }
    }
    return fail(res, 404, 'Not found'), true
  }

  if (rest[0] === 'voices' && rest.length === 1 && method === 'POST') {
    if (!ctx.canWrite) return locked(), true
    const chunks: Buffer[] = []
    for await (const chunk of req) chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk))
    try { const voice = await mockUploadVoice(ctx.p, Buffer.concat(chunks), req.headers['content-type'] ?? ''); audioChanged(state); return json(res, 200, voice), true }
    catch (error) { return fail(res, 400, error instanceof Error ? error.message : 'Không đọc được mẫu giọng.'), true }
  }
  if (rest[0] === 'voices' && method === 'GET') {
    if (rest[1] && rest[2] === 'sample') {
      const clip = mockVoiceSample(ctx.p, rest[1])
      if (!clip) return fail(res, 404, 'Không tìm thấy mẫu giọng.'), true
      sendWav(req, res, clip)
      return true
    }
    return json(res, 200, mockProjectVoices(ctx.p)), true
  }
  // ─── cast / settings / queue ───
  if (rest[0] === 'cast') {
    const castView = () => ({
      ...state.cast,
      speakers: state.cast.speakers.map(speaker => {
        let oldSegments = 0, oldEtaSec = 0
        for (const segments of state.scripts.values()) for (const segment of segments) {
          if (segment.speaker === speaker.id && segment.audio && segment.audio.state !== 'none') { oldSegments++; oldEtaSec += segment.audio.durationSec * (ttsStatus().rtf ?? 1) }
        }
        return { ...speaker, oldSegments, oldEtaSec }
      }),
      version: audioVersion(state, 'cast'),
    })
    if (rest[1] && method === 'PUT') {
      if (!ctx.canWrite) return locked(), true
      if (conflict('cast', castView())) return true
      const body = await ctx.readBody(), role = rest[1]
      const voiceId = String(body.voiceId ?? '')
      if (!mockProjectVoices(ctx.p).some(voice => voice.id === voiceId)) return fail(res, 409, 'Giọng đã chọn không còn có sẵn. Hãy chọn lại.'), true
      const speaker = state.cast.speakers.find(speaker => speaker.id === role)
      if (!speaker && role !== 'single') return fail(res, 404, 'Không tìm thấy vai cần đổi giọng.'), true
      if (speaker) {
        const narrator = state.cast.speakers.find(item => item.id === 'dan')
        if (narrator?.sameAs === role) { state.cast.narratorCharacterId = role; delete narrator.sameAs }
        speaker.voiceId = voiceId
        speaker.sameAs = String(body.sameAs ?? '')
      } else state.cast.single.voiceId = voiceId
      state.cast.voiceMode ??= state.cast.mode
      let queuedSegments = 0
      if (body.rerenderOld === true) for (const [chapter, segments] of state.scripts) {
        const count = segments.filter(segment => (role === 'single' || segment.speaker === role) && segment.audio && segment.audio.state !== 'none').length
        if (!count) continue
        const job = startRender(state, chapter, count); job.segmentsDone = 0; job.percent = 0
        queuedSegments += count
      }
      audioChanged(state, 'cast')
      return json(res, 200, { ...castView(), queuedSegments }), true
    }
    if (method === 'GET') return json(res, 200, castView()), true
    if (method === 'PUT') {
      if (!ctx.canWrite) return locked(), true
      if (conflict('cast', { ...state.cast, version: audioVersion(state, 'cast') })) return true
      const body = await ctx.readBody()
      state.cast = { mode: body.mode, voiceMode: body.voiceMode, narratorCharacterId: body.narratorCharacterId, single: body.single, speakers: body.speakers } as MockAudioState['cast']
      if (body.voiceMode === 'pack') {
        const narrator = state.cast.speakers.find(item => item.id === 'dan')
        if (narrator) { narrator.voiceId = 'giong-ke-lanh'; delete narrator.sameAs }
      }
      audioChanged(state, 'cast')
      return json(res, 200, { ...state.cast, version: audioVersion(state, 'cast') }), true
    }
  }
  if (rest[0] === 'settings') {
    if (method === 'GET') return json(res, 200, { ...state.settings, version: audioVersion(state, 'settings') }), true
    if (method === 'PUT') {
      if (!ctx.canWrite) return locked(), true
      if (conflict('settings', { ...state.settings, version: audioVersion(state, 'settings') })) return true
      const body = await ctx.readBody()
      const quality = String(body.quality ?? state.settings.quality) as MockAudioState['settings']['quality']
      state.settings = {
        ...state.settings,
        quality,
        autoOnLock: body.autoOnLock === undefined ? state.settings.autoOnLock : body.autoOnLock === true,
        normalize: (body.normalize as MockAudioState['settings']['normalize'] | undefined) ?? state.settings.normalize,
      }
      audioChanged(state, 'settings')
      return json(res, 200, { ...state.settings, version: audioVersion(state, 'settings') }), true
    }
  }
  if (rest[0] === 'queue' && method === 'GET') return json(res, 200, queueTimelineOf(state)), true
  if (rest[0] === 'queue' && method === 'POST') {
    if (!ctx.canWrite) return locked(), true
    const body = await ctx.readBody()
    const quality = body.quality ?? state.settings.quality
    if (!['fast','standard','high'].includes(String(quality))) return fail(res,400,'Chất lượng giọng không hợp lệ.'),true
    if (rest[1]) {
      const expected = rest[1] === 'pause' ? ['queued','rendering'] : rest[1] === 'resume' ? ['paused'] : rest[1] === 'retry' ? ['error'] : []
      const jobs = queueOf(state).filter(j=>expected.includes(j.state))
      if (!jobs.length) return fail(res,409,'Không có việc phù hợp để thực hiện.'),true
      for (const job of jobs) { job.quality=quality as MockAudioState['settings']['quality'];setJobState(state,job.jobId,rest[1]==='pause'?'paused':'rendering') }
      return json(res,200,{ok:true}),true
    }
    if (!Array.isArray(body.chapters) || body.chapters.length===0 || body.chapters.some(n=>typeof n!=='number'||!Number.isSafeInteger(n)||!state.chapters[n])) return fail(res,400,'Danh sách chương không hợp lệ.'),true
    // §AK — `redo`: tạo lại từ đầu; phải rời `chapters` (một chương không thể vừa cập nhật vừa đọc lại).
    const redo = Array.isArray(body.redo) ? [...new Set(body.redo as number[])] : []
    if (redo.some(n=>typeof n!=='number'||!Number.isSafeInteger(n)||!state.chapters[n])) return fail(res,400,'Danh sách chương tạo lại không hợp lệ.'),true
    const chapters=[...new Set(body.chapters as number[])]
    if (chapters.some(n=>redo.includes(n))) return fail(res,409,'Chương vừa cập nhật vừa tạo lại từ đầu.'),true
    const invalid=chapters.find(n=>state.chapters[n].status!=='locked'&&body.lock!==true)
    if(invalid) return fail(res,409,`Chương ${invalid} chưa có bản đọc đã chốt.`),true
    const warnings=chapters.filter(n=>(state.chapters[n].issues??0)>0).map(chapter=>({chapter,reason:'Bản đọc còn chỗ cần xem; có thể sửa trên máy tính rồi tạo lại.'}))
    if(body.voiceMode){
      if(!['cast','single','pack'].includes(String(body.voiceMode)))return fail(res,400,'Kiểu giọng không hợp lệ.'),true
      state.cast.voiceMode=body.voiceMode as 'cast'|'single'|'pack';state.cast.mode=body.voiceMode==='single'?'single':'cast'
      if(body.voiceMode==='pack'){const narrator=state.cast.speakers.find(item=>item.id==='dan');if(narrator){narrator.voiceId='giong-ke-lanh';delete narrator.sameAs}}
    }
    for(const chapter of chapters){
      const needsScript=!state.scripts.get(chapter)?.length
      if(!needsScript)lockChapter(state,chapter,false)
      const job=startRender(state,chapter)
      job.quality=quality as MockAudioState['settings']['quality']
      job.needsScript=needsScript
      job.step=needsScript?'compose':'render'
      if(needsScript){job.etaSec=0;job.segments=0}
    }
    for(const chapter of redo){
      const job=startRender(state,chapter,undefined,'redo')
      job.quality=quality as MockAudioState['settings']['quality']
      job.step='render'
    }
    audioChanged(state)
    return json(res,200,{chapters:[...chapters,...redo],redo,etaSec:[...chapters,...redo].reduce((sum,n)=>sum+(state.jobs.get(n)?.etaSec??0),0),warnings}),true
  }

  // ─── renders/:job/pause|resume|cancel ───
  if (rest[0] === 'renders' && rest[1] && rest[2]) {
    if (!ctx.canWrite) return locked(), true
    const jobId = rest[1]
    if (rest[2] === 'pause') {
      setJobState(state, jobId, 'paused')
      return json(res, 200, { ok: true }), true
    }
    if (rest[2] === 'resume') {
      setJobState(state, jobId, 'rendering')
      return json(res, 200, { ok: true }), true
    }
    if (rest[2] === 'cancel') {
      cancelJob(state, jobId)
      return json(res, 200, { ok: true }), true
    }
  }

  // ─── segments/:chapter-segId/preview|rerender|accept|reject|audio ───
  if (rest[0] === 'segments' && rest[1]) {
    const key = rest[1]
    const chapter = Number.parseInt(key.split('-')[0] ?? '', 10)
    const segmentId = key.slice(String(chapter).length + 1)
    if (!Number.isFinite(chapter)) return fail(res, 404, 'Không tìm thấy đoạn'), true
    const action = rest[2]
    if (action === 'audio' && method === 'GET') {
      sendWav(req, res, wavFor(4))
      return true
    }
    if (action === 'preview' && method === 'POST') {
      if (!ctx.canWrite) return locked(), true
      if (!engineStatus.ready) return fail(res, 503, 'Bộ đọc trên máy chưa sẵn sàng'), true
      const script = state.scripts.get(chapter) ?? []
      const index = script.findIndex((segment) => segment.id === segmentId)
      if (index >= 0 && !script[index].pending) {
        script[index] = {
          ...script[index],
          audio: { url: `/api/projects/${encodeURIComponent(ctx.p)}/stories/${encodeURIComponent(ctx.s)}/audio/segments/${key}/audio?volume=${volume}`, state: 'ready', durationSec: 4.1 },
        }
        state.scripts.set(chapter, script)
        audioChanged(state, `script:${chapter}`)
      }
      await delay(650)
      return json(res, 200, {
        url: `/api/projects/${encodeURIComponent(ctx.p)}/stories/${encodeURIComponent(ctx.s)}/audio/segments/${key}/audio?volume=${volume}`,
        durationSec: 4.1,
        cached: false,
      }), true
    }
    if (action === 'rerender' && method === 'POST') {
      if (!ctx.canWrite) return locked(), true
      if (!engineStatus.ready) return fail(res, 503, 'Bộ đọc trên máy chưa sẵn sàng'), true
      await delay(700)
      const base = `/api/projects/${encodeURIComponent(ctx.p)}/stories/${encodeURIComponent(ctx.s)}/audio/segments/${key}/audio?volume=${volume}`
      return json(res, 200, {
        candidateId: `cand-${Date.now()}`, url: base, durationSec: 8.2,
        previousUrl: base, previousDurationSec: 7.4,
      }), true
    }
    if ((action === 'accept' || action === 'reject') && method === 'POST') {
      if (!ctx.canWrite) return locked(), true
      audioChanged(state, `script:${chapter}`)
      return json(res, 200, { ok: true }), true
    }
    return fail(res, 404, 'Not found'), true
  }

  // ─── flags ───
  if (rest[0] === 'flags') {
    if (method === 'GET') return json(res, 200, { flags: state.flags }), true
    if (method === 'POST') {
      if (!ctx.canWrite) return locked(), true
      const body = await ctx.readBody()
      const flag = addFlag(state, {
        chapter: Number(body.chapter ?? 1),
        segmentId: String(body.segmentId ?? ''),
        atSec: Number(body.atSec ?? 0),
        reason: String(body.reason ?? 'Cần nghe lại'),
      })
      audioChanged(state)
      return json(res, 200, flag), true
    }
    if (method === 'DELETE' && rest[1]) {
      if (!ctx.canWrite) return locked(), true
      state.flags = state.flags.filter((flag) => flag.id !== rest[1])
      audioChanged(state)
      return json(res, 200, { ok: true }), true
    }
  }

  return fail(res, 404, 'Not found'), true
}
