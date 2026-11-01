// Gói AT — mọi trạng thái của thẻ Máy đọc (đầu màn + Tuỳ chỉnh) không lộ từ kỹ thuật hay mã lỗi.
import { renderToStaticMarkup } from 'react-dom/server'
import { expect, it } from 'vitest'
import { MachineCard } from './MachineCard'
import { MACHINE_FAILURES, machineAdvanced, machineStepLine, machineView, type MachineStatus } from './machine'

const FORBIDDEN = /worker|container|launchd|\buv\b|python|ffmpeg|~\/|\/Users|\.env|token|code=|_failed|no_disk|hash_mismatch|token_rejected/i

/** Chữ người dùng thấy: bỏ thẻ và thuộc tính (đường dẫn biểu tượng không phải chữ trên giao diện). */
const visibleText = (html: string) => html.replace(/<[^>]*>/g, ' ')

const hosted = (over: MachineStatus): MachineStatus => ({ machine: { name: 'Mac mini ở nhà' }, ffmpeg: true, ...over })
const STATES: [string, MachineStatus][] = [
  ['noMachine', hosted({ state: 'missing' })],
  ['offline', hosted({ state: 'offline', lastSeenAt: '2026-11-01T22:14:00', queueWaiting: 5 })],
  ['provisioning source', hosted({ state: 'provisioning', step: 'source', percent: null })],
  ['provisioning model', hosted({ state: 'provisioning', step: 'model', percent: 42, bytesDone: 244 * 1_048_576, bytesTotal: 581 * 1_048_576, etaSec: 240 })],
  ...Object.keys(MACHINE_FAILURES).map((code): [string, MachineStatus] => [code, hosted({ state: 'error', error: { code, message: `uv sync failed: ${code}` } })]),
  ['ready', hosted({ state: 'ready', rtf: 0.41, diskBytes: 1_500_000_000 })],
  ['missingFfmpeg', hosted({ state: 'ready', ffmpeg: false })],
  ['local missing', { state: 'missing', engine: 'vieneu', ffmpeg: true }],
  ['local ready', { state: 'ready', engine: 'vieneu', ffmpeg: true }],
]

it.each(STATES)('%s: thẻ đầu màn và thẻ Tuỳ chỉnh không có từ cấm', (_, status) => {
  const view = machineView(status)
  const html = renderToStaticMarkup(<>
    <MachineCard view={view} />
    <MachineCard view={view} variant="settings" stepLine={machineStepLine(status)}
      tiles={[{ label: 'Máy đọc', value: view.machineName }]} />
  </>)
  expect(visibleText(html)).not.toMatch(FORBIDDEN)
  expect(visibleText(html).trim().length).toBeGreaterThan(0)
  expect(view.waitSub).not.toMatch(FORBIDDEN)
})

it('bản máy chủ không có nút “Cài bộ đọc trên máy”', () => {
  for (const [, status] of STATES.filter(([name]) => !name.startsWith('local'))) {
    expect(renderToStaticMarkup(<MachineCard view={machineView(status)} />)).not.toContain('Cài bộ đọc trên máy')
  }
})

it('đang chuẩn bị ở bước không đo được: vòng xoay, không có thanh %', () => {
  const html = renderToStaticMarkup(<MachineCard view={machineView(hosted({ state: 'provisioning', step: 'source' }))} />)
  expect(html).not.toContain('role="progressbar"')
  const measured = renderToStaticMarkup(<MachineCard view={machineView(hosted({ state: 'provisioning', step: 'model', percent: 42 }))} />)
  expect(measured).toContain('aria-valuenow="42"')
})

it('sẵn sàng: thẻ đầu màn ẩn, thẻ Tuỳ chỉnh có 4 ô', () => {
  const view = machineView(hosted({ state: 'ready', rtf: 0.41 }))
  expect(renderToStaticMarkup(<MachineCard view={view} />)).toBe('')
  const settings = renderToStaticMarkup(<MachineCard view={view} variant="settings"
    tiles={[{ label: 'Máy đọc', value: 'a' }, { label: 'Mẫu giọng', value: 'b' }, { label: 'Tốc độ tạo', value: 'c' }, { label: 'Dung lượng đang dùng', value: 'd' }]} />)
  expect(settings).toContain('Sẵn sàng')
  expect(settings).toContain('Dung lượng đang dùng')
})

// 10-audio.md §8.2: mã lỗi và nhật ký chỉ sống ở Nâng cao — thẻ Giọng đọc chỉ nói câu người dùng hiểu.
it.each(Object.keys(MACHINE_FAILURES))('%s: mã lỗi không rò ra thẻ Giọng đọc', (code) => {
  const status = hosted({ state: 'error', error: { code, message: `uv sync failed: code=${code}` } })
  const card = visibleText(renderToStaticMarkup(<>
    <MachineCard view={machineView(status)} />
    <MachineCard view={machineView(status)} variant="settings" stepLine={machineStepLine(status)} />
  </>))
  expect(card).not.toContain(code)
  expect(card).not.toContain('uv sync failed')
  const advanced = machineAdvanced(status)
  expect(advanced.rows.find((row) => row.label === 'Trạng thái gửi về')?.value).toBe(`error · ${code}`)
})
