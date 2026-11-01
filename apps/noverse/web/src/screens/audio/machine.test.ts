// Gói AT — bộ chuyển trạng thái máy đọc (10-audio.md §8.1, §8.2, §8.3).
import { describe, expect, it } from 'vitest'
import {
  MACHINE_FAILURES, machineAdvanced, machineKind, machineSeenNote, machineStepLine, machineSteps, machineView, prepLine, type MachineStatus,
} from './machine'

const status = (over: MachineStatus = {}): MachineStatus => ({ state: 'ready', ffmpeg: true, ...over })

it('năm trạng thái của thẻ Giọng đọc đúng bảng 8.1', () => {
  expect(machineView(undefined).kind).toBe('noMachine')
  expect(machineView(status({ state: 'missing' })).kind).toBe('noMachine')
  expect(machineView(status({ state: 'offline', lastSeenAt: '2026-11-01T22:14:00' })).kind).toBe('offline')
  expect(machineView(status({ state: 'provisioning', step: 'model', percent: 42 })).kind).toBe('provisioning')
  expect(machineView(status({ state: 'starting' })).kind).toBe('provisioning')
  expect(machineView(status({ state: 'error', error: { code: 'hash_mismatch' } })).kind).toBe('provisionFailed')
  expect(machineView(status({ state: 'ready' })).kind).toBe('ready')
})

it('máy đọc offline là trạng thái trung tính; Cảnh báo chỉ khi có chương đang chờ', () => {
  const idle = machineView(status({ state: 'offline', lastSeenAt: '2026-11-01T22:14:00', queueWaiting: 0 }))
  expect(idle.tone).toBe('neutral')
  expect(idle.chip).toBeNull()
  expect(idle.sentence).toContain('không phải lỗi')
  const busy = machineView(status({ state: 'offline', queueWaiting: 5 }))
  expect(busy.tone).toBe('neutral')
  expect(busy.chip).toEqual({ text: '5 chương đang chờ', tone: 'warn' })
})

it('bước đo được có % + MB; bước không đo được chỉ có vòng xoay, không bịa %', () => {
  const model = machineView(status({ state: 'provisioning', step: 'model', percent: 42, bytesDone: 244 * 1_048_576, bytesTotal: 581 * 1_048_576, etaSec: 240 }))
  expect(model.progress).toEqual({ percent: 42, label: '42% · 244 / 581 MB · còn khoảng 4 phút' })
  expect(model.spinning).toBe(false)
  expect(model.title).toBe('Đang chuẩn bị bộ đọc · Tải mẫu giọng')

  const source = machineView(status({ state: 'provisioning', step: 'source', percent: null }))
  expect(source.progress).toBeNull()
  expect(source.spinning).toBe(true)

  // Máy chủ cũ gửi -1 cho “không đo được”.
  expect(machineView(status({ state: 'provisioning', step: 'deps', percent: -1 })).progress).toBeNull()
})

it('danh sách 6 bước: bước xong, bước đang chạy, bước chờ', () => {
  const steps = machineSteps(status({ state: 'provisioning', step: 'model', percent: 42 }))
  expect(steps.map((step) => step.label)).toEqual([
    'Tải công cụ cài đặt', 'Chuẩn bị môi trường chạy', 'Tải bộ đọc', 'Cài thư viện', 'Tải mẫu giọng', 'Khởi động bộ đọc',
  ])
  expect(steps.map((step) => step.state)).toEqual(['done', 'done', 'done', 'done', 'now', 'wait'])
  expect(steps[0].right).toBe('xong')
  expect(steps[4].right).toBe('42%')
  expect(steps[5].right).toBe('')
  expect(machineStepLine(status({ state: 'provisioning', step: 'model', percent: 42 }))).toBe('Bước 5/6 · Tải mẫu giọng · 42%')
})

it('bảy câu lỗi 8.2: đúng tiêu đề, đúng một câu lý do, đúng nút', () => {
  const expected: Record<string, [string, string, string]> = {
    no_disk: ['Máy đọc không đủ chỗ trống', 'Cần ít nhất 4 GB trống', 'Thử lại'],
    download_failed: ['Không tải được bộ đọc', 'Mạng của máy đọc bị ngắt giữa chừng', 'Thử lại'],
    hash_mismatch: ['Tệp tải về không khớp bản gốc', 'đã bị bỏ để giữ an toàn', 'Thử lại'],
    deps_failed: ['Không chuẩn bị được thư viện', 'Bước cài thư viện cho bộ đọc dừng giữa chừng', 'Thử lại'],
    model_failed: ['Không tải được mẫu giọng', 'Mẫu giọng (khoảng 580 MB)', 'Thử lại'],
    start_failed: ['Bộ đọc không khởi động được', 'không bật lên được', 'Thử lại'],
    token_rejected: ['Máy đọc đã bị thu hồi quyền', 'không còn được nhận việc tạo giọng', 'Mở Nâng cao'],
  }
  for (const [code, [title, fragment, label]] of Object.entries(expected)) {
    const view = machineView(status({ state: 'error', error: { code } }))
    expect(view.title, code).toBe(title)
    expect(view.sentence, code).toContain(fragment)
    expect(view.actions, code).toEqual([{ label, kind: label === 'Thử lại' ? 'retry' : 'advanced' }])
    expect(view.chip).toEqual({ text: 'Bắt sửa', tone: 'error' })
  }
  expect(Object.keys(MACHINE_FAILURES).sort()).toEqual(
    ['deps_failed', 'download_failed', 'hash_mismatch', 'model_failed', 'no_disk', 'start_failed', 'token_rejected', 'uv_failed'].sort(),
  )
})

it('uv_failed dùng chung câu với download_failed; mã lạ vẫn có câu an toàn', () => {
  const uv = machineView(status({ state: 'error', error: { code: 'uv_failed' } }))
  const download = machineView(status({ state: 'error', error: { code: 'download_failed' } }))
  expect(uv.sentence).toBe(download.sentence)
  const unknown = machineView(status({ state: 'error', error: { code: 'boom' } }))
  expect(unknown.tone).toBe('danger')
  expect(unknown.actions[0]).toEqual({ label: 'Thử lại', kind: 'retry' })
})

it('thiếu công cụ ghép chỉ thành biến thể khi bộ đọc đã sẵn sàng', () => {
  expect(machineView(status({ state: 'ready', ffmpeg: false })).kind).toBe('missingFfmpeg')
  expect(machineView(status({ state: 'ready', ffmpeg: false })).title).toBe('Chưa ghép và xuất tập được')
  expect(machineView(status({ state: 'offline', ffmpeg: false })).kind).toBe('offline')
  expect(machineView(status({ state: 'provisioning', step: 'deps', ffmpeg: false })).kind).toBe('provisioning')
})

it('bản cá nhân cùng khung, tên “Máy này”, nút Chuẩn bị bộ đọc', () => {
  const local = machineView({ state: 'missing', engine: 'vieneu', ffmpeg: true })
  expect(local.mode).toBe('local')
  expect(local.machineName).toBe('Máy này')
  expect(local.title).toBe('Máy này chưa có bộ đọc')
  expect(local.actions).toEqual([{ label: 'Chuẩn bị bộ đọc', kind: 'prepare' }])
  expect(machineView({ state: 'ready', engine: 'vieneu' }).machineName).toBe('Máy này')
  // Bản máy chủ có tên máy đọc riêng.
  const hosted = machineView(status({ machine: { name: 'Mac mini ở nhà' } }))
  expect(hosted.mode).toBe('server')
  expect(hosted.machineName).toBe('Mac mini ở nhà')
})

it('trạng thái chưa có máy đọc của bản máy chủ chỉ đường sang Nâng cao', () => {
  const view = machineView(status({ state: 'missing', machine: { name: 'Mac mini ở nhà' } }))
  expect(view.actions).toEqual([{ label: 'Cách bật máy đọc', kind: 'advanced' }])
  expect(view.statusLabel).toBe('Chưa có máy đọc')
})

it('hàng dock Chuẩn bị bộ đọc: % chỉ khi đo được', () => {
  expect(prepLine('model', 51, 299 * 1_048_576, 581 * 1_048_576)).toEqual({
    line: 'Bước 5/6 · Tải mẫu giọng · 51% · 299 / 581 MB',
    percent: 51,
  })
  expect(prepLine('source', null).line).toBe('Bước 3/6 · Tải bộ đọc')
  expect(prepLine('source', null).percent).toBeNull()
  expect(prepLine(undefined, 40)).toEqual({ line: 'Đang chuẩn bị bộ đọc…', percent: null })
})

it('sẵn sàng: tốc độ đổi từ rtf, dung lượng đọc từ status', () => {
  const view = machineView(status({ rtf: 0.41, diskBytes: 1_500_000_000, machine: { name: 'Mac mini' } }))
  expect(view.speedLabel).toBe('1 phút audio ≈ 25 giây')
  expect(view.diskLabel).toBe('1,4 GB')
  expect(view.ready).toBe(true)
})

it('không có trường mới của §AT vẫn đọc được status cũ', () => {
  const legacy = machineView({ state: 'installing', engine: 'vieneu', model: 'VieNeu', device: 'cpu' } as MachineStatus)
  expect(legacy.kind).toBe('provisioning')
  expect(legacy.mode).toBe('local')
  expect(legacy.progress).toBeNull()
})

it('kind luôn khớp với state để mọi màn chọn đúng biến thể', () => {
  for (const state of ['missing', 'offline', 'provisioning', 'starting', 'error', 'ready'] as const) {
    expect(machineKind(status({ state }))).toBe(machineView(status({ state })).kind)
  }
})

// ─── Nâng cao › Máy đọc · chi tiết kỹ thuật (10-audio.md §8.5) ───
describe('machineAdvanced', () => {
  const hosted = (over: MachineStatus): MachineStatus => ({
    state: 'ready', ffmpeg: true, mode: 'server',
    machine: { name: 'mac-mini', grantedAt: '2026-10-28T09:12:00', capabilities: ['tts'] },
    lastSeenAt: '2026-11-01T21:52:07', version: 'VieNeu-TTS v3.8.3', diskBytes: 1_500_000_000,
    channel: 'http://127.0.0.1:8081', rtf: 0.41, ...over,
  })

  it('đủ bảy dòng kỹ thuật, giá trị mono + chú thích chữ thường', () => {
    const adv = machineAdvanced(hosted({}), new Date('2026-11-01T21:52:19'))
    expect(adv.rows.map((row) => row.label)).toEqual([
      'Tên máy đọc', 'Lần cuối kết nối', 'Phiên bản bộ đọc', 'Dung lượng', 'Kênh nối', 'Quyền', 'Trạng thái gửi về',
    ])
    const seen = adv.rows.find((row) => row.label === 'Lần cuối kết nối')
    expect(seen?.value).toBe('2026-11-01 21:52:07')
    expect(seen?.note).toBe('12 giây trước')
    expect(adv.rows.find((row) => row.label === 'Quyền')?.value).toBe('tts')
    expect(adv.rows.find((row) => row.label === 'Trạng thái gửi về')?.value).toBe('ready · rtf 0.41')
    expect(adv.canRevoke).toBe(true)
    expect(adv.setup).toBe(false)
  })

  it('lỗi: dòng gửi về có mã, nhật ký giữ đúng sáu dòng', () => {
    const adv = machineAdvanced(hosted({
      state: 'error', error: { code: 'hash_mismatch', message: 'x' }, logTail: ['a', 'b', 'c', 'd', 'e', 'f', 'g'],
    }))
    expect(adv.rows.find((row) => row.label === 'Trạng thái gửi về')?.value).toBe('error · hash_mismatch')
    expect(adv.log.split('\n')).toEqual(['a', 'b', 'c', 'd', 'e', 'f'])
  })

  it('offline: chú thích “cũ · offline suy từ lease”', () => {
    const adv = machineAdvanced(hosted({ state: 'offline' }))
    expect(adv.rows.find((row) => row.label === 'Trạng thái gửi về')).toEqual({
      label: 'Trạng thái gửi về', value: 'ready', note: 'cũ · offline suy từ lease',
    })
  })

  it('bị thu hồi: nhãn “Đã thu hồi”, không còn nút thu hồi', () => {
    const adv = machineAdvanced(hosted({
      state: 'error', error: { code: 'token_rejected', message: 'x' },
      machine: { name: 'mac-mini', revokedAt: '2026-11-01T21:58:00', capabilities: ['tts'] },
    }))
    expect(adv.statusLabel).toBe('Đã thu hồi')
    expect(adv.canRevoke).toBe(false)
    expect(adv.rows.find((row) => row.label === 'Quyền')?.note).toBe('đã thu hồi 2026-11-01 21:58:00')
  })

  it('chưa có máy đọc: hướng dẫn + lệnh bật một lần, không có bảng chi tiết', () => {
    const adv = machineAdvanced({ state: 'missing', mode: 'server', ffmpeg: true })
    expect(adv.setup).toBe(true)
    expect(adv.has).toBe(false)
    expect(adv.rows).toEqual([])
    expect(adv.commands).toContain('worker token create --cap tts')
  })

  it('bản cá nhân: không hiện bảng chi tiết máy đọc', () => {
    const adv = machineAdvanced({ state: 'ready', mode: 'local', engine: 'vieneu', ffmpeg: true })
    expect(adv.has).toBe(false)
    expect(adv.setup).toBe(false)
  })
})

it('machineSeenNote: giây, phút, giờ, hôm qua, ngày', () => {
  const now = new Date('2026-11-01T22:00:00')
  expect(machineSeenNote('2026-11-01T21:59:48', now)).toBe('12 giây trước')
  expect(machineSeenNote('2026-11-01T21:40:00', now)).toBe('20 phút trước')
  expect(machineSeenNote('2026-11-01T19:00:00', now)).toBe('3 giờ trước')
  expect(machineSeenNote('2026-10-31T22:14:00', now)).toBe('hôm qua')
  expect(machineSeenNote('2026-10-20T22:14:00', now)).toBe('12 ngày trước')
})
