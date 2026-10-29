import { execFile } from 'node:child_process'
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join, extname } from 'node:path'
import { promisify } from 'node:util'
import type { AudioVoice } from '../src/api/types'
import { VOICES, wavFor } from './audio'

const exec = promisify(execFile)
export const projectVoiceExtra = new Map<string, AudioVoice[]>()
const clips = new Map<string, Buffer>()
export const mockProjectVoices = (project: string) => [...VOICES, ...(projectVoiceExtra.get(project) ?? [])].map(voice => ({ ...voice, sampleUrl: `/api/projects/${encodeURIComponent(project)}/voices/${encodeURIComponent(voice.id)}/sample` }))

/** Parse the actual upload and measure it, matching production validation. */
export async function mockUploadVoice(project: string, raw: Buffer, contentType: string): Promise<AudioVoice> {
  const boundary = /boundary=(?:"([^"]+)"|([^;]+))/.exec(contentType)
  const marker = boundary?.[1] ?? boundary?.[2]
  if (!marker) throw new Error('Không đọc được tệp gửi lên.')
  let file: Buffer | undefined, filename = '', name = '', gender = ''
  const delimiter = Buffer.from(`--${marker}`)
  let cursor = raw.indexOf(delimiter)
  while (cursor >= 0) {
    const start = cursor + delimiter.length + 2, headerEnd = raw.indexOf('\r\n\r\n', start)
    if (headerEnd < 0) break
    const next = raw.indexOf(delimiter, headerEnd + 4)
    if (next < 0) break
    const header = raw.subarray(start, headerEnd).toString(), body = raw.subarray(headerEnd + 4, next - 2)
    const field = /\bname="([^"]+)"/.exec(header)?.[1]
    if (field === 'file') { filename = /filename="([^"]+)"/.exec(header)?.[1] ?? ''; file = body }
    if (field === 'name') name = body.toString()
    if (field === 'gender') gender = body.toString()
    cursor = next
  }
  if (!file || !filename) throw new Error('Thiếu tệp ghi âm.')
  if (!/^\.(mp3|wav|m4a)$/i.test(extname(filename))) throw new Error('Định dạng chưa được hỗ trợ. Hãy chọn tệp .mp3, .wav hoặc .m4a.')
  const dir = await mkdtemp(join(tmpdir(), 'ainovel-mock-voice-'))
  try {
    const src = join(dir, `upload${extname(filename)}`), sample = join(dir, 'sample.wav')
    await writeFile(src, file)
    let duration: number
    try { const { stdout } = await exec('ffprobe', ['-v', 'error', '-show_entries', 'format=duration', '-of', 'default=noprint_wrappers=1:nokey=1', src]); duration = Number(stdout.trim()) }
    catch { throw new Error('Không đọc được tệp ghi âm. Hãy chọn tệp .mp3, .wav hoặc .m4a nghe được.') }
    if (!Number.isFinite(duration) || duration <= 0) throw new Error('Không đọc được thời lượng mẫu giọng.')
    if (duration < 10) throw new Error(`Mẫu giọng chỉ có ${Math.round(duration)} giây — cần từ 10 đến 30 giây.`)
    if (duration > 30) throw new Error(`Mẫu giọng dài ${Math.round(duration)} giây — cần từ 10 đến 30 giây.`)
    await exec('ffmpeg', ['-hide_banner', '-nostdin', '-y', '-i', src, '-t', '5', '-ar', '8000', '-ac', '1', '-c:a', 'pcm_s16le', sample])
    const extra = projectVoiceExtra.get(project) ?? []
    const voice: AudioVoice = { id: `giong-${project}-${extra.length + 1}`, name: name.trim() || 'Mẫu giọng mới', durationSec: duration, gender, builtIn: false }
    clips.set(`${project}/${voice.id}`, await readFile(sample))
    projectVoiceExtra.set(project, [...extra, voice])
    return { ...voice, sampleUrl: `/api/projects/${encodeURIComponent(project)}/voices/${encodeURIComponent(voice.id)}/sample` }
  } finally { await rm(dir, { recursive: true, force: true }) }
}

export const mockVoiceSample = (project: string, id: string) => clips.get(`${project}/${id}`) ?? (mockProjectVoices(project).some(voice => voice.id === id) ? wavFor(5) : null)
