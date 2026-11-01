import { afterEach, describe, expect, it, vi } from 'vitest'
import { acceptCloneExport, setCloneQualityAcceptance } from './measurement-runtime'

// "Vẫn xuất như hiện tại" phải đi đúng hành động thật: mở cổng + xếp xuất
// (POST /api/clone/:id/export {accept:true}) và trả lại kiểm tra qua
// POST /api/clone/:id/quality {accept:false}. Trước đây client gọi
// {action:"acceptQuality"} tới một endpoint không tồn tại.
describe('the owner\'s Vẫn xuất call', () => {
  afterEach(() => { vi.unstubAllGlobals() })

  const stubFetch = () => {
    const calls: { url: string; init: RequestInit }[] = []
    vi.stubGlobal('fetch', vi.fn(async (input: string, init: RequestInit) => {
      calls.push({ url: String(input), init })
      return Response.json({})
    }))
    return calls
  }

  it('accepts the gate and queues the clone export', async () => {
    const calls = stubFetch()
    await acceptCloneExport('job/a')
    expect(calls).toHaveLength(1)
    expect(calls[0]!.url).toBe('/api/clone/job%2Fa/export')
    expect(calls[0]!.init.method).toBe('POST')
    expect(JSON.parse(String(calls[0]!.init.body))).toEqual({ accept: true })
  })

  it('puts the checklist back in charge through the quality route', async () => {
    const calls = stubFetch()
    await setCloneQualityAcceptance('job/a', false)
    expect(calls).toHaveLength(1)
    expect(calls[0]!.url).toBe('/api/clone/job%2Fa/quality')
    expect(JSON.parse(String(calls[0]!.init.body))).toEqual({ accept: false })
  })
})
