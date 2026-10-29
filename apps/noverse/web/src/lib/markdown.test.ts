import { describe, expect, it } from 'vitest'
import type { RevisionEntry } from '../api/types'
import { applyRevisions, parseChapterText, parseInline } from './markdown'

describe('parseChapterText', () => {
  it('tách tiêu đề, đoạn, trích, danh sách và dấu ngắt cảnh', () => {
    const blocks = parseChapterText([
      '# Chương 1',
      '',
      'Đoạn một.',
      'Vẫn đoạn một.',
      '',
      '> Trích dẫn',
      '',
      '- Một',
      '- Hai',
      '',
      '***',
      '',
      'Đoạn cuối.',
    ].join('\n'))
    expect(blocks).toEqual([
      { kind: 'heading', level: 1, text: 'Chương 1' },
      { kind: 'paragraph', text: 'Đoạn một.\nVẫn đoạn một.' },
      { kind: 'quote', text: 'Trích dẫn' },
      { kind: 'list', items: ['Một', 'Hai'] },
      { kind: 'break' },
      { kind: 'paragraph', text: 'Đoạn cuối.' },
    ])
  })

  it('không trả khối cho văn bản rỗng và chuẩn hoá CRLF', () => {
    expect(parseChapterText('   \n\n')).toEqual([])
    expect(parseChapterText('Một\r\n\r\nHai')).toEqual([
      { kind: 'paragraph', text: 'Một' },
      { kind: 'paragraph', text: 'Hai' },
    ])
  })

  it('giới hạn cấp tiêu đề tối đa 3', () => {
    expect(parseChapterText('##### Sâu')[0]).toEqual({ kind: 'heading', level: 3, text: 'Sâu' })
  })
})

describe('parseInline', () => {
  it('tách đậm, nghiêng, mã và giữ chữ thường', () => {
    expect(parseInline('a **b** *c* `d` e')).toEqual([
      { text: 'a ' },
      { text: 'b', bold: true },
      { text: ' ' },
      { text: 'c', italic: true },
      { text: ' ' },
      { text: 'd', code: true },
      { text: ' e' },
    ])
  })

  it('không coi dấu sao đơn lẻ là nhấn mạnh', () => {
    expect(parseInline('5 * 3 = 15')).toEqual([{ text: '5 * 3 = 15' }])
  })
})

describe('applyRevisions', () => {
  const revision: RevisionEntry = {
    time: null, kind: 'edit_chapter', old: 'hai người', new: 'ba người', replaceAll: false,
  }

  it('chèn diff vào đúng đoạn và giữ phần trước/sau', () => {
    const blocks = parseChapterText('Trước đó hai người đọc bản thảo. Sau đó im lặng.')
    const result = applyRevisions(blocks, [revision])
    expect(result.unmatched).toEqual([])
    expect(result.blocks).toHaveLength(3)
    expect(result.blocks[0]).toEqual({ kind: 'paragraph', text: 'Trước đó' })
    expect(result.blocks[1]).toMatchObject({ kind: 'diff', old: 'hai người', new: 'ba người' })
    expect(result.blocks[1]).toHaveProperty('meta', 'Gọt giũa')
    expect(result.blocks[2]).toEqual({ kind: 'paragraph', text: 'đọc bản thảo. Sau đó im lặng.' })
  })

  it('trả revision không khớp văn bản để panel riêng hiển thị', () => {
    const result = applyRevisions(parseChapterText('Không liên quan.'), [revision])
    expect(result.blocks).toHaveLength(1)
    expect(result.unmatched).toEqual([revision])
  })
})
