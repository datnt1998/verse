import { describe, expect, it } from 'vitest'
import type { DiagFinding } from '../../api/types'
import {
  exportGrid, exportPathArg, filterFindings, formatChapterRanges, formatWordDelta, paragraphDiff, severityCounts,
  withExportFlag, withExportFormat,
} from './logic'

const finding = (fingerprint: string, severity: DiagFinding['severity']): DiagFinding => ({
  fingerprint, rule: 'r', category: 'flow', severity, confidence: 'high', autoLevel: 'none',
  target: '', title: fingerprint, evidence: '', suggestion: '',
})

describe('diag findings', () => {
  const list = [finding('a', 'info'), finding('b', 'critical'), finding('c', 'warning'), finding('d', 'critical')]

  it('sorts by severity and keeps engine order inside a level', () => {
    expect(filterFindings(list, 'all').map((f) => f.fingerprint)).toEqual(['b', 'd', 'c', 'a'])
  })

  it('filters to one severity', () => {
    expect(filterFindings(list, 'critical').map((f) => f.fingerprint)).toEqual(['b', 'd'])
    expect(severityCounts(list)).toEqual({ all: 4, critical: 2, warning: 1, info: 1 })
  })
})

describe('word delta', () => {
  it('uses vi-VN grouping and a real minus sign', () => {
    expect(formatWordDelta(1212, '+')).toBe('+1.212')
    expect(formatWordDelta(48, '-')).toBe('−48')
  })
})

describe('export grid', () => {
  it('maps merged chapters onto 24 cells with the next cell running', () => {
    const { cells, percent } = exportGrid(36, 48)
    expect(cells).toHaveLength(24)
    expect(cells.filter((c) => c.state === 'done')).toHaveLength(18)
    expect(cells[18].state).toBe('now')
    expect(cells[0].label).toBe('Chương 1–2')
    expect(percent).toBe(75)
  })

  it('keeps 24 cells for short books and lights all when done', () => {
    expect(exportGrid(0, 5).cells).toHaveLength(24)
    const done = exportGrid(3, 5, true)
    expect(done.cells.every((c) => c.state === 'done')).toBe(true)
    expect(done.percent).toBe(100)
  })
})

describe('chapter ranges', () => {
  it('compacts consecutive chapters', () => {
    expect(formatChapterRanges([30, 25, 26, 27, 28, 29, 3])).toBe('3, 25–30')
  })
})

describe('export command line', () => {
  it('swaps the on-exists flag and finds the path argument', () => {
    expect(withExportFlag('/export exports/a.epub --overwrite', 'keep')).toBe('/export exports/a.epub --keep')
    expect(exportPathArg('/export from=2 exports/a.txt')).toBe('exports/a.txt')
    expect(exportPathArg('/export')).toBe('')
  })

  it('rewrites the format word, keeps the chapter range and flags', () => {
    expect(withExportFormat('/export exports/tra-xanh.epub from=1 to=30 --keep', 'narration'))
      .toBe('/export narration from=1 to=30 --keep')
    expect(withExportFormat('/export', 'txt')).toBe('/export txt')
    expect(withExportFormat('/export narration from=3 to=7 doc-so', 'epub')).toBe('/export epub from=3 to=7 doc-so')
    expect(withExportFormat('/export format=txt from=2', 'narration')).toBe('/export narration from=2')
  })
})

describe('paragraph diff', () => {
  it('marks changed paragraphs and collapses far unchanged ones', () => {
    const before = ['A', 'B', 'C', 'D', 'E'].join('\n')
    const after = ['A', 'B', 'C', 'D', 'E2'].join('\n')
    expect(paragraphDiff(before, after)).toEqual([
      { kind: 'same', text: '… 3 đoạn không đổi …' },
      { kind: 'same', text: 'D' },
      { kind: 'add', text: 'E2' },
      { kind: 'del', text: 'E' },
    ])
  })
})
