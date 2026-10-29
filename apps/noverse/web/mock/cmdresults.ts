// Mock dữ liệu cho CmdResults (14-jobs-dock §2, GAP-27…29): báo cáo /diag, /sync --check,
// payload việc nền export/sync/simulate, danh mục model cho bảng /model.
import type {
  DiagReport, ExportConflict, ExportFormat, ExportJobResult, JobEvent, ModelsResponse, SimulateJobResult, SyncChanged, SyncDiff, SyncResult,
} from '../src/api/types'
import type { CoCreateProposal, ImportPreview } from '../src/api/types'
import { CHAPTER_TITLES, NOW, TRAXANH_CHAPTERS } from './data'

const iso = (ms: number) => new Date(ms).toISOString()
const HOUR = 3_600_000

export const diagSent = new Set<string>()

export function mockDiagReport(completed: number, total: number): DiagReport {
  return {
    generatedAt: iso(NOW),
    stats: {
      completedChapters: completed, totalChapters: total, totalWords: completed * 3240, avgWordsPerChapter: 3240,
      phase: 'writing', flow: 'writing', planningTier: 'mid', reviewCount: completed + 2, rewriteCount: 2, avgReviewScore: 8.2,
      foreshadowOpen: 4, foreshadowStale: 1, startedAt: iso(NOW - 12 * 24 * HOUR), currentVolume: 1, totalVolumes: 2,
    },
    findings: [
      {
        fingerprint: 'fs-stale-ngoc-boi', rule: 'foreshadow_stale', category: 'planning', severity: 'warning', confidence: 'high',
        autoLevel: 'suggest', target: 'foreshadow:ngoc-boi',
        title: 'Phục bút “ngọc bội của mẹ” đã 9 chương chưa nhắc lại',
        evidence: 'Gài ở chương 2; chương 3–11 không còn câu nào nhắc tới ngọc bội.',
        suggestion: 'Cho nhân vật chạm vào ngọc bội trong chương 12 hoặc đưa phục bút vào đại cương tập 2.',
      },
      {
        fingerprint: 'flow-rewrite-loop-7', rule: 'rewrite_loop', category: 'flow', severity: 'critical', confidence: 'medium',
        autoLevel: 'none', target: 'chapter:7',
        title: 'Chương 7 bị viết lại 2 lần liên tiếp vì cùng một lỗi',
        evidence: 'Biên tập viên hai lần ghi “mạch thời gian lệch với chương 6”; điểm 6,1 → 6,4.',
        suggestion: 'Ghi rõ mốc thời gian chương 6 trong ghi chú chương rồi để Người viết làm lại một lần.',
      },
      {
        fingerprint: 'ctx-long-summary', rule: 'summary_bloat', category: 'context', severity: 'info', confidence: 'low',
        autoLevel: 'safe', target: 'summary',
        title: 'Tóm tắt các chương đầu dài hơn mức cần thiết',
        evidence: 'Tóm tắt chương 1–3 chiếm 38% ngữ cảnh gửi cho Người viết.',
        suggestion: 'Rút gọn tóm tắt chương cũ để chừa chỗ cho đại cương tập hiện tại.',
      },
      {
        fingerprint: 'quality-dialogue-ratio', rule: 'dialogue_ratio', category: 'quality', severity: 'warning', confidence: 'medium',
        autoLevel: 'none', target: 'chapter:9',
        title: 'Chương 9 gần như toàn thoại',
        evidence: 'Thoại chiếm 81% số chữ, cao hơn hẳn trung bình 46% của truyện.',
        suggestion: 'Thêm đoạn miêu tả phản ứng và bối cảnh giữa các lượt thoại.',
      },
    ],
    actions: [
      { fingerprint: 'act-fs-ngoc-boi', sourceRule: 'foreshadow_stale', kind: 'steer', severity: 'warning', summary: 'Nhắc lại phục bút ngọc bội trong chương 12', message: 'Hãy cho Lam chạm vào ngọc bội của mẹ ở chương 12.', sent: diagSent.has('act-fs-ngoc-boi') },
      { fingerprint: 'act-rewrite-7', sourceRule: 'rewrite_loop', kind: 'steer', severity: 'critical', summary: 'Chốt mốc thời gian chương 6 trước khi viết lại chương 7', message: 'Chương 6 diễn ra tối thứ Bảy; viết lại chương 7 khớp mốc này.', sent: diagSent.has('act-rewrite-7') },
    ],
  }
}

export function mockSyncChanged(): SyncChanged[] {
  return [
    { chapter: 3, title: CHAPTER_TITLES[3] ?? 'Chương 3', kind: 'edit', added: 212, removed: 48, modifiedAt: iso(NOW - 25 * 60_000) },
    { chapter: 7, title: CHAPTER_TITLES[7] ?? 'Chương 7', kind: 'edit', added: 64, removed: 0, modifiedAt: iso(NOW - 26 * HOUR) },
    { chapter: 12, title: CHAPTER_TITLES[12] ?? 'Chương 12', kind: 'edit', added: 9, removed: 31, modifiedAt: iso(NOW - 4 * 24 * HOUR) },
  ]
}

export function mockSyncDiff(chapter: number): SyncDiff | null {
  const row = mockSyncChanged().find((c) => c.chapter === chapter)
  if (!row) return null
  const base = TRAXANH_CHAPTERS[chapter]?.text ?? 'Quán trà mở cửa lúc sáu giờ.\n\nLam lau bàn.\n\nMưa bắt đầu rơi.'
  const paragraphs = base.split(/\n+/).filter(Boolean)
  const edited = [...paragraphs]
  edited[0] = edited[0].replace(/Trà Mộc/g, 'Trà Mộc Lan') + ' Tấm biển mới vẫn còn mùi sơn.'
  edited.splice(Math.min(2, edited.length), 0, 'Ngoài hiên, mưa lất phất như sợi chỉ bạc.')
  return { chapter, title: row.title, before: paragraphs.join('\n\n'), after: edited.join('\n\n') }
}

export const mockSyncResult = (): SyncResult => ({
  schemaVersion: 1,
  applied: [3, 7, 12],
  analyses: [
    { schemaVersion: 1, chapter: 3, title: CHAPTER_TITLES[3] ?? '', changeSummary: 'Đổi tên quán trà thành “Trà Mộc Lan”', storyChanged: true, classification: 'conflicts', conflictCount: 1, downstreamIssues: [{ text: 'Chương 14 vẫn gọi quán là “Trà Mộc” — người đọc sẽ thấy lệch.', chapter: 14, issueIndex: 0, sent: false }] },
    { schemaVersion: 1, chapter: 7, title: CHAPTER_TITLES[7] ?? '', changeSummary: 'Thêm cảnh mưa trước buổi thử vai', storyChanged: false, classification: 'wording', conflictCount: 0, downstreamIssues: [] },
    { schemaVersion: 1, chapter: 12, title: CHAPTER_TITLES[12] ?? '', changeSummary: 'Cập nhật bí mật của đạo diễn Phùng', storyChanged: true, classification: 'facts', conflictCount: 0, downstreamIssues: [] },
  ],
})

export function mockImportPreview(path: string, guide = ''): ImportPreview {
  const chapters = Array.from({ length: guide ? 25 : 24 }, (_, i) => ({
    n: i + 1, title: CHAPTER_TITLES[i + 1] ?? `Chương ${i + 1}`,
    words: i === 7 && !guide ? 8800 : 2600 + i * 37,
    firstLine: `Dòng đầu chương ${i + 1} trong bản thảo.`,
    newlySplit: !!guide && (i === 7 || i === 8),
    ...(i === 7 && !guide ? { warning: 'Chương dài hơn gấp đôi trung bình' } : {}),
  }))
  return { schemaVersion: 1, filename: path.split('/').pop() ?? path, size: 182400, words: chapters.reduce((n, ch) => n + ch.words, 0), splitHow: guide ? 'Theo hướng dẫn của bạn' : 'Theo dòng «Chương N»', chapters, warnings: guide ? [] : ['Chương 8 dài hơn gấp đôi trung bình'], encoding: 'UTF-8' }
}

export const mockCoCreateProposal = (revision: number, nextChapter: number, kept: number): CoCreateProposal => ({
  schemaVersion: 1, title: 'Hướng đi cho ba chương tiếp theo', revision,
  chapters: ['Dấu vết trong phòng thu', 'Người giữ bản gốc', 'Đối mặt trước ống kính'].map((title, i) => ({
    schemaVersion: 1, chapter: nextChapter + i, title, summary: 'Kiều My lần theo bằng chứng mới và đẩy xung đột tới lựa chọn tiếp theo.',
  })),
  impacts: [`Sửa dàn ý ch.${nextChapter}–${nextChapter + 2}`, `Giữ nguyên ${kept} chương`, 'Đẩy 1 phục bút'],
})

export function mockExportPath(line: string, story: string): { path: string; format: ExportFormat } {
  const args = line.trim().split(/\s+/).slice(1)
  const arg = args.find((a) => !a.startsWith('--') && !/^(from|to|format)=/.test(a))
  const word = arg === 'txt' || arg === 'epub' || arg === 'narration' ? arg : args.find((a) => a.startsWith('format='))?.slice(7)
  if (word === 'narration') return { path: `exports/${story}-narration.txt`, format: 'narration' }
  if (word === 'txt' || word === 'epub') return { path: `exports/${story}.${word}`, format: word }
  const path = arg || `exports/${story}.epub`
  return { path, format: /-narration\.txt$/i.test(path) ? 'narration' : path.toLowerCase().endsWith('.txt') ? 'txt' : 'epub' }
}

export const mockExportResult = (path: string, format: ExportFormat, chapters: number, skipped: number[]): ExportJobResult => ({
  path, name: path.split('/').pop() ?? path, title: 'Trà Xanh', format, bytes: format === 'epub' ? 1_258_291 : 412_876, chapters, skipped,
})

export const mockExportConflict = (path: string, format: ExportFormat, chapters: number): ExportConflict => ({
  path, format,
  existing: { path, size: 1_048_576, modTime: iso(NOW - 2 * 24 * HOUR), bytes: 1_048_576, chapters: Math.max(1, chapters - 6), modifiedAt: iso(NOW - 2 * 24 * HOUR) },
  incoming: { path, size: 1_258_291, chapters, skipped: [], bytesEstimate: 1_258_291 },
})

export const mockSimulateResult = (): SimulateJobResult => ({
  sourceDir: 'simulate', sources: 12, words: 84_210, profilePath: 'meta/simulation/profile.json',
  synthesis: {
    style: { sentence_rhythm: ['Ngắn, nhịp nhanh'], perspective: ['Ngôi ba, “cô”'], narrative_voice: ['Ấm, hơi châm biếm'] },
    pacing_density: { dialogue_action_ratio: ['Nhiều, ngắt quãng'] },
    lexicon: { signature_phrases: ['Trà nguội rồi thì pha lại, người đi rồi thì thôi.'] },
  },
})

export const mockModels = (): ModelsResponse => ({
  roles: [
    { role: 'default', provider: 'openai-codex', model: 'gpt-5.4', thinking: 'high', thinkingOptions: ['', 'low', 'medium', 'high'] },
    { role: 'architect', provider: 'openai-codex', model: 'gpt-5.4', thinking: 'high', thinkingOptions: ['', 'low', 'medium', 'high'] },
    { role: 'writer', provider: 'openrouter', model: 'anthropic/claude-sonnet-4.5', thinking: '', thinkingOptions: ['', 'low', 'medium', 'high'] },
    { role: 'editor', provider: 'deepseek', model: 'deepseek-chat', thinking: '', thinkingOptions: [''] },
  ],
  providers: [
    { provider: 'deepseek', label: 'DeepSeek', ready: true, models: [
      { name: 'deepseek-chat', contextWindow: 128_000, contextSource: 'registry', registryWindow: 128_000, thinking: [''] },
      { name: 'deepseek-reasoner', contextWindow: 128_000, contextSource: 'registry', registryWindow: 128_000, thinking: ['', 'low', 'medium', 'high'] },
    ] },
    { provider: 'openai-codex', label: 'ChatGPT', ready: true, models: [
      { name: 'gpt-6.1-sol', contextWindow: 400_000, contextSource: 'registry', registryWindow: 400_000, thinking: ['', 'low', 'medium', 'high', 'xhigh'] },
      { name: 'gpt-6-luna', contextWindow: 400_000, contextSource: 'registry', registryWindow: 400_000, thinking: ['', 'low', 'medium', 'high', 'xhigh'] },
      { name: 'gpt-5.6-terra', contextWindow: 200_000, contextSource: 'registry', registryWindow: 200_000, thinking: ['', 'low', 'medium', 'high'] },
      { name: 'gpt-5.4', contextWindow: 400_000, contextSource: 'registry', registryWindow: 400_000, thinking: ['', 'minimal', 'low', 'medium', 'high', 'xhigh'] },
      { name: 'gpt-5.4-mini', contextWindow: 400_000, contextSource: 'registry', registryWindow: 400_000, thinking: ['', 'low', 'medium', 'high'] },
    ] },
    { provider: 'openrouter', label: 'OpenRouter', ready: true, models: [
      { name: 'claude-3.5-sonnet', contextWindow: 200_000, contextSource: 'registry', registryWindow: 200_000, thinking: ['', 'low', 'medium', 'high'] },
      { name: 'anthropic/claude-sonnet-4.5', contextWindow: 1_000_000, contextSource: 'registry', registryWindow: 1_000_000, thinking: ['', 'low', 'medium', 'high'] },
      { name: 'google/gemini-2.5-pro', contextWindow: 1_000_000, contextSource: 'registry', registryWindow: 1_000_000, thinking: ['', 'low', 'medium', 'high'] },
    ] },
    { provider: 'gemini', label: 'Google Gemini', ready: false, models: [
      { name: 'gemini-2.5-flash', contextWindow: 1_000_000, contextSource: 'registry', registryWindow: 1_000_000, thinking: ['', 'off', 'low', 'medium', 'high'] },
    ] },
  ],
})

/** stage theo sim.Stage (scan → analyze → merge) cho thẻ học văn phong. */
export function simulateTick(job: JobEvent) {
  job.stage = job.current <= 1 ? 'scan' : job.current <= 3 ? 'analyze' : 'merge'
  job.message = job.stage === 'scan' ? 'Đang đọc văn mẫu' : job.stage === 'analyze' ? 'Đang phân tích văn mẫu' : 'Đang tạo hồ sơ'
  if (job.current >= 1) job.simulate = { ...mockSimulateResult(), synthesis: undefined }
}

/** Tệp xuất mock có đúng nội dung/định dạng để kiểm tra nút tải xuống, không đổi đuôi TXT thành EPUB. */
export function mockExportDownload(
  path: string, title: string, chapters: { title: string; text: string }[],
  opts?: { narration?: boolean; readNumbers?: boolean },
) {
  if (opts?.narration) {
    // D3: bản đọc TTS — không tiêu đề chương, không in nghiêng/đậm, các chương cách nhau hai dòng trống.
    const words = opts.readNumbers
      ? (text: string) => text.replace(/\d+/g, (digits) => digits.split('').map((digit) => 'không một hai ba bốn năm sáu bảy tám chín'.split(' ')[Number(digit)]).join(' '))
      : (text: string) => text
    return { mime: 'text/plain; charset=utf-8', body: Buffer.from(chapters.map((chapter) => words(chapter.text)).join('\n\n\n')) }
  }
  if (!path.toLowerCase().endsWith('.epub')) {
    return { mime: 'text/plain; charset=utf-8', body: Buffer.from(chapters.map((chapter) => `${chapter.title}\n\n${chapter.text}`).join('\n\n---\n\n')) }
  }
  const xml = (text: string) => text.replace(/[&<>"]/g, (char) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[char]!)
  const names = chapters.map((_, i) => `chapter-${i + 1}`)
  const files: [string, string][] = [
    ['mimetype', 'application/epub+zip'],
    ['META-INF/container.xml', '<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>'],
    ['OEBPS/content.opf', `<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="book"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="book">urn:ainovel:mock-export</dc:identifier><dc:title>${xml(title)}</dc:title><dc:language>vi</dc:language><meta property="dcterms:modified">${iso(NOW).replace(/\.\d{3}Z$/, 'Z')}</meta></metadata><manifest><item id="nav" href="nav.xhtml" properties="nav" media-type="application/xhtml+xml"/>${names.map((name) => `<item id="${name}" href="${name}.xhtml" media-type="application/xhtml+xml"/>`).join('')}</manifest><spine>${names.map((name) => `<itemref idref="${name}"/>`).join('')}</spine></package>`],
    ['OEBPS/nav.xhtml', `<?xml version="1.0" encoding="UTF-8"?><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="vi"><head><title>Mục lục</title></head><body><nav epub:type="toc"><h1>Mục lục</h1><ol>${chapters.map((chapter, i) => `<li><a href="${names[i]}.xhtml">${xml(chapter.title)}</a></li>`).join('')}</ol></nav></body></html>`],
    ...chapters.map((chapter, i): [string, string] => [`OEBPS/${names[i]}.xhtml`, `<?xml version="1.0" encoding="UTF-8"?><html xmlns="http://www.w3.org/1999/xhtml" lang="vi"><head><title>${xml(chapter.title)}</title></head><body><h1>${xml(chapter.title)}</h1>${chapter.text.split(/\n+/).filter(Boolean).map((paragraph) => `<p>${xml(paragraph)}</p>`).join('')}</body></html>`]),
  ]
  return { mime: 'application/epub+zip', body: storedZip(files.map(([name, text]) => [name, Buffer.from(text)])) }
}

/** Uncompressed ZIP shared by EPUB and audio fixtures. */
export function storedZip(files: readonly (readonly [string, Buffer])[]): Buffer {
  const locals: Buffer[] = []
  const directory: Buffer[] = []
  let offset = 0
  for (const [name, data] of files) {
    const filename = Buffer.from(name)
    let crc = 0xffffffff
    for (const byte of data) {
      crc ^= byte
      for (let bit = 0; bit < 8; bit++) crc = (crc >>> 1) ^ (0xedb88320 & -(crc & 1))
    }
    crc = (crc ^ 0xffffffff) >>> 0
    const local = Buffer.alloc(30)
    local.writeUInt32LE(0x04034b50); local.writeUInt16LE(20, 4); local.writeUInt16LE(0x800, 6)
    local.writeUInt16LE(0x21, 12); local.writeUInt32LE(crc, 14)
    local.writeUInt32LE(data.length, 18); local.writeUInt32LE(data.length, 22); local.writeUInt16LE(filename.length, 26)
    locals.push(local, filename, data)
    const central = Buffer.alloc(46)
    central.writeUInt32LE(0x02014b50); central.writeUInt16LE(20, 4); central.writeUInt16LE(20, 6)
    central.writeUInt16LE(0x800, 8); central.writeUInt16LE(0x21, 14); central.writeUInt32LE(crc, 16)
    central.writeUInt32LE(data.length, 20); central.writeUInt32LE(data.length, 24); central.writeUInt16LE(filename.length, 28)
    central.writeUInt32LE(offset, 42)
    directory.push(central, filename)
    offset += local.length + filename.length + data.length
  }
  const central = Buffer.concat(directory)
  const end = Buffer.alloc(22)
  end.writeUInt32LE(0x06054b50); end.writeUInt16LE(files.length, 8); end.writeUInt16LE(files.length, 10)
  end.writeUInt32LE(central.length, 12); end.writeUInt32LE(offset, 16)
  return Buffer.concat([...locals, central, end])
}
