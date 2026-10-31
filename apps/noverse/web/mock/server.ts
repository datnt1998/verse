// Mock API — toàn bộ contract 11-api-contract.md trong bộ nhớ (npm run dev:mock).
// Tài khoản: dat/admin (mật khẩu "mat-khau-123") · vien/writer ("vien-12345") · lan/reader ("lan-123456").
// SSE: /events phát reset→snapshot→replay→delta (scripted live run) + job/cocreate.
import qrcode from 'qrcode-generator'
import { once } from 'node:events'
import type { IncomingMessage, Server, ServerResponse } from 'node:http'
import { setTimeout as sleep } from 'node:timers/promises'
import type { Plugin } from 'vite'
import type {
  ChapterFacts, ChapterPlan, ChapterMetric, CoCreateState, Config, ConfigAccount, ConfigDoc, ConfigResponse, ConfigScope, ActivityEvent, ActivityHistory, Frame, HostEvent, JobEvent, LibraryConfig, Outline,
  ProviderPreset, Snapshot, StorySummary, TiredWord, User, AudioVoice, ReviewDecision, ReviewEntry, ReviewItem, Quality, LessonState,
} from '../src/api/types'
import { CHAPTER_EVENTS, CHAPTER_TITLES, COMMANDS, facts, foreshadow, MOCK_FRAME, MOCK_RETENTION_AT, MOCK_RETENTION_CHECKS, MOCK_RETENTION_ROWS, NOW, plans, review3Blocking, review3Clean, review5, review6RuleAware, review9Blocking, reviewEvent3Items, snapshotTraXanh, TRAXANH_CHAPTERS, users } from './data'
import { audioExportDuration, audioExportText, audioStateOf, DEFAULT_PRONUNCIATIONS, rangeOf, wavFor } from './audio'
import { handleAudioApi } from './audioApi'
import { mockCoverPng } from './cover'
import { mockProjectVoices, mockUploadVoice, mockVoiceSample } from './audioVoices'
import { handleRulesApi, handleRulesGlobal } from './rules'
import { installTts, ttsEngines, ttsInstallProgress, ttsStatus, ttsVoices } from './tts'
import { handleJobsApi, mirrorJobEvent, startExportJob, syncWriteJob } from './jobs'
import { emitGlobalEvent, emitLiveFrame, openGlobalEvents, seedLiveEvents, sseConnectionCount, updateGlobalWatch, mockVersion, bumpMockVersion, mockConflict } from './events'
import { mockActivityContext, mockActivityHistory } from './activity'
import { mobileLogFixture } from './mobileLog.fixture'
import { MOCK_CAST, addressStateOf, handleAddressApi, mockBrainOf } from './address'
import { mockBrainDetailOf, sampleAddress, sampleChapterTitle, sampleCharacters, sampleProse, trimBrain } from './brain'
import { mockDisplay, mockInspect, mockLibraryConfigured, mockLibraryRoot, mockList, mockMkdir, mockPlaces, mockSwitchRoot } from './library'
import { mockDecideLesson, mockLessons, mockPendingLessons, mockViolations } from './lessons'
import { golden } from './golden'
import { appendAudit, readAudit, validProjects } from './members'
import {
  diagSent, mockDiagReport, mockExportConflict, mockExportDownload, mockExportPath, mockExportResult, mockModels, storedZip,
  mockCoCreateProposal, mockImportPreview, mockSimulateResult, mockSyncChanged, mockSyncDiff, mockSyncResult, simulateTick,
} from './cmdresults'
import type { Character, CharacterKind, DiagReport, ImportState, ModelsResponse } from '../src/api/types'

/** Bản sửa loại vai / lộ vai của mock (Gói S · PUT /characters) — mất khi tải lại server. */
const CHARACTER_EDITS = new Map<string, { kind?: CharacterKind; revealAt?: number | null }>()
const CHARACTER_KINDS = ['lead', 'antagonist', 'ally', 'support']

/**
 * GET/PUT `/characters` của mock: giữ bản sửa trong `CHARACTER_EDITS` (khoá `project/story/tên`)
 * rồi trả danh sách đã ghép — hành vi giống máy chủ thật: kiểm cả lô trước, lỗi thì không ghi.
 */
async function handleCharacters(req: IncomingMessage, res: ServerResponse, key: string, payload: {
  characters: (Character & { kind?: CharacterKind; revealAt?: number | null })[]
  supporting: unknown[]
}) {
  const edits = new Map(payload.characters.map((item) => [item.name, CHARACTER_EDITS.get(`${key}/${item.name}`)]))
  if (req.method === 'PUT') {
    const body = await readBody(req) as { characters?: { name?: string; kind?: CharacterKind; revealAt?: number | null }[] }
    const patches = body.characters ?? []
    if (patches.length === 0) return json(res, 400, { error: 'Thiếu danh sách nhân vật cần sửa' })
    for (const patch of patches) {
      if (!patch.name || !payload.characters.some((item) => item.name === patch.name)) {
        return json(res, 404, { error: `Không tìm thấy nhân vật ${patch.name ?? ''}` })
      }
      if (patch.kind !== undefined && !CHARACTER_KINDS.includes(patch.kind)) {
        return json(res, 400, { error: 'Loại vai phải là lead, antagonist, ally hoặc support' })
      }
      if (patch.revealAt !== undefined && patch.revealAt !== null && (!Number.isInteger(patch.revealAt) || patch.revealAt < 1)) {
        return json(res, 400, { error: 'Lộ vai ở chương phải là số chương từ 1 trở lên, hoặc null' })
      }
    }
    for (const patch of patches) {
      const id = `${key}/${patch.name}`
      const prev = CHARACTER_EDITS.get(id) ?? {}
      CHARACTER_EDITS.set(id, {
        ...prev,
        ...(patch.kind !== undefined ? { kind: patch.kind } : {}),
        ...(patch.revealAt !== undefined ? { revealAt: patch.revealAt } : {}),
      })
      edits.set(patch.name!, CHARACTER_EDITS.get(id))
    }
  }
  return json(res, 200, { ...payload, characters: payload.characters.map((item) => ({ ...item, ...edits.get(item.name) })) })
}
import type { ParagraphComment } from '../src/api/types'
import { PARAGRAPH_COMMENT_CHIPS } from '../src/api/types'
import { exportExt, friendlyExportName, inferExportVolume } from '../src/lib/exportName'
import { asciiFileName } from '../src/lib/audioName'
import { mockLedger, mockLedgerCheck, mockLedgerSuggest, mockUniverse, mockUnkeep, mockProjectSearch, MOCK_NEW_STORY_BRIEF } from './projbrain'

const mockExportedFiles = new Set<string>()
const commandModels = new Map<string, ModelsResponse>()
const diagnosticReports = new Map<string, DiagReport>()
const paragraphFeedback = new Map<string, ParagraphComment[]>()
/** D5 — khung truyện đã lưu của từng truyện (PUT …/frame); thiếu = trạng thái gói điền sẵn. */
const frameState = new Map<string, Frame>()

// ─── helpers http ─────────────────────────────────────────────────────────
const iso = (ms: number) => new Date(ms).toISOString()
const json = (res: ServerResponse, status: number, data: unknown) => {
  const body = JSON.stringify(data)
  res.writeHead(status, { 'content-type': 'application/json; charset=utf-8', 'content-length': Buffer.byteLength(body) })
  res.end(body)
}
const fail = (res: ServerResponse, status: number, error: string, retryAfter?: number) =>
  json(res, status, retryAfter === undefined ? { error } : { error, retryAfter })
/** §AF — header tải về: filename* UTF-8 (tên tiếng Việt) + filename= dự phòng không dấu. */
const contentDisposition = (name: string) =>
  `attachment; filename="${asciiFileName(name)}"; filename*=UTF-8''${encodeURIComponent(name)}`
const readBody = (req: IncomingMessage) => new Promise<Record<string, unknown>>((resolve) => {
  let raw = ''
  req.on('data', (c) => { raw += c })
  req.on('end', () => { try { resolve(JSON.parse(raw || '{}')) } catch { resolve({}) } })
})
const readRaw = async (req: IncomingMessage): Promise<Buffer> => {
  const chunks: Buffer[] = []
  req.on('data', (chunk: Buffer) => chunks.push(chunk))
  await once(req, 'end')
  return Buffer.concat(chunks)
}

// Mẫu giọng + từ điển cách đọc thêm của từng dự án (mock trong bộ nhớ).
const projectPronunciations = new Map<string, { written: string; spoken: string }[]>()

const cookie = (req: IncomingMessage, name: string) =>
  (req.headers.cookie ?? '').split(';').map((v) => v.trim()).find((v) => v.startsWith(`${name}=`))?.slice(name.length + 1)
// Cookie không phân biệt cổng: nhiều máy chủ mock (mỗi người một cổng) cùng 127.0.0.1 sẽ ghi đè cookie của nhau
// → phiên "tự rơi" về #/login. Tên cookie gắn với cổng đang phục vụ.
const sessionCookieName = (req: IncomingMessage) => `ainovel_session_${req.socket.localPort ?? 0}`
const setSessionCookie = (res: ServerResponse, token: string) =>
  res.setHeader('set-cookie', `${sessionCookieName(res.req)}=${token}; Path=/; HttpOnly; SameSite=Lax; Max-Age=${30 * 86400}`)

// ─── phiên & pairing ──────────────────────────────────────────────────────
type MockSession = { id: string; token: string; username: string; device: string; ip: string; createdAt: string; expiresAt: string }
// Vite khởi động lại máy chủ mỗi khi bất kỳ tệp mock/** đổi (vite.config.ts import ./mock/server → nó là phụ thuộc của config)
// và nạp lại module này: Map phiên mới → cookie cũ thành 401 → app về #/login. Giữ phiên trên globalThis
// (cùng tiến trình) để chỉnh mock không đá người đang xem ra khỏi phiên.
const mockGlobal = globalThis as typeof globalThis & {
  __ainovelMockSessions?: Map<string, MockSession>
  __ainovelMockStyleSamples?: Map<string, { id: string; title: string; chars: number; addedAt: string }[]>
}
const sessions = (mockGlobal.__ainovelMockSessions ??= new Map<string, MockSession>())
const pairings = new Map<string, { code: string; username: string; expiresAt: number }>()
// seq tiếp nối số lớn nhất trong id/token của phiên còn sống để id không trùng sau khi nạp lại.
let seq = Math.max(0, ...[...sessions.values()].flatMap((s) => [Number(s.id.slice(2)), Number(s.token.split('-')[1])]).filter(Number.isFinite))
let loginFails = 0
// AINOVEL_MOCK_NO_LAN=1 → phiên ghép máy không có địa chỉ dùng được (A1 › qrNoLan)
// Cho phép/đóng Wi-Fi nhà: PUT /api/system/network {lan} đổi cờ này (A1 › qrNoLan → qr).
let lanOpen = process.env.AINOVEL_MOCK_NO_LAN !== '1'
// D1 — có bản mới: `POST /__mock/update {on:true}` bật banner + badge “Có bản mới” + hộp “Có gì mới”.
// Mặc định tắt (artboard `update=none`); AINOVEL_MOCK_UPDATE=1 để bật sẵn.
let mockHasUpdate = process.env.AINOVEL_MOCK_UPDATE === '1'
// GAP-35 — văn mẫu của /simulate theo truyện (mất khi khởi động lại mock).
const styleSamples = (mockGlobal.__ainovelMockStyleSamples ??= new Map<string, { id: string; title: string; chars: number; addedAt: string }[]>())
// Gói AL — artboard Outline.dc.html › tab=style › wstyle=samples: Trà Xanh đã có 2 văn mẫu
// (thẻ “Văn phong của bạn” chọn được, hàng danh sách đúng artboard).
styleSamples.set('dit-dang-audio/01-tra-xanh-cuop-kich-ban', [
  { id: 'mo-man-chuong-1.txt', title: 'Mở màn chương 1 — bản tay', chars: 3120, addedAt: iso(NOW - 5 * 86400000) },
  { id: 'nguyen-tac-khach-hang-vip.txt', title: 'Nguyên tắc Khách hàng VIP', chars: 1840, addedAt: iso(NOW - 3 * 86400000) },
])
// D2 — gói thể loại kèm chi tiết packOpen; số mục lấy đúng bộ luật thật (Master 43/50, A 22/11, B 18/10, C 20/10, Định dạng 0/0).
const MOCK_SECTIONS = [
  { ref: 'Master §0', title: 'Phạm vi và thứ tự ưu tiên', level: 'error', errors: 1, warnings: 0 },
  { ref: 'Master §1', title: 'Vai trò, mục tiêu, người nghe', level: 'error', errors: 1, warnings: 0 },
  { ref: 'Master §2', title: 'Xưng hô và giọng kể', level: 'error', errors: 2, warnings: 2 },
  { ref: 'Master §3', title: 'Ngôi kể và điểm nhìn', level: 'error', errors: 2, warnings: 2 },
  { ref: 'Master §4', title: 'Cấu trúc chương', level: 'error', errors: 3, warnings: 2 },
  { ref: 'Master §5', title: 'Mở đầu chương', level: 'error', errors: 1, warnings: 3 },
  { ref: 'Master §6', title: 'Thân chương và nhịp', level: 'error', errors: 2, warnings: 1 },
  { ref: 'Master §7', title: 'Đầu chương và cuối chương', level: 'error', errors: 3, warnings: 1 },
  { ref: 'Master §8', title: 'Móc cuối chương', level: 'error', errors: 2, warnings: 1 },
  { ref: 'Master §9', title: 'Cao trào và hạ nhiệt', level: 'warning', errors: 0, warnings: 3 },
  { ref: 'Master §10', title: 'Thoại và tỷ lệ thoại', level: 'error', errors: 4, warnings: 2 },
  { ref: 'Master §11', title: 'Độ dài câu và đoạn', level: 'error', errors: 4, warnings: 2 },
  { ref: 'Master §12', title: 'Từ ngữ cấm và từ tiếng Anh', level: 'error', errors: 2, warnings: 5 },
  { ref: 'Master §13', title: 'Số, ngày, đơn vị', level: 'error', errors: 2, warnings: 4 },
  { ref: 'Master §14', title: 'Cảm xúc và nội tâm', level: 'error', errors: 2, warnings: 2 },
  { ref: 'Master §15', title: 'Tả cảnh và giác quan', level: 'error', errors: 4, warnings: 10 },
  { ref: 'Master §16', title: 'Tuyến nhân vật phụ', level: 'error', errors: 2, warnings: 2 },
  { ref: 'Master §17', title: 'Logic và mâu thuẫn', level: 'error', errors: 4, warnings: 2 },
  { ref: 'Master §18', title: 'Định dạng đoạn và câu', level: 'error', errors: 1, warnings: 4 },
  { ref: 'Master §19', title: 'Từ lặp và câu mẫu', level: 'error', errors: 1, warnings: 2 },
  { ref: 'Master §20', title: 'Báo cáo chấm và mức độ', level: '', errors: 0, warnings: 0 },
  { ref: 'Master §21', title: 'Chú giải và ví dụ', level: '', errors: 0, warnings: 0 },
  { ref: 'Master §22', title: 'Ghi chú phiên bản luật', level: '', errors: 0, warnings: 0 },
]
// `short`/`desc`/`meta` = `goi.json` › `ten_ngan`/`mo_ta` + `so_chuong`/`thoai` (Câu 7); `params` chỉ lấy từ `do_do`;
// `chapterWords` = `tieng_moi_chuong` (Câu 1) — FE tự viết câu “1.400–1.600 tiếng mỗi chương”.
const MOCK_LAYERS = (bo: string) => [
  { key: 'master', title: 'Luật chung · Master', desc: '22 mục cho mọi truyện audio giọng AI tiếng Việt', errors: 43, warnings: 50 },
  { key: 'pack', title: `Luật thể loại · ${bo}`, desc: 'Luật riêng của thể loại này. Không ghi đè mục nào của Master.', errors: 0, warnings: 0 },
  { key: 'measure', title: 'Cách đo · Định dạng', desc: 'Cách đo thoại, câu, đoạn khi chấm tự động.', errors: 0, warnings: 0 },
]
/** Hàng thông số của chi tiết gói (Settings.dc.html › `pk.params`), cùng `do_do` của ba gói thật. */
const mockPackParams = (chuong: number, thoai: string) => [
  { key: 'chapters', text: `${chuong} chương` },
  { key: 'dialogue', text: `Thoại ${thoai}` },
  { key: 'sentence', text: 'Câu ≤ 40 tiếng · đoạn ≤ 90 tiếng' },
  { key: 'quiet', text: 'Quãng không thoại ≤ 250 tiếng' },
  { key: 'procedural', text: 'Từ thủ tục ≤ 10 mỗi chương' },
  { key: 'opening', text: '220 tiếng mở đầu trùng truyện cũ ≤ 20%' },
]
const MOCK_PACKS = [
  {
    id: 'nu-tan-bao-thu', ten: 'Nữ tần trọng sinh báo thù, vả mặt', short: 'Nữ tần báo thù',
    desc: 'Trọng sinh, vả mặt, lấy lại từng thứ trước đám đông.', meta: '8 chương · thoại 40–55%',
    so_chuong: 8, nguoi_nghe: 'Nữ 18–35',
    // Câu 1 — `goi.json` › `tieng_moi_chuong` của cả ba gói đang có.
    chapterWords: { min: 1400, max: 1600 },
    hasRules: true, available: true,
    layers: MOCK_LAYERS('Bộ A').map((layer) => layer.key === 'pack' ? { ...layer, errors: 22, warnings: 11 } : layer),
    sections: MOCK_SECTIONS, params: mockPackParams(8, '40–55%'),
    overrides: { count: 0, keys: [], rows: [] }, voice: 'Một giọng nữ trẻ kể cả truyện',
    // D11: `audio` đúng như goi.json của gói (assets/packs/nu-tan-bao-thu) — máy chủ thật mới trả câu `voice`.
    audio: { che_do: 'single', giong: 'nu' },
  },
  {
    id: 'nguoc-hon-nhan', ten: 'Ngược hôn nhân, ly hôn, hối hận muộn', short: 'Ngược hôn nhân',
    desc: 'Ly hôn, mang con, người kia hối hận muộn.', meta: '7 chương · thoại 35–50%',
    so_chuong: 7, nguoi_nghe: 'Phụ nữ trung niên, nghe khi làm việc nhà',
    chapterWords: { min: 1400, max: 1600 },
    hasRules: true, available: true,
    layers: MOCK_LAYERS('Bộ B').map((layer) => layer.key === 'pack' ? { ...layer, errors: 18, warnings: 10 } : layer),
    sections: MOCK_SECTIONS, params: mockPackParams(7, '35–50%'),
    overrides: { count: 0, keys: [], rows: [] }, voice: 'Một giọng nữ kể cả truyện',
    audio: { che_do: 'single' },
  },
  {
    id: 'nien-dai-van', ten: 'Niên đại văn, trọng sinh thập niên, làm giàu', short: 'Niên đại văn',
    desc: 'Trọng sinh thập niên, làm giàu từ hai bàn tay.', meta: '10 chương · thoại 30–45%',
    so_chuong: 10, nguoi_nghe: 'Nam trung niên (C-Nam) / nữ 18–35 (C-Nữ)',
    chapterWords: { min: 1400, max: 1600 },
    hasRules: true, available: true,
    layers: MOCK_LAYERS('Bộ C').map((layer) => layer.key === 'pack' ? { ...layer, errors: 20, warnings: 10 } : layer),
    sections: MOCK_SECTIONS, params: mockPackParams(10, '30–45%'),
    overrides: { count: 0, keys: [], rows: [] }, voice: 'Một giọng nam kể, xen giọng nữ ở đoạn thoại',
    audio: { che_do: 'single' },
  },
]
// D6 — chín số đo của gói (`internal/rules/metrics.go`, goi.json › do_do) theo hợp đồng đợt 3:
// `value` là số trần (null = không áp cho chương), `unit`/`limit` là chữ máy chủ viết sẵn,
// `severity`/`ref` khi tệp luật của gói ghi mức, `detail` khi số > 0. `tiredWord` là từ lặp
// có mật độ cao nhất chương, đứng ngoài `metrics[]`. Bộ mẫu lấy từ Settings.dc.html (`metrics`)
// và Activity.dc.html (“Đo bằng máy · chương 3”).
type MockMetric = ChapterMetric
type MockTiredWord = TiredWord
/** Chuẩn cố định của D6 (22-brief-2510.md câu 16). */
const METRIC_ROWS: MockMetric[] = [
  { key: 'thoai', name: 'Tỷ lệ thoại', value: 52, unit: '%', limit: '40–55%', ok: true },
  { key: 'cau_dai_nhat', name: 'Câu quá 40 tiếng', value: 0, unit: 'câu', limit: '0', ok: true },
  { key: 'doan_dai_nhat', name: 'Đoạn quá 90 tiếng', value: 0, unit: 'đoạn', limit: '0', ok: true },
  { key: 'quang_khong_thoai', name: 'Quãng không thoại quá 250 tiếng', value: 0, unit: 'quãng', limit: '0', ok: true },
  { key: 'tu_thu_tuc', name: 'Từ thủ tục', value: 4, unit: 'từ', limit: '≤ 10', ok: true },
  { key: 'tu_tieng_anh', name: 'Từ tiếng Anh dễ đọc sai', value: 0, unit: 'từ', limit: '0', ok: true },
  { key: 'ngay_thu', name: '“ngày thứ” + số', value: 0, unit: 'dòng', limit: '0', ok: true },
  { key: 'cum_lap', name: 'Cụm 5 tiếng lặp lại', value: 0, unit: 'cụm', limit: '0', ok: true },
  { key: 'trung_mo_dau', name: 'Mở đầu trùng truyện cũ', value: 12, unit: '%', limit: '≤ 20% · chỉ đo chương 1', ok: true },
]
const TIRED_KHE: MockTiredWord = { word: 'khẽ', per1000: 3.1, limit: 2, ok: false }
/**
 * Số đo theo chương, lấy mẫu từ hai artboard: chương mới nhất (và chương 5) = bảng “Xem đủ số đo”
 * của Settings (2 chỗ lệch), chương ngay trước = sạch (“Đạt mọi số đo”), chương cách hai = một chỗ lệch
 * (quãng không thoại 332), chương 4 = bộ mẫu của dòng Nhật ký (3 chỗ lệch).
 * Chương khác chương 1: `trung_mo_dau` value null + ok (UI hiện “—”).
 */
const mockMetricsOf = (n: number, total: number): { metrics: MockMetric[]; tiredWord: MockTiredWord | null } => {
  const rows = METRIC_ROWS.map((metric) => metric.key === 'trung_mo_dau' && n !== 1
    ? { ...metric, value: null }
    : metric)
  if (n === total || n === 5) {
    return {
      metrics: rows.map((m) => m.key === 'thoai'
        ? { ...m, value: 34, ok: false, severity: 'warning' as const, ref: 'Master §13' }
        : m),
      tiredWord: { word: 'dường như', per1000: 2.4, limit: 1.5, ok: false },
    }
  }
  if (n === 4) {
    return {
      metrics: rows.map((m) => m.key === 'thoai'
        ? { ...m, value: 34, ok: false, severity: 'warning' as const, ref: 'Master §13' }
        : m.key === 'doan_dai_nhat'
          ? { ...m, value: 1, ok: false, detail: 'dài nhất 96 tiếng', severity: 'warning' as const, ref: 'Master §14' }
          : m),
      tiredWord: TIRED_KHE,
    }
  }
  if (n === 1 || n === total - 2) {
    return {
      metrics: rows.map((m) => m.key === 'quang_khong_thoai'
        ? { ...m, value: 1, ok: false, detail: 'dài nhất 332 tiếng', severity: 'error' as const, ref: 'Master §5' }
        : m),
      tiredWord: { word: 'khẽ', per1000: 0.4, limit: 2, ok: true },
    }
  }
  return { metrics: rows, tiredWord: { word: 'khẽ', per1000: 0.4, limit: 2, ok: true } }
}

// D1 — nội dung “Có gì mới” (markdown như thân release trên GitHub).
const mockReleases = (hasUpdate: boolean) => {
  const running = {
    version: 'v0.9.3', date: '2026-10-24T14:02:00+07:00',
    notes: '## Mới\n- Chọn nơi lưu truyện ngay khi thiết lập\n## Sửa\n- Mã QR đăng nhập điện thoại to và dễ quét hơn',
  }
  if (!hasUpdate) return [running]
  return [{
    version: 'v0.9.4', date: '2026-10-25T10:00:00Z',
    notes: '## Mới\n- Xuất bản đọc cho TTS: bỏ tiêu đề và định dạng, có thể đọc số thành chữ\n- Gói thể loại “Nữ tần báo thù”: trợ lý đọc đủ bộ luật, chấm từng chương\n## Sửa\n- Biên tập viên ghi rõ số mục luật cho từng lỗi',
  }, running]
}
// Mã khôi phục quản trị: active=true nghĩa là đã có mã chưa dùng (POST /api/me/recovery → 409, phải rotate).
// Mặc định có sẵn mã chưa dùng để mở được hộp Dialogs › state=recovery; AINOVEL_MOCK_RECOVERY=empty để tắt.
const recoveryCodes = ['TRAX-8K2M-QP7D-41ZN', 'MQ4V-7ZTD-R2KX-9B3F', 'H8NC-5WJP-3LYA-6ERD']
const recoveryCode = { value: recoveryCodes[0], active: process.env.AINOVEL_MOCK_RECOVERY !== 'empty', createdAt: iso(NOW - 86400000 * 12), n: 0 }
/** Mã che giữa như artboard: giữ 4 ký tự đầu và 4 ký tự cuối. */
const maskRecovery = (code: string) => {
  const groups = code.split('-')
  return groups.length === 4 ? `${groups[0]}-••••-••••-${groups[3]}` : '••••-••••-••••-••••'
}
let recoverFails = 0

// GAP-41 · A0 Lần chạy đầu. Mặc định needed=false để luồng đăng nhập của mock không đổi;
// bật bằng POST /api/__mock/setup {needed:true} (mã in ra ở GET /api/__mock/setup) hoặc GET /api/setup?mocksetup=1.
const SETUP_TTL = 30 * 60_000
const SETUP_LOCK = 10 * 60_000
const setup = { needed: false, code: 'TRAX41ZN', attempts: 0, lockedUntil: 0, expiresAt: Date.now() + SETUP_TTL, token: '', tokenExpiresAt: 0, doneAt: 0 }
const resetSetup = (needed: boolean) => {
  setup.needed = needed; setup.attempts = 0; setup.lockedUntil = 0
  setup.expiresAt = Date.now() + SETUP_TTL; setup.token = ''; setup.tokenExpiresAt = 0
}
const setupState = () => setup.needed
  ? {
    needed: true, expiresAt: iso(setup.expiresAt), attemptsLeft: Math.max(0, 5 - setup.attempts), version: '0.9.3',
    // GAP-41 bổ sung: hết hạn giữ `expiresAt` cũ và thêm cờ này; UI mở codeExpired trước cả khi đang khoá.
    ...(setup.expiresAt <= Date.now() ? { expired: true } : {}),
    ...(setup.lockedUntil > Date.now() ? { lockedUntil: iso(setup.lockedUntil) } : {}),
  }
  : { needed: false, attemptsLeft: 0 }

const mkSession = (username: string, device: string): MockSession => {
  const s: MockSession = {
    id: `s-${++seq}`, token: `tok-${++seq}-${Math.random().toString(36).slice(2)}`,
    username, device, ip: '127.0.0.1', createdAt: iso(NOW), expiresAt: iso(NOW + 30 * 86400000),
  }
  sessions.set(s.token, s)
  return s
}
const sessionOf = (req: IncomingMessage): MockSession | null => {
  const tok = cookie(req, sessionCookieName(req))
  return tok ? sessions.get(tok) ?? null : null
}
const sessionJson = (s: MockSession, reqToken: string) => ({
  id: s.id, username: s.username, device: s.device, ip: s.ip,
  createdAt: s.createdAt, lastSeen: s.createdAt, expiresAt: s.expiresAt, current: s.token === reqToken,
})
const sessionListJson = (s: MockSession, reqToken: string) => {
  const [device, browser = ''] = s.device.split(' · ')
  return { id: s.id, device, browser, lastActiveAt: s.createdAt, expiresAt: s.expiresAt, current: s.token === reqToken }
}

// §AD · permissions are resolved from the current account on every request.
const canRead = (u: User, sid: string) => !u.disabled && (u.role === 'admin' || !!u.projects[sid.split('/')[0]])
const canWrite = (u: User, sid: string) => !u.disabled && (u.role === 'admin' || ['owner', 'collab'].includes(u.projects[sid.split('/')[0]]))

// ─── model dữ liệu truyện ─────────────────────────────────────────────────
type SseClient = { res: ServerResponse }
type MockStory = {
  p: string; s: string; folder: string; title: string; status: 'idea' | 'writing' | 'paused' | 'done' | 'published'
  tags: string[]; idea: string; updatedAt: string; locked: boolean
  chapters: Record<number, { title: string; text: string }>; totalChapters: number; completed: number
  /** §AF — tên ngắn tệp audio (story.json); thiếu thì suy ra từ title. */
  shortTitle?: string
  snapshot: Snapshot; clients: Set<SseClient>; replay: HostEvent[]
  advanceMode: 'auto' | 'review'; cocreate: CoCreateState; jobs: Map<string, JobEvent>
  jobResolvers: Map<string, () => void>; streamTimer: ReturnType<typeof setInterval> | null
  /** Gói O: kịch bản luồng riêng (truyện lập kế hoạch); rỗng = script chương mặc định */
  streamScript?: string[]
  pauseAfterStep?: boolean
  importState?: ImportState; importConfirmed?: boolean
  activityFixture?: ActivityHistory
}
const snapshotRevisions = new WeakMap<MockStory, Record<string, string>>()
function changedSnapshotParts(st: MockStory) {
  const chapters = JSON.stringify([st.chapters, st.completed, st.totalChapters, st.snapshot.currentChapter, st.snapshot.isRunning, st.snapshot.waitingFor])
  const next = { chapters, outline: JSON.stringify([chapters, st.snapshot.outline]), reviews: JSON.stringify([st.completed, st.snapshot.lastReviewSummary]) }
  const previous = snapshotRevisions.get(st)
  snapshotRevisions.set(st, next)
  return ['snapshot', ...Object.entries(next).filter(([part, revision]) => previous && previous[part] !== revision).map(([part]) => part)]
}

const mkEvent = (n: number, e: Partial<ActivityEvent> & { summary: string }): HostEvent => ({
  id: `ev-${n}`, time: iso(NOW - (100 - n) * 900), finishedAt: iso(NOW - (100 - n) * 900 - 400),
  failed: false, category: 'arbiter', agent: 'arbiter', kind: 'arbitration',
  level: 'info', depth: 0, durationNs: 400_000_000, retryAt: '', sessionId: 'sess-2', canRetryNow: false, ...e,
})

const REPLAY: HostEvent[] = [
  mkEvent(1, { category: 'DECISION', agent: 'arbiter', summary: '启动裁定', durationNs: 17_000_000_000, sessionId: 'sess-1' }),
  mkEvent(2, { category: 'DISPATCH', agent: 'writer', summary: '写第 6 章', level: 'info', durationNs: 0, finishedAt: '', sessionId: 'sess-1' }),
  mkEvent(3, { category: 'TOOL', agent: 'writer', summary: 'novel_context(Ch.6)', depth: 1, durationNs: 137_000_000 }),
  mkEvent(4, { category: 'SYSTEM', agent: 'editor', summary: '一致性告警: chương 5 "tám người đọc bản thảo" mâu thuẫn ch.3', level: 'warn', finishedAt: '' }),
  mkEvent(5, { category: 'USER', agent: 'user', summary: '[用户干预] 打磨第 5 章', level: 'info' }),
  // G4: một lượt đang chờ backoff → Nhật ký hiện “Thử lại sau Ns” + nút “Thử lại ngay”
  mkEvent(6, { category: 'SYSTEM', agent: 'writer', summary: 'Thử lại (2/7): lỗi mạng tạm thời', level: 'warn', retryAt: iso(NOW + 7000), canRetryNow: true }),
  // Lời gọi công cụ đúng định dạng host (displayToolName) — Nhật ký phải đổi sang lời thường, tên gốc chỉ trong chi tiết.
  mkEvent(7, { category: 'TOOL', agent: 'writer', summary: 'read_chapter(chương 5 · bản nháp)', tool: 'read_chapter', args: '{"chapter":5,"source":"draft","max_runes":10000}', result: '{"chapter":5,"source":"draft","word_count":1546,"content":"Bữa trưa hôm ấy bàn dài đặt giữa phòng ăn nhân viên…"}', depth: 1, durationNs: 237_000_000 }),
  mkEvent(8, { category: 'TOOL', agent: 'writer', summary: 'commit_chapter(chương 5)', tool: 'commit_chapter', args: '{"chapter":5,"hook_type":"crisis"}', result: '{"ok":true}', depth: 1, durationNs: 8_200_000_000 }),
  mkEvent(9, { category: 'TOOL', agent: 'architect', summary: 'save_foundation[premise]', tool: 'save_foundation', args: '{"type":"premise"}', result: '{"ok":true}', depth: 1, durationNs: 410_000_000 }),
  mkEvent(10, { category: 'TOOL', agent: 'editor', summary: 'save_review(toàn cục·polish)', depth: 1, durationNs: 6_800_000_000 }),
  mkEvent(11, { category: 'TOOL', agent: 'writer', summary: 'export_book', depth: 1, durationNs: 90_000_000 }),
  mkEvent(12, { category: 'TOOL', agent: 'writer', summary: 'commit_chapter(chương 5) lỗi: kết nối bị ngắt', depth: 1, failed: true, level: 'error', durationNs: 1_200_000_000 }),
  // D4/D9: dòng Biên tập viên mang mục thẩm duyệt (lọc “Thẩm duyệt”, thẻ thu gọn ở MLog).
  // 2 bắt sửa gồm lỗi lint Xưng hô — nó chặn duyệt y như Master §7 (22-brief-2510.md › câu 13).
  mkEvent(13, { category: 'REVIEW', agent: 'editor', summary: 'Thẩm chương 3 · 2 bắt sửa · 3 cảnh báo', chapter: 3, durationNs: 6_800_000_000, review: { round: 2, items: reviewEvent3Items } }),
  mkEvent(14, { category: 'REVIEW', agent: 'editor', summary: 'Chấm lại chương 3 · đạt · còn 2 cảnh báo', chapter: 3, durationNs: 4_100_000_000, review: { round: 2, items: review3Clean.items ?? [] } }),
]

// Script luồng viết (✻ round + delta) — xem 06-data-contract.md › Stream
const STREAM_SCRIPT = [
  '✻ 查询上下文\nchapter: 6',
  '✻ 读章节\nchapter: 5\nsource: draft\nfrom: 0\nto: 0\nmax_runes: 10000\ncharacter: null',
  'Bữa trưa hôm ấy bàn dài đặt giữa phòng ăn nhân viên, người đến đông hơn thường ngày — mười hai ghế mà tới mười bốn người.',
  ' Quách ngồi đầu bàn, đũa đặt ngay ngắn bên hộp cơm của mình như thể đây là một phiên tòa nhỏ.',
  '\n\n✻ 一致性检查\nchapter: 6',
  ' Nhân Phát nhìn quanh: "Ai cũng đến cả, trừ một người." Câu nói ấy chưa ai trả lời, nhưng tất cả đều biết Trần Nga đang ở đâu — chỗ ngồi của bà trong phòng quy hoạch đã vắng từ sáng.',
  '\n\n✻ 打磨\nchapter: 4\nold_string: Kiều My đặt đũa xuống trước tiên.\nCô nhận ra người vắng mặt.\nnew_string: Kiều My đặt đũa xuống trước tiên, rồi nhìn quanh.\nCô nhận ra người vắng mặt hôm nay chính là người ký giúp mười hai vụ còn lại.\nreplace_all: false',
  ' Kiều My đặt đũa xuống trước tiên. Cô nhận ra người vắng mặt hôm nay chính là người ký giúp cho mười hai vụ còn lại chưa bị xóa trắng.',
  '\n\n✻ 章节提交\nchapter: 4\nhook_type: lat_thong_tin\ndominant_strand: quest\nfeedback: null\ncast_intros:\n  - name: Trần Nga\n    brief_role: người giữ chìa khoá phòng quy hoạch\ncharacters:\n  - Kiều My\n  - Trần Nga\n  - Nhân Phát\n  - Quách\nforeshadow_updates:\n  - id: fs-phong-chien-luoc\n    action: plant\n    description: Phòng quy hoạch ghi nhiều hơn mức cần: ai đã vào kho, đọc những gì, ký những gì.\n  - id: fs-tac-gia-giau-ten\n    action: advance\n    description: Vụ cuối cùng trong bản thảo bỏ trống tên tác giả.\n  - id: fs-ngoac-ky-ten\n    action: resolve\n    description: null\nkey_events:\n  - Kiều My bác thẻ nhường và yêu cầu xáo công khai.\n  - Trần Nga xuất hiện ở phòng quy hoạch sau hai ngày vắng.',
  '\n\n✻ 规划\nchapter: 5\ntitle: Bữa cơm trưa của người vắng mặt\ngoal: Hé lộ việc Trần Nga vắng mặt.\nconflict: Quách ép Kiều My ký biên bản mới.\nhook: Chiếc ghế trống ở đầu bàn.\nemotion_arc: căng → nghi ngờ → lạnh\nnotes: Giữ nhịp chậm ở cảnh 2.',
].map((line) => (line.endsWith('\n') ? line : `${line}\n`))

/** D4/D9 — bộ thẩm duyệt mẫu theo truyện: Trình đọc ch.3 (còn bắt sửa), gate ch.3 (đã sạch), cửa duyệt ch.9 (còn bắt sửa). */
const REVIEWS: Record<string, Record<number, ReviewEntry>> = {
  'dit-dang-audio/01-tra-xanh-cuop-kich-ban': { 3: review3Blocking, 5: review5, 6: review6RuleAware },
  'dit-dang-audio/04-giang-ho-tri': { 3: review3Clean },
  '_/giang-ho-tri': { 9: review9Blocking },
}

// GAP-47 — quyết định “Giữ nguyên” theo truyện + itemId (bền trong phiên mock, như reviews/decisions.json).
const reviewDecisions = new Map<string, Map<string, ReviewDecision>>()

/** Gắn `items[].decision` đã lưu vào bản thẩm duyệt trả về. */
const withReviewDecisions = (key: string, entry: ReviewEntry | null): ReviewEntry | null => {
  const decisions = reviewDecisions.get(key)
  if (!entry || !decisions || !entry.items) return entry
  return { ...entry, items: entry.items.map((item) => decisions.has(item.id) ? { ...item, decision: decisions.get(item.id) } : item) }
}

/** Mục của bản thẩm duyệt mới nhất của truyện (mọi chương đã lưu) — để tìm itemId khi giữ/bỏ giữ. */
const reviewItemOf = (key: string, itemId: string): ReviewItem | null => {
  for (const entry of Object.values(REVIEWS[key] ?? {})) {
    const item = entry.items?.find((candidate) => candidate.id === itemId)
    if (item) return item
  }
  // Mục của màn Chất lượng đến từ golden — quyết định giữ/bỏ giữ cần nhận cả
  // những itemId đó (màn Vũ trụ gọi DELETE qua quoteId của golden).
  const quality = QUALITY_GOLDEN[key]
  if (quality) {
    const issue = golden<Quality>(quality).issues.find((candidate) => candidate.id === itemId)
    if (issue) return { id: issue.id, level: issue.level, ref: issue.ref, refName: '', text: issue.text, quote: issue.quote, fix: issue.fix } as ReviewItem
  }
  return null
}

// Màn Chất lượng phát lại golden của Go test theo truyện; truyện không có bản
// chấm nào dùng quality-empty (chapters trống vì trạng thái là tính toán của
// Go, mock không còn cài đặt lại).
const QUALITY_GOLDEN: Record<string, string> = {
  'dit-dang-audio/01-tra-xanh-cuop-kich-ban': 'quality.json',
  'dit-dang-audio/04-giang-ho-tri': 'quality-clean.json',
  '_/giang-ho-tri': 'quality-blocking.json',
  '_/chat-luong-40': 'quality-40.json',
}

const mkStory = (p: string, s: string, folder: string, title: string, status: MockStory['status'], opts: {
  chapters?: Record<number, { title: string; text: string }>; completed?: number; total?: number
  snapshot?: Snapshot; locked?: boolean; advanceMode?: 'auto' | 'review'; tags?: string[]; idea?: string
  shortTitle?: string
} = {}): MockStory => ({
  p, s, folder, title, status, tags: opts.tags ?? ['đô-thị'], idea: opts.idea ?? '', shortTitle: opts.shortTitle,
  updatedAt: iso(NOW - 3600000), locked: opts.locked ?? false,
  chapters: opts.chapters ?? {}, totalChapters: opts.total ?? 14, completed: opts.completed ?? 0,
  snapshot: opts.snapshot ?? { ...snapshotTraXanh, isRunning: status === 'writing', runtimeState: status === 'writing' ? 'running' : status },
  clients: new Set(), replay: REPLAY, advanceMode: opts.advanceMode ?? 'auto',
  cocreate: { active: false, messages: [], draft: '', ready: false, suggestions: [] },
  jobs: new Map(), jobResolvers: new Map(), streamTimer: null,
})

// Dòng Đại cương trong snapshot: `total` chương đầu của CHAPTER_TITLES, `done` chương đã xong.
const outlineFor = (total: number, _done: number) => Object.entries(CHAPTER_TITLES).slice(0, total)
  .map(([n, title]) => ({ chapter: Number(n), title, coreEvent: CHAPTER_EVENTS[Number(n)] ?? '' }))

const TEMP_SLUG = 'truyen-moi'

/** QR thật (module ma trận → path SVG), dùng cho cả hai route pairing. */
const qrSvg = (text: string): string => {
  const qr = qrcode(0, 'M')
  qr.addData(text)
  qr.make()
  const count = qr.getModuleCount()
  let path = ''
  for (let row = 0; row < count; row += 1) {
    for (let col = 0; col < count; col += 1) {
      if (qr.isDark(row, col)) path += `M${col} ${row}h1v1h-1z`
    }
  }
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${count} ${count}" shape-rendering="crispEdges"><rect width="${count}" height="${count}" fill="#fff"/><path d="${path}" fill="#000"/></svg>`
}


const storySummary = (st: MockStory): StorySummary => ({
  id: `${st.p}/${st.s}`, project: st.p, story: st.s, folder: st.folder, title: st.title,
  status: st.status, tags: st.tags, idea: st.idea,
  phase: st.totalChapters === 0 ? 'premise' : st.snapshot.phase,
  completedChapters: st.completed, totalChapters: st.totalChapters,
  totalWords: Object.values(st.chapters).reduce((a, c) => a + c.text.split(/\s+/).length, 0),
  volumes: Math.ceil(st.totalChapters / 40) || 1, open: true, running: st.status === 'writing',
  locked: st.locked, updatedAt: st.updatedAt, exportsCount: st.p === 'dit-dang-audio' ? 2 : 0,
  tempName: (st.folder.split('/').pop() ?? st.folder).replace(/^\d+-/, '') === TEMP_SLUG,
  // §AF — tên ngắn tệp audio tải về (shortTitle lưu ở story.json, thiếu thì suy ra).
  shortTitle: st.shortTitle,
  // D13 · GAP-42 — số mục Bắt sửa còn mở (không lặp id) trên các bản thẩm duyệt mới nhất, như storyOpenErrors.
  openErrors: new Set(Object.values(REVIEWS[`${st.p}/${st.s}`] ?? {}).flatMap((entry) => (entry.items ?? []).filter((item) => item.level === 'error').map((item) => item.id))).size,
})

const stories = new Map<string, MockStory>()
// Trạng thái gieo mầm lúc khởi động — POST /__mock/reset dựng lại để mỗi lần soi
// artboard có đúng trạng thái seed (luồng mô phỏng tự chạy làm truyện trôi state).
const storySeed = new Map<string, MockStory>()
const put = (st: MockStory) => {
  const { clients, jobs, jobResolvers, streamTimer, ...seedable } = st
  storySeed.set(`${st.p}/${st.s}`, structuredClone(seedable) as MockStory)
  changedSnapshotParts(st)
  stories.set(`${st.p}/${st.s}`, st)
}
const resetMockStories = (only?: string): string[] => {
  const done: string[] = []
  for (const [id, st] of stories) {
    const seed = storySeed.get(id)
    if (!seed || (only && only !== id && only !== st.s)) continue
    clearInterval(st.streamTimer ?? undefined)
    const { clients, jobs, jobResolvers, streamTimer, pauseAfterStep, ...rest } = structuredClone(seed)
    Object.assign(st, rest, { pauseAfterStep: false })
    st.jobs.clear()
    removeStoryJobs(st.p, st.s)
    syncWriteJob(st.p, st.s, st.snapshot)
    broadcast(st, 'reset', {})
    broadcast(st, 'snapshot', st.snapshot)
    done.push(id)
  }
  return done
}

put(mkStory('dit-dang-audio', '01-tra-xanh-cuop-kich-ban', 'kich-ban/01-tra-xanh-cuop-kich-ban',
  'Trà Xanh Cướp Kịch Bản', 'writing', {
    chapters: TRAXANH_CHAPTERS, completed: 3, total: 8,
    tags: ['satira', 'đô-thị', 'hiện-đại'],
    shortTitle: 'Trà Xanh Cướp Kịch Bản',
    // Main.dc.html › phase=rewrite: đang viết lại chương 4, hàng đợi 4–8
    snapshot: {
      ...snapshotTraXanh, flow: 'rewriting', statusLabel: '重写第 4 章',
      currentChapter: 4, inProgressChapter: 4, completedCount: 3, totalChapters: 8,
      totalWordCount: 58699, pendingRewrites: [4, 5, 6, 7, 8],
      rewriteReason: 'Người dùng yêu cầu sửa cụ thể trên cả tám chương',
      recoveryLabel: 'Khôi phục: 5 chương chờ viết lại', isRunning: true,
      outline: outlineFor(8, 3),
      agents: (snapshotTraXanh.agents ?? []).map((agent) => (agent.name === 'writer'
        ? { ...agent, state: 'running', summary: '重写第 4 章: Bảy mươi ba giây', taskKind: 'rewrite' }
        : agent)),
    },
  }))
// Bug 2710 — ngăn kéo Não truyện với 12 thực thể và mô tả quan hệ dài (web/mock/brain.ts ›
// DRAWER_STORY). Chương 2 đang viết dở: mở Bàn viết rồi bấm G là thấy đúng cảnh bị lỗi.
put(mkStory('dit-dang-audio', '07-thien-kim-giong-noi', 'kich-ban/07-thien-kim-giong-noi',
  'Thiên Kim Thật Đòi Lại Giọng Nói', 'paused', {
    chapters: Object.fromEntries(Object.entries(TRAXANH_CHAPTERS).filter(([n]) => +n <= 4)
      .map(([n, item]) => [n, { title: item.title, text: item.text }])),
    completed: 3, total: 8, tags: ['đô-thị', 'giọng-nói'],
    snapshot: {
      ...snapshotTraXanh, bookTitle: 'Thiên Kim Thật Đòi Lại Giọng Nói', flow: 'writing',
      isRunning: false, runtimeState: 'paused', statusLabel: 'Tạm dừng',
      currentChapter: 4, inProgressChapter: 4, completedCount: 3, totalChapters: 8,
      pendingRewrites: null, recoveryLabel: '', waitingFor: '',
      hasAdvanceHold: false, advanceHoldReason: '', advanceHoldChapter: 0,
      outline: outlineFor(8, 3),
    },
  }))
put(mkStory('dit-dang-audio', '04-giang-ho-tri', 'kich-ban/04-giang-ho-tri', 'Giang Hồ Trí', 'paused', {
  chapters: Object.fromEntries(Object.entries(CHAPTER_TITLES).filter(([n]) => +n <= 3)
    .map(([n, t]) => [n, { title: t, text: TRAXANH_CHAPTERS[((+n - 1) % 5) + 1]?.text ?? '' }])),
  completed: 3, total: 8, advanceMode: 'review',
  // Main.dc.html › phase=gate: chờ duyệt chương 4
  snapshot: {
    ...snapshotTraXanh, bookTitle: 'Giang Hồ Trí', flow: 'writing', isRunning: false,
    runtimeState: 'paused', statusLabel: '审阅', advanceMode: 'review', waitingFor: 'gate',
    hasAdvanceHold: true, advanceHoldReason: 'Chờ duyệt chương 4',
    currentChapter: 4, totalChapters: 8, completedCount: 3, inProgressChapter: 0, pendingRewrites: null,
    recoveryLabel: '', outline: outlineFor(8, 3),
    agents: snapshotTraXanh.agents?.map((agent) => ({ ...agent, state: 'idle' })) ?? null,
  },
}))
put(mkStory('dit-dang-audio', '05-cho-trong-tai', 'kich-ban/05-cho-trong-tai', 'Chờ Trọng Tài', 'paused', {
  chapters: Object.fromEntries(Object.entries(CHAPTER_TITLES).filter(([n]) => +n <= 5)
    .map(([n, t]) => [n, { title: t, text: TRAXANH_CHAPTERS[((+n - 1) % 5) + 1]?.text ?? '' }])),
  completed: 5, total: 8,
  // Main.dc.html › phase=wait: Trọng tài dừng lại hỏi người dùng (Gói P · P4).
  snapshot: {
    ...snapshotTraXanh, bookTitle: 'Chờ Trọng Tài', flow: 'writing', isRunning: false,
    runtimeState: 'paused', statusLabel: 'WAITING', waitingFor: 'input',
    hasAdvanceHold: false, advanceHoldReason: '', advanceHoldChapter: 0,
    currentChapter: 6, totalChapters: 8, completedCount: 5, inProgressChapter: 0, pendingRewrites: null,
    recoveryLabel: '', outline: outlineFor(8, 5),
    pendingAsk: {
      question: 'Chương 5 dài hơn dàn ý hai cảnh (cảnh bếp và cuộc gọi cho luật sư Hàn). Giữ nguyên rồi dời dàn ý, hay cắt hai cảnh cho khớp?',
      chapter: 5,
      options: [
        { label: 'Giữ nguyên, dời dàn ý', reply: 'Giữ cảnh bếp và cuộc gọi, dời dàn ý các chương sau cho khớp', isDefault: true },
        { label: 'Cắt cho khớp dàn ý', reply: 'Cắt cảnh bếp và cuộc gọi cho luật sư Hàn khỏi chương 5' },
        { label: 'Viết lại chương 5 gọn hơn', reply: 'Viết lại chương 5, nén hai cảnh thành một, giữ nhịp nhanh' },
      ],
    },
    agents: snapshotTraXanh.agents?.map((agent) => ({ ...agent, state: 'idle' })) ?? null,
  },
}))
put(mkStory('dit-dang-audio', '06-ke-hoach-xong', 'kich-ban/06-ke-hoach-xong', 'Kế Hoạch Xong', 'paused', {
  completed: 0, total: 8,
  // Main.dc.html › phase=ready: kế hoạch đã lưu, chưa viết chương nào.
  snapshot: {
    ...snapshotTraXanh, bookTitle: 'Kế Hoạch Xong', flow: 'writing', isRunning: false,
    runtimeState: 'idle', statusLabel: 'READY', waitingFor: 'input', phase: 'ready',
    plan: null, hasAdvanceHold: false, advanceHoldReason: '', advanceHoldChapter: 0,
    currentChapter: 0, totalChapters: 8, completedCount: 0, inProgressChapter: 0, pendingRewrites: null,
    totalWordCount: 0, recoveryLabel: '', outline: outlineFor(8, 0),
    characters: (snapshotTraXanh.characters ?? []).slice(0, 4),
    agents: snapshotTraXanh.agents?.map((agent) => ({ ...agent, state: 'idle' })) ?? null,
  },
}))
put(mkStory('dit-dang-audio', '02-chua-co-ten', 'kich-ban/02-chua-co-ten', 'Chưa có tên', 'idea', {
  idea: 'Truyện lịch sử về nghị định xuất bản thời Pháp và một hiệu in nhỏ.', total: 0,
  snapshot: {
    ...snapshotTraXanh, bookTitle: 'Chưa có tên', isRunning: false, runtimeState: 'idle', waitingFor: 'input',
    statusLabel: '就绪', phase: 'idea', currentChapter: 0, totalChapters: 0, completedCount: 0,
    inProgressChapter: 0, outline: [], characters: [], recentSummaries: [], recentSupporting: [],
    supportingCount: 0, lastCommitSummary: '', lastReviewSummary: '', lastCheckpointName: '',
    recoveryLabel: '', hasAdvanceHold: false, advanceHoldReason: '',
    compassDirection: '', compassScale: '',
    currentVolumeArc: '', nextVolumeTitle: '', layered: false,
    agents: [],
  },
}))
put(mkStory('dit-dang-audio', '03-tuyen-bo-binh-dang', 'kich-ban/03-tuyen-bo-binh-dang',
  'Tuyên Bố Đẩy Bình Đẳng', 'done', {
    chapters: Object.fromEntries(Object.entries(CHAPTER_TITLES).filter(([n]) => +n <= 8)
      .map(([n, t]) => [n, { title: t, text: TRAXANH_CHAPTERS[((+n - 1) % 5) + 1]?.text ?? 'Nội dung chương.' }])),
    completed: 8, total: 8,
    snapshot: {
      ...snapshotTraXanh, bookTitle: 'Tuyên Bố Đẩy Bình Đẳng', isRunning: false,
      // serve thật phát statusLabel = COMPLETE (mã engine chữ hoa) — UI phải dịch thành "Hoàn thành"
      runtimeState: 'completed', statusLabel: 'COMPLETE', phase: 'complete',
      currentChapter: 8, totalChapters: 8, completedCount: 8, inProgressChapter: 0,
      outline: outlineFor(8, 8),
    },
  }))
put(mkStory('_', 'giang-ho-tri', 'giang-ho-tri', 'Giang Hồ Trí', 'paused', {
  chapters: Object.fromEntries(Object.entries(CHAPTER_TITLES).filter(([n]) => +n <= 9)
    .map(([n, t]) => [n, { title: t, text: TRAXANH_CHAPTERS[((+n - 1) % 5) + 1]?.text ?? '' }])),
  completed: 9, total: 20, advanceMode: 'review',
  snapshot: {
    ...snapshotTraXanh, bookTitle: 'Giang Hồ Trí', isRunning: false, runtimeState: 'paused',
    statusLabel: '暂停', advanceMode: 'review', hasAdvanceHold: true, waitingFor: 'gate',
    advanceHoldReason: 'Chờ duyệt chương 10', currentChapter: 10, totalChapters: 20, completedCount: 9,
    inProgressChapter: 0,
  },
}))
put(mkStory('_', 'nguoc-dong', 'nguoc-dong', 'Ngược Dòng', 'writing', {
  locked: true, completed: 4, total: 12,
  chapters: Object.fromEntries(Object.entries(CHAPTER_TITLES).filter(([n]) => +n <= 4)
    .map(([n, t]) => [n, { title: t, text: TRAXANH_CHAPTERS[((+n - 1) % 5) + 1]?.text ?? '' }])),
  snapshot: {
    ...snapshotTraXanh, bookTitle: 'Ngược Dòng', isRunning: true, runtimeState: 'running', statusLabel: '运行中',
    currentChapter: 5, inProgressChapter: 5, completedCount: 4, totalChapters: 12, outline: outlineFor(12, 4),
  },
}))
// MWork.dc.html (M1 Viết) › state=idle|paused|error: hero suy ra từ snapshot + sự kiện cuối, nên mỗi trạng thái có một truyện riêng.
const mwChapters = (n: number) => Object.fromEntries(Object.entries(CHAPTER_TITLES).filter(([k]) => +k <= n)
  .map(([k, t]) => [k, { title: t, text: TRAXANH_CHAPTERS[((+k - 1) % 5) + 1]?.text ?? '' }]))
const mwSnapshot = (bookTitle: string, runtimeState: string, patch: Partial<Snapshot> = {}): Snapshot => ({
  ...snapshotTraXanh, bookTitle, isRunning: false, runtimeState, statusLabel: runtimeState, waitingFor: '',
  hasAdvanceHold: false, advanceHoldReason: '', currentChapter: 5, inProgressChapter: 0, completedCount: 4, totalChapters: 8,
  pendingRewrites: null, recoveryLabel: '', outline: outlineFor(8, 4),
  agents: snapshotTraXanh.agents?.map((agent) => ({ ...agent, state: 'idle' })) ?? null, ...patch,
})
const mwIdle = mkStory('_', 'viet-cho-lenh', 'viet-cho-lenh', 'Chờ Lệnh', 'paused', { chapters: mwChapters(4), completed: 4, total: 8, snapshot: mwSnapshot('Chờ Lệnh', 'idle') })
mwIdle.replay = REPLAY.slice(0, 5)
put(mwIdle)
put(mkStory('_', 'viet-tam-dung', 'viet-tam-dung', 'Tạm Dừng Giữa Chừng', 'paused', { chapters: mwChapters(4), completed: 4, total: 8, snapshot: mwSnapshot('Tạm Dừng Giữa Chừng', 'paused', { flow: 'reviewing', inProgressChapter: 5 }) }))
const mwError = mkStory('_', 'viet-loi', 'viet-loi', 'Dừng Vì Lỗi', 'paused', { chapters: mwChapters(4), completed: 4, total: 8, snapshot: mwSnapshot('Dừng Vì Lỗi', 'error', { flow: 'reviewing', inProgressChapter: 5, recoveryLabel: 'Khôi phục: chương 5 đang viết' }) })
mwError.replay = [...REPLAY.slice(0, 5), mkEvent(90, { category: 'ERROR', agent: 'writer', summary: 'Không kết nối được nhà cung cấp AI — kiểm tra kết nối và cấu hình nhà cung cấp', level: 'error', failed: true })]
put(mwError)

// Gói O — truyện đang ở giai đoạn lập kế hoạch: chip khôi phục ghi bước kế tiếp,
// Nhật ký có dòng “Bắt đầu sáng tác (… — lý do dài)” và công cụ đang chạy giữ nét “├”.
const planReplay: HostEvent[] = [
  mkEvent(60, { category: 'USER', agent: 'user', summary: 'Yêu cầu sáng tác: truyện dài về một nữ thư ký pháp án vạch trần vụ cướp kịch bản', level: 'info' }),
  mkEvent(61, { category: 'DECISION', agent: 'arbiter', summary: 'Phán định khởi động', durationNs: 17_000_000_000 }),
  mkEvent(62, { category: 'SYSTEM', agent: 'arbiter', summary: 'Bắt đầu sáng tác (Kiến trúc sư lập kế hoạch truyện dài — Câu chuyện kéo dài bốn mươi chương nên cần khung phân tầng đầy đủ trước khi viết, ưu tiên nhân vật phản diện và tuyến tình cảm phụ để giữ độ căng suốt nửa sau của truyện)' }),
  mkEvent(63, { category: 'DISPATCH', agent: 'architect_long', summary: 'architect_long (Xây dựng kịch bản ở cấp độ dàn ý cho truyện dài: đọc lại bối cảnh rồi bổ sung các mục thiếu của thiết lập nền — dàn ý, nhân vật, quy tắc thế giới — và lưu từng phần trước khi kết thúc)', durationNs: 0, finishedAt: '' }),
  mkEvent(64, { category: 'TOOL', agent: 'architect_long', summary: 'novel_context', tool: 'novel_context', args: '{}', result: '{"ready":false}', depth: 1, durationNs: 210_000_000 }),
  mkEvent(65, { category: 'TOOL', agent: 'architect_long', summary: 'save_book', tool: 'save_book', args: '{"title":"Tuổi Sáng Phản Bộn"}', result: '{"ok":true}', depth: 1, durationNs: 190_000_000 }),
  mkEvent(66, { category: 'TOOL', agent: 'architect_long', summary: 'save_foundation[premise]', tool: 'save_foundation', args: '{"type":"premise"}', result: '{"ok":true}', depth: 1, durationNs: 410_000_000 }),
  mkEvent(67, { category: 'TOOL', agent: 'architect_long', summary: 'save_foundation[outline]', tool: 'save_foundation', args: '{"type":"outline"}', depth: 1, durationNs: 0, finishedAt: '', status: 'running' }),
]
const planStory = mkStory('dit-dang-audio', 'tuoi-sang-phan-bon', 'kich-ban/tuoi-sang-phan-bon', 'Tuổi Sáng Phản Bộn', 'paused', {
  total: 0, completed: 0,
  snapshot: {
    ...snapshotTraXanh, bookTitle: 'Tuổi Sáng Phản Bộn', phase: 'outline', flow: '', isRunning: true, runtimeState: 'running', statusLabel: '运行中',
    currentChapter: 0, totalChapters: 0, completedCount: 0, inProgressChapter: 0, pendingRewrites: null, outline: [],
    recoveryLabel: 'Khôi phục · tiếp tục từ Lưu dàn ý',
    agents: [{ name: 'architect_long', state: 'running', taskId: 'task-plan-1', taskKind: 'plan', summary: 'architect_long → save_foundation[outline]', tool: 'save_foundation', turn: 3, context: { tokens: 48200, contextWindow: 320000, percent: 15.1, scope: 'baseline', strategy: 'none', activeMessages: 9, summaryMessages: 0, compactedCount: 0, keptCount: 9 }, updatedAt: iso(NOW - 4000) }],
    // Gói P · P1: khung “Kế hoạch truyện” — 4/5 bước xong, đang lưu dàn ý (5/8 chương).
    plan: {
      runningStep: 'outline', done: 4, total: 5,
      steps: [
        { key: 'book', status: 'done', summary: 'Tuổi Sáng Phản Bộn', durationMs: 122000,
          detail: { kind: 'book', title: 'Tuổi Sáng Phản Bộn', synopsis: 'Một nữ thư ký pháp án chứng kiến kịch bản của cô bị đổi tên tác giả; cô quyết đoạt lại bằng chính những ký tự từng viết ra.' } },
        { key: 'premise', status: 'done', summary: 'Thư ký pháp án đoạt lại kịch bản bị chiếm đoạt', durationMs: 305000,
          detail: { kind: 'premise', text: 'Lâm Tri Hạ, thư ký pháp án mười năm kinh nghiệm, phát hiện kịch bản bán chạy “Tuổi Sáng Phản Bộn” do cô viết bị ký tên người khác. Không ai tin lời một thư ký — vậy cô sẽ chứng minh bằng hồ sơ tòa án, từng chứng cứ một.' } },
        { key: 'characters', status: 'done', summary: '6 nhân vật', durationMs: 238000,
          detail: { kind: 'characters', characters: [
            { name: 'Lâm Tri Hạ', role: 'Nữ chính · thư ký pháp án' },
            { name: 'Tống Nhược Vy', role: 'Phản diện · ảnh hậu' },
            { name: 'Phùng Chấn', role: 'Đồng phạm' },
            { name: 'Kỳ Dương', role: 'Đồng minh' },
            { name: 'Hứa Hoài Châu', role: 'Đạo diễn' },
            { name: 'Đỗ Mẫn', role: 'Tổng đạo diễn' },
          ] } },
        { key: 'worldRules', status: 'done', summary: '3 quy tắc', durationMs: 191000,
          detail: { kind: 'worldRules', rules: [
            { rule: 'Mọi tranh chấp bản quyền phải có chứng cứ hồ sơ, không đối đầu bằng tin đồn' },
            { rule: 'Phòng quay chỉ có một phía quan sát — nhân vật trong phòng không nhìn thấy người ngoài' },
            { rule: 'Tin nhắn đe dọa không bao giờ để lại số thật của người gửi' },
          ] } },
        { key: 'outline', status: 'running', summary: 'đã lưu 5/8 chương', durationMs: 0, startedAt: iso(NOW - 47000),
          detail: { kind: 'outline', saved: 5, total: 8, chapters: [
            { chapter: 1, title: 'Chữ ký sai hợp đồng', hook: 'Tri Hạ phát hiện tên tác giả bị thay' },
            { chapter: 2, title: 'Phòng quy hoạch vắng chủ', hook: 'Người giữ chìa khoá lưu trữ mất tích' },
            { chapter: 3, title: 'Bản scan thứ ba', hook: 'Một bản nháp sớm tồn tại ngoài hệ thống' },
            { chapter: 4, title: 'Số lạ mỗi đêm', hook: 'Tin nhắn đe dọa đầu tiên' },
            { chapter: 5, title: 'Người thay vai', hook: 'Diễn viên mới biết quá nhiều' },
          ] } },
      ],
    },
  },
})
planStory.replay = planReplay

// Luồng lập kế hoạch: Kiến trúc sư đang gọi save_foundation — các round ✻ Thiết lập
// nối nhau (không có văn xuôi), vòng lặp mô phỏng từng lượt lưu.
planStory.streamScript = [
  '✻ Thiết lập\ntype: premise\ntext: Một nữ thư ký pháp án chứng kiến kịch bản của cô bị đổi tên tác giả; cô quyết đoạt lại bằng chính những ký tự từng viết ra.',
  '✻ Thiết lập\ntype: characters\n- name: Lâm Tri Hạ\n  brief_role: nữ thư ký pháp án\n- name: Tống Nhược Vy\n  brief_role: ảnh hậu chiếm đoạt kịch bản\n- name: Hứa Hoài Châu\n  brief_role: đạo diễn thuộc phe Tri Hạ',
  '✻ Thiết lập\ntype: outline\n- chapter: 1\n  title: Chữ ký sai hợp đồng\n  core_event: Tri Hạ phát hiện bản ký kết bị thay tên tác giả.\n- chapter: 2\n  title: Phòng quy hoạch vắng chủ\n  core_event: Người giữ chìa khoá lưu trữ bản thảo vắng mặt.',
].map((line) => (line.endsWith('\n') ? line : `${line}\n`))
put(planStory)

// D13 — truyện dài cho Chất lượng truyện `len=40` (Quality.dc.html › IS40, cùng bộ sinh số cố định):
// 40 chương đã viết, 35 đang viết lại, 36–40 bản trước. Mỗi chương = 1 bản thẩm duyệt; mục “fixed”
// nằm ở previous[] (lần chấm 1), mục “kept” có quyết định giữ nguyên — đi qua cùng route /quality.
{
  const SEC40: Record<string, [string, string]> = {
    m2: ['Master §2', 'Động cơ giữ chân'], m4: ['Master §4', 'Mở đầu 30 giây, 2 phút, 5 phút'], m5: ['Master §5', 'Nhịp sóng và mật độ'],
    m6: ['Master §6', 'Leo thang'], m7: ['Master §7', 'Đầu chương và cuối chương'], m12: ['Master §12', 'Nhân vật'],
    m13: ['Master §13', 'Văn phong, thoại, hài'], m15: ['Master §15', 'Viết cho audio'], m16: ['Master §16', 'Kết truyện'],
    xh: ['Xưng hô', 'Bảng xưng hô của truyện'], ur: ['Quy tắc của bạn', 'Quy tắc viết của truyện'],
  }
  const TX: Record<string, string> = {
    m2: 'Chỉ 1 câu hỏi đang treo.', m4: 'Câu đầu chưa có người bị hại.', m5: 'Quãng dài không có thông tin mới.', m6: 'Cược chưa lớn hơn chương trước.',
    m7: 'Kết chương lặp kiểu móc.', m12: 'Nhân vật đổi ý chưa có lý do.', m13: 'Tỉ lệ thoại ngoài chuẩn của gói.', m15: 'Đoạn dài hơn 90 tiếng.',
    xh: 'Cách gọi lệch bảng xưng hô.', ur: 'Từ bị cấm theo quy tắc của bạn.',
  }
  const rows: [number, string, 'error' | 'warning', 'open' | 'fixed' | 'kept', string][] = []
  let r = 7
  const rnd = () => { r = (r * 9301 + 49297) % 233280; return r / 233280 }
  for (let c = 1; c <= 40; c++) {
    for (const id of ['m2', 'm4', 'm5', 'm6', 'm7', 'm12', 'm13', 'm15', 'xh', 'ur']) {
      if (rnd() >= 0.16) continue
      const e = rnd() < 0.18
      const st = c <= 34 ? (e ? 'fixed' : (rnd() < 0.55 ? 'fixed' : (rnd() < 0.5 ? 'kept' : 'open'))) : 'open'
      rows.push([c, id, e ? 'error' : 'warning', st, TX[id]!])
    }
  }
  rows.push([37, 'm16', 'error', 'open', 'Kết cục phản diện phụ chưa tương xứng.'])
  const key = '_/chat-luong-40'
  const decisions = new Map<string, ReviewDecision>()
  const entries: Record<number, ReviewEntry> = {}
  for (let c = 1; c <= 40; c++) {
    const mine = rows.map((row, i) => ({ row, i })).filter(({ row }) => row[0] === c)
    const item = ({ row, i }: { row: typeof rows[number]; i: number }): ReviewItem => ({
      id: `q40-${c}-${row[1]}-${i}`, level: row[2], ref: SEC40[row[1]]![0], refName: SEC40[row[1]]![1], text: row[4], quote: '', fix: '',
    })
    for (const entry of mine.filter(({ row }) => row[3] === 'kept')) {
      decisions.set(item(entry).id, { state: 'kept', reason: 'Giữ giọng kể của chương', by: 'Đạt', at: iso(NOW - 86400000) })
    }
    const fixed = mine.filter(({ row }) => row[3] === 'fixed').map(item)
    entries[c] = {
      chapter: c, scope: 'chapter', issues: [], items: mine.filter(({ row }) => row[3] !== 'fixed').map(item), round: fixed.length ? 2 : 1,
      pack: { id: 'nu-tan-bao-thu', ten: 'Nữ tần báo thù' }, blocking: mine.some(({ row }) => row[3] === 'open' && row[2] === 'error'),
      at: iso(NOW - (41 - c) * 3600000), verdict: 'accept', summary: '',
      previous: fixed.length ? [{ round: 1, verdict: 'polish', items: fixed, rule_aware: false }] : [],
    }
  }
  REVIEWS[key] = entries
  reviewDecisions.set(key, decisions)
  const chapters = Object.fromEntries(Array.from({ length: 40 }, (_, i) => [i + 1, { title: `Chương ${i + 1}`, text: TRAXANH_CHAPTERS[(i % 5) + 1]?.text ?? '' }]))
  put(mkStory('_', 'chat-luong-40', 'chat-luong-40', 'Truyện Dài Bốn Mươi Chương', 'paused', {
    chapters, completed: 40, total: 40,
    snapshot: mwSnapshot('Truyện Dài Bốn Mươi Chương', 'paused', {
      flow: 'rewriting', currentChapter: 35, inProgressChapter: 35, completedCount: 40, totalChapters: 40,
      pendingRewrites: [35, 36, 37, 38, 39, 40], outline: outlineFor(40, 40),
    }),
  }))
}

// Gói AG — truyện 240 chương: sidebar mục lục lazy (?volume=), tập 7 khung trống (locked).
put(mkStory('dit-dang-audio', '08-dai-240-chuong', 'kich-ban/08-dai-240-chuong',
  'Truyện Dài 240 Chương', 'paused', {
    chapters: Object.fromEntries(Array.from({ length: 240 }, (_, i) => [i + 1, { title: `Chương ${i + 1}`, text: TRAXANH_CHAPTERS[(i % 5) + 1]?.text ?? '' }])),
    completed: 80, total: 240,
    snapshot: {
      ...snapshotTraXanh, isRunning: false, runtimeState: 'paused', flow: 'writing', statusLabel: '暂停中',
      currentChapter: 81, inProgressChapter: 81, completedCount: 80, totalChapters: 240,
      totalWordCount: 80 * 3240, pendingRewrites: null, outline: outlineFor(240, 80),
    },
  }))

const findStory = (p: string, s: string) => stories.get(`${p}/${s}`)

// ─── SSE broadcast ─────────────────────────────────────────────────────────
const sseSend = (client: SseClient, event: string, data: unknown) => {
  try { client.res.write(`event: ${event}\ndata: ${JSON.stringify(data)}\n\n`) } catch { /* đóng */ }
}
const broadcast = (st: MockStory, event: string, data: unknown) => {
  if (event === 'event') {
    const incoming = data as HostEvent
    const previous = st.replay.find((entry) => entry.id === incoming.id)
    data = { ...incoming, chapter: st.snapshot.currentChapter, time: previous?.time ?? new Date().toISOString() }
    st.replay = [...st.replay.filter((entry) => entry.id !== incoming.id), data as HostEvent]
  }
  if (event === 'snapshot' && (data as Snapshot).isRunning) {
    st.snapshot = { ...(data as Snapshot), inProgressChapter: (data as Snapshot).currentChapter }
    data = st.snapshot
  }
  // Việc chạy nền (GAP-26): gương của job engine + trạng thái viết sang /api/jobs.
  if (event === 'job' && (data as JobEvent).kind === 'import') {
    const job = data as JobEvent
    const preview = st.importState?.preview ?? mockImportPreview('ban-thao.txt')
    const ids = ['ingesting', 'segmenting', 'analyzing', 'synthesizing', 'publishing']
    const index = job.done ? 5 : Math.max(0, ids.indexOf(job.stage === 'awaiting_confirmation' ? 'segmenting' : job.stage === 'awaiting_story_status' ? 'synthesizing' : job.stage))
    job.import = { schemaVersion: 1, preview, continued: !!st.importState?.options?.continueAfter, steps: ids.map((id, i) => ({
      schemaVersion: 1, id, title: ['Đọc tệp', 'Chia chương', 'Tóm tắt từng chương', 'Dựng nhân vật & bối cảnh', 'Lập dàn ý & phục bút'][i],
      current: i < index ? preview.chapters.length : i === index && i === 2 ? 13 : 0,
      total: preview.chapters.length, status: i < index ? 'done' : i === index ? 'running' : 'pending',
    })), ...(index === 2 ? { etaSec: Math.round((1 - 13 / preview.chapters.length) * 540) } : {}), ...(job.done ? { summary: { schemaVersion: 1, chapters: preview.chapters.length, characters: 14, foreshadows: 9 } } : {}) }
  }
  if (event === 'job') mirrorJobEvent(st.p, st.s, data as JobEvent)
  if (event === 'snapshot') syncWriteJob(st.p, st.s, data as Snapshot)
  if (event.endsWith('.changed')) emitGlobalEvent(event, data)
  if (event === 'snapshot') emitGlobalEvent('story.changed', { project: st.p, story: st.s, parts: changedSnapshotParts(st) })
  // Ghép khung thực thể vào luồng /api/events chung: mỗi tab chỉ cần một SSE.
  // Khung audio phải cùng hình với máy chủ thật (`{type, category, data}`).
  emitLiveFrame(st.p, st.s, event, event.startsWith('audio.') ? { type: event, category: 'AUDIO', data } : data)
  st.clients.forEach((c) => sseSend(c, event, data))
}
const broadcastEvent = (st: MockStory, e: HostEvent) => broadcast(st, 'event', e)
for (const st of stories.values()) syncWriteJob(st.p, st.s, st.snapshot)
// Tab mới mở nhận lịch sử “Vừa xong” qua khung `live`, không cần SSE riêng của worker.
for (const st of stories.values()) seedLiveEvents(st.p, st.s, st.replay)

function ensureStream(st: MockStory) {
  if (st.streamTimer || !st.snapshot.isRunning) return
  let idx = 0
  st.streamTimer = setInterval(() => {
    if (!st.snapshot.isRunning) { clearInterval(st.streamTimer!); st.streamTimer = null; return }
    if (st.clients.size === 0) return
    const script = st.streamScript ?? STREAM_SCRIPT
    const text = script[idx % script.length]
    const step = 80 + Math.floor(Math.random() * 60)
    for (let i = 0; i < text.length; i += step) {
      broadcast(st, 'delta', { text: text.slice(i, i + step) })
    }
    idx++
    // Kịch bản riêng (lập kế hoạch): hết vòng chỉ xoá luồng cho lượt kế tiếp, không chốt chương.
    if (idx % script.length === 0 && st.streamScript) { broadcast(st, 'clear', {}); return }
    if (idx % script.length === 0) {
      broadcast(st, 'done', {})
      broadcast(st, 'clear', {})
      const ch = st.snapshot.currentChapter
      const rewriting = st.snapshot.flow === 'rewriting' && (st.snapshot.pendingRewrites ?? []).includes(ch)
      const verb = rewriting ? '重写' : '写'
      broadcastEvent(st, mkEvent(90 + idx, { agent: 'writer', summary: `${verb}第 ${ch} 章`, level: 'success' }))
      st.snapshot = {
        ...st.snapshot, currentChapter: Math.min(ch + 1, st.snapshot.totalChapters || ch + 1), completedCount: Math.min(st.snapshot.totalChapters || ch, Math.max(st.snapshot.completedCount, ch)),
        statusLabel: `${verb}第 ${ch + 1} 章`,
        pendingRewrites: rewriting ? (st.snapshot.pendingRewrites ?? []).filter((n) => n !== ch) : st.snapshot.pendingRewrites,
        recoveryLabel: rewriting && (st.snapshot.pendingRewrites ?? []).filter((n) => n !== ch).length > 0
          ? `重写恢复: ${(st.snapshot.pendingRewrites ?? []).filter((n) => n !== ch).length} 章待处理` : '',
      }
      st.completed = st.snapshot.completedCount
      st.chapters[ch] ??= TRAXANH_CHAPTERS[ch] ?? { title: CHAPTER_TITLES[ch] ?? `Chương ${ch}`, text: STREAM_SCRIPT.filter((line) => !line.startsWith('✻')).join('\n') }
      const finished = st.snapshot.totalChapters > 0 && st.completed >= st.snapshot.totalChapters
      if (st.pauseAfterStep || finished) {
        st.status = finished ? 'done' : 'paused'
        st.snapshot = { ...st.snapshot, isRunning: false, runtimeState: finished ? 'completed' : 'paused', statusLabel: finished ? 'Hoàn thành' : 'Đã tạm dừng sau bước hiện tại', inProgressChapter: 0, agents: st.snapshot.agents?.map((agent) => ({ ...agent, state: 'idle' })) ?? null }
        st.pauseAfterStep = false
        clearInterval(st.streamTimer!); st.streamTimer = null
      }
      broadcast(st, 'snapshot', st.snapshot)
    }
  }, 1100)
}

// Stage thật của engine nhập truyện (internal/host/imp/types.go)
const IMPORT_STEPS: { stage: string; message: string; awaiting?: JobEvent['awaiting'] }[] = [
  { stage: 'ingesting', message: 'Đang đọc tệp' },
  { stage: 'segmenting', message: 'Đang chia đoạn' },
  { stage: 'awaiting_confirmation', message: 'Xác nhận nhập 12 chương thành truyện "Ngập trời mở mắt"?', awaiting: 'confirm' },
  { stage: 'analyzing', message: 'Đang phân tích' },
  { stage: 'synthesizing', message: 'Đang tổng hợp' },
  { stage: 'awaiting_story_status', message: 'Chọn trạng thái truyện sau khi nhập', awaiting: 'storyStatus' },
  { stage: 'validating', message: 'Đang kiểm tra' },
  { stage: 'publishing', message: 'Đang ghi truyện' },
]

/** `hooks` (thẻ kết quả lệnh — CmdResults): đổi stage/message mỗi nhịp và gắn payload khi xong. */
const runJob = (st: MockStory, kind: JobEvent['kind'], total = 6, hooks?: {
  interval?: number; onTick?: (job: JobEvent) => void; onDone?: (job: JobEvent) => void
}) => {
  const job: JobEvent = {
    job: `${kind === 'importsim' ? 'simulate' : kind}/${++seq}`, kind, stage: 'start', current: 0,
    total: kind === 'import' ? IMPORT_STEPS.length : total,
    message: 'Khởi động…', level: 'info', retryAt: null, done: false, error: '', awaiting: '',
  }
  st.jobs.set(job.job, job)
  broadcast(st, 'job', job)
  const steps = ['Đọc tệp', 'Phân tích cấu trúc', 'Tạo đại cương', 'Viết bản nháp chương 1', 'Kiểm tra nhất quán', 'Hoàn tất']
  let ticks = 0
  const finish = () => {
    job.done = true; job.stage = 'done'; job.message = 'Hoàn tất'; job.awaiting = ''
    clearInterval(timer)
    broadcastEvent(st, mkEvent(200 + seq, { agent: 'arbiter', summary: `${kind} 完成`, level: 'success' }))
    broadcast(st, 'done', {})
  }
  const timer = setInterval(async () => {
    if (!stories.has(`${st.p}/${st.s}`)) { clearInterval(timer); return }
    if (job.awaiting) return // chờ confirm / storyStatus
    if (kind === 'import') {
      const step = IMPORT_STEPS[job.current]
      job.current++
      if (step) {
        job.stage = step.stage
        job.message = step.message
        if (step.awaiting && !(step.awaiting === 'confirm' && st.importConfirmed) && !(step.awaiting === 'storyStatus' && st.importState?.options?.storyStatus)) job.awaiting = step.awaiting
      }
      if (!step || job.current >= IMPORT_STEPS.length) finish()
      broadcast(st, 'job', job)
      return
    }
    ticks++
    job.current++
    job.stage = steps[Math.min(job.current - 1, steps.length - 1)].toLowerCase().replace(/ /g, '-')
    job.message = steps[Math.min(job.current - 1, steps.length - 1)]
    hooks?.onTick?.(job)
    if (ticks >= total) { finish(); hooks?.onDone?.(job) }
    broadcast(st, 'job', job)
  }, hooks?.interval ?? 800)
  return job
}

// ─── plugin ────────────────────────────────────────────────────────────────
export function mockApiPlugin(): Plugin {
  return {
    name: 'ainovel-mock-api',
    configureServer(server) {
      const external = process.env.AINOVEL_MOCK_EXTERNAL_EDIT === '1' ? setInterval(() => {
        const key = docKey('global', '', '')
        const config = configStore.get(key) ?? CONFIG_GLOBAL
        configStore.set(key, { ...config, style: config.style === 'wuxia' ? 'default' : 'wuxia' })
        bumpMockVersion(`config:${key}`)
        emitGlobalEvent('config.changed', { scope: 'global' })
      }, 12000) : undefined
      server.httpServer?.once('close', () => clearInterval(external))
      // eslint-disable-next-line @typescript-eslint/no-misused-promises
      // Điều khiển riêng của mock (không phải API thật): POST /__mock/update {on} bật/tắt trạng thái có bản mới.
      server.middlewares.use('/__mock', async (req: IncomingMessage & { url?: string }, res: ServerResponse, next: () => void) => {
        if (req.url?.startsWith('/update')) {
          if (req.method === 'POST') {
            const body = (await readBody(req)) as { on?: boolean }
            mockHasUpdate = !!body.on
          }
          res.writeHead(200, { 'content-type': 'application/json' })
          return res.end(JSON.stringify({ hasUpdate: mockHasUpdate }))
        }
        // GET /__mock/connections — số luồng SSE toàn cục đang mở (kiểm “mỗi tab một luồng”).
        if (req.url?.startsWith('/connections')) {
          res.writeHead(200, { 'content-type': 'application/json' })
          return res.end(JSON.stringify({ sse: sseConnectionCount() }))
        }
        // Bug 2710: a finished read tool followed by the editor's unanswered final reply.
        if (req.url?.startsWith('/ask-surface') && req.method === 'POST') {
          const body = (await readBody(req)) as { story?: string; pending?: boolean }
          const st = stories.get(body.story ?? 'dit-dang-audio/01-tra-xanh-cuop-kich-ban')
          if (!st) return json(res, 404, { error: 'Không tìm thấy truyện mẫu' })
          clearInterval(st.streamTimer ?? undefined)
          st.streamTimer = null
          st.replay = []
          st.status = body.pending ? 'paused' : 'writing'
          st.snapshot = {
            ...st.snapshot, phase: 'writing', flow: 'review', plan: null, advanceMode: 'auto',
            isRunning: !body.pending, runtimeState: body.pending ? 'paused' : 'running',
            waitingFor: body.pending ? 'input' : '', hasAdvanceHold: false,
            pendingAsk: body.pending ? {
              question: 'Bạn muốn tôi tóm tắt cung hay kiểm tra lại chương?',
              chapter: 2,
              options: [
                { label: 'Tóm tắt cung', reply: 'Tóm tắt cung hiện tại theo nhiệm vụ đang làm', isDefault: true },
                { label: 'Kiểm tra lại chương', reply: 'Đọc lại chương 2 trước khi tóm tắt cung' },
              ],
            } : null,
            agents: st.snapshot.agents?.map((agent) => ({ ...agent, state: agent.name === 'editor' && !body.pending ? 'working' : 'idle', tool: agent.name === 'editor' && !body.pending ? 'read_chapter' : '' })) ?? null,
          }
          broadcast(st, 'reset', {})
          broadcast(st, 'snapshot', st.snapshot)
          broadcastEvent(st, mkEvent(0, {
            id: 'ask-surface-read', category: 'TOOL', agent: 'editor', tool: 'read_chapter',
            args: '{"chapter":2,"source":"draft"}', status: 'ok', failed: false,
            finishedAt: new Date().toISOString(), summary: 'read_chapter(chương 2 · bản nháp)',
            result: '{"chapter":2,"word_count":1546}', level: 'success',
          }))
          broadcast(st, 'delta', { text: '✻ Đọc chương\nchapter: 2\nsource: draft\n\n**Đã đọc chương.**\n\n- Chưa gọi `save_arc_summary`.\n- Cần chọn hướng đi.\n\nBạn muốn tôi tóm tắt cung hay kiểm tra lại chương?' })
          return json(res, 200, { ok: true, story: `${st.p}/${st.s}`, pending: !!body.pending })
        }
        // POST /__mock/reset {story?: "p/s" hoặc slug} — dựng lại trạng thái gieo mầm.
        if (req.url?.startsWith('/reset') && req.method === 'POST') {
          const body = (await readBody(req)) as { story?: string }
          const done = resetMockStories(body.story)
          res.writeHead(200, { 'content-type': 'application/json' })
          return res.end(JSON.stringify({ reset: done }))
        }
        // POST /__mock/drop-connections — đóng mọi socket cũ (SSE/app) để trình duyệt
        // giải phóng slot kết nối; trang sẽ tự mở lại luồng.
        if (req.url?.startsWith('/drop-connections') && req.method === 'POST') {
          res.writeHead(200, { 'content-type': 'application/json' })
          res.end('{"ok":true}')
          setTimeout(() => (server.httpServer as Server | null)?.closeAllConnections?.(), 25)
          return
        }
        next()
      })
      server.middlewares.use('/api', async (req: IncomingMessage & { url?: string }, res: ServerResponse, next: () => void) => {
        const url = new URL(req.url ?? '/', 'http://x')
        const path = url.pathname // đã bỏ prefix /api
        const seg = path.split('/').filter(Boolean).map(decodeURIComponent)
        const session = sessionOf(req)
        const user = session ? users.find((u) => u.user.username === session.username)?.user ?? null : null
        if (seg[0] !== 'login' && seg[0] !== 'pair' && seg[0] !== 'recover' && seg[0] !== 'setup' && seg[0] !== '__mock' && !user) {
          return fail(res, 401, 'Chưa đăng nhập')
        }

        try {
          // GAP-41 · A0 Lần chạy đầu — không cần phiên. Mã mẫu: TRAX-41ZN (đổi được qua /api/__mock/setup).
          if (seg[0] === '__mock' && seg[1] === 'setup') {
            if (req.method === 'POST') {
              const body = await readBody(req)
              resetSetup(body.needed !== false)
              if (typeof body.code === 'string' && body.code.trim()) setup.code = body.code.replace(/[^A-Za-z0-9]/g, '').toUpperCase().slice(0, 8)
              return json(res, 200, { needed: setup.needed, code: setup.code })
            }
            return json(res, 200, { ...setupState(), code: setup.code })
          }
          if (seg[0] === 'setup') {
            if (req.method === 'GET' && !seg[1]) {
              if (url.searchParams.get('mocksetup') === '1') resetSetup(true)
              return json(res, 200, setupState())
            }
            if (seg[1] === 'verify' && req.method === 'POST') {
              const body = await readBody(req)
              if (!setup.needed) return json(res, 409, { error: 'Đã có tài khoản quản trị', at: iso(setup.doneAt) })
              // Thứ tự của máy chủ thật (GAP-41): hết hạn 410 → khoá 423 → sai định dạng 400 → sai 400.
              if (setup.expiresAt <= Date.now()) return json(res, 410, { error: 'Mã thiết lập đã hết hạn' })
              if (setup.lockedUntil > Date.now()) return json(res, 423, { error: 'Đã thử sai 5 lần', lockedUntil: iso(setup.lockedUntil) })
              const code = String(body.code ?? '').replace(/[^A-Za-z0-9]/g, '').toUpperCase()
              // Sai định dạng (không đủ 8 ký tự hợp lệ) không bị tính vào 5 lượt.
              if (code.length !== 8) return json(res, 400, { error: 'Mã thiết lập sai định dạng', format: true })
              if (code !== setup.code) {
                setup.attempts++
                if (setup.attempts >= 5) {
                  setup.lockedUntil = Date.now() + SETUP_LOCK
                  return json(res, 423, { error: 'Đã thử sai 5 lần', lockedUntil: iso(setup.lockedUntil) })
                }
                return json(res, 400, { error: 'Mã thiết lập chưa đúng', attemptsLeft: 5 - setup.attempts })
              }
              setup.attempts = 0
              setup.token = `setup-${Math.random().toString(36).slice(2, 12)}`
              setup.tokenExpiresAt = Date.now() + SETUP_TTL
              return json(res, 200, { setupToken: setup.token, expiresAt: iso(setup.tokenExpiresAt) })
            }
            if (seg[1] === 'admin' && req.method === 'POST') {
              const body = await readBody(req)
              if (!setup.needed) return json(res, 409, { error: 'Đã có tài khoản quản trị', at: iso(setup.doneAt) })
              if (!setup.token || body.setupToken !== setup.token || setup.tokenExpiresAt <= Date.now()) {
                return json(res, 410, { error: 'Mã thiết lập đã hết hạn hoặc đã dùng' })
              }
              const username = String(body.username ?? '').trim().toLowerCase()
              const name = String(body.name ?? '').trim()
              if (!/^[a-z0-9._-]{2,32}$/.test(username)) return json(res, 400, { error: 'Tên đăng nhập chỉ dùng chữ thường không dấu, số, dấu chấm, gạch (2–32 ký tự)', field: 'username' })
              if (users.some((u) => u.user.username === username)) return json(res, 400, { error: 'Tên đăng nhập này đã có người dùng', field: 'username' })
              if (!name) return json(res, 400, { error: 'Cần tên hiển thị', field: 'name' })
              if (String(body.password ?? '').length < 8) return json(res, 400, { error: 'Mật khẩu cần ít nhất 8 ký tự', field: 'password' })
              const admin: User = {
                username, name, role: 'admin', projects: {}, disabled: false,
                createdAt: iso(NOW), updatedAt: iso(NOW), onboardedAt: null, mustChangePassword: false,
                lastSeenAt: iso(NOW), sessionCount: 1,
              }
              users.push({ user: admin, password: String(body.password) })
              setup.needed = false; setup.doneAt = Date.now(); setup.token = ''
              recoveryCode.active = true
              const s = mkSession(username, 'MacBook — web')
              setSessionCookie(res, s.token)
              emitGlobalEvent('account.changed', { username })
              return json(res, 200, { user: admin, recoveryCode: recoveryCode.value })
            }
          }
          // GAP-33: mã khôi phục quản trị. Mã mẫu hợp lệ: TRAX-8K2M-QP7D-41ZN (mỗi mã dùng một lần).
          if (seg[0] === 'recover' && req.method === 'POST') {
            const body = await readBody(req)
            if (recoverFails >= 5) return fail(res, 429, 'Thử sai quá nhiều lần, vui lòng đợi rồi thử lại', 60)
            if (String(body.newPassword ?? '').length < 8) return fail(res, 400, 'Mật khẩu cần ít nhất 8 ký tự')
            const found = users.find((u) => u.user.username === body.username && u.user.role === 'admin')
            if (!found || !recoveryCode.active || String(body.code ?? '').replaceAll('-', '').toUpperCase() !== recoveryCode.value.replaceAll('-', '')) {
              recoverFails++
              return fail(res, 401, 'Mã khôi phục không đúng hoặc đã dùng')
            }
            recoverFails = 0
            recoveryCode.active = false
            found.password = String(body.newPassword)
            found.user.mustChangePassword = false
            for (const item of [...sessions.values()]) if (item.username === found.user.username) sessions.delete(item.token)
            const s = mkSession(found.user.username, 'MacBook — web')
            setSessionCookie(res, s.token)
            found.user.sessionCount = 1
            found.user.version = bumpMockVersion(`account:${found.user.username}`)
            emitGlobalEvent('account.changed', { username: found.user.username })
            return json(res, 200, { user: found.user })
          }
          if (user && session && ['notify', 'notifications', 'push'].includes(seg[0] ?? '')) {
            return handleNotifyMock(req, res, url, user, session)
          }
          if (seg[0] === 'events') {
            if (seg[1] === 'watch' && req.method === 'POST') {
              const body = await readBody(req)
              const ok = updateGlobalWatch(String(body.subscriber ?? ''), Array.isArray(body.keys) ? body.keys.map(String) : [])
              return json(res, ok ? 200 : 404, { ok })
            }
            if (req.method === 'GET') return openGlobalEvents(req, res, user!, url)
          }
          // ─── auth ───
          if (seg[0] === 'login' && req.method === 'POST') {
            const body = await readBody(req)
            if (loginFails >= 5) return fail(res, 429, 'Thử sai quá nhiều lần, vui lòng đợi rồi thử lại', 60)
            const found = users.find((u) => u.user.username === body.username && u.password === body.password)
            if (!found || found.user.disabled) { loginFails++; if (loginFails === 5) appendAudit(null, 'sec', '5 lần đăng nhập sai vào tài khoản', String(body.username ?? '')); return fail(res, 401, 'Sai tên đăng nhập hoặc mật khẩu') }
            loginFails = 0
            found.user.lastSeenAt = iso(Date.now())
            const s = mkSession(found.user.username, 'MacBook — web')
            setSessionCookie(res, s.token)
            found.user.sessionCount = Array.from(sessions.values()).filter((x) => x.username === found.user.username).length
            found.user.version = mockVersion(`account:${found.user.username}`)
            emitGlobalEvent('account.changed', { username: found.user.username })
            return json(res, 200, { user: found.user })
          }
          if (seg[0] === 'pair' && req.method === 'POST') {
            const body = await readBody(req)
            const code = String(body.code ?? '')
            const pairing = [...pairings.values()].find((p) => p.code === code && p.expiresAt > Date.now())
            if (!pairing) return fail(res, 401, 'Mã QR không hợp lệ, đã dùng hoặc đã hết hạn')
            pairings.delete(pairing.code)
            const owner = users.find((u) => u.user.username === pairing.username)
            if (!owner) return fail(res, 401, 'Mã QR không hợp lệ, đã dùng hoặc đã hết hạn')
            const s = mkSession(owner.user.username, 'iPhone — web')
            setSessionCookie(res, s.token)
            owner.user.sessionCount = Array.from(sessions.values()).filter((item) => item.username === owner.user.username).length
            owner.user.version = mockVersion(`account:${owner.user.username}`)
            emitGlobalEvent('account.changed', { username: owner.user.username })
            return json(res, 200, { user: owner.user })
          }
          if (seg[0] === 'logout') {
            if (session) {
              sessions.delete(session.token)
              emitGlobalEvent('session.revoked', { username: session.username, session: session.id })
              emitGlobalEvent('account.changed', { username: session.username })
            }
            res.setHeader('set-cookie', `${sessionCookieName(req)}=; Path=/; HttpOnly; Max-Age=0`)
            return json(res, 200, { ok: true })
          }
          if (seg[0] === 'me') {
            if (!user || !session) return fail(res, 401, 'Chưa đăng nhập')
            const sub = seg[1]
            user.version = mockVersion(`account:${user.username}`)
            // 19-gap-closure.md: đổi tên hiển thị của chính mình.
            if (!sub && (req.method === 'POST' || req.method === 'PUT')) {
              const body = await readBody(req)
              const rec = users.find((u) => u.user.username === user.username)!
              if (mockConflict(req, res, `account:${user.username}`, { user: rec.user }, body)) return
              rec.user.name = String(body.name ?? rec.user.name)
              rec.user.version = bumpMockVersion(`account:${user.username}`)
              emitGlobalEvent('account.changed', { username: user.username })
              return json(res, 200, { user: rec.user })
            }
            // D15: số bài học Não hệ thống chờ duyệt — chỉ quản trị.
            if (!sub) return json(res, 200, { user, session: sessionJson(session, session.token), ...(user.role === 'admin' ? { pendingLessons: mockPendingLessons() } : {}) })
            if (sub === 'onboarded' && req.method === 'POST') {
              const rec = users.find((u) => u.user.username === user.username)!
              rec.user.onboardedAt = iso(NOW)
              rec.user.version = bumpMockVersion(`account:${user.username}`)
              emitGlobalEvent('account.changed', { username: user.username })
              return json(res, 200, { user: rec.user })
            }
            if (sub === 'password' && req.method === 'POST') {
              const body = await readBody(req)
              const rec = users.find((u) => u.user.username === user.username)!
              if (mockConflict(req, res, `account:${user.username}`, { user: rec.user }, body)) return
              if (rec.password !== body.current) return fail(res, 403, 'Mật khẩu hiện tại không đúng')
              rec.password = String(body.new)
              rec.user.mustChangePassword = false
              rec.user.passwordChangedAt = iso(Date.now())
              appendAudit(user, 'sec', 'tự đặt mật khẩu mới', user.username)
              rec.user.version = bumpMockVersion(`account:${user.username}`)
              for (const item of sessions.values()) {
                if (item.username !== user.username || item.id === session.id) continue
                sessions.delete(item.token)
                emitGlobalEvent('session.revoked', { username: item.username, session: item.id })
              }
              emitGlobalEvent('account.changed', { username: user.username })
              return json(res, 200, { ok: true })
            }
            if (sub === 'recovery') {
              if (user.role !== 'admin') return fail(res, 403, 'Không có quyền')
              if (seg.length === 2 && req.method === 'GET') {
                const has = recoveryCode.n > 0 || recoveryCode.active
                return json(res, 200, {
                  createdAt: has ? recoveryCode.createdAt : null,
                  used: !recoveryCode.active && recoveryCode.n > 0,
                  // mã che giữa: máy chủ thật chỉ trả khi còn mã chưa dùng (Dialogs › state=recovery)
                  ...(has && recoveryCode.active ? { mask: maskRecovery(recoveryCode.value) } : {}),
                })
              }
              if (req.method === 'POST') {
                const rotate = seg[2] === 'rotate'
                if (!rotate && recoveryCode.active) return fail(res, 409, 'Đã có mã khôi phục chưa dùng — hãy tạo mã mới')
                recoveryCode.n += 1
                recoveryCode.value = recoveryCodes[recoveryCode.n % recoveryCodes.length]
                recoveryCode.active = true
                recoveryCode.createdAt = iso(Date.now())
                res.setHeader('cache-control', 'no-store')
                return json(res, 200, { code: recoveryCode.value, createdAt: recoveryCode.createdAt })
              }
            }
            if (sub === 'sessions') {
              return json(res, 200, { sessions: Array.from(sessions.values()).filter((s) => s.username === user.username).map((s) => sessionListJson(s, session.token)) })
            }
            if (sub === 'pairings' && req.method === 'POST') {
              const code = crypto.randomUUID().replaceAll('-', '')
              const pairing = { code, username: user.username, expiresAt: Date.now() + 5 * 60_000 }
              pairings.set(code, pairing)
              const host = req.headers.host ?? 'localhost:5173'
              const link = (label: string, h: string) => ({ label, url: `http://${h}/p/${code}`, svg: qrSvg(`http://${h}/p/${code}`) })
              return json(res, 200, {
                code, username: user.username, expiresAt: iso(pairing.expiresAt),
                links: !lanOpen ? [] : [link('Wi-Fi nhà', host), link('Tailscale', `100.x.x.x:${(req.headers.host ?? ':8686').split(':')[1] ?? '8686'}`)],
              })
            }
            return fail(res, 404, 'Not found')
          }
          if (seg[0] === 'sessions' && seg[2] === 'revoke' && req.method === 'POST') {
            const target = Array.from(sessions.values()).find((s) => s.id === seg[1])
            if (!target || (user?.role !== 'admin' && target.username !== user?.username)) return fail(res, 403, 'Không có quyền đăng xuất thiết bị này')
            sessions.delete(target.token)
            appendAudit(user ?? null, 'sec', `đăng xuất “${target.device}” của`, target.username)
            emitGlobalEvent('session.revoked', { username: target.username, session: target.id })
            emitGlobalEvent('account.changed', { username: target.username })
            return json(res, 200, { ok: true })
          }
          // ─── việc chạy nền (14-jobs-dock.md §1.7 · GAP-26) ───
          if (seg[0] === 'jobs') {
            if (!user) return fail(res, 401, 'Chưa đăng nhập')
            return handleJobsApi({
              req, res, seg, user, canRead, readBody: () => readBody(req),
              stopWrite: (p, s) => {
                const target = findStory(p, s)
                if (!target) return
                target.status = 'paused'
                target.snapshot = { ...target.snapshot, isRunning: false, runtimeState: 'paused', statusLabel: 'Đã tạm dừng' }
                broadcast(target, 'snapshot', target.snapshot)
              },
            })
          }
          // ─── lệnh (13-commands.md › G11): mảng trần, không bọc ───
          if (seg[0] === 'commands' && seg.length === 1) {
            if (req.method !== 'GET') return fail(res, 405, 'Method not allowed')
            return json(res, 200, COMMANDS)
          }
          if (seg[0] === 'admin' && seg[1] === 'audit') {
            if (user?.role !== 'admin') return fail(res, 403, 'Chỉ quản trị viên')
            return json(res, 200, readAudit(url.searchParams))
          }
          if (seg[0] === 'users') {
            if (!user || user.role !== 'admin') return fail(res, 403, 'Chỉ quản trị viên')
            if (seg.length === 1 && req.method !== 'POST') return json(res, 200, { users: users.map((entry) => ({ ...entry.user, version: mockVersion(`account:${entry.user.username}`), sessionCount: [...sessions.values()].filter((item) => item.username === entry.user.username).length })) })
            const body = req.method === 'POST' || req.method === 'PUT' ? await readBody(req) : {}
            if (seg.length === 1 && req.method === 'POST') {
              const username = String(body.username ?? '')
              if (users.some((u) => u.user.username === username)) {
                let n = 2
                while (users.some((u) => u.user.username === `${username}.${n}`)) n++
                return json(res, 409, { error: `Tên đăng nhập “${username}” đã có`, field: 'username', suggest: `${username}.${n}` })
              }
              if (!/^[a-z0-9._-]{2,32}$/.test(username) || !String(body.name ?? '').trim() || String(body.password ?? '').length < 8 || !['admin', 'member'].includes(String(body.role))) return fail(res, 400, 'Thông tin thành viên không hợp lệ')
              if (body.projects !== undefined && !validProjects(body.projects)) return fail(res, 400, 'Vai trò dự án không hợp lệ')
              const rec = {
                user: { username, name: String(body.name), role: body.role as User['role'], projects: (body.projects ?? {}) as User['projects'], disabled: false, createdAt: iso(Date.now()), updatedAt: iso(Date.now()), onboardedAt: null, mustChangePassword: true, lastSeenAt: null, sessionCount: 0 } as User,
                password: String(body.password),
              }
              users.push(rec)
              appendAudit(user, 'acct', 'tạo tài khoản', username)
              rec.user.version = bumpMockVersion(`account:${username}`)
              emitGlobalEvent('account.changed', { username })
              return json(res, 201, rec.user)
            }
            const target = users.find((u) => u.user.username === seg[1])
            if (!target) return fail(res, 404, 'Không tìm thấy tài khoản')
            if (seg[2] === 'sessions') {
              if (seg[3] === 'revoke-all' && req.method === 'POST') {
                for (const item of sessions.values()) {
                  if (item.username !== target.user.username || item.id === session?.id) continue
                  sessions.delete(item.token)
                  emitGlobalEvent('session.revoked', { username: item.username, session: item.id })
                }
                appendAudit(user, 'sec', 'đăng xuất mọi thiết bị của', target.user.username)
                emitGlobalEvent('account.changed', { username: target.user.username })
                return json(res, 200, { ok: true })
              }
              return json(res, 200, { sessions: Array.from(sessions.values()).filter((s) => s.username === target.user.username).map((s) => sessionListJson(s, session?.token ?? '')) })
            }
            if (seg[2] === 'pairings' && req.method === 'POST') {
              const code = crypto.randomUUID().replaceAll('-', '')
              const pairing = { code, username: target.user.username, expiresAt: Date.now() + 5 * 60_000 }
              pairings.set(code, pairing)
              const host = req.headers.host ?? 'localhost:5173'
              return json(res, 200, { code, username: target.user.username, expiresAt: iso(pairing.expiresAt), links: !lanOpen ? [] : [{ label: 'Wi-Fi nhà', url: `http://${host}/p/${code}`, svg: qrSvg(`http://${host}/p/${code}`) }] })
            }
            if (seg[2] === 'delete' && req.method === 'POST') {
              const admins = users.filter((u) => u.user.role === 'admin' && !u.user.disabled)
              if (target.user.username === user.username || target.user.role === 'admin' && !target.user.disabled && admins.length <= 1) return fail(res, 409, 'Phải còn ít nhất một quản trị viên đang hoạt động; không tự xoá tài khoản đang dùng')
              appendAudit(user, 'acct', 'xoá tài khoản', `${target.user.name} (đã xoá)`, { reason: typeof body.reason === 'string' ? body.reason : undefined })
              users.splice(users.indexOf(target), 1)
              for (const item of sessions.values()) {
                if (item.username !== target.user.username) continue
                sessions.delete(item.token)
                emitGlobalEvent('session.revoked', { username: item.username, session: item.id })
              }
              bumpMockVersion(`account:${target.user.username}`)
              emitGlobalEvent('account.changed', { username: target.user.username })
              return json(res, 200, { ok: true })
            }
            if (req.method === 'POST' || req.method === 'PUT') {
              if (mockConflict(req, res, `account:${target.user.username}`, target.user, body)) return
              if (body.role !== undefined && !['admin', 'member'].includes(String(body.role))) return fail(res, 400, 'Quyền hệ thống không hợp lệ')
              if (body.projects !== undefined && !validProjects(body.projects)) return fail(res, 400, 'Vai trò dự án không hợp lệ')
              if (body.password !== undefined && String(body.password).length < 8) return fail(res, 400, 'Mật khẩu cần ít nhất 8 ký tự')
              // Phải còn ≥1 quản trị viên đang hoạt động (cả khi hạ quyền hoặc vô hiệu hoá)
              const targetActiveAdmin = target.user.role === 'admin' && !target.user.disabled
              const demoting = (body.role !== undefined && body.role !== 'admin') || body.disabled === true
              const admins = users.filter((u) => u.user.role === 'admin' && !u.user.disabled)
              if (targetActiveAdmin && demoting && admins.length <= 1) {
                return fail(res, 409, 'Phải còn ít nhất một quản trị viên đang hoạt động')
              }
              if (body.disabled === true && target.user.username === user.username) return fail(res, 409, 'Không tự khoá tài khoản đang dùng')
              if (body.name !== undefined) target.user.name = String(body.name)
              if (body.role !== undefined && body.role !== target.user.role) {
                appendAudit(user, 'perm', 'đổi quyền hệ thống của', target.user.username, { from: target.user.role, to: String(body.role) })
                target.user.role = body.role as User['role']
              }
              if (body.projects !== undefined) {
                const next = body.projects as User['projects']
                for (const project of new Set([...Object.keys(next), ...Object.keys(target.user.projects)])) {
                  if (next[project] !== target.user.projects[project]) appendAudit(user, 'perm', 'đổi vai trò của', target.user.username, { project, from: target.user.projects[project] ?? 'none', to: next[project] ?? 'none' })
                }
                target.user.projects = next
              }
              if (body.disabled !== undefined && Boolean(body.disabled) !== target.user.disabled) {
                target.user.disabled = Boolean(body.disabled)
                target.user.disabledAt = iso(Date.now()); target.user.disabledBy = user.username; target.user.disabledReason = typeof body.reason === 'string' ? body.reason : undefined
                appendAudit(user, 'acct', target.user.disabled ? 'khoá tài khoản' : 'mở khoá tài khoản', target.user.username, { reason: target.user.disabledReason })
              }
              if (body.password !== undefined) {
                target.password = String(body.password); target.user.mustChangePassword = true
                appendAudit(user, 'sec', 'đặt lại mật khẩu cho', target.user.username)
              }
              target.user.updatedAt = iso(Date.now())
              target.user.version = bumpMockVersion(`account:${target.user.username}`)
              if (body.password !== undefined || body.disabled === true) {
                for (const item of sessions.values()) {
                  if (item.username !== target.user.username) continue
                  sessions.delete(item.token)
                  emitGlobalEvent('session.revoked', { username: item.username, session: item.id })
                }
              }
              emitGlobalEvent('account.changed', { username: target.user.username })
              return json(res, 200, target.user)
            }
          }

          // ─── GAP-34: duyệt & kiểm tra thư mục (admin) ───
          if (seg[0] === 'fs' || (seg[0] === 'library' && seg[1] === 'inspect')) {
            if (user!.role !== 'admin') return fail(res, 403, 'Không có quyền')
            const query = url.searchParams.get('path') ?? ''
            await sleep(180)
            if (seg[1] === 'places') return json(res, 200, mockPlaces())
            if (seg[1] === 'list') {
              if (!query) return fail(res, 400, 'thiếu tham số path')
              const listed = mockList(query)
              return json(res, listed.status, listed.body)
            }
            if (seg[1] === 'mkdir' && req.method === 'POST') {
              const body = await readBody(req)
              const made = mockMkdir(String(body.parent ?? ''), String(body.name ?? ''))
              return json(res, made.status, made.body)
            }
            if (seg[1] === 'inspect') {
              if (!query) return fail(res, 400, 'thiếu tham số path')
              return json(res, 200, mockInspect(query))
            }
          }

          // GAP-34 — cài mới chưa chọn nơi lưu truyện: mọi đường phụ thuộc thư viện trả 409 (không sập).
          if (seg[0] === 'projects' && !mockLibraryConfigured()) return fail(res, 409, 'Chưa chọn nơi lưu truyện')

          // ─── library ───
          if (seg[0] === 'library') {
            const admin = user!.role === 'admin'
            if (req.method === 'PUT') {
              if (!admin) return fail(res, 403, 'Chỉ quản trị viên sửa thư viện')
              const body = await readBody(req)
              // G2 — {root}: đổi gốc thư viện lúc chạy. serve thật trả 409 khi còn truyện đang chạy;
              // mock mặc định cho đổi để xem được luồng thành công, bật AINOVEL_MOCK_LIBRARY_BUSY=1 để thử 409.
              if (typeof body.root === 'string') {
                const root = body.root.trim().replace(/\/+$/, '')
                if (!root) return fail(res, 400, 'Thư mục thư viện không được để trống')
                if (process.env.AINOVEL_MOCK_LIBRARY_BUSY === '1') {
                  return fail(res, 409, 'Đang có truyện chạy — hãy đóng tất cả trước khi đổi thư viện')
                }
                const switched = mockSwitchRoot(root)
                if (switched.status !== 200) return json(res, switched.status, switched.body)
                return json(res, 200, { ok: true, root: mockLibraryRoot(), rootDisplay: mockDisplay(mockLibraryRoot()), config: libraryConfig })
              }
              const nextDirs = { ...libraryConfig.dirs, ...((body.dirs as LibraryConfig['dirs'] | undefined) ?? {}) }
              const reserved = ['output', '.ainovel', 'project.json', 'story.json']
              for (const dir of Object.values(nextDirs)) {
                if (!/^[a-z0-9][a-z0-9_-]*$/.test(dir) || dir.length > 100 || reserved.includes(dir)) {
                  return fail(res, 400, `Tên thư mục không hợp lệ: ${dir}`)
                }
              }
              libraryConfig = {
                ...libraryConfig,
                ...(body as Partial<LibraryConfig>),
                dirs: nextDirs,
                autoExport: { ...libraryConfig.autoExport, ...((body.autoExport as LibraryConfig['autoExport'] | undefined) ?? {}) },
              }
              bumpMockVersion('library')
              emitGlobalEvent('library.changed', {})
              return json(res, 200, { ok: true, config: libraryConfig, version: mockVersion('library') })
            }
            // Chỉ thư viện mẫu (Truyen) có dự án; thư mục khác vừa chọn là trống.
            const hasProjects = mockLibraryConfigured() && mockInspect(mockLibraryRoot()).kind === 'found'
            return json(res, 200, {
              ...(mockLibraryConfigured() ? { version: mockVersion('library') } : {}),
              configured: mockLibraryConfigured(),
              rootDisplay: mockDisplay(mockLibraryRoot()),
              rootPath: admin ? mockLibraryRoot() : '',
              candidates: admin && hasProjects ? candidates : [],
              config: libraryConfig,
              projects: hasProjects ? projectList().filter((pr) => canRead(user!, `${pr.id}/`)) : [],
            })
          }
          if (seg[0] === 'projects' && seg.length === 1) {
            if (req.method === 'POST') {
              if (user!.role !== 'admin') return fail(res, 403, 'Chỉ quản trị viên tạo dự án')
              const body = await readBody(req)
              const rawName = String(body.name ?? '').trim()
              let slug = String(body.slug ?? '').trim()
              if (!slug && !rawName) return fail(res, 400, 'Nhập tên dự án')
              if (rawName && [...projectsExtra.values()].some((existing) => existing.trim().toLowerCase() === rawName.toLowerCase())) {
                return fail(res, 409, 'Đã có dự án cùng tên — đặt tên khác nhé')
              }
              if (slug) {
                if (!/^[a-z0-9][a-z0-9_-]*$/.test(slug)) return fail(res, 400, 'Tên dự án không hợp lệ')
                if (projectsExtra.has(slug)) return fail(res, 409, 'Dự án đã tồn tại')
              } else {
                const base = slugify(rawName, 12) || 'du-an'
                slug = base
                for (let n = 2; projectsExtra.has(slug); n += 1) slug = `${base}-${n}`
              }
              const name = rawName || slug
              projectsExtra.set(slug, name)
              return json(res, 201, { id: slug, name, description: String(body.description ?? ''), legacy: false, storyCount: 0, runningCount: 0, updatedAt: iso(NOW), notes: String(body.notes ?? ''), createdAt: iso(NOW), path: slug })
            }
          }
          if (seg[0] === 'projects' && seg[1] === 'adopt' && req.method === 'POST') {
            if (user!.role !== 'admin') return fail(res, 403, 'Chỉ quản trị viên nhận dự án')
            const body = await readBody(req)
            const dir = String(body.dir ?? '')
            if (!candidates.includes(dir)) return fail(res, 400, 'Thư mục này không phải ứng viên dự án')
            projectsExtra.set(dir, String(body.name ?? dir))
            return json(res, 201, { id: dir, name: String(body.name ?? dir), description: '', legacy: false, storyCount: 0, runningCount: 0, updatedAt: iso(NOW), notes: '', createdAt: iso(NOW), path: dir })
          }
          if (seg[0] === 'projects' && seg[1]) {
            const p = seg[1]
            if (seg[2] === 'ledger' || seg[2] === 'universe' || seg[2] === 'search') {
              const viewable = (id: string) => canRead(user!, id)
              if (seg[2] === 'ledger' && seg[3] === undefined && req.method === 'GET') return json(res, 200, mockLedger(p, viewable))
              if (seg[2] === 'ledger' && req.method === 'POST' && (seg[3] === 'check' || seg[3] === 'suggest')) {
                if (!canWrite(user!, `${p}/*`)) return fail(res, 403, 'Bạn không có quyền sửa brief trong dự án này')
                const body = await readBody(req)
                const brief = String(body.brief ?? '').trim()
                if (!brief) return fail(res, 400, 'Yêu cầu trống')
                await sleep(1400)
                return json(res, 200, seg[3] === 'check' ? mockLedgerCheck(p, brief, viewable) : mockLedgerSuggest())
              }
              if (seg[2] === 'universe' && req.method === 'GET') {
                // §H câu 3: tên vũ trụ = universeName || tên dự án; dự án ẩn danh (không có tên) → "".
                const base = p === '_' ? '' : projectsExtra.get(p) ?? ''
                return json(res, 200, mockUniverse(p, viewable, projectMeta.get(p)?.universeName?.trim() || base))
              }
              if (seg[2] === 'search' && req.method === 'GET') {
                const q = (url.searchParams.get('q') ?? '').trim()
                if (!q || q.length > 200) return fail(res, 400, 'Nhập từ cần tìm, tối đa 200 ký tự')
                return json(res, 200, mockProjectSearch(p, q, viewable))
              }
              return fail(res, 405, 'Thao tác không được hỗ trợ')
            }
            const detail = () => {
              const meta = projectMeta.get(p)
              return {
                version: mockVersion(`project:${p}`),
                project: {
                  version: mockVersion(`project:${p}`),
                  id: p, name: projectsExtra.get(p) ?? p,
                  description: meta?.description ?? (p === 'dit-dang-audio' ? 'Truyện chuyển audio' : ''),
                  // §H câu 3: luôn có khoá, "" = chưa đặt tên vũ trụ riêng.
                  universeName: meta?.universeName ?? '',
                  legacy: p === '_',
                  storyCount: [...stories.values()].filter((s) => s.p === p).length,
                  runningCount: [...stories.values()].filter((s) => s.p === p && s.status === 'writing').length,
                  updatedAt: iso(NOW - 3600000), notes: meta?.notes ?? '', createdAt: iso(NOW - 90 * 86400000), path: p,
                },
                stories: [...stories.values()].filter((s) => s.p === p && canRead(user!, `${s.p}/${s.s}`)).map(storySummary),
                ideas: p === 'dit-dang-audio'
                  ? [
                      { id: '20251025-mot-truyen-moi', title: 'Một truyện mới', excerpt: 'Ý tưởng nhanh…', createdAt: iso(NOW - 86400000), updatedAt: iso(NOW - 86400000), usedBy: [] },
                      { id: '20251020-hieu-in-dem', title: 'Hiệu in đêm', excerpt: 'Một hiệu in nhỏ giữ những bản thảo bị xóa tên tác giả.', createdAt: iso(NOW - 5 * 86400000), updatedAt: iso(NOW - 5 * 86400000), usedBy: [] },
                    ]
                  : [],
                assets: p === 'dit-dang-audio'
                  ? [
                      { path: 'bia/tra-xanh.png', kind: 'file', size: 184320, updatedAt: iso(NOW - 9 * 86400000) },
                      { path: 'nhac/nen.mp3', kind: 'file', size: 2621440, updatedAt: iso(NOW - 4 * 86400000) },
                      { path: 'giong/doc-mau.wav', kind: 'file', size: 5242880, updatedAt: iso(NOW - 2 * 86400000) },
                    ]
                  : [],
              }
            }
            if (seg.length === 2) {
              if (req.method === 'PUT') {
                if (p === '_') return fail(res, 409, 'Dự án truyện cũ không sửa được')
                const body = await readBody(req)
                if (mockConflict(req, res, `project:${p}`, detail(), body)) return
                projectsExtra.set(p, String(body.name ?? projectsExtra.get(p) ?? p))
                projectMeta.set(p, {
                  description: String(body.description ?? projectMeta.get(p)?.description ?? ''),
                  notes: String(body.notes ?? projectMeta.get(p)?.notes ?? ''),
                  universeName: String(body.universeName ?? projectMeta.get(p)?.universeName ?? '').trim(),
                })
                bumpMockVersion(`project:${p}`)
                emitGlobalEvent('project.changed', { project: p })
                return json(res, 200, detail().project)
              }
              return json(res, 200, detail())
            }
            /** Tạo truyện mới trong dự án — dùng chung cho POST /stories và /cocreate/commit. */
            const addStory = (name: string, idea = ''): MockStory => {
              const idx = [...stories.values()].filter((item) => item.p === p).length + 1
              const slug = name ? (slugify(name, libraryConfig.slugWords) || TEMP_SLUG) : TEMP_SLUG
              const sid = `${String(idx).padStart(2, '0')}-${slug}`
              const st = mkStory(p, sid, `kich-ban/${sid}`, name, 'idea', { idea, total: 0 })
              st.snapshot = { ...st.snapshot, isRunning: false, runtimeState: 'idle', statusLabel: 'Sẵn sàng', currentChapter: 1, totalChapters: 0, completedCount: 0, totalWordCount: 0, phase: 'premise' }
              put(st)
              return st
            }

            // G8 — đồng sáng tác cấp dự án: chưa tạo truyện cho tới /commit; nháp nằm ở .ainovel/drafts/<id>.
            if (seg[2] === 'cocreate') {
              if (!canWrite(user!, `${p}/*`)) return fail(res, 403, 'Bạn không có quyền tạo truyện trong dự án này')
              const draftId = url.searchParams.get('draft') ?? ''
              const key = draftId ? `${p}/${draftId}` : ''
              if (seg[3] === undefined && req.method === 'GET') {
                const state = key ? projectDrafts.get(key) : null
                if (key && !state) return fail(res, 404, 'Không tìm thấy bản nháp đồng sáng tác')
                return json(res, 200, {
                  draftId: key ? draftId : '',
                  schemaVersion: 1, proposal: null, committed: false, nextChapter: null,
                  ...(state ?? { active: false, messages: [], draft: '', ready: false, suggestions: [] }),
                })
              }
              if (seg[3] === 'message' && req.method === 'POST') {
                const body = await readBody(req)
                const msg = String(body.message ?? '').trim()
                if (!msg) return fail(res, 400, 'Tin nhắn trống')
                const id = String(body.draft ?? draftId)
                if (id && !projectDrafts.has(`${p}/${id}`)) return fail(res, 404, 'Không tìm thấy bản nháp đồng sáng tác')
                const nextId = id || `nhap-${++draftSeq}`
                const state: CoCreateState = projectDrafts.get(`${p}/${nextId}`) ?? { active: true, messages: [], draft: '', ready: false, suggestions: [] }
                state.active = true
                state.messages.push({ role: 'user', content: msg })
                const turns = state.messages.filter((m) => m.role === 'user').length
                const ledgerExample = state.messages.some(message => message.role === 'user' && message.content.includes('thợ may'))
                const reply = ledgerExample
                  ? turns < 2
                    ? 'Mình hiểu là truyện gói Nữ tần báo thù, 8 chương. Bằng chứng để lật mẹ kế là gì: giấy tờ tiệm, sổ sách, hay một món đồ của mẹ? Và kết: cô rời đi hay giữ lại tiệm?'
                    : 'Kiến trúc sư đã tổng hợp yêu cầu. Bấm Bắt đầu sáng tác để so với các truyện trước.'
                  : turns < 2
                    ? 'Đã rõ phần lớn ý tưởng. Bạn muốn nhấn mạnh vào xung đột nào nhất — cá nhân trong phòng thu hay phe đấu toàn nhà xuất bản?'
                    : 'Đủ rõ rồi. Tôi tổng hợp thành: "Truyện đô thị trinh thám hồi hộp 12 chương, nhân vật chính nữ giám định pháp y, mạch kép: hồ sơ vụ án + đấu trí nội bộ." Bấm Ctrl+S để bắt đầu viết.'
                state.messages.push({ role: 'assistant', content: reply })
                if (turns >= 2) {
                  state.ready = true
                  state.draft = ledgerExample ? MOCK_NEW_STORY_BRIEF : 'Truyện đô thị trinh thám hồi hộp 12 chương — nhân vật chính nữ giám định pháp y Kiều My; mạch kép hồ sơ vụ án + đấu trí nội bộ; phong cách sắc, không nước mắt.'
                  state.suggestions = ['Thêm nhân vật phản diện trong phòng pháp lý', 'Đặt hạn chót 30 ngày cho giải quyết vụ án', 'Viết theo góc nhìn thứ nhất của từng nhân vật luân phiên']
                }
                state.schemaVersion = 1
                state.proposal = state.ready ? mockCoCreateProposal((state.proposal?.revision ?? 0) + 1, 1, 0) : null
                state.committed = false
                state.nextChapter = state.proposal?.chapters[0]?.chapter ?? null
                projectDrafts.set(`${p}/${nextId}`, state)
                return json(res, 200, { draftId: nextId, reply, draft: state.draft, ready: state.ready, suggestions: state.suggestions, schemaVersion: 1, proposal: state.proposal, committed: false, nextChapter: state.nextChapter })
              }
              if (seg[3] === 'commit' && req.method === 'POST') {
                const body = await readBody(req)
                const id = String(body.draft ?? draftId)
                const state = id ? projectDrafts.get(`${p}/${id}`) : null
                if (!state) return fail(res, 409, 'Không có phiên đồng sáng tác để bắt đầu')
                const brief = typeof body.brief === 'string' ? body.brief : state.draft
                const check = mockLedgerCheck(p, brief, id => canRead(user!, id))
                if (check.checks.some(item => item.level === 'error')) return json(res, 409, { error: 'Còn mục Bắt sửa khi so với truyện trước', checks: check.checks })
                state.draft = brief
                const name = String(body.name ?? '').trim()
                const st = addStory(name, state.draft)
                st.status = 'writing'
                st.snapshot = { ...st.snapshot, isRunning: true, runtimeState: 'running', premise: state.draft, statusLabel: 'Đang viết chương 1', currentChapter: 1, totalChapters: 14, phase: 'writing' }
                put(st)
                broadcast(st, 'snapshot', st.snapshot)
                ensureStream(st)
                projectDrafts.delete(`${p}/${id}`)
                return json(res, 201, { story: storySummary(st), schemaVersion: 1, proposal: state.proposal ?? null, committed: true, nextChapter: state.nextChapter ?? null })
              }
              if (seg[3] === 'cancel' && req.method === 'POST') {
                const body = await readBody(req)
                const id = String(body.draft ?? draftId)
                if (id) projectDrafts.delete(`${p}/${id}`)
                return json(res, 200, { ok: true })
              }
              return fail(res, 404, 'Not found')
            }

            if (seg[2] === 'ideas') {
              if (seg[3]) return json(res, 200, { id: seg[3], title: 'Một truyện mới', text: 'Ý tưởng nhanh viết gọn…', excerpt: 'Ý tưởng nhanh…', createdAt: iso(NOW - 86400000), updatedAt: iso(NOW - 86400000), usedBy: [] })
              if (req.method === 'POST') {
                const body = await readBody(req)
                return json(res, 201, { id: `idea-${++seq}`, title: String(body.title ?? String(body.text ?? '').slice(0, 30)), excerpt: String(body.text ?? '').slice(0, 120), createdAt: iso(NOW), updatedAt: iso(NOW), usedBy: [] })
              }
              return json(res, 200, { ideas: detail().ideas })
            }
            if (seg[2] === 'assets') return json(res, 200, { assets: detail().assets })
            if (seg[2] === 'validate') {
              if (seg[3] === 'fix') {
                if (req.method !== 'POST') return fail(res, 405, 'Method not allowed')
                if (user!.role !== 'admin') return fail(res, 403, 'Chỉ quản trị viên sửa cấu trúc')
                await readBody(req)
              }
              const node = (name: string, path: string, kind: 'dir' | 'file', role: string, status: string, children: unknown[] = []) => ({ name, path, kind, role, status, children })
              const storyNodes = [...stories.values()].filter((st) => st.p === p).map((st) => node(st.s, st.folder, 'dir', 'story', 'ok', [
                node('story.json', `${st.folder}/story.json`, 'file', 'storyMeta', 'ok'),
                node('output', `${st.folder}/output/novel`, 'dir', 'engine', 'engine', [
                  node('premise.md', `${st.folder}/output/novel/premise.md`, 'file', 'engine', 'ok'),
                  node('outline.json', `${st.folder}/output/novel/outline.json`, 'file', 'engine', 'ok'),
                  node('characters.json', `${st.folder}/output/novel/characters.json`, 'file', 'engine', 'ok'),
                  node('foreshadow_ledger.json', `${st.folder}/output/novel/foreshadow_ledger.json`, 'file', 'engine', 'ok'),
                  node('chapters', `${st.folder}/output/novel/chapters`, 'dir', 'engine', 'engine'),
                ]),
              ]))
              const tree = node(p, p, 'dir', 'project', 'ok', [
                node('project.json', `${p}/project.json`, 'file', 'project', 'ok'),
                node('.ainovel', `${p}/.ainovel`, 'dir', 'config', 'ok', [
                  node('config.json', `${p}/.ainovel/config.json`, 'file', 'config', 'ok'),
                  node('rules', `${p}/.ainovel/rules`, 'dir', 'rules', 'ok', [
                    node('van-phong.md', `${p}/.ainovel/rules/van-phong.md`, 'file', 'rules', 'ok'),
                  ]),
                ]),
                node('y-tuong', `${p}/y-tuong`, 'dir', 'ideas', 'ok'),
                node('tai-san', `${p}/tai-san`, 'dir', 'assets', 'ok'),
                node('kich_ban', `${p}/kich_ban`, 'dir', 'stories', 'ok', storyNodes),
                node('output', `${p}/output`, 'dir', 'engine', 'engine'),
                node('xuat-ban', `${p}/xuat-ban`, 'dir', 'exports', 'ok', [
                  node('tap-01', `${p}/xuat-ban/tap-01`, 'dir', 'exports', 'ok', [
                    node('tra-xanh.tap-01.txt', `${p}/xuat-ban/tap-01/tra-xanh.tap-01.txt`, 'file', 'exports', 'ok'),
                    node('tra-xanh.tap-01.epub', `${p}/xuat-ban/tap-01/tra-xanh.tap-01.epub`, 'file', 'exports', 'missing'),
                  ]),
                ]),
              ])
              return json(res, 200, { ok: true, issues: p === '_' ? [{ path: 'nguoc-dong', level: 'info', code: 'legacy', message: 'Truyện cũ chưa nằm trong dự án', fixable: false }] : [], tree })
            }
            if (seg[2] === 'voices') {
              if (req.method === 'POST') {
                if (!canWrite(user!, `${p}/`)) return fail(res, 403, 'Bạn chỉ có quyền Chỉ xem ở dự án này')
                try {
                  const voice = await mockUploadVoice(p, await readRaw(req), req.headers['content-type'] ?? '')
                  emitGlobalEvent('story.changed', { project: p, parts: ['audio'] })
                  return json(res, 200, voice)
                } catch (error) { return fail(res, 400, error instanceof Error ? error.message : 'Không đọc được mẫu giọng.') }
              }
              if (seg[3] && seg[4] === 'sample') {
                const clip = mockVoiceSample(p, seg[3])
                if (!clip) return fail(res, 404, 'Không tìm thấy mẫu giọng.')
                res.writeHead(200, { 'content-type': 'audio/wav', 'content-length': clip.length })
                return res.end(clip)
              }
              return json(res, 200, mockProjectVoices(p))
            }
            if (seg[2] === 'pronunciations') {
              const versionKey = `pronunciations:${p}`
              const current = projectPronunciations.get(p) ?? DEFAULT_PRONUNCIATIONS
              if (req.method === 'PUT') {
                if (!canWrite(user!, `${p}/`)) return fail(res, 403, 'Bạn chỉ có quyền Chỉ xem ở dự án này')
                if (mockConflict(req, res, versionKey, current)) return
                const body = await readBody(req)
                const entries = Array.isArray(body) ? body : (Array.isArray(body.entries) ? body.entries : [])
                projectPronunciations.set(p, entries as { written: string; spoken: string }[])
                const version = bumpMockVersion(versionKey)
                res.setHeader('ETag', version)
                for (const story of stories.values()) {
                  if (story.p !== p) continue
                  const change = { project: p, story: story.s, parts: ['audio'] }
                  broadcast(story, 'story.changed', change)
                  emitGlobalEvent('story.changed', change)
                }
                return json(res, 200, { ok: true, version })
              }
              res.setHeader('ETag', mockVersion(versionKey))
              return json(res, 200, projectPronunciations.get(p) ?? DEFAULT_PRONUNCIATIONS)
            }
            if (seg[2] === 'stories' && seg[3] === undefined) {
              if (req.method === 'POST') {
                if (!canWrite(user!, `${p}/*`)) return fail(res, 403, 'Bạn không có quyền tạo truyện trong dự án này')
                const body = await readBody(req)
                const st = addStory(String(body.name ?? '').trim(), String(body.idea ?? ''))
                return json(res, 201, storySummary(st))
              }
              return json(res, 200, { stories: [...stories.values()].filter((s) => s.p === p && canRead(user!, `${s.p}/${s.s}`)).map(storySummary) })
            }

            // ─── story routes ───
            const s = seg[3]
            const st = s ? findStory(p, s) : null
            if (!st) return fail(res, 404, 'Không tìm thấy truyện')
            if (seg.length === 4 && req.method === 'DELETE') return deleteMockStory(req, res, user!, st)
            if (!canRead(user!, `${p}/${s}`)) return fail(res, 404, 'Không tìm thấy truyện')
            const rest = seg.slice(4)

            const writeLive = (fn: () => unknown | Promise<unknown>) => {
              if (!canWrite(user!, `${p}/${s}`)) return fail(res, 404, 'Không tìm thấy truyện')
              if (st.locked) return fail(res, 409, 'truyện đang được mở bởi tiến trình khác')
              return fn()
            }

            // ─── audio (GAP-17…20) — chạy ở gateway, không phụ thuộc worker viết truyện ───
            if (rest[0] === 'audio') {
              return handleAudioApi({
                req, res, p, s, rest: rest.slice(1),
                state: audioStateOf(p, s, (event, data) => broadcast(st, event, data), {
                  title: st.title,
                  shortTitle: st.shortTitle,
                  volume: Number(['volumes', 'exports'].includes(rest[1]) ? rest[2] : url.searchParams.get('volume')) || 1,
                  totalChapters: st.totalChapters,
                  chapterTitles: Object.fromEntries(Object.entries(st.chapters).map(([chapter, value]) => [chapter, value.title])),
                }),
                canWrite: canWrite(user!, `${p}/${s}`),
                readBody: () => readBody(req),
              })
            }

            // ─── quy tắc viết (GAP-32) — chạy ở gateway, `quy-tac.md` dùng chung ruleStore ───
            if (rest[0] === 'rules') {
              return handleRulesApi({
                req, res, p, s, rest: rest.slice(1), store: ruleStore,
                story: { completed: Object.keys(st.chapters).length, running: !!st.snapshot.isRunning },
                canWrite: canWrite(user!, `${p}/${s}`),
                readBody: () => readBody(req),
                emit: () => { const change = { project: p, story: s, parts: ['rules'] }; broadcast(st, 'story.changed', change); emitGlobalEvent('story.changed', change) },
              })
            }

            if (rest[0] === 'style-samples') {
              if (!canWrite(user!, `${p}/${s}`)) return fail(res, 404, 'Không tìm thấy truyện')
              const key = `${p}/${s}`
              const list = styleSamples.get(key) ?? []
              const total = () => ({ count: list.length, chars: list.reduce((sum, sample) => sum + sample.chars, 0) })
              if (req.method === 'GET') return json(res, 200, { samples: list, total: total() })
              if (req.method === 'POST') {
                const body = await readBody(req)
                const text = String(body.text ?? '').trim()
                if (!text) return fail(res, 400, 'Đoạn văn mẫu đang trống')
                if (list.length >= 20) return fail(res, 409, 'Đã đủ 20 văn mẫu, xoá bớt trước khi thêm')
                const title = String(body.title ?? '').trim() || `Văn mẫu ${list.length + 1}`
                const sample = { id: `${title}.txt`, title, chars: text.split(/\s+/).filter(Boolean).length, addedAt: iso(NOW) }
                styleSamples.set(key, [...list, sample])
                emitGlobalEvent('story.changed', { project: p, story: s, parts: ['style'] })
                return json(res, 201, sample)
              }
              if (req.method === 'DELETE') {
                const id = decodeURIComponent(rest.slice(1).join('/'))
                const next = list.filter((sample) => sample.id !== id)
                if (next.length === list.length) return fail(res, 404, 'Không tìm thấy văn mẫu')
                styleSamples.set(key, next)
                emitGlobalEvent('story.changed', { project: p, story: s, parts: ['style'] })
                res.writeHead(204)
                return res.end()
              }
              return fail(res, 404, 'Not found')
            }
            if (rest[0] === 'meta') {
              // D1 — “Viết bằng phiên bản”: các khoảng chương cùng một bản (gộp liền nhau), đúng thứ tự.
              const builds = () => {
                const written = Object.keys(st.chapters).map(Number).sort((a, b) => a - b)
                const ranges: { from: number; to: number; version: string; commit: string; at: string }[] = []
                written.forEach((n) => {
                  const version = n <= 5 ? '0.9.2' : '0.9.3'
                  const commit = 'a41c9e2'
                  const last = ranges[ranges.length - 1]
                  if (last && last.version === version && last.to === n - 1) last.to = n
                  else ranges.push({ from: n, to: n, version, commit, at: iso(NOW - (40 - n) * 86400000) })
                })
                return ranges
              }
              const meta = () => ({ version: mockVersion(`story:${p}/${s}`), title: st.title, status: st.status, idea: st.idea, tags: st.tags, shortTitle: st.shortTitle, publishedAt: st.status === 'published' ? iso(NOW) : null, createdAt: iso(NOW - 40 * 86400000), builds: builds() })
              if (req.method === 'PUT') {
                if (!canWrite(user!, `${p}/${s}`)) return fail(res, 404, 'Không tìm thấy truyện')
                const body = await readBody(req)
                if (mockConflict(req, res, `story:${p}/${s}`, meta(), body)) return
                st.title = String(body.title ?? st.title)
                st.status = (body.status as MockStory['status'] | undefined) ?? st.status
                st.idea = String(body.idea ?? st.idea)
                // §AF — shortTitle ≤ 32 ký tự; rỗng = xoá (về suy ra từ title).
                if (body.shortTitle !== undefined && String(body.shortTitle).trim().length > 32) return fail(res, 400, 'Tên ngắn tối đa 32 ký tự')
                if (body.shortTitle !== undefined) { const next = String(body.shortTitle).trim(); if (next) st.shortTitle = next; else delete st.shortTitle }
                st.updatedAt = iso(NOW)
                bumpMockVersion(`story:${p}/${s}`)
                emitGlobalEvent('story.changed', { project: p, story: s, parts: ['meta'] })
              }
              return json(res, 200, meta())
            }
            if (rest[0] === 'cover' && req.method === 'GET') {
              // §AF — bìa đã tải (mô phỏng: dự án audio có tai-san/bia) hoặc bìa tự tạo.
              const size = url.searchParams.get('size')
              const n = size === null ? 512 : Number(size)
              if (!Number.isInteger(n) || n < 96 || n > 1400) return fail(res, 400, 'Cỡ bìa không hợp lệ')
              const kind = st.p === 'dit-dang-audio' ? 'up' : 'gen'
              const png = mockCoverPng(n, kind)
              res.writeHead(200, { 'content-type': 'image/png', 'cache-control': `max-age=${kind === 'up' ? 3600 : 86400}, immutable`, 'content-length': png.length, 'x-mock-cover': kind })
              return res.end(png)
            }
            if (rest[0] === 'rename' && req.method === 'POST') {
              if (!canWrite(user!, `${p}/${s}`)) return fail(res, 404, 'Không tìm thấy truyện')
              if (st.locked) return fail(res, 409, 'Truyện đang được mở bởi tiến trình khác')
              if (st.snapshot.isRunning) return fail(res, 409, 'Truyện đang viết, không đổi tên được')
              const body = await readBody(req)
              const slug = String(body.slug ?? '')
              if (!/^[a-z0-9][a-z0-9_-]*$/.test(slug)) return fail(res, 400, 'Tên thư mục truyện không hợp lệ')
              const prefix = /^(\d+)-/.exec(st.s)?.[1]
              const next = prefix ? `${prefix}-${slug}` : slug
              const oldId = `${st.p}/${st.s}`
              stories.delete(oldId)
              st.s = next
              st.folder = st.folder.replace(/[^/]+$/, next)
              put(st)
              return json(res, 200, storySummary(st))
            }
            if (rest[0] === 'close' && req.method === 'POST') {
              if (!canWrite(user!, `${p}/${s}`)) return fail(res, 404, 'Không tìm thấy truyện')
              st.status = 'paused'
              return json(res, 200, { ok: true })
            }

            if (rest[0] === 'chapters' && rest[2] === 'comments') {
              const chapter = Number(rest[1])
              if (!Number.isInteger(chapter) || chapter < 1) return fail(res, 400, 'Số chương không hợp lệ')
              const key = `${p}/${s}/${chapter}`
              const list = paragraphFeedback.get(key) ?? []
              if (req.method === 'GET') return json(res, 200, { comments: list })
              if (!canWrite(user!, `${p}/${s}`)) return fail(res, 404, 'Không tìm thấy truyện')
              if (req.method === 'POST') {
                const input = await readBody(req)
                const paragraph = input.paragraph === null || input.paragraph === undefined ? null : Number(input.paragraph)
                const quote = String(input.quote ?? '').trim()
                const text = String(input.text ?? '').trim()
                const chips = Array.isArray(input.chips) ? input.chips.map(String) : []
                if ((paragraph !== null && (!Number.isInteger(paragraph) || paragraph < 0 || !quote)) ||
                    (!chips.length && !text) || chips.some((chip) => !(PARAGRAPH_COMMENT_CHIPS as readonly string[]).includes(chip))) {
                  return fail(res, 400, 'Góp ý không hợp lệ')
                }
                const comment: ParagraphComment = { id: `gop-y-${++seq}`, paragraph, quote, chips, text, author: user!.username, createdAt: new Date().toISOString() }
                paragraphFeedback.set(key, [...list, comment])
                broadcast(st, 'story.changed', { project: p, story: s, parts: ['comments'] })
                return json(res, 201, comment)
              }
              if (req.method === 'DELETE') {
                const id = url.searchParams.get('id') || String((await readBody(req)).id ?? '')
                if (!list.some((comment) => comment.id === id)) return fail(res, 404, 'Không tìm thấy góp ý')
                paragraphFeedback.set(key, list.filter((comment) => comment.id !== id))
                broadcast(st, 'story.changed', { project: p, story: s, parts: ['comments'] })
                return json(res, 200, { ok: true })
              }
              return fail(res, 405, 'Phương thức không được hỗ trợ')
            }

            // §AI — PATCH hành động trên góp ý: resolve / reopen / relink / resend_whole.
            if (rest[0] === 'chapters' && rest[3] === 'comments' && req.method === 'PATCH') {
              const chapter = Number(rest[1])
              if (!Number.isInteger(chapter) || chapter < 1) return fail(res, 400, 'Số chương không hợp lệ')
              if (!canWrite(user!, `${p}/${s}`)) return fail(res, 404, 'Không tìm thấy truyện')
              const key = `${p}/${s}/${chapter}`
              const list = paragraphFeedback.get(key) ?? []
              const comment = list.find((item) => item.id === decodeURIComponent(rest[2] ?? ''))
              if (!comment) return fail(res, 404, 'Không tìm thấy góp ý')
              const body = await readBody(req)
              const now = new Date().toISOString()
              if (body.action === 'resolve') { comment.status = 'done'; comment.resolvedAt = now }
              else if (body.action === 'reopen') { comment.status = 'pending'; delete comment.resolvedAt; delete comment.fixedAt; delete comment.fixedBy }
              else if (body.action === 'resend_whole') { comment.status = 'pending'; comment.whole = true; delete comment.sentAt; delete comment.fixedAt; delete comment.resolvedAt }
              else if (body.action === 'relink') {
                const para = Number(body.paragraph)
                if (!Number.isInteger(para) || para < 0) return fail(res, 400, 'Đoạn gắn lại không hợp lệ')
                comment.status = 'pending'; comment.relinked = true; comment.moved = false; comment.whole = false
                comment.anchor = { paragraph: para, quote: body.quote ? String(body.quote) : '' }
                comment.candidates = []
              } else return fail(res, 400, 'Hành động không hợp lệ')
              paragraphFeedback.set(key, [...list])
              broadcast(st, 'story.changed', { project: p, story: s, parts: ['comments'] })
              return json(res, 200, comment)
            }

            // đọc trực tiếp
            if (rest[0] === 'chapters' && rest[1] && rest[2] === 'plan') {
              const n = Number(rest[1]); const plan: ChapterPlan | undefined = plans[n]
              return json(res, plan ? 200 : 404, plan ?? { error: 'Chương chưa có plan' })
            }
            if (rest[0] === 'chapters' && rest[1] && rest[2] === 'facts') {
              const n = Number(rest[1]); const f: ChapterFacts | undefined = facts[n]
              return json(res, f ? 200 : 404, f ?? { error: 'Chưa có facts' })
            }
            if (rest[0] === 'chapters' && rest[1] && rest[2] === 'revisions') {
              // Chỉ chương 5 có bản sửa trong fixture; chương khác trả rỗng
              if (Number(rest[1]) !== 5) return json(res, 200, { revisions: [] })
              return json(res, 200, {
                revisions: [{
                  time: iso(NOW - 86400000), kind: 'edit_chapter',
                  old: 'Mười hai chương, mười hai lần ai đó tin vào cái bẫy khác màu.',
                  new: 'Mười một chương, mười một lần ai đó tin vào cái bẫy khác màu.',
                  replaceAll: false,
                }],
              })
            }
            if (rest[0] === 'chapters') {
              if (rest[1] && rest[2] === undefined) {
                const n = Number(rest[1])
                const ch = st.chapters[n]
                const source = url.searchParams.get('source') ?? 'committed'
                const committed = ch?.text ?? ''
                // Chương đang viết: bản nháp khác bản nộp (nút Bản nộp / Bản nháp)
                const drafting = source === 'draft' && n === st.snapshot.currentChapter && (st.snapshot.isRunning || st.snapshot.waitingFor === 'gate')
                const text = drafting
                  ? `${committed || TRAXANH_CHAPTERS[n]?.text || ''}\n\n[Bản nháp] Kiều My dừng ở câu cuối, gạch chân "mười hai" rồi sửa thành "mười một" — chưa nộp.`
                  : committed
                return json(res, 200, { chapter: n, title: sampleChapterTitle(s, n) ?? ch?.title ?? CHAPTER_TITLES[n] ?? `Chương ${n}`, text: sampleProse(s, text), source, words: text ? text.split(/\s+/).length : 0 })
              }
              const infos = Array.from({ length: st.totalChapters }, (_, i) => {
                const n = i + 1
                // §AI — badge hàng chương (03b): need = lost+unsure, pending = chờ gửi.
                const fb = (paragraphFeedback.get(`${p}/${s}/${n}`) ?? []).reduce((acc, c) => {
                  const status = c.status ?? (c.appliedAt ? 'sent' : 'pending')
                  if (status === 'lost' || status === 'unsure') acc.need++
                  else if (status === 'pending') acc.pending++
                  return acc
                }, { need: 0, pending: 0 })
                return { chapter: n, title: sampleChapterTitle(s, n) ?? st.chapters[n]?.title ?? CHAPTER_TITLES[n] ?? `Chương ${n}`, completed: n <= st.completed, words: st.chapters[n] ? st.chapters[n].text.split(/\s+/).length : 0, hasDraft: n === st.snapshot.currentChapter && (st.snapshot.isRunning || st.snapshot.waitingFor === 'gate'), volume: Math.ceil(n / 14) || 1, arc: n <= 4 ? 1 : n <= 10 ? 2 : 3, comments: fb.need || fb.pending ? fb : null }
              })
              if (s === 'm2-reader' || s === 'm2-reader-locked') for (const info of infos) info.volume = 1
              // G10 — revision thô đổi khi chương/bản nháp/trạng thái viết đổi (mock: dấu vân tay trạng thái).
              const revision = `${st.completed}:${st.totalChapters}:${st.snapshot.currentChapter}:${st.snapshot.isRunning ? 'run' : 'idle'}:${Object.values(st.chapters).reduce((sum, ch) => sum + ch.text.length, 0)}`
              return json(res, 200, { chapters: infos, revision })
            }
            if (rest[0] === 'outline') {
              const allChapters = Array.from({ length: st.totalChapters }, (_, i) => {
                const n = i + 1
                return { chapter: n, title: sampleChapterTitle(s, n) ?? st.chapters[n]?.title ?? CHAPTER_TITLES[n] ?? `Chương ${n}`, coreEvent: n <= st.completed ? 'Đã xong' : 'Đang viết', summary: '', completed: n <= st.completed, words: st.chapters[n] ? st.chapters[n].text.split(/\s+/).length : 0, status: n <= st.completed ? 'done' : n === st.snapshot.currentChapter && st.snapshot.isRunning ? 'writing' : 'pending', hasPlan: Boolean(plans[n]), hasReview: Boolean(REVIEWS[`${p}/${s}`]?.[n]) }
              }) as Outline['volumes'][0]['chapters']
              // Gói AG — tập 40 chương khớp storySummary.volumes; ?volume=N chỉ kèm chương tập N (0 = chỉ header).
              const volumeFilter = url.searchParams.has('volume') ? Number(url.searchParams.get('volume')) : -1
              const volumeCount = Math.max(1, Math.ceil(st.totalChapters / 40))
              const ARC_TITLES = ['Hồi 1 · Ký tên giữa phòng thu', 'Hồi 2 · Phe bảo vệ lớn lên', 'Hồi 3 · Tuyên bố']
              const volumes: Outline['volumes'] = Array.from({ length: volumeCount }, (_, v) => {
                const index = v + 1
                const list = allChapters.filter((c) => Math.ceil(c.chapter / 40) === index)
                const want = volumeFilter < 0 || volumeFilter === index
                return {
                  index, title: `Tập ${index}`, summary: index === 1 ? 'Cướp kịch bản đầu tay' : '',
                  arcs: [
                    { index: 1, title: ARC_TITLES[0], summary: 'Kiều My ký hợp đồng và gặp phe bảo vệ', chapters: list.filter((c, i) => i < 14).map((c) => c.chapter) },
                    { index: 2, title: ARC_TITLES[1], summary: 'Trần Nga ẩn danh, phe chống cướp hình thành', chapters: list.filter((c, i) => i >= 14 && i < 28).map((c) => c.chapter) },
                    { index: 3, title: ARC_TITLES[2], summary: 'Đối đầu công khai tại cuộc họp cấp trên', chapters: list.filter((c, i) => i >= 28).map((c) => c.chapter) },
                  ].filter((arc) => arc.chapters.length > 0),
                  chapters: want ? list : [],
                }
              })
              // Truyện >200: thêm tập khung trống (locked) để test “mở khi Tập N xong”.
              if (st.totalChapters > 200) volumes.push({ index: volumeCount + 1, title: `Tập ${volumeCount + 1}`, summary: '', arcs: [], chapters: [] })
              return json(res, 200, { layered: true, path: 'outline/00-ROOT.yml', volumes, nextVolumeTitle: `Tập ${volumeCount + 1} · Tuyên bố của người vắng mặt`, compassDirection: 'đoạt lại tác quyền', compassScale: 'mạch đô thị' } as Outline)
            }
            if (rest[0] === 'characters') {
              const sample = sampleCharacters(s)
              if (sample) return handleCharacters(req, res, `${p}/${s}`, sample)
              return handleCharacters(req, res, `${p}/${s}`, {
                characters: [
                  { name: 'Kiều My', aliases: ['My'], role: 'main', description: 'Tác giả mới — ký sai hợp đồng rồi đoạt lại tác quyền bằng chữ', arc: 'đơn độc → lãnh đạo phe', traits: ['tỉ mỉ', 'cứng đầu', 'nhạy bén'], tier: 'main' },
                  { name: 'Trần Nga', role: 'main', description: 'Chủ phòng quy hoạch — người giữ bản thảo tay và chìa khóa của mọi cuộc đấu', arc: 'ẩn danh → đồng tác giả', traits: ['kín đáo', 'sắc sảo'], tier: 'main' },
                  { name: 'Nhân Phát', role: 'main', description: 'Biên tập viên — người đánh máy bản thảo đầu tiên và tin vào Kiều My', arc: 'quan sát → hành động', traits: ['cẩn thận', 'trung thực'], tier: 'main' },
                  { name: 'Quách', role: 'antagonist', description: 'Phụ trách tác quyền — chủ của những bản ký kết mờ ám', arc: 'lạc quan → sứt mẻ', traits: ['mịn', 'thực dụng'], tier: 'main' },
                  { name: 'Luật sư Hàn', role: 'villain', description: 'Cố vấn pháp lý của nhà xuất bản — người soạn bản ký kết mờ ám', arc: '', traits: ['lạnh'], tier: 'important' },
                  { name: 'Cô lao công Lụa', role: 'minor', description: 'Người lao công của toà nhà — chứng kiến mọi cuộc ra vào', arc: '', traits: ['chăm chỉ'], tier: 'secondary' },
                  { name: 'Bà Tư', ini: 'TƯ', role: 'minor', description: 'Chủ nhà trọ nơi Kiều My ở — giữ bản thảo giấu kín', arc: '', traits: ['tử tế'], tier: 'secondary' },
                ],
                supporting: [
                  { name: 'bà Hiền', brief_role: 'chủ trà cũ', first_seen_chapter: 1, last_seen_chapter: 5, appearance_count: 4, appearance_chapters: [1, 3, 4, 5] },
                  { name: 'biên tập viên Minh', brief_role: 'bạn đồng nghiệp của Nhân Phát', first_seen_chapter: 3, last_seen_chapter: 6, appearance_count: 2, appearance_chapters: [3, 6] },
                  { name: 'giám đốc Hùng', brief_role: 'đứng đầu nhà xuất bản', first_seen_chapter: 1, last_seen_chapter: 7, appearance_count: 3, appearance_chapters: [1, 4, 7] },
                ],
              })
            }
            // GAP-38 — xưng hô & quan hệ (D7/D8); 423 khi trợ lý đang viết, 409 khi chạm chương đã viết.
            if (rest[0] === 'address-terms' || (rest[0] === 'relations' && req.method === 'POST')) {
              return handleAddressApi({
                req, res, p, s, rest,
                state: addressStateOf(`${p}/${s}`, MOCK_CAST.map((item) => item.name), sampleAddress(s)),
                latestWritten: st.completed,
                writing: st.snapshot.isRunning
                  ? { chapter: st.snapshot.currentChapter || st.snapshot.inProgressChapter, role: 'writer', state: st.snapshot.flow === 'rewriting' ? 'rewriting' as const : 'writing' as const }
                  : null,
                canWrite: canWrite(user!, `${p}/${s}`),
                readBody: () => readBody(req),
                changed: () => {
                  const change = { project: p, story: s, parts: ['address'] }
                  broadcast(st, 'story.changed', change)
                  emitGlobalEvent('story.changed', change)
                },
              })
            }
            // GAP-39 — Não truyện (D10): màn Nhân vật dùng `chapters` + `entities[].appears`.
            if (rest[0] === 'brain' && req.method === 'GET') {
              const upTo = url.searchParams.get('upTo')
              const limit = upTo === null ? null : Number.parseInt(upTo, 10)
              if (limit !== null && (!Number.isFinite(limit) || limit < 1)) return fail(res, 400, 'upTo phải là số chương dương')
              const brain = mockBrainDetailOf(s) ?? mockBrainOf({
                chapters: Array.from({ length: st.totalChapters }, (_, index) => ({
                  n: index + 1,
                  title: st.chapters[index + 1]?.title ?? CHAPTER_TITLES[index + 1] ?? `Chương ${index + 1}`,
                })),
                written: st.completed,
                writingChapter: st.snapshot.isRunning ? st.snapshot.currentChapter : null,
                // Câu 23: sửa lại → `rewriting`, viết lần đầu → `writing`.
                writingStatus: st.snapshot.flow === 'rewriting' ? 'rewriting' : 'writing',
                state: addressStateOf(`${p}/${s}`, MOCK_CAST.map((item) => item.name)),
              })
              return json(res, 200, limit ? trimBrain(brain, limit) : brain)
            }
            // D6 — Đo bằng máy: chín số đo của gói cho từng chương (máy chủ thật chỉ trả chương đã viết;
            // mock lấy các chương đã có bản nộp trong fixture).
            if (rest[0] === 'metrics' && req.method === 'GET') {
              const scope = url.searchParams.get('chapters') ?? 'last3'
              if (scope !== 'last3' && scope !== 'all') return fail(res, 400, 'chapters phải là last3 hoặc all')
              const written = Object.keys(st.chapters).map(Number).sort((a, b) => a - b)
              const picked = scope === 'last3' ? written.slice(-3) : written
              return json(res, 200, { chapters: picked.map((n) => ({ n, ...mockMetricsOf(n, written.length) })) })
            }
            // GAP-37 — Khung truyện (D5): gói thể loại điền sẵn, PUT cần quyền viết (403), 400 khi nhóm
            // ngoài 0..4 hoặc ô chữ nhiều dòng/quá 200 ký tự; lưu xong `source` thành "user".
            if (rest[0] === 'frame' && (req.method === 'GET' || req.method === 'PUT')) {
              const frameKey = `${p}/${s}`
              const stored = frameState.get(frameKey) ?? MOCK_FRAME
              if (req.method === 'PUT') {
                if (!canWrite(user!, frameKey)) return fail(res, 404, 'Không tìm thấy truyện')
                const body = await readBody(req)
                const next = { ...stored }
                if (body.audience !== undefined) {
                  // Câu 18: `audience` là mảng chỉ số trong `audiences[]` (gói C mặc định [2, 1]).
                  const audience = Array.isArray(body.audience) ? body.audience.map(Number) : null
                  if (audience === null) return fail(res, 400, 'Người nghe phải là mảng chỉ số nhóm')
                  if (stored.audiences.length === 0) return fail(res, 400, 'Gói thể loại chưa khai nhóm người nghe')
                  if (audience.some((index) => !Number.isInteger(index) || index < 0 || index >= stored.audiences.length)) {
                    return fail(res, 400, `Nhóm người nghe phải từ 0 đến ${stored.audiences.length - 1}`)
                  }
                  next.audience = [...new Set(audience)]
                }
                for (const [field, label] of [['difference', 'Điểm khác'], ['sequelSeed', 'Hạt giống phần hai']] as const) {
                  const value = body[field]
                  if (value === undefined) continue
                  const text = String(value).trim()
                  if (/[\r\n]/.test(text)) return fail(res, 400, `${label} phải là một dòng`)
                  if ([...text].length > 200) return fail(res, 400, `${label} tối đa 200 ký tự`)
                  next[field] = text
                }
                next.source = 'user'
                frameState.set(frameKey, next)
                const change = { project: p, story: s, parts: ['frame'] }
                broadcast(st, 'story.changed', change)
                return json(res, 200, next)
              }
              return json(res, 200, stored)
            }
            // GAP-37 — Bản đồ giữ chân (D5): trạng thái chương theo tiến độ thật của mock, còn
            // câu hỏi/lời hứa/móc và bốn dòng kiểm chép từ artboard (máy chủ thật tính ở retention_api.go).
            // `AINOVEL_MOCK_RETENTION=empty` = dàn ý cũ chưa có trường giữ chân (Câu 19).
            if (rest[0] === 'retention' && req.method === 'GET') {
              const chapters = MOCK_RETENTION_ROWS.map((row) => ({
                ...row,
                title: st.chapters[row.n]?.title ?? CHAPTER_TITLES[row.n] ?? `Chương ${row.n}`,
                status: row.n <= st.completed ? 'written' : row.n === st.snapshot.inProgressChapter ? 'writing' : 'unwritten',
              }))
              if (process.env.AINOVEL_MOCK_RETENTION === 'empty') {
                return json(res, 200, { empty: true, chapters, promiseAt: null, twistAt: null, checks: [] })
              }
              return json(res, 200, { empty: false, chapters, promiseAt: MOCK_RETENTION_AT.promiseAt, twistAt: MOCK_RETENTION_AT.twistAt, checks: MOCK_RETENTION_CHECKS })
            }
            if (rest[0] === 'foreshadow') return json(res, 200, { entries: foreshadow })
            // D13 · GAP-42 — Chất lượng truyện, cùng thuật toán với quality_api.go: mục lần chấm mới nhất = open
            // (giữ nguyên → kept), mục chỉ còn ở previous[] = fixed; trạng thái chương theo `rewrites` đã ghi (Câu 9).
            if (rest[0] === 'quality' && req.method === 'GET') {
              // Golden do Go test ghi (internal/entry/web TestMockGoldenQuality):
              // thứ tự mục, số ô và trạng thái chương đều là output handler thật.
              const key = `${p}/${s}`
              return json(res, 200, golden(QUALITY_GOLDEN[key] ?? 'quality-empty.json'))
            }
            if (rest[0] === 'reviews') {
              // D4: mỗi truyện có bộ thẩm duyệt riêng (GAP-36 items/round/pack/blocking).
              const key = `${p}/${s}`
              const reviews = REVIEWS[key] ?? {}
              const latest = withReviewDecisions(key, Object.entries(reviews).sort((a, b) => Number(b[0]) - Number(a[0]))[0]?.[1] ?? null)
              if (rest[1] === 'latest') {
                if (!latest) return fail(res, 404, 'Chưa có thẩm duyệt')
                return json(res, 200, latest)
              }
              const n = Number(rest[1])
              const chapter = withReviewDecisions(key, reviews[n] ?? null)
              return json(res, 200, { chapter, global: chapter ? null : latest ? { ...latest, scope: 'global', summary: 'Toàn cục ổn; các chương đã viết đều đạt.' } : null })
            }
            if (rest[0] === 'exports' && (rest[1] === 'file' || rest[1] === 'download') && req.method === 'GET') {
              const isDownload = rest[1] === 'download'
              const target = url.searchParams.get(isDownload ? 'file' : 'path') ?? ''
              if (target.startsWith('/') || /^[A-Za-z]:/.test(target) || target.split(/[\\/]/).includes('..')) return fail(res, 400, 'Đường dẫn không hợp lệ')
              const mobileFile = mobileChapterExports.get(`${p}/${s}`)?.get(target)
              if (mobileFile) {
                res.writeHead(200, { 'content-type': mobileFile.mime, 'content-length': mobileFile.body.length, 'content-disposition': contentDisposition(mobileFile.name) })
                return res.end(mobileFile.body)
              }
              const audioState = audioStateOf(p, s, (event, data) => broadcast(st, event, data), {
                title: st.title,
                shortTitle: st.shortTitle,
                volume: Number(target.match(/tap-(\d+)/)?.[1] ?? 1),
                totalChapters: st.totalChapters,
                chapterTitles: Object.fromEntries(Object.entries(st.chapters).map(([chapter, value]) => [chapter, value.title])),
              })
              if (target.endsWith('.zip') && audioState.exported?.zipUrl === target) {
                const body = storedZip(audioState.exported.files.filter((file) => !file.name.endsWith('.zip')).map((file) => [
                  file.name, file.name.endsWith('.srt') ? Buffer.from(audioExportText(audioState, file.path) ?? '') : wavFor(audioExportDuration(p, s, file.path), 4000),
                ]))
                res.writeHead(200, { 'content-type': 'application/zip', 'content-length': body.length, 'content-disposition': contentDisposition(target.split('/').pop() ?? target) })
                return res.end(body)
              }
              const text = audioExportText(audioState, target)
              if (text !== null) {
                res.writeHead(200, {
                  'content-type': target.endsWith('.srt') ? 'application/x-subrip; charset=utf-8' : 'text/plain; charset=utf-8',
                  'content-disposition': contentDisposition(target.split('/').pop() ?? target),
                })
                return res.end(text)
              }
              const registered = mockExportedFiles.has(`${p}/${s}/${target}`) || audioState.exported?.files.some((file) => file.path === target)
                || target === 'xuat-ban/tra-xanh-cuop-kich-ban-tap1.txt' || target === 'xuat-ban/tra-xanh-cuop-kich-ban-tap1.epub'
              if (!registered) return fail(res, 404, 'Không tìm thấy tệp xuất')
              if (target.endsWith('.m4b') || target.endsWith('.mp3') || target.endsWith('.wav')) {
                // Tệp audio đã ghép: phục vụ WAV thật (4 kHz mono) kèm Range để tua được.
                const audio = wavFor(audioExportDuration(p, s, target), 4000)
                const range = rangeOf(audio, req.headers.range)
                if (!range) { res.writeHead(416, { 'content-range': `bytes */${audio.length}` }); return res.end() }
                res.writeHead(req.headers.range ? 206 : 200, {
                  'content-type': 'audio/wav', 'accept-ranges': 'bytes',
                  'content-disposition': contentDisposition(target.split('/').pop() ?? target),
                  'content-length': range.body.length, 'cache-control': 'no-store',
                  ...(req.headers.range ? { 'content-range': `bytes ${range.start}-${range.end}/${range.total}` } : {}),
                })
                return res.end(range.body)
              }
              const { body, mime } = mockExportDownload(target, st.title, Object.entries(st.chapters).filter(([chapter]) => Number(chapter) <= st.completed).map(([, chapter]) => chapter))
              const dlName = isDownload ? friendlyExportName(st.title, inferExportVolume(target) ?? 1, exportExt(target)) : (target.split('/').pop() || `${st.s}.txt`)
              res.writeHead(200, { 'content-type': mime, 'content-length': body.length, 'content-disposition': contentDisposition(dlName) })
              return res.end(body)
            }
            if (rest[0] === 'exports' && req.method === 'POST') {
              const body = await readBody(req)
              if (s === 'm2-reader' || s === 'm2-reader-locked') return createMobileChapterExport(st, body, res)
              const format = body.format ?? 'txt'
              if (format !== 'txt' && format !== 'epub' && format !== 'narration') return fail(res, 400, 'Định dạng không hợp lệ')
              // GAP-40: `overwrite` đã bị bỏ — chỉ còn onExists (BE từ chối trường lạ).
              if ('overwrite' in body) return fail(res, 400, 'Tham số không hợp lệ: overwrite')
              const ext = format === 'epub' ? 'epub' : 'txt'
              let file = `xuat-ban/${st.s}-tap1${format === 'narration' ? '-narration' : ''}.${ext}`
              const strategy = body.onExists ?? 'fail'
              if (mockExportedFiles.has(`${p}/${s}/${file}`)) {
                if (strategy === 'keep') {
                  const stamp = new Date().toISOString().replace(/[-:TZ.]/g, '').slice(0, 14)
                  const stem = file.slice(0, -(ext.length + 1))
                  file = `${stem}-${stamp}.${ext}`
                  let suffix = 1
                  while (mockExportedFiles.has(`${p}/${s}/${file}`)) file = `${stem}-${stamp}-${suffix++}.${ext}`
                } else if (strategy !== 'replace') {
                  const exists = mockExportConflict(file, format, st.completed)
                  return json(res, 409, { error: `Tệp ${file} đã tồn tại`, existing: exists.existing, incoming: exists.incoming, result: { kind: 'exportExists', exists } })
                }
              }
              mockExportedFiles.add(`${p}/${s}/${file}`)
              const job = startExportJob(p, s, file, st.completed)
              return json(res, 200, { path: file, bytes: 84210, chapters: st.completed, skipped: [], job })
            }
            if (rest[0] === 'exports') {
              if (s === 'm2-reader' || s === 'm2-reader-locked') return json(res, 200, {
                dir: 'xuat-ban', files: [...(mobileChapterExports.get(`${p}/${s}`)?.values() ?? [])].map(({ body: _body, mime: _mime, ...file }) => file),
                matrix: [], autoExport: { onVolume: false, onBook: false, onChapter: false, formats: ['txt', 'epub'], pattern: '', onExists: 'keep' },
              })
              const fileTxt = { path: 'xuat-ban/tra-xanh-cuop-kich-ban-tap1.txt', name: 'tra-xanh-cuop-kich-ban-tap1.txt', format: 'txt' as const, size: 84210, updatedAt: iso(NOW - 7200000), volume: 1 }
              const fileEpub = { path: 'xuat-ban/tra-xanh-cuop-kich-ban-tap1.epub', name: 'tra-xanh-cuop-kich-ban-tap1.epub', format: 'epub' as const, size: 210400, updatedAt: iso(NOW - 7200000), volume: 1 }
              // Bản đọc TTS: chỉ có sau khi người dùng tự xuất (không nằm trong tự xuất mặc định).
              const narrPath = `xuat-ban/${st.s}-tap1-narration.txt`
              const fileNarr = mockExportedFiles.has(`${p}/${s}/${narrPath}`)
                ? { path: narrPath, name: narrPath.split('/').pop() ?? narrPath, format: 'txt' as const, size: 76000, updatedAt: iso(NOW - 3600000), volume: 1 }
                : null
              return json(res, 200, {
                dir: 'xuat-ban',
                files: [fileTxt, fileEpub, ...(fileNarr ? [fileNarr] : [])],
                matrix: [{ volume: 1, title: 'Tập 1', from: 1, to: Math.min(st.completed, 8) || st.completed, complete: st.completed >= 8, txt: fileTxt, epub: fileEpub, narration: fileNarr, audio: [] }],
                autoExport: { onVolume: true, onBook: true, onChapter: false, formats: ['txt', 'epub'], pattern: '', onExists: 'keep' },
              })
            }
            if (rest[0] === 'export' && req.method === 'GET') {
              const raw = url.searchParams.get('format')
              const format = raw === 'epub' ? 'epub' : raw === 'narration' ? 'narration' : 'txt'
              const from = Number(url.searchParams.get('from')) || 1
              const to = Number(url.searchParams.get('to')) || st.completed
              const readNumbers = url.searchParams.get('readNumbers') === '1'
              const chapters = Object.entries(st.chapters).filter(([chapter]) => Number(chapter) >= from && Number(chapter) <= Math.min(to, st.completed)).map(([, chapter]) => chapter)
              const { body, mime } = mockExportDownload(`${st.s}${format === 'narration' ? '-narration' : ''}.${format === 'epub' ? 'epub' : 'txt'}`, st.title, chapters, { narration: format === 'narration', readNumbers })
              const dl = format === 'narration' ? `${st.title}-narration.txt` : `${st.s}.${format}`
              res.writeHead(200, { 'content-type': mime, 'content-length': body.length, 'content-disposition': `attachment; filename*=UTF-8''${encodeURIComponent(dl)}` })
              return res.end(body)
            }

            if (rest[0] === 'events' && !rest[1] && req.method === 'GET') {
              return json(res, 200, mockActivityHistory(st.replay, st.snapshot.currentChapter, st.chapters, user!, url, st.activityFixture))
            }
            // ─── live (worker) ───
            if (rest[0] === 'events') {
              if (st.locked) return fail(res, 409, 'truyện đang được mở bởi tiến trình khác')
              if (!canRead(user!, `${p}/${s}`)) return fail(res, 404, 'Không tìm thấy truyện')
              res.writeHead(200, { 'content-type': 'text/event-stream', 'cache-control': 'no-cache', connection: 'keep-alive' })
              const client: SseClient = { res }
              st.clients.add(client)
              sseSend(client, 'reset', {})
              sseSend(client, 'snapshot', st.snapshot)
              st.replay.forEach((e) => sseSend(client, 'event', e))
              st.jobs.forEach((job) => sseSend(client, 'job', job))
              const ka = setInterval(() => { try { res.write(':ka\n\n') } catch { /* */ } }, 15000)
              ensureStream(st)
              req.on('close', () => { clearInterval(ka); st.clients.delete(client) })
              return
            }
            if (rest[0] === 'snapshot') return writeLive(() => json(res, 200, st.snapshot))
            if (rest[0] === 'input' && req.method === 'POST') {
              return writeLive(async () => {
                const body = await readBody(req)
                const text = String(body.text ?? '')
                // /cocreate: bật/tắt đồng sáng tác (GAP-6) — Workspace refetch GET /cocreate
                if (text.trim() === '/cocreate' || text.trim() === '/cocreate off') {
                  broadcastEvent(st, mkEvent(0, { id: `user-${++seq}`, time: new Date().toISOString(), category: 'USER', agent: 'user', kind: 'user', username: user!.username, command: 'cocreate', input: text, summary: text, level: 'info' }))
                  const active = text.trim() === '/cocreate'
                  st.cocreate = { active, messages: [], draft: '', ready: false, suggestions: [] }
                  broadcast(st, 'snapshot', st.snapshot)
                  return json(res, 200, { ok: true, action: 'steer' })
                }
                const action = st.snapshot.isRunning ? 'steer' : st.snapshot.phase === '' && st.snapshot.bookTitle === '' ? 'start' : 'continue'
                if (action === 'start') {
                  const check = mockLedgerCheck(p, text, id => canRead(user!, id))
                  if (check.checks.some(item => item.level === 'error')) return json(res, 409, { error: 'Còn mục Bắt sửa khi so với truyện trước', checks: check.checks })
                }
                broadcastEvent(st, mkEvent(0, { id: `user-${++seq}`, time: new Date().toISOString(), category: 'USER', agent: 'user', kind: 'user', username: user!.username, command: action, input: text, summary: text, level: 'info' }))
                if (action !== 'steer') { st.status = 'writing'; st.snapshot = { ...st.snapshot, pendingAsk: null, waitingFor: '', isRunning: true, runtimeState: 'running', statusLabel: `写第 ${st.snapshot.currentChapter} 章` }; broadcast(st, 'reset', {}); broadcast(st, 'snapshot', st.snapshot); ensureStream(st) }
                return json(res, 200, { ok: true, action })
              })
            }
            // G4 — “Thử lại ngay”: bỏ thời gian chờ backoff của lượt đang chờ (14-gap-closure.md)
            if (rest[0] === 'retry' && req.method === 'POST') {
              return writeLive(() => {
                const waiting = st.replay.find((event) => event.retryAt !== '')
                if (!waiting) return fail(res, 409, 'Không có lượt nào đang chờ thử lại')
                if (!waiting.canRetryNow) return fail(res, 409, 'Lượt thử lại này không đánh thức được, hãy chờ hết thời gian')
                const resumed: HostEvent = { ...waiting, retryAt: '', canRetryNow: false, finishedAt: iso(Date.now()), summary: 'Đã thử lại ngay, bỏ qua thời gian chờ' }
                st.replay = st.replay.map((event) => (event.id === waiting.id ? resumed : event))
                broadcastEvent(st, resumed)
                return json(res, 200, { ok: true })
              })
            }
            // G13 — lệnh gõ trong ô chat web; mock chạy đúng nhánh của registry G11/G12
            if (rest[0] === 'command' && req.method === 'POST') {
              return writeLive(async () => {
                const body = await readBody(req)
                const line = String(body.line ?? '').trim()
                const [head, ...tail] = line.split(/\s+/)
                const arg = tail.join(' ')
                if (!head.startsWith('/')) return fail(res, 400, `Lệnh không xác định: ${line}`)
                broadcastEvent(st, mkEvent(0, { id: `user-${++seq}`, time: new Date().toISOString(), category: 'USER', agent: 'user', kind: 'user', username: user!.username, command: head.slice(1), input: line, summary: line, level: 'info' }))
                const name = head.slice(1)
                const say = (summary: string, level: HostEvent['level'] = 'success') => {
                  broadcastEvent(st, mkEvent(500 + seq, { category: 'SYSTEM', agent: 'host', summary, level }))
                  return summary
                }
                if (['next', 'import', 'reopen', 'simulate', 'importsim', 'sync'].includes(name) && st.snapshot.isRunning) {
                  return fail(res, 409, `Lệnh chỉ có thể thực hiện khi hệ thống đang rảnh: ${head}`)
                }
                switch (name) {
                  case 'help':
                    return json(res, 200, { ok: true, message: 'Mở hướng dẫn lệnh' })
                  case 'review': {
                    if (arg !== 'on' && arg !== 'off') return fail(res, 400, 'Cách dùng: /review on|off')
                    st.advanceMode = arg === 'on' ? 'review' : 'auto'
                    st.snapshot = { ...st.snapshot, advanceMode: st.advanceMode }
                    broadcast(st, 'snapshot', st.snapshot)
                    const message = say(st.advanceMode === 'review'
                      ? 'Đã chuyển cách đẩy chương: Nghiệm thu từng chương'
                      : `Đã chuyển cách đẩy chương: Tự động${!st.snapshot.isRunning && st.snapshot.phase !== 'complete' ? '; sáng tác đang tạm dừng, bấm Tiếp tục để chạy lại' : ''}`)
                    return json(res, 200, { ok: true, message })
                  }
                  case 'next': {
                    if (st.advanceMode !== 'review') return fail(res, 409, 'Truyện đang ở chế độ tự động, không có chương nào chờ duyệt')
                    const chapter = st.snapshot.currentChapter
                    st.status = 'writing'
                    st.snapshot = { ...st.snapshot, hasAdvanceHold: false, advanceHoldReason: '', isRunning: true, runtimeState: 'running', statusLabel: `Viết chương ${chapter}`, waitingFor: '' }
                    broadcast(st, 'snapshot', st.snapshot); ensureStream(st)
                    return json(res, 200, { ok: true, message: say(`Đã duyệt chương ${chapter}, writer bắt đầu viết`) })
                  }
                  case 'export': {
                    const { path, format } = mockExportPath(line, st.s)
                    const ext = /\.([a-z0-9]+)$/i.exec(path)?.[1]?.toLowerCase()
                    if (ext && ext !== 'txt' && ext !== 'epub') return fail(res, 400, `Không xuất được đuôi .${ext} — chỉ nhận .txt hoặc .epub`)
                    let target = path
                    if (tail.includes('--keep')) target = path.replace(/(\.[a-z0-9]+)$/i, `-${new Date().toTimeString().slice(0, 5).replace(':', '')}$1`)
                    else if (mockExportedFiles.has(`${st.p}/${st.s}/${path}`) && !tail.includes('--overwrite')) {
                      const exists = mockExportConflict(path, format, st.completed)
                      return json(res, 409, { error: `Tệp ${path} đã tồn tại`, existing: exists.existing, incoming: exists.incoming, result: { kind: 'exportExists', exists } })
                    }
                    const total = st.completed
                    const job = runJob(st, 'export', total + 1, {
                      interval: 260,
                      onTick: (j) => {
                        j.current = Math.min(j.current, total)
                        j.stage = j.current >= total ? 'pack' : 'merge'
                        j.message = j.current >= total ? 'Đang đóng gói' : `Ghép chương ${j.current + 1}`
                      },
                      onDone: (j) => {
                        mockExportedFiles.add(`${st.p}/${st.s}/${target}`)
                        j.export = mockExportResult(target, format, total, [])
                      },
                    })
                    job.total = total
                    return json(res, 200, { ok: true, result: { kind: 'job', job: job.job, jobKind: 'export', path: target } })
                  }
                  case 'sync': {
                    if (arg === '--check') return json(res, 200, { ok: true, result: { kind: 'syncCheck', changed: mockSyncChanged() } })
                    const job = runJob(st, 'sync', 3, { onDone: (j) => { j.sync = mockSyncResult() } })
                    return json(res, 200, { ok: true, result: { kind: 'job', job: job.job, jobKind: 'sync' } })
                  }
                  case 'diag':
                    return json(res, 200, { ok: true, result: { kind: 'panel', panel: 'diag' } })
                  case 'simulate': {
                    const job = runJob(st, 'simulate', 5, { interval: 900, onTick: simulateTick, onDone: (j) => { j.simulate = mockSimulateResult() } })
                    return json(res, 200, { ok: true, result: { kind: 'job', job: job.job, jobKind: 'simulate' } })
                  }
                  case 'importsim': {
                    if (arg === '') return fail(res, 400, 'Cách dùng: /importsim <tệp.json>')
                    runJob(st, 'importsim', 5)
                    return json(res, 200, { ok: true, message: say(`Đang nhập hồ sơ văn phong ${arg}`) })
                  }
                  case 'model':
                    return json(res, 200, { ok: true, result: { kind: 'panel', panel: 'model', ...(arg ? { role: arg } : {}) } })
                  case 'cocreate':
                    st.cocreate = { active: true, messages: [], draft: '', ready: false, suggestions: [] }
                    st.snapshot = { ...st.snapshot, waitingFor: 'cocreate' }
                    broadcast(st, 'snapshot', st.snapshot)
                    return json(res, 200, { ok: true, message: say('Đã bật đồng sáng tác, nhập yêu cầu để bắt đầu') })
                  case 'reopen':
                    st.status = 'writing'
                    st.snapshot = { ...st.snapshot, isRunning: true, runtimeState: 'running', statusLabel: 'Đã mở lại truyện để viết tiếp', currentChapter: st.totalChapters + 1, totalChapters: st.totalChapters + 6 }
                    broadcast(st, 'reset', {}); broadcast(st, 'snapshot', st.snapshot); ensureStream(st)
                    return json(res, 200, { ok: true, message: say('Đã mở lại truyện để viết tiếp') })
                  case 'import': {
                    const path = arg.split(/\s--/)[0] || 'ban-thao.txt'
                    const guide = /--guide=([\s\S]*)$/.exec(arg)?.[1] ?? ''
                    const storyStatus = /--story=(open|closed)/.exec(arg)?.[1] as 'open' | 'closed' | undefined
                    st.importConfirmed = false
                    st.importState = { schemaVersion: 1, resumeHint: 'import', job: null, path, preview: mockImportPreview(path, guide), options: { schemaVersion: 1, storyStatus: storyStatus ?? '', continueAfter: arg.includes('--continue'), guide } }
                    const job = runJob(st, 'import')
                    return json(res, 200, { ok: true, result: { kind: 'job', job: job.job, jobKind: 'import' } })
                  }
                  default:
                    return fail(res, 400, `Lệnh không xác định: ${head}`)
                }
              })
            }
            if (rest[0] === 'resume' && req.method === 'POST') {
              return writeLive(() => {
                broadcastEvent(st, mkEvent(0, { id: `user-${++seq}`, time: new Date().toISOString(), category: 'USER', agent: 'user', kind: 'user', username: user!.username, command: 'resume', input: '', summary: '', level: 'info' }))
                // Gói P · P1: chạy lại sau tạm dừng lập kế hoạch — bước tạm dừng quay lại đang chạy.
                const plan = st.snapshot.plan && {
                  ...st.snapshot.plan,
                  steps: st.snapshot.plan.steps.map((step) =>
                    step.status === 'paused' ? { ...step, status: 'running' } : step),
                }
                st.status = 'writing'; st.snapshot = { ...st.snapshot, isRunning: true, runtimeState: 'running', statusLabel: `写第 ${st.snapshot.currentChapter} 章`, plan: plan ?? st.snapshot.plan }
                broadcast(st, 'snapshot', st.snapshot); ensureStream(st)
                return json(res, 200, { ok: true, label: `Viết tiếp chương ${st.snapshot.currentChapter}` })
              })
            }
            if (rest[0] === 'abort' && req.method === 'POST') {
              const body = await readBody(req)
              return writeLive(() => {
                broadcastEvent(st, mkEvent(0, { id: `user-${++seq}`, time: new Date().toISOString(), category: 'USER', agent: 'user', kind: 'user', username: user!.username, command: 'abort', input: '', summary: '', level: 'info' }))
                if (body.afterStep === true && st.snapshot.isRunning) {
                  st.pauseAfterStep = true
                  return json(res, 200, { ok: true, stopped: true, afterStep: true })
                }
                clearInterval(st.streamTimer!); st.streamTimer = null; st.pauseAfterStep = false
                // Gói P · P1: tạm dừng giữa lập kế hoạch — bước đang lưu đổi qua icon tạm dừng.
                const plan = st.snapshot.plan && {
                  ...st.snapshot.plan,
                  steps: st.snapshot.plan.steps.map((step) =>
                    step.status === 'running' ? { ...step, status: 'paused' } : step),
                }
                st.status = 'paused'; st.snapshot = { ...st.snapshot, isRunning: false, runtimeState: 'paused', statusLabel: '暂停', plan: plan ?? st.snapshot.plan }
                broadcast(st, 'snapshot', st.snapshot)
                return json(res, 200, { ok: true, stopped: true })
              })
            }
            if (rest[0] === 'next' && req.method === 'POST') {
              const commentApproval = await readBody(req)
              return writeLive(() => {
                if (st.advanceMode !== 'review') return fail(res, 400, 'Truyện đang ở chế độ tự động')
                // GAP-36: chương còn mục Bắt sửa thì cửa duyệt đóng.
                const gateChapter = Number(commentApproval.chapter ?? st.completed)
                const blocking = (REVIEWS[`${p}/${s}`]?.[gateChapter]?.items ?? []).filter((item) => item.level === 'error')
                if (blocking.length > 0) {
                  return json(res, 409, { error: `Chương ${gateChapter} còn mục Bắt sửa, chưa duyệt được.`, items: blocking })
                }
                if (commentApproval.applyComments === true) {
                  if (st.snapshot.isRunning) return fail(res, 409, 'Việc sáng tác vẫn đang chạy')
                  const chapter = Number(commentApproval.chapter ?? st.completed)
                  const key = `${p}/${s}/${chapter}`
                  const list = paragraphFeedback.get(key) ?? []
                  const pending = list.filter((comment) => (comment.status ?? (comment.appliedAt ? 'sent' : 'pending')) === 'pending')
                  pending.sort((a, b) => (a.paragraph ?? Number.MAX_SAFE_INTEGER) - (b.paragraph ?? Number.MAX_SAFE_INTEGER) || a.createdAt.localeCompare(b.createdAt))
                  const instruction = pending.map((comment) => `${comment.paragraph === null ? 'Cả chương' : `Đoạn ${comment.paragraph + 1}`}\nTrích dẫn: ${comment.quote}\nYêu cầu: ${[...comment.chips, comment.text].filter(Boolean).join('; ')}`).join('\n\n')
                  const sentAt = new Date().toISOString()
                  for (const comment of pending) { comment.status = 'sent'; comment.sentAt = sentAt; delete comment.appliedAt }
                  st.snapshot = { ...st.snapshot, hasAdvanceHold: false, advanceHoldReason: '', isRunning: true, runtimeState: 'running', waitingFor: '',
                    ...(pending.length ? { flow: 'rewriting', pendingRewrites: [chapter], rewriteReason: instruction } : {}) }
                  st.status = 'writing'
                  broadcast(st, 'story.changed', { project: p, story: s, parts: ['comments'] })
                  broadcast(st, 'snapshot', st.snapshot); ensureStream(st)
                  if (instruction) broadcastEvent(st, mkEvent(410 + seq, { agent: 'writer', summary: `Áp dụng ${pending.length} góp ý chương ${chapter}`, level: 'info' }))
                  // §AI — giả lập Người viết sửa xong: giải lại mốc neo + phát comments.reanchored.
                  if (pending.length) setTimeout(() => simulateRewrite(st, chapter), 2600)
                  return json(res, 200, { ok: true, appliedComments: pending.length })
                }
                st.snapshot = { ...st.snapshot, hasAdvanceHold: false, advanceHoldReason: '', isRunning: true, runtimeState: 'running', statusLabel: `写第 ${st.snapshot.currentChapter} 章` }
                st.status = 'writing'
                broadcast(st, 'snapshot', st.snapshot); ensureStream(st)
                broadcastEvent(st, mkEvent(410 + seq, { agent: 'arbiter', summary: `已放行第 ${st.snapshot.currentChapter} 章`, level: 'success' }))
                return json(res, 200, { ok: true })
              })
            }
            // GAP-47 — giữ nguyên / bỏ giữ một mục thẩm duyệt (D4). Bắt sửa không giữ được.
            if (rest[0] === 'review' && rest[1] === 'items' && rest[3] === 'keep') {
              const key = `${p}/${s}`
              if (!canWrite(user!, key)) return fail(res, 404, 'Không tìm thấy truyện')
              const itemId = decodeURIComponent(rest[2] ?? '')
              const item = reviewItemOf(key, itemId)
              if (!item) {
                // §J câu 1 — `kept[]` tĩnh của mock cũng bỏ giữ được: lần tải vũ trụ kế tiếp
                // trả mục ấy trong `checks` (như server thật sau DELETE GAP-47).
                if (req.method === 'DELETE' && mockUnkeep(p, s, itemId)) {
                  const change = { project: p, story: s, parts: ['reviews'] }
                  broadcast(st, 'story.changed', change)
                  emitGlobalEvent('story.changed', change)
                  res.writeHead(204)
                  return res.end()
                }
                return fail(res, 404, 'Không tìm thấy mục thẩm duyệt')
              }
              if (req.method === 'DELETE') {
                reviewDecisions.get(key)?.delete(itemId)
                const change = { project: p, story: s, parts: ['reviews'] }
                broadcast(st, 'story.changed', change)
                emitGlobalEvent('story.changed', change)
                res.writeHead(204)
                return res.end()
              }
              if (req.method !== 'POST') return fail(res, 405, 'Không hỗ trợ thao tác này')
              const body = await readBody(req)
              const reason = typeof body.reason === 'string' ? body.reason.trim() : ''
              if (!reason) return fail(res, 400, 'Hãy ghi lý do giữ nguyên')
              if (item.level === 'error') return fail(res, 409, 'Mục Bắt sửa không giữ nguyên được — hãy sửa chương')
              const decision: ReviewDecision = { state: 'kept', reason, by: user!.name || user!.username, at: new Date().toISOString() }
              const stored = reviewDecisions.get(key) ?? new Map<string, ReviewDecision>()
              stored.set(itemId, decision)
              reviewDecisions.set(key, stored)
              const change = { project: p, story: s, parts: ['reviews'] }
              broadcast(st, 'story.changed', change)
              emitGlobalEvent('story.changed', change)
              return json(res, 200, decision)
            }
            if (rest[0] === 'review' && req.method === 'POST') {
              return writeLive(async () => {
                const body = await readBody(req)
                st.advanceMode = body.on ? 'review' : 'auto'
                st.snapshot = { ...st.snapshot, advanceMode: st.advanceMode }
                broadcast(st, 'snapshot', st.snapshot)
                const message = st.advanceMode === 'review'
                  ? 'Đã chuyển cách đẩy chương: Nghiệm thu từng chương'
                  : `Đã chuyển cách đẩy chương: Tự động${!st.snapshot.isRunning && st.snapshot.phase !== 'complete' ? '; sáng tác đang tạm dừng, bấm Tiếp tục để chạy lại' : ''}`
                broadcastEvent(st, mkEvent(420 + seq, { category: 'SYSTEM', agent: 'host', summary: message }))
                return json(res, 200, { ok: true })
              })
            }
            if (rest[0] === 'reopen' && req.method === 'GET') {
              const complete = st.snapshot.phase === 'complete'
              const idle = !st.snapshot.isRunning && !st.cocreate.active
              return json(res, 200, { schemaVersion: 1, title: st.title, chapters: st.completed, words: st.snapshot.totalWordCount, lockedAt: null, closedAt: null, allowed: complete && idle, checks: [
                { id: 'completed', ok: complete, label: 'Truyện đã hoàn thành', note: '', chapters: [] },
                { id: 'idle', ok: idle, label: 'Hệ thống đang rảnh', note: '', chapters: [] },
                { id: 'clean', ok: true, label: `${st.completed}/${st.completed} chương đã duyệt sạch`, note: 'Các chương cũ được giữ nguyên', chapters: [] },
              ] })
            }
            if (rest[0] === 'reopen' && req.method === 'POST') {
              return writeLive(() => {
                st.status = 'writing'; st.snapshot = { ...st.snapshot, isRunning: true, runtimeState: 'running', statusLabel: '已重开本书为创作状态（用户撤销完结裁定）', currentChapter: st.totalChapters + 1, totalChapters: st.totalChapters + 6 }
                broadcast(st, 'reset', {}); broadcast(st, 'snapshot', st.snapshot); ensureStream(st)
                return json(res, 200, { ok: true })
              })
            }
            // cocreate
            if (rest[0] === 'cocreate') {
              if (!rest[1]) return writeLive(() => json(res, 200, { schemaVersion: 1, proposal: null, committed: false, nextChapter: null, ...st.cocreate }))
              return writeLive(async () => {
                const body = await readBody(req)
                if (rest[1] === 'message') {
                  const msg = String(body.message ?? '')
                  st.cocreate.active = true
                  st.cocreate.messages.push({ role: 'user', content: msg })
                  st.snapshot = { ...st.snapshot, waitingFor: 'cocreate' }
                  broadcast(st, 'snapshot', st.snapshot)
                  const reply = st.cocreate.messages.filter((m) => m.role === 'user').length < 2
                    ? 'Đã rõ phần lớn ý tưởng. Bạn muốn nhấn mạnh vào xung đột nào nhất — cá nhân trong phòng thu hay phe đấu toàn nhà xuất bản?'
                    : 'Đủ rõ rồi. Tôi tổng hợp thành: "Truyện đô thị trinh thám hồi hộp 12 chương, nhân vật chính nữ giám định pháp y, mạch kép: hồ sơ vụ án + đấu trí nội bộ." Bấm Ctrl+S để bắt đầu viết.'
                  st.cocreate.messages.push({ role: 'assistant', content: reply })
                  if (st.cocreate.messages.filter((m) => m.role === 'user').length >= 2) {
                    st.cocreate.ready = true
                    st.cocreate.draft = 'Truyện đô thị trinh thám hồi hộp 12 chương — nhân vật chính nữ giám định pháp y Kiều My; mạch kép hồ sơ vụ án + đấu trí nội bộ; phong cách sắc, không nước mắt.'
                    st.cocreate.suggestions = ['Thêm nhân vật phản diện trong phòng pháp lý', 'Đặt hạn chót 30 ngày cho giải quyết vụ án', 'Viết theo góc nhìn thứ nhất của từng nhân vật luân phiên']
                  }
                  st.cocreate.schemaVersion = 1
                  st.cocreate.proposal = st.cocreate.ready ? mockCoCreateProposal((st.cocreate.proposal?.revision ?? 0) + 1, st.completed + 1, st.completed) : null
                  st.cocreate.committed = false
                  st.cocreate.nextChapter = st.completed + 1
                  for (const c of reply.split('')) broadcast(st, 'cocreate', { text: c })
                  return json(res, 200, { ...st.cocreate, reply })
                }
                if (rest[1] === 'commit') {
                  const brief = typeof body.draft === 'string' ? body.draft : st.cocreate.draft
                  const check = mockLedgerCheck(p, brief, id => canRead(user!, id))
                  if (check.checks.some(item => item.level === 'error')) return json(res, 409, { error: 'Còn mục Bắt sửa khi so với truyện trước', checks: check.checks })
                  st.cocreate.draft = brief
                  st.cocreate.active = false
                  st.cocreate.schemaVersion = 1; st.cocreate.committed = true; st.cocreate.nextChapter = st.completed + 1
                  const action = st.snapshot.isRunning ? 'resume' : 'start'
                  st.status = 'writing'
                  st.snapshot = { ...st.snapshot, isRunning: true, runtimeState: 'running', waitingFor: '', premise: st.cocreate.draft || st.snapshot.premise }
                  broadcast(st, 'reset', {}); broadcast(st, 'snapshot', st.snapshot); ensureStream(st)
                  broadcastEvent(st, mkEvent(430 + seq, { agent: 'arbiter', summary: '开始创作（规划师: architect — premise mới từ đồng sáng tác）' }))
                  return json(res, 200, { ok: true, action, schemaVersion: 1, proposal: st.cocreate.proposal ?? null, committed: true, nextChapter: st.cocreate.nextChapter })
                }
                if (rest[1] === 'cancel') {
                  st.cocreate = { active: false, messages: [], draft: '', ready: false, suggestions: [] }
                  st.snapshot = { ...st.snapshot, waitingFor: '' }
                  broadcast(st, 'snapshot', st.snapshot)
                  return json(res, 200, { ok: true })
                }
                return fail(res, 404, 'Not found')
              })
            }
            // jobs & lệnh
            // G9 — xem trước cách chia chương, không ghi dữ liệu.
            if (rest[0] === 'import' && rest[1] === 'preview' && req.method === 'POST') {
              const body = await readBody(req)
              const path = String(body.path ?? '').trim()
              if (!path) return fail(res, 400, 'Thiếu đường dẫn tệp nguồn')
              const guide = String(body.guide ?? '')
              const preview = mockImportPreview(path, guide)
              st.importState = { schemaVersion: 1, resumeHint: 'import', job: null, path, preview, options: { schemaVersion: 1, storyStatus: '', continueAfter: false, guide } }
              st.importConfirmed = false
              return json(res, 200, preview)
            }
            if (rest[0] === 'import' && !rest[1] && req.method === 'POST') {
              return writeLive(async () => {
                const body = await readBody(req)
                const path = String(body.path ?? 'ban-thao.txt')
                const guide = String(body.guide ?? '')
                st.importConfirmed = !!body.autoConfirm
                st.importState = { schemaVersion: 1, resumeHint: 'import', job: null, path, preview: mockImportPreview(path, guide), options: { schemaVersion: 1, storyStatus: body.storyStatus === 'closed' ? 'closed' : body.storyStatus === 'open' ? 'open' : '', continueAfter: !!body.continueAfter, guide } }
                const job = runJob(st, 'import')
                return json(res, 200, { job: job.job })
              })
            }
            if (rest[0] === 'import' && !rest[1] && req.method === 'GET') {
              const job = [...st.jobs.values()].filter((j) => j.kind === 'import').at(-1) ?? null
              return json(res, 200, { schemaVersion: 1, resumeHint: 'import', path: '', preview: null, ...st.importState, job })
            }
            if (rest[0] === 'import' && rest[1] === 'confirm' && req.method === 'POST') {
              const body = await readBody(req)
              if (!body.accept) return json(res, 200, { ok: true })
              if (body.storyStatus !== undefined && body.storyStatus !== 'open' && body.storyStatus !== 'closed') return fail(res, 400, 'Trạng thái truyện không hợp lệ')
              if (st.importState?.options) st.importState.options = { ...st.importState.options, storyStatus: body.storyStatus as 'open' | 'closed', continueAfter: !!body.continueAfter }
              st.importConfirmed = true
              const job = [...st.jobs.values()].find((j) => j.awaiting === 'confirm') ?? runJob(st, 'import')
              job.awaiting = ''; broadcast(st, 'job', job)
              return json(res, 200, { ok: true })
            }
            if (rest[0] === 'simulate' && rest[1] === 'profile' && req.method === 'GET') {
              const result = mockSimulateResult()
              const body = JSON.stringify({ synthesis: result.synthesis }, null, 2)
              res.writeHead(200, { 'content-type': 'application/json', 'content-disposition': 'attachment; filename="ho-so-van-phong.json"' })
              return res.end(body)
            }
            if (rest[0] === 'simulate' && req.method === 'POST') return writeLive(() => {
              const job = runJob(st, 'simulate', 5, { interval: 900, onTick: simulateTick, onDone: (j) => { j.simulate = mockSimulateResult() } })
              return json(res, 200, { job: job.job })
            })
            if (rest[0] === 'importsim' && req.method === 'POST') return writeLive(() => { const job = runJob(st, 'importsim', 5); return json(res, 200, { job: job.job }) })
            if (rest[0] === 'sync' && rest[1] === 'diff' && req.method === 'GET') {
              const diff = mockSyncDiff(Number(rest[2]))
              return diff ? json(res, 200, diff) : fail(res, 404, 'Chương không còn sửa ngoài ứng dụng')
            }
            if (rest[0] === 'sync' && rest[1] === 'fix' && req.method === 'POST') {
              return writeLive(async () => {
                const body = await readBody(req)
                const job = body.job ? st.jobs.get(String(body.job)) : [...st.jobs.values()].filter((j) => j.kind === 'sync').at(-1)
                const analysis = job?.sync?.analyses.find((a) => a.chapter === Number(body.chapter))
                const issue = analysis?.downstreamIssues[Number(body.issueIndex ?? 0)]
                if (!job || !issue) return fail(res, 404, 'Không tìm thấy mâu thuẫn cần sửa')
                issue.sent = true; broadcast(st, 'job', job)
                return json(res, 200, { schemaVersion: 1, ok: true, sent: true, action: st.snapshot.isRunning ? 'steer' : 'continue' })
              })
            }
            if (rest[0] === 'sync' && req.method === 'POST') {
              return writeLive(async () => {
                const body = await readBody(req)
                if (body.check) return json(res, 200, { changed: mockSyncChanged() })
                const job = runJob(st, 'sync', 3, { onDone: (j) => { j.sync = mockSyncResult() } })
                return json(res, 200, { job: job.job })
              })
            }
            if (rest[0] === 'diag') {
              const key = `${st.p}/${st.s}`
              if (rest[1] === 'last') {
                const report = diagnosticReports.get(key)
                return report ? json(res, 200, report) : fail(res, 404, 'Chưa có báo cáo chẩn đoán')
              }
              if (rest[1] === 'actions' && req.method === 'POST') {
                const fingerprint = rest[2]
                const report = diagnosticReports.get(key)
                if (!report?.actions.some((action) => action.fingerprint === fingerprint)) return fail(res, 404, 'Đề xuất này đã cũ — hãy phân tích lại.')
                diagSent.add(fingerprint)
                report.actions.forEach((action) => { if (action.fingerprint === fingerprint) action.sent = true })
                return json(res, 200, { ok: true, action: st.snapshot.isRunning ? 'steer' : 'continue' })
              }
              if (rest[1] === 'export') {
                const report = diagnosticReports.get(key)
                if (!report) return fail(res, 404, 'Chưa có báo cáo chẩn đoán')
                const text = ['# Chẩn đoán ẩn danh', '', ...report.findings.map((finding) => `- ${finding.severity}: ${finding.title}`)].join('\n')
                res.writeHead(200, { 'content-type': 'text/markdown; charset=utf-8', 'content-disposition': 'attachment; filename="chan-doan-an-danh.md"' })
                return res.end(text)
              }
              runJob(st, 'diag', 5, { onDone: () => {
                const report = mockDiagReport(st.completed, st.totalChapters)
                report.generatedAt = new Date().toISOString()
                diagnosticReports.set(key, report)
                json(res, 200, report)
              } })
              return
            }
            if (rest[0] === 'start-file' && req.method === 'POST') {
              return writeLive(async () => {
                const body = await readBody(req)
                st.status = 'writing'
                st.snapshot = { ...st.snapshot, isRunning: true, runtimeState: 'running', premise: `Từ tệp: ${body.path}`, statusLabel: '启动' }
                broadcast(st, 'reset', {}); broadcast(st, 'snapshot', st.snapshot); ensureStream(st)
                return json(res, 200, { ok: true, action: 'start' })
              })
            }
            if (rest[0] === 'models' || rest[0] === 'thinking') {
              const key = `${st.p}/${st.s}`
              const data = commandModels.get(key) ?? mockModels()
              commandModels.set(key, data)
              if (req.method === 'POST') {
                const body = await readBody(req)
                const role = data.roles.find((entry) => entry.role === body.role)
                if (!role) return fail(res, 400, 'Vai trợ lý không hợp lệ')
                if (rest[0] === 'thinking') {
                  if (!role.thinkingOptions.includes(String(body.level))) return fail(res, 400, 'Model không hỗ trợ mức suy luận này')
                  role.thinking = String(body.level)
                } else {
                  const provider = data.providers.find((entry) => entry.provider === body.provider)
                  const model = provider?.models.find((entry) => entry.name === body.model)
                  if (!model) return fail(res, 400, 'Model không có trong danh mục')
                  role.provider = provider!.provider
                  role.model = model.name
                  role.thinkingOptions = model.thinking
                  if (!model.thinking.includes(role.thinking)) role.thinking = ''
                }
                return json(res, 200, { ok: true })
              }
              const config = (configStore.get(docKey('global', '', '')) ?? CONFIG_GLOBAL) as Config
              return json(res, 200, { ...data, providers: data.providers.map(provider => ({ ...provider, models: provider.models.map(model => {
                const own = config.providers?.[provider.provider]?.models?.find(entry => entry.name === model.name)?.context_window
                return { ...model, contextWindow: own || config.context_window || model.registryWindow || 200000, contextSource: own ? 'model_config' : config.context_window ? 'config' : model.registryWindow ? 'registry' : 'default' }
              }) })) })
            }
          }
          // config/rules/system/reveal/tts xử lý ở middleware thứ hai
          if (seg[0] === 'config' || seg[0] === 'rules' || seg[0] === 'system' || seg[0] === 'reveal' || seg[0] === 'tts') return next()
          return fail(res, 404, 'Not found')
        } catch (e) {
          return fail(res, 500, e instanceof Error ? e.message : 'Lỗi mock')
        }
      })

      // ─── config/rules/system (không yêu cầu auth riêng — đã check ở trên) ───
      server.middlewares.use('/api', async (req, res, next) => {
        const url = new URL(req.url ?? '/', 'http://x')
        const seg = url.pathname.split('/').filter(Boolean).map(decodeURIComponent)
        const session = sessionOf(req)
        const user = session ? users.find((u) => u.user.username === session.username)?.user ?? null : null
        if (!user) return next()

        // ─── bộ đọc trên máy (GAP-21) ───
        if (seg[0] === 'tts') {
          if (seg[1] === 'status') return json(res, 200, ttsStatus())
          if (seg[1] === 'engines') return json(res, 200, { engines: ttsEngines(), active: 'vieneu' })
          if (seg[1] === 'voices') return json(res, 200, { voices: ttsVoices() })
          if (seg[1] === 'install' && req.method === 'GET') return json(res, 200, ttsInstallProgress())
          if (seg[1] === 'install' && req.method === 'POST') {
            if (user!.role !== 'admin') return fail(res, 403, 'Chỉ quản trị viên cài bộ đọc')
            if (ttsStatus().state === 'installing') return fail(res, 409, 'Đang cài bộ đọc, chờ lượt này xong')
            if (ttsStatus().state !== 'missing') return json(res, 200, { job: '', state: 'done', message: 'Bộ đọc trên máy đã có sẵn' })
            installTts()
            return json(res, 202, { job: 'voice/install', state: 'running' })
          }
          if (seg[1] === 'preview' && req.method === 'POST') {
            const body = await readBody(req)
            if (!String(body.voiceId ?? '')) return fail(res, 400, 'Chưa chọn giọng đọc')
            if (!ttsStatus().ready) return fail(res, 503, 'Bộ đọc trên máy chưa sẵn sàng')
            const wav = wavFor(2)
            res.writeHead(200, { 'content-type': 'audio/wav', 'content-length': wav.length, 'cache-control': 'no-store' })
            return res.end(wav)
          }
          return fail(res, 404, 'Not found')
        }

        const scope = (url.searchParams.get('scope') ?? 'global') as 'global' | 'project' | 'story'
        const project = url.searchParams.get('project') ?? ''
        const story = url.searchParams.get('story') ?? ''

        if (seg[0] === 'config') {
          const isAdmin = user!.role === 'admin'
          if (seg[1] === 'presets') {
            return json(res, 200, {
              providers: MOCK_PROVIDER_PRESETS,
              styles: ['default', 'romance', 'suspense', 'fantasy', 'wuxia', 'giong-tre'],
              reasoning: ['off', 'low', 'medium', 'high', 'xhigh', 'max'],
              // D2 — gói thể loại: đúng 4 trường máy chủ thật trả (assets/packs/*/goi.json).
              packs: MOCK_PACKS,
            })
          }
          if (seg[1] === 'test') {
            if (!isAdmin) return fail(res, 403, 'Chỉ quản trị viên thử kết nối')
            const body = await readBody(req)
            // G6 — thiếu `model` thì dùng defaultModel của preset, và luôn trả model đã dùng.
            const provider = String(body.provider ?? '')
            const model = String(body.model ?? '') || MOCK_PROVIDER_PRESETS.find((preset) => preset.id === provider)?.defaultModel || 'gpt-5.4'
            // Khi ok trả kèm ngữ cảnh đã giải: model_config (bản nháp) > provider > registry > default.
            const draftModel = ((body.config as Config | undefined)?.providers?.[provider]?.models ?? []).find((entry) => entry.name === model)
            const listed = [...MOCK_OPENROUTER_MODELS, ...Object.values(MOCK_KEY_PROVIDER_MODELS).flat()].find((entry) => entry.name === model)
            const registry = mockModels().providers.flatMap((entry) => entry.models).find((entry) => entry.name === model)?.registryWindow ?? 0
            const contextWindow = draftModel?.context_window || listed?.contextWindow || registry || 200000
            const contextSource = draftModel?.context_window ? 'model_config' : listed ? 'provider' : registry ? 'registry' : 'default'
            return json(res, 200, {
              ok: true, latencyMs: 210 + Math.floor(Math.random() * 200),
              message: `Provider ${provider} phản hồi bình thường`, model, contextWindow, contextSource,
            })
          }
          // POST /api/config/models/list {provider, config?, refresh?}: danh sách model của
          // nhà cung cấp (luôn 200; lỗi trong body). OpenRouter ~312 model · Ollama · OAuth · bad→key_rejected.
          if (seg[1] === 'models' && seg[2] === 'list') {
            if (!isAdmin) return fail(res, 403, 'Chỉ quản trị viên lấy danh sách model')
            const body = await readBody(req)
            const provider = String(body.provider ?? '')
            const draftConfig = (body.config ?? {}) as Config
            const preset = MOCK_PROVIDER_PRESETS.find((entry) => entry.id === provider)
            const savedConfig = (configStore.get(docKey('global', '', '')) ?? CONFIG_GLOBAL) as Config
            const providerConf = draftConfig.providers?.[provider] ?? savedConfig.providers?.[provider] ?? {}
            const type = providerConf.type || preset?.type || provider
            const key = providerConf.api_key ?? ''
            const keySet = providerConf.api_key_set ?? (key ? true : savedConfig.providers?.[provider]?.api_key_set ?? false)
            const empty = (extra: Record<string, unknown>) => json(res, 200, { models: [], total: 0, fetchedAt: '', ...extra })
            await sleep(600)
            if (type === 'bedrock') return empty({ unsupported: true })
            const OAUTH = ['openai-codex', 'xai-oauth', 'anthropic-oauth']
            if (OAUTH.includes(type)) {
              if (accountConnected.get(provider) === false) return empty({ error: 'not_logged_in', message: `Chưa đăng nhập ${preset?.label.split(' (')[0] ?? provider} — đăng nhập rồi thử lại` })
              const models = provider === 'xai-oauth'
                ? [{ name: 'grok-4.6', displayName: 'Grok 4.6', contextWindow: 256000 }, { name: 'grok-4.3', displayName: 'Grok 4.3', contextWindow: 131072 }, { name: 'grok-4.20-0309-reasoning', displayName: 'Grok 4.20 (reasoning)', contextWindow: 200000 }, { name: 'grok-4.20-0309-non-reasoning', displayName: 'Grok 4.20 (non-reasoning)', contextWindow: 200000 }]
                : provider === 'anthropic-oauth'
                  ? [{ name: 'claude-opus-4-8', displayName: 'Claude Opus 4.8', contextWindow: 200000 }, { name: 'claude-sonnet-4-6', displayName: 'Claude Sonnet 4.6', contextWindow: 200000 }, { name: 'claude-haiku-4-5', displayName: 'Claude Haiku 4.5', contextWindow: 200000 }]
                  : mockModels().providers.find(entry => entry.provider === provider)?.models.map(entry => ({ name: entry.name, displayName: entry.name, contextWindow: entry.registryWindow }))
                    ?? [{ name: 'gpt-5.4', displayName: 'GPT-5.4', contextWindow: 320000 }]
              return json(res, 200, modelListResponse(models, providerConf, preset, false))
            }
            if (preset?.keyRequired && !key && !keySet) {
              const pname = preset.label.split(' (')[0]
              return empty({ error: 'missing_key', message: `${pname} cần khoá API — dán khoá rồi thử lại` })
            }
            if (key.includes('bad')) {
              const pname = preset?.label.split(' (')[0] ?? provider
              return empty({ error: 'key_rejected', message: `${pname} từ chối khoá API này — kiểm tra lại khoá`, detail: '401 unauthorized: invalid api key' })
            }
            if (type === 'ollama' || /^https?:\/\/(localhost|127\.0\.0\.1)/.test(providerConf.base_url ?? '')) {
              return json(res, 200, modelListResponse([
                { name: 'qwen3-local', displayName: 'qwen3-local', contextWindow: 128000 },
                { name: 'qwen3:14b', displayName: 'qwen3:14b', contextWindow: 40960, description: '9,3 GB' },
                { name: 'gemma3:12b', displayName: 'gemma3:12b', contextWindow: 131072, description: '8,1 GB' },
                { name: 'qwen2.5:14b', displayName: 'qwen2.5:14b', contextWindow: 131072, description: '9,0 GB' },
              ], providerConf, preset, false))
            }
            if (type === 'openrouter') return json(res, 200, modelListResponse(MOCK_OPENROUTER_MODELS, providerConf, preset, true))
            const generic = MOCK_KEY_PROVIDER_MODELS[type] ?? [{ name: preset?.defaultModel || `${provider}-model`, displayName: preset?.defaultModel || `${provider}-model`, contextWindow: 200000 }]
            return json(res, 200, modelListResponse(generic, providerConf, preset, true))
          }
          if (seg[1] === 'models') {
            const provider = url.searchParams.get('provider') ?? 'openai-codex'
            const preset = MOCK_PROVIDER_PRESETS.find((entry) => entry.id === provider)
            const models = provider === 'openai-codex'
              ? [...mockModels().providers.find(entry => entry.provider === provider)!.models.map(entry => ({ name: entry.name, displayName: entry.name, contextWindow: entry.registryWindow })), { name: 'o4-mini', displayName: 'o4-mini', contextWindow: 200000 }]
              : provider === 'xai-oauth'
                ? [{ name: 'grok-4.6', displayName: 'Grok 4.6', contextWindow: 0 }, { name: 'grok-4.3', displayName: 'Grok 4.3', contextWindow: 0 }, { name: 'grok-4.20-0309-reasoning', displayName: 'Grok 4.20 (reasoning)', contextWindow: 0 }, { name: 'grok-4.20-0309-non-reasoning', displayName: 'Grok 4.20 (non-reasoning)', contextWindow: 0 }]
                : provider === 'anthropic-oauth'
                  ? [{ name: 'claude-opus-4-8', displayName: 'Claude Opus 4.8', contextWindow: 200000 }, { name: 'claude-sonnet-4-6', displayName: 'Claude Sonnet 4.6', contextWindow: 200000 }, { name: 'claude-haiku-4-5', displayName: 'Claude Haiku 4.5', contextWindow: 200000 }]
                  : mockModels().providers.find(entry => entry.provider === provider)?.models.map(entry => ({ name: entry.name, displayName: entry.name, contextWindow: entry.registryWindow })) ?? (preset?.defaultModel ? [{ name: preset.defaultModel, displayName: preset.defaultModel, contextWindow: 200000 }] : [])
            const config = (configStore.get(docKey('global', '', '')) ?? CONFIG_GLOBAL) as Config
            const configured = config.providers?.[provider]?.models ?? []
            const entries = [...models, ...configured.filter((entry) => !models.some((model) => model.name === entry.name)).map((entry) => ({ name: entry.name, displayName: entry.name, contextWindow: 0 }))]
            for (const name of url.searchParams.getAll('model')) if (name && !entries.some((entry) => entry.name === name)) entries.push({ name, displayName: name, contextWindow: 0 })
            return json(res, 200, { models: entries.map((model) => {
              const own = configured.find((entry) => entry.name === model.name)?.context_window
              const contextWindow = own || config.context_window || model.contextWindow || 200000
              return { ...model, contextWindow, registryWindow: model.contextWindow, contextSource: own ? 'model_config' : config.context_window ? 'config' : model.contextWindow ? 'registry' : 'default', summaryThreshold: Math.max(0, contextWindow - Math.max(8000, contextWindow - Math.floor(contextWindow * .85))), automaticWindow: config.context_window || model.contextWindow || 200000, automaticSource: config.context_window ? 'config' : model.contextWindow ? 'registry' : 'default' }
            }) })
          }
          if (seg[1] === 'account') {
            if (!isAdmin) return fail(res, 403, 'Chỉ quản trị viên xem tài khoản nhà cung cấp')
            const provider = url.searchParams.get('provider') ?? 'openai-codex'
            if (accountConnected.get(provider) === false) {
              return json(res, 200, { connected: false, label: '', email: '', plan: '', expiresAt: null })
            }
            return json(res, 200, mockAccount(provider))
          }
          if (seg[1] === 'oauth' && seg[2] === 'start') {
            if (!isAdmin) return fail(res, 403, 'Chỉ quản trị viên đăng nhập nhà cung cấp')
            const body = await readBody(req)
            const id = `oauth-${++seq}`
            const preset = MOCK_PROVIDER_PRESETS.find((entry) => entry.id === body.provider)
            const paste = preset?.pasteCode === true && (body.device === true || req.headers['x-mock-remote'] === '1')
            const device = !paste && (body.device === true || preset?.deviceOnly === true)
            oauthStates.set(id, { started: Date.now(), done: false, provider: String(body.provider ?? 'openai-codex'), device, paste, pasted: false })
            if (paste) return json(res, 200, { id, mode: 'paste', url: 'https://claude.ai/oauth/authorize?code=true&state=mock-state', userCode: '', verifyURL: '', expiresAt: iso(NOW + 600000) })
            if (preset?.pasteCode) return json(res, 200, { id, mode: 'browser', url: 'https://claude.ai/oauth/authorize?code=true&state=mock-state', userCode: '', verifyURL: '', expiresAt: iso(NOW + 600000) })
            return json(res, 200, device
              ? { id, mode: 'device', url: '', userCode: 'ABCD-EFGH', verifyURL: body.provider === 'xai-oauth' ? 'https://accounts.x.ai/device?user_code=ABCD-EFGH' : 'https://auth.openai.com/device?user_code=ABCD-EFGH', expiresAt: iso(NOW + 600000) }
              : { id, mode: 'browser', url: 'https://auth.openai.com/device', userCode: 'ABCD-EFGH', verifyURL: 'https://auth.openai.com/device?user_code=ABCD-EFGH', expiresAt: iso(NOW + 600000) })
          }
          if (seg[1] === 'oauth' && seg[2] === 'status') {
            if (!isAdmin) return fail(res, 403, 'Chỉ quản trị viên xem trạng thái đăng nhập')
            const id = url.searchParams.get('id') ?? ''
            const stt = oauthStates.get(id)
            if (!stt) return fail(res, 404, 'Không tìm thấy phiên OAuth')
            const done = stt.paste ? stt.pasted : Date.now() - stt.started > 4000
            if (done) { stt.done = true; accountConnected.set(stt.provider, true); setProviderConnected(stt.provider, true) }
            const account = mockAccount(stt.provider)
            return json(res, 200, {
              state: done ? 'done' : 'pending', mode: stt.paste ? 'paste' : stt.device ? 'device' : 'browser',
              account: done ? { label: account.label, email: account.email, plan: account.plan } : null,
              error: '', models: done ? [stt.provider === 'xai-oauth' ? { name: 'grok-4.6', displayName: 'Grok 4.6', contextWindow: 0 } : stt.provider === 'anthropic-oauth' ? { name: 'claude-sonnet-4-6', displayName: 'Claude Sonnet 4.6', contextWindow: 200000 } : { name: 'gpt-5.4', displayName: 'GPT-5.4', contextWindow: 320000 }] : [],
            })
          }
          if (seg[1] === 'oauth' && seg[2] === 'complete') {
            if (!isAdmin) return fail(res, 403, 'Chỉ quản trị viên đăng nhập nhà cung cấp')
            const body = await readBody(req)
            const stt = oauthStates.get(String(body.id ?? ''))
            if (!stt) return fail(res, 404, 'Không tìm thấy phiên OAuth')
            if (!stt.paste) return fail(res, 400, 'Phiên đăng nhập này không nhận mã dán tay')
            const code = String(body.code ?? '').trim()
            if (!code) return fail(res, 400, 'Hãy dán mã xác thực hoặc URL chuyển hướng')
            if (code.includes('#') && !code.endsWith('#mock-state')) return fail(res, 400, 'mã dán thuộc một phiên đăng nhập khác (state không khớp) — hãy mở lại URL đăng nhập mới nhất')
            stt.pasted = true
            return json(res, 200, { ok: true })
          }
          if (seg[1] === 'oauth' && (seg[2] === 'cancel' || seg[2] === 'logout')) {
            if (!isAdmin) return fail(res, 403, 'Chỉ quản trị viên đổi kết nối nhà cung cấp')
            const body = await readBody(req)
            if (seg[2] === 'logout') { const provider = String(body.provider ?? 'openai-codex'); accountConnected.set(provider, false); setProviderConnected(provider, false) }
            return json(res, 200, { ok: true })
          }

          // GET/PUT /api/config?scope=…
          if (req.method === 'PUT') {
            if (scope === 'global' && !isAdmin) return fail(res, 403, 'Chỉ quản trị viên sửa thiết lập toàn cục')
            const body = await readBody(req)
            for (const provider of Object.values((body.providers ?? {}) as NonNullable<Config['providers']>)) {
              for (const model of provider.models ?? []) {
                if (!isAdmin && model.context_window !== undefined) return fail(res, 403, 'Chỉ quản trị được đổi ngữ cảnh')
                if (model.context_window !== undefined && model.context_window !== 0 && (!Number.isInteger(model.context_window) || model.context_window < 8000 || model.context_window > 2000000)) return fail(res, 400, 'Ngữ cảnh phải từ 8.000 đến 2.000.000')
              }
            }
            const versionKey = `config:${docKey(scope, scope === 'global' ? '' : project, scope === 'story' ? story : '')}`
            if (mockConflict(req, res, versionKey, configResponse(scope, project, story), body)) return
            // 409 model_in_use: bỏ model đã lưu mà cấu hình mới vẫn gán cho trợ lý (Mặc định · vai · dự phòng).
            if (scope === 'global') {
              const savedConfig = (configStore.get(docKey('global', '', '')) ?? CONFIG_GLOBAL) as Config
              const next = body as unknown as Config
              const nextProviders = next.providers ?? {}
              const topProvider = next.provider ?? ''
              const topModel = next.model ?? ''
              for (const [pid, savedProvider] of Object.entries(savedConfig.providers ?? {})) {
                const still = new Set((nextProviders[pid]?.models ?? []).map((model) => model.name))
                const removed = (savedProvider.models ?? []).map((model) => model.name).filter((modelName) => !still.has(modelName))
                for (const modelName of removed) {
                  const roles: string[] = []
                  const add = (label: string) => { if (!roles.includes(label)) roles.push(label) }
                  if (topProvider === pid && topModel === modelName) add('Mặc định')
                  for (const [key, role] of Object.entries(next.roles ?? {})) {
                    const label = ROLE_LABELS_MOCK[key] ?? key
                    if ((role.provider || topProvider) === pid && (role.model || topModel) === modelName) add(key === 'default' ? 'Mặc định' : label)
                    if ((role.fallbacks ?? []).some((fb) => fb.provider === pid && fb.model === modelName)) add(`${key === 'default' ? 'Mặc định' : label} (dự phòng)`)
                  }
                  if (roles.length) {
                    return json(res, 409, {
                      error: `Đang dùng cho ${roles.join(', ')}. Đổi model của trợ lý đó trước khi bỏ chọn.`,
                      code: 'model_in_use', provider: pid, model: modelName, roles,
                    })
                  }
                }
              }
            }
            if (scope !== 'global') {
              const providers = (body.providers ?? {}) as Record<string, { api_key?: string; api_key_set?: boolean }>
              if (Object.values(providers).some((provider) => provider.api_key || provider.api_key_set)) {
                return fail(res, 400, 'Cấu hình dự án/truyện không nhận khoá API')
              }
              if (body.notify) return fail(res, 400, 'Cấu hình dự án/truyện không nhận thiết lập thông báo')
            }
            configStore.set(docKey(scope, scope === 'global' ? '' : project, scope === 'story' ? story : ''), applyProviderKeys(body as unknown as Config))
            bumpMockVersion(versionKey)
            emitGlobalEvent('config.changed', { scope, project: project || undefined, story: story || undefined })
            return json(res, 200, withAccounts(redactConfig(configResponse(scope, project, story), isAdmin), isAdmin))
          }
          return json(res, 200, withAccounts(redactConfig(configResponse(scope, project, story), isAdmin), isAdmin))
        }
        if (seg[0] === 'rules' && (seg[1] === 'understand' || seg[1] === 'effective')) {
          if (await handleRulesGlobal({ req, res, seg, store: ruleStore, readBody: () => readBody(req) })) return
        }
        if (seg[0] === 'rules') {
          const scopeKey = `${scope}:${project}:${story}`
          const base: Record<ConfigScope, { name: string; size: number; updatedAt: string }[]> = {
            global: [
              { name: 'README.md', size: 420, updatedAt: iso(NOW - 30 * 86400000) },
              { name: '10-chan.md', size: 1200, updatedAt: iso(NOW - 20 * 86400000) },
              { name: '20-giong-van.md', size: 800, updatedAt: iso(NOW - 15 * 86400000) },
            ],
            project: [
              { name: 'README.md', size: 180, updatedAt: iso(NOW - 12 * 86400000) },
              { name: '30-nhan-vat.md', size: 940, updatedAt: iso(NOW - 9 * 86400000) },
            ],
            story: [{ name: '40-chuong-5.md', size: 360, updatedAt: iso(NOW - 2 * 86400000) }],
          }
          // Tệp PUT chỉ thuộc đúng scope hiện tại (như handler thật)
          const stored = [...ruleStore.entries()]
            .filter(([key]) => key.startsWith(`${scopeKey}:`))
            .map(([key, text]) => ({ name: key.slice(scopeKey.length + 1), size: text.length, updatedAt: iso(Date.now()) }))
          const filesOf = (s: ConfigScope) => (s === scope
            ? [...base[s].map((seed) => stored.find((file) => file.name === seed.name) ?? seed), ...stored.filter((file) => !base[s].some((seed) => seed.name === file.name))].filter((file) => !deletedRuleFiles.has(`${scopeKey}:${file.name}`))
            : base[s])
          if (seg[1] === 'file' && !seg[2]) {
            const name = url.searchParams.get('name') ?? 'README.md'
            if (req.method === 'PUT') {
              const body = await readBody(req)
              const written = String(body.name ?? name)
              const versionKey = `rule:${scopeKey}:${written}`
              const current = { name: written, text: ruleStore.get(`${scopeKey}:${written}`) ?? `# ${written}\n\nNội dung quy tắc viết cho phạm vi ${scope}.`, version: mockVersion(versionKey) }
              if (mockConflict(req, res, versionKey, current, body)) return
              if (!/^[\p{L}\p{N}._-]+\.md$/u.test(written)) return fail(res, 400, 'Tên tệp quy tắc phải kết thúc bằng .md')
              const text = String(body.text ?? '')
              ruleStore.set(`${scopeKey}:${written}`, text)
              deletedRuleFiles.delete(`${scopeKey}:${written}`)
              bumpMockVersion(versionKey)
              emitGlobalEvent('config.changed', { scope, project: project || undefined, story: story || undefined, section: 'rules' })
              return json(res, 200, { name: written, size: text.length, updatedAt: iso(Date.now()), version: mockVersion(versionKey) })
            }
            if (!filesOf(scope).some((file) => file.name === name)) return fail(res, 404, `Không có tệp quy tắc: ${name}`)
            const body = ruleStore.get(`${scopeKey}:${name}`)
            return json(res, 200, { name, text: body ?? `# ${name}\n\nNội dung quy tắc viết cho phạm vi ${scope}.`, version: mockVersion(`rule:${scopeKey}:${name}`) })
          }
          if (seg[1] === 'file' && seg[2] === 'delete') {
            const body = await readBody(req)
            ruleStore.delete(`${scopeKey}:${String(body.name ?? '')}`)
            deletedRuleFiles.add(`${scopeKey}:${String(body.name ?? '')}`)
            bumpMockVersion(`rule:${scopeKey}:${String(body.name ?? '')}`)
            emitGlobalEvent('config.changed', { scope, project: project || undefined, story: story || undefined, section: 'rules' })
            return json(res, 200, { ok: true })
          }
          return json(res, 200, {
            chain: [
              { scope: 'global', dir: '~/.ainovel/rules', exists: true, files: filesOf('global') },
              ...(project ? [{ scope: 'project', dir: `${project}/.ainovel/rules`, exists: true, files: filesOf('project') }] : []),
              ...(project && story ? [{ scope: 'story', dir: `${project}/${story}/.ainovel/rules`, exists: true, files: filesOf('story') }] : []),
            ],
          })
        }
        // G1 — một chỗ tính thư mục cho cả POST (mở) và GET (chỉ trả đường dẫn).
        const revealPathOf = (target: string, p: string, s: string, ruleScope: string) => {
          const root = mockLibraryRoot()
          const folder: Record<string, string> = {
            library: root,
            project: `${root}/${p}`,
            story: `${root}/${p}/${s}`,
            exports: `${root}/${p}/${s}/xuat-ban`,
            audio: `${root}/${p}/${s}/audio`,
            activity: `${root}/${p}/${s}/output/novel/meta`,
            // D12 — kho ghi chú Não truyện (Obsidian): của truyện trong output/novel, của dự án cạnh truyện.
            'nao-story': `${root}/${p}/${s}/output/novel/nao`,
            'nao-project': `${root}/${p}/nao`,
            rules: ruleScope === 'global'
              ? '~/.ainovel/rules'
              : `${root}/${p}${ruleScope === 'story' ? `/${s}` : ''}/.ainovel/rules`,
            config: ruleScope === 'global'
              ? '~/.ainovel/config.json'
              : `${root}/${p}${ruleScope === 'story' ? `/${s}` : ''}/.ainovel/config.json`,
          }
          const path = folder[target]
          if (!path || user!.role === 'admin') return path
          return path === root ? '.' : path.replace(`${root}/`, '')
        }
        /** D12: `nao-*` chỉ trả `{ok}`/`{canReveal}` — không đường dẫn (invariant #7). */
        const isNaoTarget = (target: string) => target === 'nao-story' || target === 'nao-project'
        if (seg[0] === 'system' && seg[1] === 'network' && req.method === 'PUT') {
          if (user!.role !== 'admin') return fail(res, 403, 'Không có quyền')
          const body = await readBody(req)
          await sleep(1500)
          lanOpen = body.lan === true
          if (!lanOpen) return json(res, 200, { lan: false, links: [] })
          const code = crypto.randomUUID().replaceAll('-', '')
          const expires = Date.now() + 5 * 60_000
          pairings.set(code, { code, username: user!.username, expiresAt: expires })
          const host = req.headers.host ?? 'localhost:5173'
          return json(res, 200, { lan: true, code, expiresAt: iso(expires), links: [{ label: 'Wi-Fi nhà', url: `http://${host}/p/${code}`, svg: qrSvg(`http://${host}/p/${code}`) }] })
        }
        // D15 · GAP-46 — Não hệ thống, chỉ quản trị (serve thật: internal/entry/web/lessons_api.go).
        if (seg[0] === 'system' && (seg[1] === 'lessons' || seg[1] === 'violations')) {
          if (user!.role !== 'admin') return fail(res, 403, 'Chỉ quản trị viên xem Não hệ thống')
          if (seg[1] === 'violations' && req.method === 'GET') {
            const found = mockViolations(url.searchParams.get('pack') ?? '')
            return found ? json(res, 200, found) : fail(res, 404, 'Không tìm thấy gói')
          }
          if (seg[1] === 'lessons' && !seg[2] && req.method === 'GET') return json(res, 200, mockLessons((url.searchParams.get('state') || 'pending') as LessonState))
          if (seg[1] === 'lessons' && seg[2] && req.method === 'POST') {
            const { status, body } = mockDecideLesson(seg[2], await readBody(req) as Parameters<typeof mockDecideLesson>[1])
            return 'error' in body ? fail(res, status, body.error) : json(res, status, body)
          }
          return fail(res, 405, 'Không hỗ trợ thao tác này')
        }
        if (seg[0] === 'system' && seg[1] === 'pick-folder') {
          // G2 — hộp chọn thư mục của hệ điều hành; serve thật 501 khi thiếu osascript/zenity/kdialog.
          if (user!.role !== 'admin') return fail(res, 403, 'Chỉ quản trị viên đổi thư viện')
          if (process.env.AINOVEL_MOCK_PICK_UNSUPPORTED === '1') return fail(res, 501, 'Máy chủ không hỗ trợ chọn thư mục')
          if (process.env.AINOVEL_MOCK_PICK_CANCEL === '1') return json(res, 200, { cancelled: true })
          return json(res, 200, { path: '/mock/home/Library/Mobile Documents/com~apple~CloudDocs/Truyện của Dat' })
        }
        if (seg[0] === 'system') {
          const admin = user!.role === 'admin'
          // D1 — bản đang chạy + bản mới nhất; `POST /__mock/update {on:true}` bật banner/badge/hộp “Có gì mới”.
          return json(res, 200, {
            version: 'v0.9.3', commit: 'a41c9e2f4d8b7c6a5e4d3c2b1a09f8e7d6c5b4a3', built: '2026-10-24T14:02:00+07:00',
            root: admin ? mockLibraryRoot() : '', rootDisplay: mockDisplay(mockLibraryRoot()), retries: 7, deadlock: [3, 5],
            compactThreshold: 0.85, promptCache: true, outputDir: './output/novel', defaultContextWindow: 200000,
            canReveal: admin, lan: lanOpen,
            latestVersion: mockHasUpdate ? 'v0.9.4' : 'v0.9.3', hasUpdate: mockHasUpdate,
            // D1 — “Có gì mới”: bản mới trước (khi có), rồi bản đang chạy; `notes` là markdown nguyên văn.
            releases: mockReleases(mockHasUpdate),
          })
        }
        // G1 — chỉ trả đường dẫn (nút “Chép đường dẫn” khi canReveal=false); không mở gì.
        if (seg[0] === 'reveal' && req.method === 'GET') {
          const target = url.searchParams.get('target') ?? ''
          const p = url.searchParams.get('project') ?? project
          const s = url.searchParams.get('story') ?? story
          const ruleScope = url.searchParams.get('scope') ?? 'global'
          if (isNaoTarget(target)) {
            if (user!.role !== 'admin') return fail(res, 403, 'Không có quyền')
            return json(res, 200, { canReveal: true })
          }
          const path = revealPathOf(target, p, s, ruleScope)
          if (!path) return fail(res, 400, 'Không rõ thư mục cần mở')
          if (target === 'library' || ((target === 'rules' || target === 'config') && ruleScope === 'global')) {
            if (user!.role !== 'admin') return fail(res, 403, 'Chỉ quản trị viên xem thư mục này')
          }
          if (s && !canRead(user!, `${p}/${s}`)) return fail(res, 404, 'Không tìm thấy truyện')
          return json(res, 200, { path, canReveal: user!.role === 'admin' })
        }
        // G1 — mở thư mục trên máy chủ (mock luôn coi như loopback).
        if (seg[0] === 'reveal' && req.method === 'POST') {
          if (user!.role !== 'admin') return fail(res, 403, 'Không có quyền')
          const body = await readBody(req)
          const target = String(body.target ?? '')
          const p = String(body.project ?? project)
          const s = String(body.story ?? story)
          const path = revealPathOf(target, p, s, String(body.scope ?? 'global'))
          if (!path) return fail(res, 400, 'Không rõ thư mục cần mở')
          // D12: kho ghi chú mở bằng Obsidian, trả `{ok}` không kèm đường dẫn; `AINOVEL_MOCK_NAO_MISSING=1`
          // dựng trạng thái 404 “chưa có kho ghi chú” để thử nhánh lỗi.
          if (isNaoTarget(target)) {
            if (process.env.AINOVEL_MOCK_NAO_MISSING === '1') {
              return fail(res, 404, target === 'nao-story' ? 'Chưa có kho ghi chú của truyện' : 'Chưa có kho ghi chú của dự án')
            }
            return json(res, 200, { ok: true })
          }
          return json(res, 200, { ok: true, path })
        }
        next()
      })
    },
  }
}

const oauthStates = new Map<string, { started: number; done: boolean; provider: string; device: boolean; paste: boolean; pasted: boolean }>()

/** Tài khoản giả lập của một provider OAuth đã kết nối (Claude: email · tổ chức, không có gói). */
function mockAccount(provider: string): ConfigAccount {
  if (provider === 'anthropic-oauth') return { connected: true, label: 'dat@example.com · Truyện Việt', email: 'dat@example.com', plan: '', expiresAt: iso(NOW + 20 * 86400000) }
  return { connected: true, label: 'dat@example.com', email: 'dat@example.com', plan: provider === 'xai-oauth' ? '' : 'Plus', expiresAt: iso(NOW + 20 * 86400000) }
}
// G8 — phiên đồng sáng tác cấp dự án, key "<dự án>/<draftId>"; chưa có truyện cho tới /commit.
const projectDrafts = new Map<string, CoCreateState>()
let draftSeq = 0
// Trạng thái kết nối tài khoản nhà cung cấp: logout → false, OAuth xong → true
const accountConnected = new Map<string, boolean>()
// Tên hiển thị dự án (project.json › name); tạo mới qua POST /api/projects.
// Dự án `_` (legacy) không có project.json — serve thật trả name = "_" (11-api-contract.md §3).
const projectsExtra = new Map<string, string>([['dit-dang-audio', 'Đít Đang Audio'], ['cu-dan', 'Cư Dân'], ['_', '_']])
// .ainovel/library.json — PUT /api/library ghi đè (mock giữ trong bộ nhớ)
let libraryConfig: LibraryConfig = {
  version: 1,
  dirs: { ideas: 'y-tuong', assets: 'tai-san', stories: 'kich_ban', exports: 'xuat-ban' },
  slugWords: 5,
  autoExport: { onVolume: true, onBook: true, onChapter: false, formats: ['txt', 'epub'], pattern: 'tap-{tap}/{truyen}.tap-{tap}.{ext}', onExists: 'keep' },
}
/** Cùng thứ tự/nhãn/defaults với bootstrap.ProviderPresets và web.servePresets. */
const MOCK_PROVIDER_PRESETS: ProviderPreset[] = [
  { id: 'ollama', label: 'Ollama (Cục bộ / Offline - Miễn phí)', type: 'ollama', baseURL: 'http://localhost:11434/v1', keyRequired: false, oauth: false, api: '', defaultModel: 'qwen2.5:14b' },
  { id: 'openrouter', label: 'OpenRouter (Claude, Gemini, DeepSeek, Qwen...)', type: 'openrouter', baseURL: 'https://openrouter.ai/api/v1', keyRequired: true, oauth: false, api: '', defaultModel: 'google/gemini-2.5-flash' },
  { id: 'opencode-go', label: 'OpenCode Go (Kimi, DeepSeek, GLM, Qwen, MiniMax… - gói thuê bao, dùng API key)', type: 'opencode-go', baseURL: 'https://opencode.ai/inference/go/v1', keyRequired: true, oauth: false, api: '', defaultModel: 'kimi-k2.7-code' },
  { id: 'openai-codex', label: 'ChatGPT Plus/Pro (Codex Subscription - đăng nhập OAuth)', type: 'openai-codex', baseURL: '', keyRequired: false, oauth: true, api: 'responses', defaultModel: 'gpt-5.6-terra' },
  { id: 'xai-oauth', label: 'SuperGrok / X Premium+ (Gói thuê bao xAI - đăng nhập OAuth)', type: 'xai-oauth', baseURL: '', keyRequired: false, oauth: true, deviceOnly: true, api: '', defaultModel: 'grok-4.6' },
  { id: 'anthropic-oauth', label: 'Claude Pro/Max (Gói thuê bao Anthropic - đăng nhập OAuth)', type: 'anthropic-oauth', baseURL: '', keyRequired: false, oauth: true, pasteCode: true, api: '', defaultModel: 'claude-sonnet-4-6' },
  { id: 'gemini', label: 'Google Gemini', type: 'gemini', baseURL: '', keyRequired: true, oauth: false, api: '', defaultModel: 'gemini-2.5-flash' },
  { id: 'anthropic', label: 'Anthropic Claude', type: 'anthropic', baseURL: '', keyRequired: true, oauth: false, api: '', defaultModel: 'claude-sonnet-4' },
  { id: 'deepseek', label: 'DeepSeek', type: 'deepseek', baseURL: 'https://api.deepseek.com/v1', keyRequired: true, oauth: false, api: '', defaultModel: 'deepseek-chat' },
  { id: 'openai', label: 'OpenAI', type: 'openai', baseURL: '', keyRequired: true, oauth: false, api: '', defaultModel: 'gpt-4o-mini' },
  { id: 'qwen', label: 'Alibaba Qwen (DashScope)', type: 'qwen', baseURL: '', keyRequired: true, oauth: false, api: '', defaultModel: 'qwen-plus' },
  { id: 'glm', label: 'Zhipu GLM', type: 'glm', baseURL: '', keyRequired: true, oauth: false, api: '', defaultModel: 'glm-4.6' },
  { id: 'grok', label: 'xAI Grok', type: 'grok', baseURL: '', keyRequired: true, oauth: false, api: '', defaultModel: 'grok-4' },
  { id: 'bedrock', label: 'AWS Bedrock', type: 'bedrock', baseURL: '', keyRequired: false, oauth: false, api: '', defaultModel: 'anthropic.claude-sonnet-4' },
  { id: 'custom', label: 'Custom Proxy (Proxy / API tùy chỉnh)', type: '', baseURL: '', keyRequired: false, oauth: false, api: '', defaultModel: '' },
]
// ─── Danh sách model giả lập của POST /api/config/models/list ─────────

/** Nhãn vai trò trong 409 model_in_use: vai kỹ thuật giữ nguyên key. */
const ROLE_LABELS_MOCK: Record<string, string> = { architect: 'Kiến trúc sư', writer: 'Người viết', editor: 'Biên tập viên' }

type MockListedModel = { name: string; displayName: string; description?: string; contextWindow: number; price?: { input: number; output: number }; sug?: boolean }

/** OpenRouter ~312 model (giá USD/1 triệu token): hàng đầu là model hợp viết truyện tiếng Việt. */
const MOCK_OPENROUTER_MODELS: MockListedModel[] = (() => {
  const seed: MockListedModel[] = [
    { name: 'qwen/qwen3-235b-a22b', displayName: 'Qwen3 235B', description: 'Viết tiếng Việt tốt', contextWindow: 131072, price: { input: 0.13, output: 0.6 }, sug: true },
    { name: 'deepseek/deepseek-r1', displayName: 'DeepSeek R1', description: 'Suy nghĩ kỹ', contextWindow: 163840, price: { input: 0.4, output: 2 }, sug: true },
    { name: 'google/gemini-2.5-pro', displayName: 'Gemini 2.5 Pro', contextWindow: 1048576, price: { input: 1.25, output: 10 }, sug: true },
    { name: 'google/gemini-2.5-flash', displayName: 'Gemini 2.5 Flash', contextWindow: 1048576, price: { input: 0.3, output: 2.5 }, sug: true },
    { name: 'anthropic/claude-sonnet-4.5', displayName: 'Claude Sonnet 4.5', contextWindow: 1000000, price: { input: 3, output: 15 }, sug: true },
    { name: 'moonshotai/kimi-k2', displayName: 'Kimi K2', contextWindow: 131072, price: { input: 0.55, output: 2.2 } },
    { name: 'meta-llama/llama-4-maverick', displayName: 'Llama 4 Maverick', contextWindow: 1048576, price: { input: 0.15, output: 0.6 } },
    { name: 'mistralai/mistral-large', displayName: 'Mistral Large', contextWindow: 131072, price: { input: 2, output: 6 } },
    { name: 'x-ai/grok-4', displayName: 'Grok 4', contextWindow: 256000, price: { input: 3, output: 15 } },
    { name: 'qwen/qwen3-30b-a3b', displayName: 'Qwen3 30B', description: 'Rẻ, nhanh', contextWindow: 40960, price: { input: 0.08, output: 0.29 } },
    { name: 'qwen/qwen3-coder', displayName: 'Qwen3 Coder', description: 'Thiên về lập trình', contextWindow: 262144, price: { input: 0.22, output: 0.95 } },
    { name: 'deepseek/deepseek-v3.2', displayName: 'DeepSeek V3.2', contextWindow: 163840, price: { input: 0.28, output: 0.42 }, sug: true },
  ]
  const makers = ['qwen', 'deepseek', 'google', 'anthropic', 'meta-llama', 'mistralai', 'x-ai', 'moonshotai', 'openai', 'cohere', 'nous', 'microsoft']
  const out = [...seed]
  let i = 0
  while (out.length < 312) {
    const maker = makers[i % makers.length]
    out.push({
      name: `${maker}/model-${Math.floor(i / makers.length) + 1}${['a', 'b', 'c'][i % 3]}`,
      displayName: `${maker} model ${i + 1}`,
      contextWindow: [32768, 65536, 131072, 200000, 262144, 1048576][i % 6],
      price: { input: Math.round((0.05 + (i % 17) * 0.3) * 100) / 100, output: Math.round((0.2 + (i % 13) * 1.1) * 100) / 100 },
    })
    i++
  }
  return out
})()

/** Danh sách ngắn cho nhà cung cấp khoá API không phải OpenRouter (GET {base}/models). */
const MOCK_KEY_PROVIDER_MODELS: Record<string, MockListedModel[]> = {
  openai: [{ name: 'gpt-5.4', displayName: 'GPT-5.4', contextWindow: 320000, sug: true }, { name: 'gpt-4o-mini', displayName: 'GPT-4o mini', contextWindow: 128000 }, { name: 'o4-mini', displayName: 'o4-mini', contextWindow: 200000 }],
  deepseek: [{ name: 'deepseek-chat', displayName: 'DeepSeek Chat', description: 'Rẻ', contextWindow: 128000, sug: true }, { name: 'deepseek-reasoner', displayName: 'DeepSeek Reasoner', description: 'Suy nghĩ kỹ, chậm hơn', contextWindow: 128000 }],
  gemini: [{ name: 'gemini-2.5-pro', displayName: 'Gemini 2.5 Pro', description: 'Ngữ cảnh dài', contextWindow: 1048576, sug: true }, { name: 'gemini-2.5-flash', displayName: 'Gemini 2.5 Flash', description: 'Nhanh, rẻ', contextWindow: 1048576, sug: true }, { name: 'gemini-2.0-flash', displayName: 'Gemini 2.0 Flash', description: 'Đời cũ', contextWindow: 1048576 }],
  anthropic: [{ name: 'claude-opus-4-8', displayName: 'Claude Opus 4.8', contextWindow: 200000 }, { name: 'claude-sonnet-4-6', displayName: 'Claude Sonnet 4.6', contextWindow: 200000, sug: true }, { name: 'claude-haiku-4-5', displayName: 'Claude Haiku 4.5', contextWindow: 200000 }],
  qwen: [{ name: 'qwen-plus', displayName: 'Qwen Plus', contextWindow: 131072, sug: true }, { name: 'qwen-max', displayName: 'Qwen Max', contextWindow: 131072 }, { name: 'qwen-turbo', displayName: 'Qwen Turbo', contextWindow: 131072 }],
  glm: [{ name: 'glm-4.6', displayName: 'GLM 4.6', contextWindow: 200000, sug: true }, { name: 'glm-4.5-air', displayName: 'GLM 4.5 Air', contextWindow: 131072 }],
  grok: [{ name: 'grok-4', displayName: 'Grok 4', contextWindow: 256000, sug: true }, { name: 'grok-3-mini', displayName: 'Grok 3 mini', contextWindow: 131072 }],
  'opencode-go': [{ name: 'kimi-k2.7-code', displayName: 'Kimi K2.7 Code', contextWindow: 131072, sug: true }, { name: 'deepseek-v4', displayName: 'DeepSeek V4', contextWindow: 163840 }, { name: 'minimax-m2', displayName: 'MiniMax M2', contextWindow: 200000 }],
}

/** Cache fetchedAt theo phiên mock (đủ cho “Lấy từ … lúc …”; refresh:true gọi lại upstream). */
const modelListCache = new Map<string, string>()

/**
 * ProviderModelList của mock: registryWindow = danh mục Noverse (mockModels); recommended =
 * danh mục biết / defaultModel của preset / cờ `sug` của fixture; contextSource =
 * model_config (context_window trong bản nháp) > provider (danh sách báo) > registry > default.
 */
function modelListResponse(models: MockListedModel[], providerConf: { models?: { name: string; context_window?: number }[] }, preset: ProviderPreset | undefined, hasPrice: boolean) {
  const registry = new Map(mockModels().providers.flatMap((entry) => entry.models).map((entry) => [entry.name.toLowerCase(), entry.registryWindow]))
  const configured = new Map((providerConf?.models ?? []).map((model) => [model.name, model.context_window]))
  const fetchedAt = modelListCache.get('at') ?? iso(Date.now())
  modelListCache.set('at', fetchedAt)
  return {
    models: models.map((model) => {
      const registryWindow = registry.get(model.name.toLowerCase()) ?? 0
      const own = configured.get(model.name)
      return {
        name: model.name, displayName: model.displayName || model.name, ...(model.description ? { description: model.description } : {}),
        contextWindow: own || model.contextWindow || registryWindow || 200000,
        contextSource: own ? 'model_config' : model.contextWindow ? 'provider' : registryWindow ? 'registry' : 'default',
        registryWindow,
        ...(hasPrice && model.price ? { price: model.price } : {}),
        recommended: !!model.sug || registryWindow > 0 || preset?.defaultModel === model.name,
      }
    }),
    total: models.length,
    fetchedAt,
  }
}
// Ứng viên "nhận làm dự án": thư mục con chưa có project.json nhưng có thư mục truyện

const candidates = ['kho-truyen-cu', 'ban-thao-2024']
const projectList = () => [...projectsExtra.entries()].map(([id, name]) => {
  const own = [...stories.values()].filter((s) => s.p === id)
  return {
    id, name, description: id === 'dit-dang-audio' ? 'Truyện chuyển audio' : '', legacy: id === '_',
    storyCount: own.length, runningCount: own.filter((s) => s.status === 'writing').length,
    updatedAt: own.length ? iso(NOW - 3600000) : null,
  }
})
// Tên/mô tả/ghi chú/tên vũ trụ của dự án đã PUT (project form auto-save)
const projectMeta = new Map<string, { description: string; notes: string; universeName: string }>()
const configStore = new Map<string, Record<string, unknown>>()

/** OAuth xong/logout: ghi trạng thái kết nối vào config global để GET /api/config thấy sau khi tải lại. */
const setProviderConnected = (id: string, connected: boolean) => {
  const current = (configStore.get(docKey('global', '', '')) ?? CONFIG_GLOBAL) as Config
  const provider = current.providers?.[id]
  if (!provider) return
  configStore.set(docKey('global', '', ''), {
    ...current,
    providers: { ...current.providers, [id]: { ...provider, api_key_set: connected, api_key_hint: connected ? 'oauth' : '' } },
  } as unknown as Record<string, unknown>)
}
// Nội dung tệp quy tắc đã PUT: key = "<scope>:<project>:<story>:<tên tệp>"
const ruleStore = new Map<string, string>()
const deletedRuleFiles = new Set<string>()

// Tài liệu cấu hình mặc định 3 cấp (11-api-contract.md §6); PUT ghi đè trong configStore.
const CONFIG_GLOBAL: Config = {
  provider: 'openai-codex', model: 'gpt-5.4', reasoning_effort: 'high', style: 'default', context_window: 320000,
  providers: {
    'openai-codex': { type: 'openai-codex', api: 'responses', base_url: '', models: [{ name: 'gpt-5.4', context_window: 320000, json_schema: true }], api_key_set: true, api_key_hint: 'oauth' },
    anthropic: { type: 'anthropic', api: 'chat', base_url: 'https://api.anthropic.com', models: [{ name: 'claude-sonnet-4', context_window: 200000 }], api_key_set: false },
  },
  roles: {
    default: { provider: 'openai-codex', model: 'gpt-5.4', reasoning_effort: 'high', fallbacks: [{ provider: 'anthropic', model: 'claude-sonnet-4' }] },
    architect: { provider: 'openai-codex', model: 'o4-mini', reasoning_effort: 'medium' },
    writer: { provider: 'openai-codex', model: 'gpt-5.4', reasoning_effort: 'high' },
    editor: { provider: 'openai-codex', model: 'gpt-5.4', reasoning_effort: 'medium' },
    import_segment: { provider: 'openai-codex', model: 'o4-mini', reasoning_effort: 'low' },
    import_analyze: { provider: 'openai-codex', model: 'o4-mini', reasoning_effort: 'medium' },
    import_synthesize: { provider: 'openai-codex', model: 'gpt-5.4', reasoning_effort: 'medium' },
  },
  budget: { book_usd: 50, warn_ratio: 0.8, hard_stop: true },
  notify: { enabled: true, command: 'notify-send "$NOTIFY_TITLE" "$NOTIFY_BODY"', events: ['run_end', 'budget', 'advance_gate', 'stop_guard'] },
  advance_mode: 'auto',
}
const CONFIG_PROJECT: Config = {
  roles: { writer: { provider: 'openai-codex', model: 'o4-mini', reasoning_effort: 'high' } },
  style: 'romance', budget: { book_usd: 80 },
}
const CONFIG_STORY: Config = {
  advance_mode: 'review',
  // Gói thể loại của truyện (D2/D6/D11): `style` = id gói để `findPack()` ra tên gói + giọng đọc của gói.
  style: 'nu-tan-bao-thu',
  roles: { editor: { provider: 'anthropic', model: 'claude-sonnet-4', reasoning_effort: 'medium' } },
}

const docKey = (scope: ConfigScope, project: string, story: string) => `${scope}:${project}:${story}`

/** slugify theo library.slugWords: bỏ dấu tiếng Việt, nối bằng '-', giữ N từ đầu. */
function slugify(text: string, words: number): string {
  return text.toLowerCase().replace(/đ/g, 'd').normalize('NFD').replace(/[\u0300-\u036f]/g, '')
    .replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
    .split('-').filter(Boolean).slice(0, words).join('-')
}

const isPlainObject = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value)

/** Merge sâu global→project→story: object gộp, mảng thay thế. */
function mergeConfig(base: Config, over: Config): Config {
  const out: Record<string, unknown> = { ...base }
  for (const [key, value] of Object.entries(over)) {
    const prev = out[key]
    out[key] = isPlainObject(value) && isPlainObject(prev)
      ? mergeConfig(prev as unknown as Config, value as Config)
      : value
  }
  return out as Config
}

function flattenConfig(value: unknown, prefix = '', out: Record<string, unknown> = {}): Record<string, unknown> {
  if (isPlainObject(value)) {
    for (const [key, child] of Object.entries(value)) flattenConfig(child, prefix ? `${prefix}.${key}` : key, out)
  } else if (prefix) {
    out[prefix] = value
  }
  return out
}

function configDocs(project: string, story: string): ConfigDoc[] {
  const seededProject = project === 'dit-dang-audio'
  const seededStory = seededProject && story === '01-tra-xanh-cuop-kich-ban'
  const docs: ConfigDoc[] = [{
    scope: 'global', path: '~/.ainovel/config.json', exists: true,
    config: configStore.get(docKey('global', '', '')) ?? CONFIG_GLOBAL,
  }]
  if (project) {
    const stored = configStore.get(docKey('project', project, ''))
    docs.push({
      scope: 'project', path: `${project}/.ainovel/config.json`, exists: stored !== undefined || seededProject,
      config: stored ?? (seededProject ? CONFIG_PROJECT : {}),
    })
  }
  if (project && story) {
    const stored = configStore.get(docKey('story', project, story))
    docs.push({
      scope: 'story', path: `${project}/${story}/.ainovel/config.json`, exists: stored !== undefined || seededStory,
      config: stored ?? (seededStory ? CONFIG_STORY : {}),
    })
  }
  return docs
}

/** ConfigResponse: effective = merge chuỗi; sources = cấp của từng lá trong effective. */
function configResponse(scope: ConfigScope, project: string, story: string): ConfigResponse {
  const chain = configDocs(project, story)
  const effective = chain.reduce((acc, doc) => mergeConfig(acc, doc.config), {} as Config)
  const sources: Record<string, ConfigScope> = {}
  for (const doc of chain) {
    for (const path of Object.keys(flattenConfig(doc.config))) sources[path] = doc.scope
  }
  // api_key không bao giờ nằm trong effective; vẫn khai báo nguồn cho badge ô khoá
  for (const [id, provider] of Object.entries(effective.providers ?? {})) {
    if (provider.api_key_set) sources[`providers.${id}.api_key`] = sources[`providers.${id}.api_key_set`] ?? 'global'
  }
  const target = chain.find((doc) => doc.scope === scope) ?? chain[chain.length - 1]
  return { target, effective, chain, sources, version: mockVersion(`config:${docKey(scope, scope === 'global' ? '' : project, scope === 'story' ? story : '')}`) }
}

/** PUT: api_key → api_key_set + hint; clear_api_key → bỏ; không bao giờ giữ/echo api_key. */
function applyProviderKeys(body: Config): Config {
  const providers = { ...(body.providers ?? {}) }
  for (const [id, provider] of Object.entries(providers)) {
    const key = provider.api_key
    providers[id] = provider.clear_api_key
      ? { ...provider, api_key: undefined, api_key_set: false, api_key_hint: '', clear_api_key: undefined }
      : key
        ? { ...provider, api_key: undefined, api_key_set: true, api_key_hint: `sk-…${key.slice(-4)}`, clear_api_key: undefined }
        : { ...provider, api_key: undefined, clear_api_key: undefined }
  }
  return { ...body, providers }
}

/** G7 — tài khoản OAuth đã kết nối theo provider id (serve thật đọc từ auth.json; mock giữ trong bộ nhớ). */
function withAccounts(response: ConfigResponse, admin: boolean): ConfigResponse {
  if (!admin) return response
  const accounts: Record<string, ConfigAccount> = {}
  for (const id of Object.keys(response.effective.providers ?? {})) {
    if (!MOCK_PROVIDER_PRESETS.find((preset) => preset.id === id)?.oauth) continue
    accounts[id] = accountConnected.get(id) === false
      ? { connected: false, label: '', email: '', plan: '', expiresAt: null }
      : mockAccount(id)
  }
  return { ...response, accounts }
}

/** Người không phải admin chỉ thấy tên/type/models của provider (11-api-contract.md §6). */
function redactConfig(response: ConfigResponse, admin: boolean): ConfigResponse {
  if (admin) return response
  const strip = (config: Config) => ({
    ...config,
    providers: Object.fromEntries(Object.entries(config.providers ?? {}).map(([id, provider]) => [id, { type: provider.type, models: provider.models }])),
  }) as unknown as Config
  return {
    ...response,
    target: { ...response.target, config: strip(response.target.config) },
    effective: strip(response.effective),
    chain: response.chain.map((doc) => ({ ...doc, config: strip(doc.config) })),
  }
}

import { MOBILE_READER_CHAPTERS, MOBILE_READER_PARAGRAPHS } from './data'

// §AI — giả lập “Người viết sửa xong”: đổi văn theo góp ý sent, giải lại mốc neo
// (anchor), đặt status/fixedAt/newQuote/candidates rồi phát comments.reanchored.
function simulateRewrite(st: MockStory, chapter: number) {
  const key = `${st.p}/${st.s}/${chapter}`
  const list = paragraphFeedback.get(key) ?? []
  const sent = list.filter((comment) => comment.status === 'sent')
  if (sent.length === 0) return
  const ch = st.chapters[chapter]
  if (!ch) return
  let text = ch.text
  const paras = text.split(/\n\n+/)
  let fixed = 0, unfixed = 0, need = 0
  const now = new Date().toISOString()
  sent.forEach((comment, index) => {
    const at = paras.findIndex((para) => comment.quote && para.includes(comment.quote))
    if (index % 4 === 0 && at >= 0) {
      // Đã sửa: thay trích bằng câu đã sửa (nhạy hơn, theo chips).
      const rewritten = `${comment.quote.replace(/[.!…]?$/, '')} — ngắn lại, sắc hơn.`
      paras[at] = paras[at].replace(comment.quote, rewritten)
      comment.status = 'fixed'; comment.fixedAt = now; comment.fixedBy = 'agent'
      comment.newQuote = rewritten
      comment.anchor = { paragraph: at, quote: rewritten }
      fixed++
    } else if (index % 4 === 1 && at >= 0) {
      // Chưa sửa: trích vẫn còn → về Chờ gửi với cờ unfixed.
      comment.status = 'pending'; comment.unfixed = true
      comment.anchor = { paragraph: at, quote: comment.quote }
      unfixed++
    } else if (index % 4 === 2 && at >= 0) {
      // Không chắc vị trí: trích mất, gợi ý 2 đoạn lân cận.
      comment.status = 'unsure'; comment.fixedBy = 'quote_gone'
      const lo = Math.max(0, at - 1)
      comment.candidates = [lo, Math.min(paras.length - 1, at + 1)].filter((v, i, arr) => arr.indexOf(v) === i).slice(0, 2)
        .map((para) => ({ paragraph: para, snippet: paras[para].slice(0, 60) + '…' }))
      paras[at] = paras[at].replace(comment.quote, 'Chữ trong câu này đã được viết lại hoàn toàn, không còn dấu vết câu cũ.')
      need++
    } else if (at >= 0) {
      // Mất vị trí: trích mất và không tìm được đoạn thay.
      comment.status = 'lost'; comment.fixedBy = 'quote_gone'; comment.anchor = null
      paras[at] = paras[at].replace(comment.quote, 'Đoạn văn đã đổi cấu trúc, câu cũ không còn nơi gắn.')
      need++
    } else {
      comment.status = 'pending'; comment.unfixed = true
      unfixed++
    }
  })
  ch.text = paras.join('\n\n')
  st.snapshot = { ...st.snapshot, isRunning: false, runtimeState: 'paused', flow: '', pendingRewrites: null, waitingFor: 'gate', hasAdvanceHold: true, advanceHoldReason: `Chờ bạn duyệt chương ${chapter}` }
  st.status = 'paused'
  st.completed = Math.max(st.completed, chapter)
  emitGlobalEvent('comments.reanchored', { project: st.p, story: st.s, chapter, fixed, unfixed, need })
  broadcast(st, 'story.changed', { project: st.p, story: st.s, parts: ['chapters', 'comments', 'snapshot'] })
  broadcast(st, 'snapshot', st.snapshot)
}

// Scénarios M2 isolés: gate, lecture seule et verrouillage restent accessibles via routes réelles.
for (const [slug, locked] of [['m2-reader', false], ['m2-reader-locked', true]] as const) {
  put(mkStory('dit-dang-audio', slug, `kich-ban/${slug}`, 'Kịch bản bị cướp', 'paused', {
    chapters: MOBILE_READER_CHAPTERS, completed: 24, total: 30, advanceMode: 'review', locked,
    snapshot: {
      ...snapshotTraXanh, bookTitle: 'Kịch bản bị cướp', advanceMode: 'review', advancePermitChapter: 24,
      currentChapter: 25, completedCount: 24, totalChapters: 30, inProgressChapter: 0, isRunning: false,
      runtimeState: 'paused', waitingFor: 'gate', hasAdvanceHold: true, advanceHoldReason: 'Chờ bạn duyệt chương 24',
      pendingRewrites: null, outline: outlineFor(30, 24),
    },
  }))
  paragraphFeedback.set(`dit-dang-audio/${slug}/24`, [
    { id: `m2-${slug}-comment`, paragraph: 1, quote: MOBILE_READER_PARAGRAPHS[1],
      chips: ['Giữ giọng văn'], text: 'Giữ chi tiết cây bút chì ở đoạn này.', author: 'dat', createdAt: iso(NOW - 60000), status: 'pending', anchor: { paragraph: 1, quote: MOBILE_READER_PARAGRAPHS[1] } },
    // §AI — các trạng thái vòng đời để soi sheet/banner (fb: unsure · fixed · done).
    { id: `m2-${slug}-unsure`, paragraph: 5, quote: '“Chị thuộc kịch bản của em kỹ thật đấy.”', chips: [], text: 'Câu thoại này nên để Nhược Vy khô hơn.', author: 'dat', createdAt: iso(NOW - 52000), status: 'unsure', fixedBy: 'quote_gone', anchor: null,
      candidates: [{ paragraph: 4, snippet: MOBILE_READER_PARAGRAPHS[4].slice(0, 60) + '…' }, { paragraph: 5, snippet: MOBILE_READER_PARAGRAPHS[5].slice(0, 60) + '…' }] },
    { id: `m2-${slug}-fixed`, paragraph: 4, quote: 'Nhược Vy cười, rất khẽ.', chips: ['Kịch tính hơn'], text: 'Cho phản ứng của cô lộ ra rõ hơn.', author: 'dat', createdAt: iso(NOW - 42000), status: 'fixed', sentAt: iso(NOW - 40000), fixedAt: iso(NOW - 6000), fixedBy: 'agent', newQuote: 'tôi thấy cổ tay cô run lên một nhịp', anchor: { paragraph: 5, quote: 'tôi thấy cổ tay cô run lên một nhịp' } },
    { id: `m2-${slug}-done`, paragraph: 2, quote: '“Mời khách mời đọc đoạn mở đầu,”', chips: ['Ngắn lại'], text: 'Nén phần dẫn của MC.', author: 'dat', createdAt: iso(NOW - 90000), status: 'done', sentAt: iso(NOW - 80000), fixedAt: iso(NOW - 70000), resolvedAt: iso(NOW - 30000) },
  ])
}

// §AI — Góp ý cho demo máy tính: chương 3 của 01-tra-xanh-cuop-kich-ban (đã duyệt).
{
  const t = TRAXANH_CHAPTERS[3]?.text ?? ''
  const paras = t.split(/\n\n+/)
  if (paras.length >= 4) {
    paragraphFeedback.set('dit-dang-audio/01-tra-xanh-cuop-kich-ban/3', [
      { id: 'fb3-p1', paragraph: 1, quote: 'Tống Nhược Vy làm chủ căn phòng.', chips: ['Giữ giọng văn'], text: 'Cho Nhược Vy một câu miêu tả sắc hơn nữa.', author: 'dat', createdAt: iso(NOW - 90000), status: 'pending', anchor: { paragraph: 1, quote: 'Tống Nhược Vy làm chủ căn phòng.' } },
      { id: 'fb3-lost', paragraph: 3, quote: 'đủ để một tác giả biết mình đã thua như thế nào', chips: [], text: 'Câu kết đoạn này hay nhưng hơi mắng người đọc — cân nhắc nhẹ đi.', author: 'dat', createdAt: iso(NOW - 80000), status: 'lost', fixedBy: 'quote_gone', anchor: null },
      { id: 'fb3-fixed', paragraph: 0, quote: '"phòng chiến lược"', chips: ['Kịch tính hơn'], text: 'Tên cũ của phòng nghe hay — nhấn thêm một nhịp.', author: 'dat', createdAt: iso(NOW - 70000), status: 'fixed', sentAt: iso(NOW - 65000), fixedAt: iso(NOW - 5000), fixedBy: 'agent', newQuote: 'nơi nhân viên cũ vẫn gọi là "phòng chiến lược"', anchor: { paragraph: 0, quote: 'nơi nhân viên cũ vẫn gọi là "phòng chiến lược"' } },
      { id: 'fb3-done', paragraph: 2, quote: 'bà chạm gáy từng cuốn', chips: ['Ngắn lại'], text: 'Đã xử lý từ đợt trước — giữ làm mốc lịch sử.', author: 'dat', createdAt: iso(NOW - 60000), status: 'done', sentAt: iso(NOW - 55000), fixedAt: iso(NOW - 45000), resolvedAt: iso(NOW - 40000) },
    ])
  }
}

import { removeStoryJobs } from './jobs'
import { deleteAudioState } from './audio'

async function deleteMockStory(req: IncomingMessage, res: ServerResponse, user: User, st: MockStory) {
  const id = `${st.p}/${st.s}`
  if (!canRead(user, id)) return fail(res, 404, 'Không tìm thấy truyện')
  if (user.role !== 'admin' && user.projects[st.p] !== 'owner') return fail(res, 403, 'Chỉ Chủ dự án được xoá truyện')
  const body = await readBody(req)
  if (body.confirm !== st.s) return fail(res, 400, 'Tên truyện xác nhận không khớp')
  if (st.locked || st.snapshot.isRunning) return fail(res, 409, 'Truyện đang chạy — dừng trước khi xoá')
  clearInterval(st.streamTimer ?? undefined)
  for (const client of st.clients) client.res.end()
  st.clients.clear()
  st.jobs.clear()
  st.jobResolvers.clear()
  stories.delete(id)
  deleteAudioState(st.p, st.s)
  removeStoryJobs(st.p, st.s)
  configStore.delete(docKey('story', st.p, st.s))
  for (const key of ruleStore.keys()) if (key.startsWith(`${docKey('story', st.p, st.s)}:`)) ruleStore.delete(key)
  for (const key of deletedRuleFiles) if (key.startsWith(`${docKey('story', st.p, st.s)}:`)) deletedRuleFiles.delete(key)
  for (const key of paragraphFeedback.keys()) if (key.startsWith(`${id}/`)) paragraphFeedback.delete(key)
  for (const key of mockExportedFiles) if (key.startsWith(`${id}/`)) mockExportedFiles.delete(key)
  for (const key of commandModels.keys()) if (key === id || key.startsWith(`${id}/`)) commandModels.delete(key)
  for (const key of diagnosticReports.keys()) if (key === id || key.startsWith(`${id}/`)) diagnosticReports.delete(key)
  emitGlobalEvent('library.changed', {})
  emitGlobalEvent('project.changed', { project: st.p })
  return json(res, 200, { ok: true })
}

// GAP-30: tài khoản có hộp thư, thiết lập và đăng ký riêng trong mock.
import type { NotifyPrefs, InboxNotification, PushSubscriptionInfo } from '../src/api/types'
const notifyMockPrefs = new Map<string, NotifyPrefs>()
const notifyMockInbox = new Map<string, InboxNotification[]>()
const notifyMockSubs = new Map<string, PushSubscriptionInfo[]>()
const notifyMockTargets = new Map<string, string>()
function notifyPrefsFor(username: string): NotifyPrefs {
  let prefs = notifyMockPrefs.get(username)
  if (!prefs) {
    prefs = { enabled: true, types: { gate: true, error: true, job: true, done: false, security: true }, digest: 'five', quiet: { from: '22:00', to: '07:00' }, version: '1' }
    notifyMockPrefs.set(username, prefs)
  }
  return prefs
}
async function handleNotifyMock(req: IncomingMessage, res: ServerResponse, url: URL, user: User, session: MockSession) {
  const path = url.pathname
  if (!notifyMockInbox.has(user.username)) seedNotifyMock(user, session)
  const inbox = notifyMockInbox.get(user.username) ?? []
  notifyMockInbox.set(user.username, inbox)
  const unread = () => inbox.filter((item) => !item.read).length
  const changed = () => emitGlobalEvent('notify.changed', { username: user.username, unread: unread() })
  if (path === '/push/vapid' && req.method === 'GET') return json(res, 200, { publicKey: 'BOr_tFTLxs6bS1AQ2h8TQDeqHvmPTd3wVG3cLVIl2PS0D66RxdBfMxmvn2_UZSP7DlC1HK05iGV1pYvQepMNn1M' })
  if (path === '/notify/prefs' && req.method === 'GET') {
    const prefs = notifyPrefsFor(user.username)
    res.setHeader('ETag', `"${prefs.version}"`)
    return json(res, 200, prefs)
  }
  if (path === '/notify/prefs' && req.method === 'PUT') {
    const body = await readBody(req)
    const current = notifyPrefsFor(user.username)
    const match = String(req.headers['if-match'] ?? body.version ?? '').replaceAll('"', '')
    if (match && match !== '*' && match !== current.version) return json(res, 409, { error: 'Đã có thay đổi từ nơi khác', conflict: true, version: current.version, current })
    const prefs = body as unknown as NotifyPrefs
    if (!['each', 'five', 'day'].includes(prefs.digest) || !prefs.types || !prefs.quiet) return fail(res, 400, 'Thiết lập thông báo không hợp lệ')
    prefs.types.security = true
    prefs.types.error = true
    prefs.version = String(Number(current.version) + 1)
    notifyMockPrefs.set(user.username, prefs)
    res.setHeader('ETag', `"${prefs.version}"`)
    changed()
    return json(res, 200, prefs)
  }
  if (path === '/push/subscriptions') {
    const active = (notifyMockSubs.get(user.username) ?? []).filter((sub) => [...sessions.values()].some((s) => s.id === sub.session))
    if (req.method === 'GET') return json(res, 200, { subscriptions: active })
    const body = await readBody(req)
    const endpoint = String(body.endpoint ?? '')
    const next = active.filter((sub) => sub.endpoint !== endpoint)
    if (req.method === 'POST') next.push({ endpoint, keys: body.keys as PushSubscriptionInfo['keys'], device: String(body.device ?? session.device), session: session.id, createdAt: new Date().toISOString() })
    notifyMockSubs.set(user.username, next)
    return json(res, 200, { ok: true })
  }
  if (path === '/notifications' && req.method === 'GET') {
    const before = url.searchParams.get('before')
    const filtered = inbox.filter((item) => !before || Number(item.id) < Number(before)).reverse()
    const notifications = filtered.slice(0, 50)
    return json(res, 200, { notifications, unread: unread(), nextBefore: filtered.length > 50 ? notifications.at(-1)?.id : '' })
  }
  if (path === '/notifications/read' && req.method === 'POST') {
    const body = await readBody(req)
    inbox.forEach((item) => { if (body.all || (Array.isArray(body.ids) && body.ids.includes(item.id))) item.read = true })
    changed()
    return json(res, 200, { ok: true, unread: unread() })
  }
  if (path === '/notify/test' && req.method === 'POST') {
    await readBody(req)
    inbox.push({ id: String(++seq), type: 'security', story: '', title: 'Thông báo thử từ Noverse', body: 'Điện thoại đã sẵn sàng nhận thông báo', url: '/notifications', actions: [], createdAt: new Date().toISOString(), read: false })
    if (inbox.length > 500) inbox.splice(0, inbox.length - 500)
    changed()
    return json(res, 200, { ok: true })
  }
  const match = /^\/notifications\/([^/]+)\/action$/.exec(path)
  if (match && req.method === 'POST') {
    const item = inbox.find((n) => n.id === match[1])
    if (!item) return fail(res, 404, 'Không tìm thấy thông báo')
    const body = await readBody(req)
    const action = String(body.action ?? '')
    if (!item.actions.some((offered) => offered.action === action)) return fail(res, 400, 'Thông báo không có thao tác này')
    if (action === 'revoke') {
      const target = notifyMockTargets.get(item.id)
      if (!target) return fail(res, 400, 'Thông báo không có thiết bị để đăng xuất')
      for (const [token, candidate] of sessions) {
        if (candidate.id === target && candidate.username === user.username) sessions.delete(token)
      }
      notifyMockSubs.set(user.username, (notifyMockSubs.get(user.username) ?? []).filter((sub) => sub.session !== target))
      emitGlobalEvent('session.revoked', { username: user.username, session: target })
    }
    if (action === 'approve') {
      const [project, story] = item.story.split('/')
      const response = await fetch(`http://${req.headers.host}/api/projects/${encodeURIComponent(project)}/stories/${encodeURIComponent(story)}/next`, {
        method: 'POST', headers: { 'content-type': 'application/json', cookie: req.headers.cookie ?? '' },
        body: JSON.stringify({ applyComments: true, chapter: item.chapter }),
      })
      const result: unknown = await response.json()
      if (!response.ok) return json(res, response.status, result)
    }
    if (body.action === 'mute') {
      const prefs = notifyPrefsFor(user.username)
      if (item.type === 'security' || item.type === 'error') return fail(res, 400, 'Không thể tắt thông báo an toàn hoặc lỗi')
      const type = ['import', 'sync', 'cocreate'].includes(item.type) ? 'job' : item.type as 'gate' | 'job' | 'done'
      prefs.types[type] = false
      prefs.version = String(Number(prefs.version) + 1)
    }
    item.read = true
    changed()
    return json(res, 200, action === 'rewrite'
      ? { ok: true, url: `/read/${item.story}/${item.chapter ?? 0}?rewrite=1` }
      : action === 'share' ? { ok: true, url: item.url } : { ok: true })
  }
  return fail(res, 404, 'Không tìm thấy')
}

function seedNotifyMock(user: User, current: MockSession) {
  const story = 'dit-dang-audio/m2-reader'
  const at = (days: number, hour: number, minute: number) => {
    const date = new Date()
    date.setDate(date.getDate() - days)
    date.setHours(hour, minute, 0, 0)
    return date.toISOString()
  }
  const secondary = mkSession(user.username, 'MacBook của Dat · Chrome')
  const securityId = String(++seq)
  notifyMockTargets.set(securityId, secondary.id)
  const fixtures: InboxNotification[] = [
    { id: securityId, type: 'security', story: '', title: 'Đăng nhập trên thiết bị mới', body: 'Chrome · MacBook của Dat · Hà Nội', url: '/more/devices', actions: [{ action: 'revoke', title: 'Không phải tôi' }], createdAt: at(1, 9, 2), read: true },
    { id: String(++seq), type: 'done', story, chapter: 20, title: 'Xong 5 chương (20–24)', body: 'Điểm trung bình 8,2 · 1 chương viết lại', url: `/read/${story}/20`, actions: [], createdAt: at(1, 23, 10), read: true },
    { id: String(++seq), type: 'error', story, title: 'Truyện dừng do lỗi model', body: 'OpenRouter hết hạn mức · đã thử lại 3 lần', url: `/write/${story}`, actions: [], createdAt: at(0, 11, 20), read: true },
    { id: String(++seq), type: 'job', story, title: 'Đã xuất sách nói Tập 1', body: 'tap-1.m4b · 9 chương · 118 MB', url: `/listen/${story}`, actions: [{ action: 'share', title: 'Chia sẻ', url: `/listen/${story}` }], createdAt: at(0, 13, 58), read: false },
    { id: String(++seq), type: 'gate', story, chapter: 25, title: 'Chương 25 chờ bạn duyệt', body: 'Bản gốc trở về · Biên tập viên chấm 8,4', url: `/read/${story}/25?gate=1`, actions: [{ action: 'approve', title: 'Duyệt & viết tiếp' }, { action: 'rewrite', title: 'Nhờ viết lại…', url: `/read/${story}/25?rewrite=1` }, { action: 'mute', title: 'Tắt loại thông báo này' }], createdAt: at(0, 14, 32), read: false },
  ]
  notifyMockInbox.set(user.username, fixtures.filter((item) => !item.story || canRead(user, item.story)))
  notifyMockSubs.set(user.username, [current, secondary].map((device, index) => ({
    endpoint: `https://push.mock.ainovel/${user.username}/${device.id}`,
    keys: { p256dh: 'mock-public-key', auth: 'mock-auth' }, session: device.id,
    device: index === 0 ? 'iPhone của Dat' : 'MacBook của Dat', createdAt: device.createdAt,
  })))
}

type MobileChapterExport = { path: string; name: string; format: 'txt' | 'epub'; size: number; updatedAt: string; volume: number; body: Buffer; mime: string }
const mobileChapterExports = new Map<string, Map<string, MobileChapterExport>>()
function createMobileChapterExport(st: MockStory, input: Record<string, unknown>, res: ServerResponse) {
  const from = Number(input.from) || 1
  const to = Number(input.to) || st.completed
  const format = input.format === 'epub' ? 'epub' : input.format === 'narration' ? 'narration' : 'txt'
  const ext = format === 'epub' ? 'epub' : 'txt'
  const chapters = Object.entries(st.chapters).filter(([number]) => Number(number) >= from && Number(number) <= Math.min(to, st.completed)).map(([, chapter]) => chapter)
  if (!chapters.length) return json(res, 400, { error: 'Chương này chưa có bản đã ghi để xuất.' })
  const stem = `${st.s}-chuong-${from}${to === from ? '' : `-${to}`}${format === 'narration' ? '-narration' : ''}`
  let name = `${stem}.${ext}`
  const scope = `${st.p}/${st.s}`
  const files = mobileChapterExports.get(scope) ?? new Map<string, MobileChapterExport>()
  let path = `xuat-ban/${name}`
  const strategy = input.onExists ?? 'fail'
  if (files.has(path)) {
    if (strategy === 'keep') {
      const stamp = new Date().toISOString().replace(/[-:TZ.]/g, '').slice(0, 14)
      name = `${stem}-${stamp}.${ext}`
      path = `xuat-ban/${name}`
      let suffix = 1
      while (files.has(path)) {
        name = `${stem}-${stamp}-${suffix++}.${ext}`
        path = `xuat-ban/${name}`
      }
    } else if (strategy !== 'replace') {
      const exists = mockExportConflict(path, format, chapters.length)
      return json(res, 409, { error: `Tệp ${path} đã tồn tại`, existing: exists.existing, incoming: exists.incoming, result: { kind: 'exportExists', exists } })
    }
  }
  const { body, mime } = mockExportDownload(path, st.title, chapters, { narration: format === 'narration', readNumbers: input.readNumbers === true })
  files.set(path, { path, name, format: ext, size: body.length, updatedAt: iso(Date.now()), volume: Number(input.volume) || 1, body, mime })
  mobileChapterExports.set(scope, files)
  const job = startExportJob(st.p, st.s, path, chapters.length)
  return json(res, 200, { path, bytes: body.length, chapters: chapters.length, skipped: [], job })
}

// ─── Bug 2710 · phát lại một lượt viết THẬT ───────────────────────────────────────────
// Chuỗi dưới đây lấy từ nhật ký live (chỉ đọc): meta/sessions/agents/writer-ch02.jsonl và
// editor-ch01.jsonl của ~/NoverseLive/tron-audio, cộng meta/decisions.jsonl. Có đủ: giao
// việc, đọc bối cảnh, đọc chương, kế hoạch, MỘT lần gọi sai bị lỗi rồi gọi lại, bản nháp,
// soát nhất quán, gọt giũa, nộp chương, Biên tập viên đọc + thẩm duyệt cung, cảnh báo hệ
// thống, quyết định của Trọng tài. Dùng để chụp thẻ bước trong Luồng viết trực tiếp.
const realStory = mkStory('dit-dang-audio', 'nhat-ky-that', 'kich-ban/nhat-ky-that', 'Nhật ký thật', 'writing', {
  total: 8, completed: 8, advanceMode: 'review',
  snapshot: {
    ...snapshotTraXanh, bookTitle: 'Nhật ký thật', phase: 'writing', flow: 'polishing',
    isRunning: true, runtimeState: 'running', statusLabel: 'Đang gọt giũa chương 2',
    currentChapter: 2, totalChapters: 8, completedCount: 8, inProgressChapter: 2,
    pendingRewrites: [2, 4, 8], rewriteReason: 'Biên tập viên yêu cầu gọt giũa sau khi duyệt cung 1',
    agents: [{ name: 'writer', state: 'running', taskId: 'task-real-1', taskKind: 'write', summary: 'writer → read_chapter(chương 2 · bản nháp)', tool: 'read_chapter', turn: 7, context: { tokens: 61200, contextWindow: 320000, percent: 19.1, scope: 'chapter', strategy: 'none', activeMessages: 12, summaryMessages: 0, compactedCount: 0, keptCount: 12 }, updatedAt: iso(NOW - 3000) }],
  },
})
realStory.replay = [
  mkEvent(1, { category: 'DISPATCH', agent: 'writer', summary: 'Viết chương 2', finishedAt: '' }),
  mkEvent(2, { category: 'TOOL', agent: 'writer', summary: 'novel_context(chương 2)', tool: 'novel_context', args: '{"chapter":2}', result: '{"chapter":2,"budget":61440,"context":"…"}', depth: 1, durationNs: 640_000_000 }),
  mkEvent(3, { category: 'TOOL', agent: 'writer', summary: 'read_chapter(chương 1)', tool: 'read_chapter', args: '{"chapter":1,"source":"final"}', result: '{"chapter":1,"source":"final","word_count":1499,"content":"Hợp đồng in hai bản…"}', depth: 1, durationNs: 210_000_000 }),
  mkEvent(4, { category: 'TOOL', agent: 'writer', summary: 'plan_chapter(chương 2)', tool: 'plan_chapter', args: '{"chapter":2,"title":"Phòng quy hoạch vắng chủ"}', result: '{"chapter":2,"planned":true}', depth: 1, durationNs: 3_900_000_000 }),
  // Đúng lỗi thật: model gửi thiếu `source` → InputValidationError, rồi gọi lại.
  mkEvent(5, { category: 'TOOL', agent: 'writer', summary: 'read_chapter lỗi', tool: 'read_chapter', args: '{"chapter":1}', result: "InputValidationError: read_chapter failed due to the following issue:\nThe required parameter `source` is missing.", failed: true, level: 'error', status: 'error', depth: 1, durationNs: 40_000_000 }),
  mkEvent(6, { category: 'TOOL', agent: 'writer', summary: 'draft_chapter(chương 2)', tool: 'draft_chapter', args: '{"chapter":2,"mode":"write"}', result: '{"written":true,"chapter":2,"mode":"write","word_count":1448}', depth: 1, durationNs: 41_000_000_000 }),
  mkEvent(7, { category: 'TOOL', agent: 'writer', summary: 'read_chapter(chương 2 · bản nháp)', tool: 'read_chapter', args: '{"chapter":2,"source":"draft"}', result: '{"chapter":2,"source":"draft","word_count":1443,"content":"Phòng quy hoạch khoá trái…"}', depth: 1, durationNs: 190_000_000 }),
  mkEvent(8, { category: 'TOOL', agent: 'writer', summary: 'check_consistency(chương 2)', tool: 'check_consistency', args: '{"chapter":2}', result: '{"chapter":2,"word_count":2826,"content":"…"}', resultHint: '{"word_count":2826}', depth: 1, durationNs: 320_000_000 }),
  mkEvent(9, { category: 'TOOL', agent: 'writer', summary: 'edit_chapter(chương 2)', tool: 'edit_chapter', args: '{"chapter":2}', result: '{"chapter":2,"word_count":1440,"diff":"…"}', depth: 1, durationNs: 12_000_000_000 }),
  mkEvent(10, { category: 'TOOL', agent: 'writer', summary: 'commit_chapter(chương 2)', tool: 'commit_chapter', args: '{"chapter":2,"hook_type":"choice"}', result: '{"chapter":2,"committed":true,"word_count":1491,"hook_type":"choice"}', depth: 1, durationNs: 2_400_000_000 }),
  mkEvent(11, { category: 'SYSTEM', agent: 'editor', summary: 'Cảnh báo nhất quán: chương 5 nhắc “tám người đọc bản thảo” mâu thuẫn chương 3', level: 'warn', finishedAt: '' }),
  mkEvent(12, { category: 'TOOL', agent: 'editor', summary: 'read_chapter(chương 3)', tool: 'read_chapter', args: '{"chapter":3,"source":"final"}', result: '{"chapter":3,"source":"final","word_count":1402,"content":"…"}', depth: 1, durationNs: 180_000_000 }),
  mkEvent(13, { category: 'TOOL', agent: 'editor', summary: 'read_chapter(chương 4)', tool: 'read_chapter', args: '{"chapter":4,"source":"final"}', result: '{"chapter":4,"source":"final","word_count":1270,"content":"…"}', depth: 1, durationNs: 175_000_000 }),
  mkEvent(14, { category: 'TOOL', agent: 'editor', summary: 'save_review(chương 8 · cung)', tool: 'save_review', args: '{"chapter":8,"scope":"arc","verdict":"polish"}', result: '{"affected_chapters":[2,4,8],"chapter":8,"issues":5,"saved":true,"verdict":"polish"}', depth: 1, durationNs: 6_100_000_000 }),
  mkEvent(15, { category: 'DECISION', agent: 'arbiter', summary: 'Giữ hướng cung 1 · gọt giũa chương 2, 4, 8 rồi viết tiếp chương 9', durationNs: 17_000_000_000 }),
  mkEvent(16, { category: 'TOOL', agent: 'architect', summary: 'audit_foundation', tool: 'audit_foundation', args: '{"fingerprint":"8f3a"}', result: '{"foundation_ready":true,"issues":[]}', depth: 1, durationNs: 900_000_000 }),
]
realStory.streamScript = [
  '✻ Truy vấn ngữ cảnh\nchapter: 2',
  '✻ Đọc chương\nchapter: 1\nsource: final\nfrom: 0\nto: 0\nmax_runes: 10000',
  '✻ Lập kế hoạch\nchapter: 2\ntitle: Phòng quy hoạch vắng chủ\ncore_event: Người giữ chìa khoá lưu trữ bản thảo vắng mặt.',
  '✻ Đọc chương\nchapter: 1\nsource: null\nfrom: 0\nto: 0',
  '✻ Đọc chương\nchapter: 2\nsource: draft\nfrom: 0\nto: 0',
  '✻ Kiểm tra nhất quán\nchapter: 2',
  '✻ Gọt giũa\nchapter: 2\nold_string: “Ta sẽ trả lời đúng hạn…”\nnew_string: “Tôi sẽ trả lời đúng hạn…”',
  '✻ Nộp chương\nchapter: 2\nhook_type: choice',
  '✻ Thẩm duyệt\nchapter: 8\nscope: arc\nverdict: polish',
  '✻ audit_foundation\n',
].map((line) => (line.endsWith('\n') ? line : `${line}\n`))
put(realStory)

// Gói X: real historical dispatches/arc review, separate from the stream fixture.
const mobileLogStory = mkStory('dit-dang-audio', 'nhat-ky-cung', 'kich-ban/nhat-ky-cung', 'Thiên Kim Thật Đòi Lại Giọng Nói', 'writing', {
  total: 64, completed: 40, chapters: Object.fromEntries(mobileLogFixture.chapters.map(row => [row.chapter, { title: row.title, text: '' }])),
  snapshot: { ...snapshotTraXanh, bookTitle: 'Thiên Kim Thật Đòi Lại Giọng Nói', currentChapter: 41, totalChapters: 64, completedCount: 40, inProgressChapter: 0, pendingRewrites: [40], flow: 'rewriting', isRunning: true, runtimeState: 'running', agents: [], outline: mobileLogFixture.chapters.map(row => ({ chapter: row.chapter, title: row.title, coreEvent: '' })) },
})
mobileLogStory.activityFixture = mobileLogFixture
mobileLogStory.replay = mockActivityContext(mobileLogFixture.events, 41)
put(mobileLogStory)
