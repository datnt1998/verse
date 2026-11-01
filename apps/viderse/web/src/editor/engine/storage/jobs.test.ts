// Tác vụ nền: server trả 202 kèm job, client hỏi trạng thái tới khi xong.
// Cloudflare cắt request không có byte nào trong 100 s nên không request nào
// được giữ mở suốt quá trình tải/giải mã.
import { describe, expect, it } from 'vitest'
import { cancelJob, readJob, waitForJob, type EditorJob, type JobFetch } from './jobs'

const job = (state: EditorJob['state'], extra: Partial<EditorJob> = {}): EditorJob => ({ id: 'job-1', kind: 'media.link', state, createdAt: 1, ...extra })

const reply = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } })

/** Transport giả: trả lần lượt các phản hồi đã script và ghi lại request. */
function scripted(bodies: unknown[]) {
  const calls: { url: string; init?: RequestInit }[] = []
  const request: JobFetch = async (input, init) => {
    calls.push({ url: String(input), init })
    const body = bodies.shift()
    if (body === undefined) throw new Error(`unexpected request ${String(input)}`)
    return body instanceof Response ? body : reply(body)
  }
  return { request, calls }
}

describe('editor job polling', () => {
  it('polls the job until it succeeds and returns its result', async () => {
    const { request, calls } = scripted([
      { job: job('running') },
      { job: job('queued') },
      { job: job('succeeded', { result: { media: { id: 'media-1' }, reused: true } }) },
    ])
    await expect(waitForJob('job-1', { request, intervalMs: 0 })).resolves.toEqual({ media: { id: 'media-1' }, reused: true })
    expect(calls.map(call => call.url)).toEqual(['/api/editor/jobs/job-1', '/api/editor/jobs/job-1', '/api/editor/jobs/job-1'])
    expect(calls[0]!.init).toMatchObject({ cache: 'no-store' })
  })

  it('surfaces the server Vietnamese copy when the job fails', async () => {
    const { request } = scripted([{ job: job('failed', { error: { code: 'invalid_request', error: 'Đoạn cắt tối đa 10 phút' } }) }])
    await expect(waitForJob('job-1', { request, intervalMs: 0 })).rejects.toThrow('Đoạn cắt tối đa 10 phút')
  })

  it('treats a cancelled job as a cancel, not a failure', async () => {
    const { request } = scripted([{ job: job('cancelled', { error: { code: 'cancelled', error: 'Đã huỷ tác vụ' } }) }])
    await expect(waitForJob('job-1', { request, intervalMs: 0 })).rejects.toThrow('Đã huỷ tác vụ')
  })

  it('stops waiting as soon as the caller aborts', async () => {
    const controller = new AbortController()
    const { request } = scripted([{ job: job('running') }])
    const waiting = waitForJob('job-1', { request, signal: controller.signal, intervalMs: 50 })
    controller.abort()
    await expect(waiting).rejects.toMatchObject({ name: 'AbortError' })
  })

  it('reads an unknown job as the server error', async () => {
    const { request } = scripted([reply({ code: 'not_found', error: 'Không tìm thấy tác vụ' }, 404)])
    await expect(readJob('missing', { request })).rejects.toThrow('Không tìm thấy tác vụ')
  })

  it('cancels the job through the same resource', async () => {
    const { request, calls } = scripted([{ job: job('cancelled') }])
    await cancelJob('job-1', { request })
    expect(calls).toEqual([{ url: '/api/editor/jobs/job-1', init: expect.objectContaining({ method: 'DELETE', cache: 'no-store' }) }])
  })

  it('keeps polling through a storage transport with its own base url', async () => {
    const { request, calls } = scripted([{ job: job('succeeded', { result: { envelope: { samplesPerSecond: 50 } } }) }])
    await expect(waitForJob('job 2', { request, baseUrl: '/api/editor', intervalMs: 0 })).resolves.toEqual({ envelope: { samplesPerSecond: 50 } })
    expect(calls[0]!.url).toBe('/api/editor/jobs/job%202')
  })
})
