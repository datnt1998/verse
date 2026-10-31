import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { ExportGateDialog } from './ExportGateDialog'
import type { EditorQuality } from './quality'

afterEach(cleanup)
const measured: EditorQuality = { total: 84, changedSinceMeasure: 0, items: [{ id: 'captionAlignment', label: 'Phụ đề', status: 'fail' }] }

describe('V13 export confirmation', () => {
  it('an 84 score lists real failing times and separates continue from explicit acceptance', () => {
    let accepted = 0
    let continued = 0
    render(<ExportGateDialog quality={measured} ranges={[
      { start: 760, end: 764, checkId: 'captionAlignment', status: 'fail' },
      { start: 1812, end: 1818, checkId: 'captionAlignment', status: 'fail' },
      { start: 2282, end: 2286, checkId: 'captionAlignment', status: 'fail' },
    ]} onAccept={() => { accepted++ }} onContinue={() => { continued++ }} onClose={() => { continued++ }} />)
    expect(screen.getByRole('alertdialog', { name: 'Xuất khi chưa đạt độ hoàn thiện?' })).toBeTruthy()
    expect(screen.getByText(/84\/100, cần 90/)).toBeTruthy()
    expect(screen.getByText('Phụ đề khớp tiếng: 12:40, 30:12, 38:02')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Sửa tiếp' }))
    expect(continued).toBe(1)
    expect(accepted).toBe(0)
    fireEvent.click(screen.getByRole('button', { name: 'Vẫn xuất' }))
    expect(accepted).toBe(1)
  })
  it('unknown measurement never invents drifting captions', () => {
    let closed = false
    render(<ExportGateDialog quality={{ ...measured, total: null }} ranges={[]} onAccept={() => { closed = true }} onContinue={() => { closed = true }} onClose={() => { closed = true }} />)
    expect(screen.getByText(/Chưa có kết quả đo mới đủ điều kiện xuất/)).toBeTruthy()
    expect(within(screen.getByRole('alertdialog')).queryByText(/Phụ đề khớp tiếng:/)).toBeNull()
    expect(closed).toBe(false)
  })
  it('a repaired but stale result asks for remeasurement rather than claiming new drift', () => {
    let acted = false
    render(<ExportGateDialog quality={{ ...measured, changedSinceMeasure: 1 }} ranges={[]} onAccept={() => { acted = true }} onContinue={() => { acted = true }} onClose={() => { acted = true }} />)
    expect(screen.getByText(/Bản vừa sửa cần đo lại để cập nhật điểm/)).toBeTruthy()
    expect(within(screen.getByRole('alertdialog')).queryByText(/Phụ đề khớp tiếng:/)).toBeNull()
    expect(acted).toBe(false)
  })
})
