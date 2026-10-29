import { describe, expect, it } from 'vitest'
import { vi } from './vi'

// Job.kind 是引擎代号；看板/移动端任务行都直接用它查表。
// 漏一个键就会把 write / audio / diag 这类英文代号显示在越南语界面里。
describe('vi.workspace.jobLabel', () => {
  const engineKinds = ['write', 'import', 'importsim', 'simulate', 'sync', 'export', 'audio', 'diag', 'cocreate']

  it('覆盖全部 Job.kind 且没有英文遗留', () => {
    for (const kind of engineKinds) {
      const label = vi.workspace.jobLabel[kind]
      expect(label, `thiếu nhãn cho job kind ${kind}`).toBeTruthy()
      expect(label).not.toBe(kind)
    }
  })
})
