import { describe, expect, it } from 'vitest'
import type { AudioCast, AudioChapter, AudioFlag, AudioSegment } from '../../api/types'
import {
  allDone, chapterAudioLine, chapterMarkLines, chapterStatusLabel, clockOf, estimateSegmentSeconds,
  exportDiskNeed, filterSegments, flagPosition, followIndex, followTimeline, formatBytes, formatClock,
  formatDuration, formatEta, overallPercent, peakBars, playedFraction, renderSpeedLabel,
  renderStep, segmentParts, sortFlags, speakerFilters, stepFlagIndex, stepIssueIndex, volumeTotals,
} from './logic'

const segment = (id: string, speaker: string, text: string, opts: Partial<AudioSegment> = {}): AudioSegment => ({
  id, speaker, text, source: text, pauseMs: 400, tone: '', normalized: [], flags: [], ...opts,
})

const chapter = (n: number, opts: Partial<AudioChapter> = {}): AudioChapter => ({
  chapter: n, title: `Chương ${n}`, status: 'review', issues: 0, durationSec: 1032,
  lockedAt: null, segments: 46, speakers: 5, audio: null, changedSegments: 0, ...opts,
})

const flag = (id: string, chapterNumber: number, atSec: number): AudioFlag => ({
  id, chapter: chapterNumber, segmentId: `${chapterNumber}-s0001`, atSec, reason: 'Đọc hơi nhanh',
  speaker: 'Dẫn truyện', durationSec: 6,
})

const cast: AudioCast = {
  mode: 'cast',
  single: { voiceId: 'giong-tri-ha', dialogueShift: true, keepTones: true, dialoguePauseMs: 0 },
  speakers: [
    { id: 'dan', name: 'Dẫn truyện', role: '', voiceId: 'giong-tri-ha' },
    { id: 'th', name: 'Lâm Tri Hạ', role: '', voiceId: 'giong-tri-ha' },
    { id: 'nv', name: 'Tống Nhược Vy', role: '', voiceId: 'giong-nhuoc-vy' },
  ],
}

describe('thời gian', () => {
  it('formatClock: dưới 1 giờ mm:ss, từ 1 giờ h:mm:ss', () => {
    expect(formatClock(0)).toBe('00:00')
    expect(formatClock(372)).toBe('06:12')
    expect(formatClock(1032)).toBe('17:12')
    expect(formatClock(8671)).toBe('2:24:31')
    expect(formatClock(-5)).toBe('00:00')
  })

  it('không hiện giây kế tiếp trước đồng hồ audio', () => {
    expect(formatClock(29.95)).toBe('00:29')
    expect(formatClock(59.999)).toBe('00:59')
    expect(formatClock(3599.999)).toBe('59:59')
  })

  it('formatDuration đọc thành câu, bỏ đơn vị bằng 0', () => {
    expect(formatDuration(8671)).toBe('2 giờ 24 phút 31 giây')
    expect(formatDuration(1020)).toBe('17 phút')
    expect(formatDuration(42)).toBe('42 giây')
    expect(formatDuration(0)).toBe('0 giây')
    expect(formatDuration(3600)).toBe('1 giờ')
  })

  it('formatEta và formatBytes dùng cho phòng thu / tệp xuất', () => {
    expect(formatEta(0)).toBe('sắp xong')
    expect(formatEta(40)).toBe('còn 40 giây')
    expect(formatEta(720)).toBe('còn 12 phút')
    expect(formatBytes(207_618_048)).toBe('198 MB')
    expect(formatBytes(1_536_000_000)).toBe('1,4 GB')
    expect(formatBytes(2048)).toBe('2 KB')
  })

  it('clockOf lấy giờ địa phương của mốc RFC3339', () => {
    expect(clockOf(null)).toBe('—')
    expect(clockOf('không-phải-ngày')).toBe('—')
    const iso = new Date(2026, 9, 25, 23, 18).toISOString()
    expect(clockOf(iso)).toBe('23:18')
  })

  it('ước tính thời lượng đoạn từ số từ + ngừng', () => {
    expect(estimateSegmentSeconds('một hai ba', 0)).toBeCloseTo(3 / 2.6, 5)
    expect(estimateSegmentSeconds('một', 0)).toBeCloseTo(0.6, 5)
    expect(estimateSegmentSeconds('', 2000)).toBeCloseTo(2.6, 5)
  })
})

describe('sóng âm', () => {
  it('peakBars thu mẫu theo giá trị lớn nhất trong mỗi ô', () => {
    expect(peakBars([0.1, 0.9, 0.2, 0.4], 2)).toEqual([0.9, 0.4])
    expect(peakBars([0.1, 0.2, 0.3, 0.4], 2)).toEqual([0.2, 0.4])
  })

  it('peakBars giãn mẫu khi cần nhiều vạch hơn số đỉnh', () => {
    const bars = peakBars([0.5, 1], 4)
    expect(bars).toHaveLength(4)
    expect(bars).toEqual([0.5, 0.5, 1, 1])
  })

  it('peakBars luôn có sàn để vẽ được vạch nền', () => {
    expect(peakBars([0, 0.001], 2)).toEqual([0.06, 0.06])
    expect(peakBars([], 3)).toEqual([0.18, 0.18, 0.18])
  })

  it('playedFraction kẹp trong 0…1', () => {
    expect(playedFraction(0, 100)).toBe(0)
    expect(playedFraction(50, 100)).toBe(0.5)
    expect(playedFraction(150, 100)).toBe(1)
    expect(playedFraction(10, 0)).toBe(0)
  })
})

describe('dòng chạy theo', () => {
  const segments = [
    segment('s1', 'th', 'một hai ba bốn', { audio: { url: '', state: 'ready', durationSec: 4 } }),
    segment('s2', 'dan', 'năm sáu', { audio: null, pauseMs: 1000 }),
    segment('s3', 'nv', 'bảy', { audio: { url: '', state: 'ready', durationSec: 2 } }),
  ]

  it('dùng thời lượng thật khi đã có giọng, ước tính khi chưa', () => {
    const timeline = followTimeline(segments)
    expect(timeline[0]).toMatchObject({ startSec: 0, endSec: 4.4 })
    expect(timeline[1].startSec).toBe(4.4)
    expect(timeline[1].endSec).toBeCloseTo(4.4 + 2 / 2.6 + 1, 5)
    expect(timeline[2].endSec).toBeCloseTo(timeline[1].endSec + 2.4, 5)
  })

  it('followIndex chọn đúng đoạn đang đọc, kẹp hai đầu', () => {
    const timeline = followTimeline(segments)
    expect(followIndex(timeline, 0)).toBe(0)
    expect(followIndex(timeline, 3.9)).toBe(0)
    expect(followIndex(timeline, 4.4)).toBe(1)
    expect(followIndex(timeline, timeline[1].endSec - 0.01)).toBe(1)
    expect(followIndex(timeline, 9_999)).toBe(2)
    expect(followIndex([], 3)).toBe(-1)
  })

  it('giữ đoạn hiện tại qua khoảng lặng và cộng đủ khoảng lặng sau 30 giây', () => {
    const rendered = Array.from({ length: 12 }, (_, index) => segment(`s${index}`, 'dan', 'Một câu.', {
      pauseMs: 500,
      audio: { url: '', state: 'ready', durationSec: 2.5 },
    }))
    const timeline = followTimeline(rendered)
    expect(timeline[1].startSec).toBe(3)
    expect(followIndex(timeline, 2.9)).toBe(0)
    expect(timeline[10].startSec).toBe(30)
    expect(followIndex(timeline, 29.99)).toBe(9)
    expect(followIndex(timeline, 30)).toBe(10)
  })

  it('cộng khoảng lặng thêm của lời thoại chỉ trong chế độ một giọng', () => {
    const single = { ...cast, mode: 'single' as const, single: { ...cast.single, dialoguePauseMs: 300 } }
    const timeline = followTimeline(segments, single)
    expect(timeline[0].endSec).toBeCloseTo(4.7)
    expect(timeline[1].endSec - timeline[1].startSec).toBeCloseTo(2 / 2.6 + 1)
    expect(timeline[2].endSec - timeline[2].startSec).toBeCloseTo(2.7)
    expect(followTimeline(segments, cast)[0].endSec).toBeCloseTo(4.4)
  })
})

describe('cờ & chỗ cần xem', () => {
  const flags = [flag('f1', 3, 354), flag('f2', 3, 378), flag('f3', 7, 662)]

  it('sortFlags theo chương rồi thời điểm', () => {
    expect(sortFlags([flags[2], flags[1], flags[0]]).map((item) => item.id)).toEqual(['f1', 'f2', 'f3'])
  })

  it('flagPosition đặt cờ trên sóng theo thời lượng chương', () => {
    expect(flagPosition(flags[0], 1032)).toBeCloseTo(354 / 1032, 5)
    expect(flagPosition(flags[0], 0)).toBe(0)
  })

  it('stepFlagIndex quay vòng tiến/lùi', () => {
    expect(stepFlagIndex(flags, 'f1', 1)).toBe(1)
    expect(stepFlagIndex(flags, 'f3', 1)).toBe(0)
    expect(stepFlagIndex(flags, 'f1', -1)).toBe(2)
    expect(stepFlagIndex(flags, null, 1)).toBe(0)
    expect(stepFlagIndex(flags, null, -1)).toBe(2)
    expect(stepFlagIndex([], null, 1)).toBe(-1)
  })

  const segments = [
    segment('s1', 'th', 'không cờ'),
    segment('s2', 'unk', 'chưa rõ người nói', { flags: ['unknown_speaker'] }),
    segment('s3', 'dan', 'câu dài', { flags: ['long_sentence'] }),
    segment('s4', 'th', 'từ tiếng Anh', { flags: ['english'] }),
  ]

  it('stepIssueIndex nhảy kế/trước và quay vòng (J/K)', () => {
    expect(stepIssueIndex(segments, -1, 1)).toBe(1)
    expect(stepIssueIndex(segments, 1, 1)).toBe(2)
    expect(stepIssueIndex(segments, 3, 1)).toBe(1)
    expect(stepIssueIndex(segments, 1, -1)).toBe(3)
    expect(stepIssueIndex(segments, 2, -1)).toBe(1)
    expect(stepIssueIndex(segments, 3, -1)).toBe(2)
  })

  it('chế độ một giọng bỏ cờ về người nói khỏi danh sách cần xem', () => {
    expect(stepIssueIndex(segments, -1, 1, 'single')).toBe(2)
    expect(stepIssueIndex([segments[1]], -1, 1, 'single')).toBe(-1)
  })
})

describe('bản đọc', () => {
  it('accepts Go segments with omitted optional flags and normalization', () => {
    const sparse: AudioSegment = { id: 's1', speaker: 'dan', text: 'Một câu.', pauseMs: 400, tone: '' }
    expect(segmentParts(sparse)).toEqual([{ text: 'Một câu.' }])
    expect(stepIssueIndex([sparse], -1, 1)).toBe(-1)
  })

  it('segmentParts tách phần đã chuẩn hoá để gạch chân xem gốc', () => {
    const parts = segmentParts(segment('s1', 'dan', 'không quá chín mươi giây, gồm kho', {
      normalized: [{ from: '90 giây', to: 'chín mươi giây', kind: 'number' }],
    }))
    expect(parts.map((part) => part.text)).toEqual(['không quá ', 'chín mươi giây', ', gồm kho'])
    expect(parts[1].normalized?.from).toBe('90 giây')
  })

  it('segmentParts tách nhiều chỗ chuẩn hoá trong một câu', () => {
    const parts = segmentParts(segment('s1', 'dan', 'ngày lai cuối và chín mươi giây ấy', {
      normalized: [
        { from: 'live', to: 'lai', kind: 'english' },
        { from: '90 giây', to: 'chín mươi giây', kind: 'number' },
      ],
    }))
    expect(parts.map((part) => part.text)).toEqual(['ngày ', 'lai', ' cuối và ', 'chín mươi giây', ' ấy'])
  })

  it('segmentParts trả nguyên câu khi không có chuẩn hoá', () => {
    expect(segmentParts(segment('s1', 'th', 'một câu'))).toEqual([{ text: 'một câu' }])
  })

  it('filterSegments lọc theo cần xem / người nói / người khác', () => {
    const segments = [
      segment('s1', 'th', 'a'),
      segment('s2', 'nv', 'b', { flags: ['long_sentence'] }),
      segment('s3', 'hc', 'c'),
      segment('s4', 'dan', 'd'),
    ]
    expect(filterSegments(segments, 'all')).toHaveLength(4)
    expect(filterSegments(segments, 'issues').map((item) => item.id)).toEqual(['s2'])
    expect(filterSegments(segments, 'spk:th').map((item) => item.id)).toEqual(['s1'])
    expect(filterSegments(segments, 'other').map((item) => item.id)).toEqual(['s3'])
  })

  it('speakerFilters lấy hai vai nhiều đoạn nhất + Người khác', () => {
    const segments = [
      segment('s1', 'th', 'a'), segment('s2', 'th', 'b'), segment('s3', 'th', 'c'),
      segment('s4', 'nv', 'd'), segment('s5', 'nv', 'e'),
      segment('s6', 'hc', 'f'), segment('s7', 'dm', 'g'),
      segment('s8', 'dan', 'h'),
    ]
    const chips = speakerFilters(segments, cast)
    expect(chips.map((chip) => [chip.label, chip.count])).toEqual([['Lâm Tri Hạ', 3], ['Tống Nhược Vy', 2], ['Người khác', 2]])
  })

})

describe('trạng thái chương', () => {
  it('chapterStatusLabel đổi theo trạng thái và chế độ giọng', () => {
    expect(chapterStatusLabel(chapter(3, { status: 'todo' }))).toBe('Chưa soạn')
    expect(chapterStatusLabel(chapter(3, { status: 'draft' }))).toBe('AI đang soạn…')
    expect(chapterStatusLabel(chapter(3, { status: 'review', issues: 4 }))).toBe('Chờ duyệt · 4 cần xem')
    expect(chapterStatusLabel(chapter(3, { status: 'review', issues: 2 }), 'single')).toBe('Chờ duyệt')
    expect(chapterStatusLabel(chapter(3, { status: 'locked' }))).toBe('Đã chốt')
    expect(chapterStatusLabel(chapter(3, { status: 'stale' }))).toBe('Sửa sau khi chốt')
  })

  it('chapterAudioLine: xong · đang tạo · số đoạn · cần đọc lại', () => {
    expect(chapterAudioLine(chapter(1, {
      status: 'locked', audio: { segmentsDone: 46, segments: 46, durationSec: 1002, rendering: false, percent: 100 },
    }))).toMatchObject({ text: 'xong · 16:42', tone: 'ok' })
    expect(chapterAudioLine(chapter(3, {
      status: 'review', audio: { segmentsDone: 18, segments: 46, durationSec: null, rendering: true, percent: 38 },
    }))).toMatchObject({ text: 'đang tạo 38%', running: true })
    expect(chapterAudioLine(chapter(3, {
      status: 'review', audio: { segmentsDone: 12, segments: 46, durationSec: null, rendering: false, percent: 26 },
    }))).toMatchObject({ text: '12/46 đoạn có giọng', tone: 'muted' })
    expect(chapterAudioLine(chapter(6, { status: 'stale', changedSegments: 2 }))).toMatchObject({ text: '2 đoạn cần đọc lại', tone: 'warn' })
    expect(chapterAudioLine(chapter(7, { status: 'todo' }))).toBeNull()
    expect(chapterAudioLine(chapter(3, {
      status: 'review', audio: { segmentsDone: 3, segments: 7, durationSec: null, rendering: true, percent: 300 / 7 },
    }))).toMatchObject({ text: 'đang tạo 43%', percent: 43 })
  })

  it('allDone đòi đủ chương đã chốt và đủ giọng', () => {
    const done = (n: number) => chapter(n, {
      status: 'locked', audio: { segmentsDone: 46, segments: 46, durationSec: 1000, rendering: false, percent: 100 },
    })
    expect(allDone([done(1), done(2)])).toBe(true)
    expect(allDone([done(1), chapter(2, { status: 'locked' })])).toBe(false)
    expect(allDone([done(1), chapter(2)])).toBe(false)
    expect(allDone([])).toBe(false)
  })

  it('overallPercent cộng theo số đoạn của cả tập', () => {
    expect(overallPercent([
      chapter(1, { audio: { segmentsDone: 46, segments: 46, durationSec: 1000, rendering: false, percent: 100 } }),
      chapter(2, { audio: { segmentsDone: 12, segments: 46, durationSec: null, rendering: false, percent: 26 } }),
    ])).toBe(63)
    expect(overallPercent([chapter(1)])).toBe(0)
  })

  it('volumeTotals đếm đoạn, phút và số người nói', () => {
    expect(volumeTotals([chapter(1, { segments: 46, durationSec: 1002 }), chapter(2, { segments: 51, durationSec: 1085 })]))
      .toEqual({ segments: 97, minutes: 35, speakers: 5 })
  })
})

describe('phòng thu & xuất', () => {
  it('renderStep chia 4 bước của đoạn', () => {
    expect(renderStep(0)).toBe(0)
    expect(renderStep(0.13)).toBe(0)
    expect(renderStep(0.14)).toBe(1)
    expect(renderStep(0.74)).toBe(1)
    expect(renderStep(0.75)).toBe(2)
    expect(renderStep(0.9)).toBe(3)
    expect(renderStep(2)).toBe(3)
  })

  it('renderSpeedLabel đọc tốc độ máy theo rtf', () => {
    expect(renderSpeedLabel(0.683)).toBe('1 phút audio ≈ 41 giây')
    expect(renderSpeedLabel(null)).toBe('Đang đo tốc độ máy')
    expect(renderSpeedLabel(0)).toBe('Đang đo tốc độ máy')
  })

  it('chapterMarkLines xuất mốc dán vào mô tả video', () => {
    expect(chapterMarkLines([
      { atSec: 0, label: 'Mở đầu' },
      { atSec: 12, label: 'Chương 1 · Vai của tôi là đồ canh' },
      { atSec: 3734, label: 'Chương 4 · Bảy mươi ba giây' },
    ])).toEqual(['00:00 Mở đầu', '00:12 Chương 1 · Vai của tôi là đồ canh', '1:02:14 Chương 4 · Bảy mươi ba giây'])
  })

  it('exportDiskNeed: WAV nặng hơn MP3 đúng 8 lần', () => {
    const mp3 = exportDiskNeed(8671, 'mp3')
    const wav = exportDiskNeed(8671, 'wav')
    expect(wav / mp3).toBe(8)
    expect(mp3).toBeGreaterThan(400_000_000)
  })
})
