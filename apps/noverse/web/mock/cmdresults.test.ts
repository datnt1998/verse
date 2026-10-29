import { describe, expect, it } from 'vitest'
import { mockCoCreateProposal, mockExportDownload, mockImportPreview, mockSyncResult } from './cmdresults'

const chapters = [{ title: 'Chương 1: Trà & mưa', text: 'Lam <khẽ> hỏi.\n\n“Trà còn ấm không?”' }]

describe('command result export downloads', () => {
  it('returns real UTF-8 TXT when a text file is requested', () => {
    const result = mockExportDownload('exports/truyen.txt', 'Trà xanh', chapters)
    expect(result.mime).toBe('text/plain; charset=utf-8')
    expect(result.body.toString()).toBe('Chương 1: Trà & mưa\n\nLam <khẽ> hỏi.\n\n“Trà còn ấm không?”')
  })

  it('packages EPUB with an uncompressed first mimetype entry and linked XHTML chapters', () => {
    const result = mockExportDownload('exports/truyen.epub', 'Trà & mưa', chapters)
    expect(result.mime).toBe('application/epub+zip')
    const entries = new Map<string, string>()
    let offset = 0
    while (result.body.readUInt32LE(offset) === 0x04034b50) {
      expect(result.body.readUInt16LE(offset + 8)).toBe(0)
      const bytes = result.body.readUInt32LE(offset + 18)
      const nameLength = result.body.readUInt16LE(offset + 26)
      const extraLength = result.body.readUInt16LE(offset + 28)
      const name = result.body.subarray(offset + 30, offset + 30 + nameLength).toString()
      const start = offset + 30 + nameLength + extraLength
      entries.set(name, result.body.subarray(start, start + bytes).toString())
      offset = start + bytes
    }
    expect([...entries.keys()][0]).toBe('mimetype')
    expect(entries.get('mimetype')).toBe('application/epub+zip')
    expect(entries.get('META-INF/container.xml')).toContain('OEBPS/content.opf')
    expect(entries.get('OEBPS/content.opf')).toContain('<dc:title>Trà &amp; mưa</dc:title>')
    expect(entries.get('OEBPS/content.opf')).toContain('<itemref idref="chapter-1"/>')
    expect(entries.get('OEBPS/nav.xhtml')).toContain('chapter-1.xhtml')
    expect(entries.get('OEBPS/chapter-1.xhtml')).toContain('<p>Lam &lt;khẽ&gt; hỏi.</p>')
    expect(result.body.readUInt32LE(offset)).toBe(0x02014b50)
    expect(result.body.readUInt32LE(result.body.length - 22)).toBe(0x06054b50)
    expect(result.body.readUInt16LE(result.body.length - 12)).toBe(entries.size)
  })
})

describe('versioned mobile command fixtures', () => {
  it('keeps guide re-split flags and long-chapter warning distinct', () => {
    const initial = mockImportPreview('ban-thao.txt')
    const revised = mockImportPreview('ban-thao.txt', 'Tách chương dài thành hai phần')
    expect(initial.chapters).toHaveLength(24)
    expect(initial.warnings).toHaveLength(1)
    expect(revised.chapters).toHaveLength(25)
    expect(revised.chapters.filter((chapter) => chapter.newlySplit)).toHaveLength(2)
    expect(revised.warnings).toHaveLength(0)
    expect(revised.schemaVersion).toBe(1)
  })
  it('preserves proposal revision and actual following chapter numbers', () => {
    const proposal = mockCoCreateProposal(2, 25, 24)
    expect(proposal.revision).toBe(2)
    expect(proposal.chapters.map((chapter) => chapter.chapter)).toEqual([25, 26, 27])
    expect(proposal.impacts).toContain('Giữ nguyên 24 chương')
    expect(proposal.chapters.every((chapter) => chapter.schemaVersion === 1 && chapter.summary.length > 0)).toBe(true)
  })
  it('demonstrates all three accepted sync classifications and actionable conflict', () => {
    const result = mockSyncResult()
    expect(result.schemaVersion).toBe(1)
    expect(result.analyses.map((analysis) => analysis.classification)).toEqual(['conflicts', 'wording', 'facts'])
    expect(result.analyses[0].downstreamIssues[0]).toMatchObject({ chapter: 14, issueIndex: 0, sent: false })
  })
})
