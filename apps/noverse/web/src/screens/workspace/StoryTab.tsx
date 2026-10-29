// Tab Truyện của cột phải Bàn viết (Main.dc.html › tab Truyện + Gói P2 · 21-dev-prompt.md:624-635).
// Lối vào nhanh Não truyện: nút Đồ thị, “Tất cả” mở dạng Danh sách, hàng nhân vật là nút có thẻ rê,
// thẻ “Đổi ở chương N”, “Khung truyện ›”, tóm tắt gần đây mở đúng chương trong Trình đọc.
// Điện thoại: chạm tên nhân vật mở tấm kéo lên của nhân vật trong MBrain (không có ngăn kéo).
import { useId, useMemo, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import type { AddressTerms, Brain, BrainEntity, CharacterKind, Snapshot } from '../../api/types'
import { t } from '../../i18n/vi'
import { translateHost } from '../../i18n/hostStrings'
import { ChapterContent, parseChapterText } from '../../lib/markdown'
import { splitCastLabel } from '../../lib/cast'
import { KIND_META, isKindHidden, kindLabel, sortByKind } from '../../lib/charKind'
import { navigate, paths } from '../../router'
import { CardTitle } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { EmptyState } from '@noverse/ui'
import { useAddressTerms, useBrain } from '../project/cast/data'
import { addressRowsAt, changesAt, initialChapter, latestWritten, relsAt } from '../brain/model'
import { openBrainDrawer } from './brainDrawer'
import { summaryChapter } from './format'
import { s } from './storyTab.stylex'
import { CharAvatar, KindLegend } from '../../ui/CharAvatar'

// base.css đặt `a { color: var(--color-fg) }` ngoài layer → mọi link trong tab phải giữ màu inline.
const fg2 = { color: 'var(--color-fg-2)' }
const fg3 = { color: 'var(--color-fg-3)' }

export function StoryTab({ snapshot, project, story, mobile = false }: {
  snapshot: Snapshot
  project: string
  story: string
  /** Điện thoại: mở MBrain thay vì ngăn kéo (P3 · Điện thoại). */
  mobile?: boolean
}) {
  const commit = translateHost(snapshot.lastCommitSummary)
  const review = translateHost(snapshot.lastReviewSummary)
  // Snapshot thật trả premise dạng markdown (`# Tiền đề … ## …`); synopsis có thể là đoạn thường.
  const synopsis = useMemo(() => parseChapterText(snapshot.synopsis ?? ''), [snapshot.synopsis])
  const premise = useMemo(() => parseChapterText(snapshot.premise ?? ''), [snapshot.premise])
  // “Tiền đề · …” của artboard là một dòng: gộp mọi khối văn bản của tiền đề lại.
  const premiseLine = useMemo(() => premise
    .flatMap((block) => ('text' in block ? [block.text] : block.kind === 'list' ? block.items : []))
    .join(' ').replace(/\s+/g, ' ').trim(), [premise])
  const brain = useBrain(project, story)
  const address = useAddressTerms(project, story)
  const data = brain.data ?? null
  // “Chương đang viết” của Não truyện: chương đang viết dở, không thì chương hiện tại của Người viết.
  const chapter = snapshot.inProgressChapter || snapshot.currentChapter
    || (data ? initialChapter(data.chapters) : 0)
  // “Chương đang xem” ở Bàn viết để xét avatar chưa lộ = chương mới nhất đã nộp (Gói S).
  const viewed = data ? latestWritten(data.chapters) : chapter
  // Gói U: danh sách, thẻ rê và đồ thị cùng đọc thực thể từ Não truyện.
  const cast = sortByKind(data?.entities.filter((entity) => entity.type === 'char' && entity.fromChapter <= chapter) ?? [])

  const open = (target: string | null) => {
    if (!mobile) { openBrainDrawer(target, chapter); return }
    const base = paths.brain(project, story, chapter > 0 ? chapter : null)
    navigate(`${base}${base.includes('?') ? '&' : '?'}person=${encodeURIComponent(target ?? '')}`)
  }

  const tiles = stylex.props(s.tiles)
  const tile = stylex.props(s.tile)
  const tileLabel = stylex.props(s.tileLabel)
  const tileValue = stylex.props(s.tileValue)
  const castCard = stylex.props(s.castCard)
  const castHead = stylex.props(s.castHead)
  const castHeadRight = stylex.props(s.castHeadRight)
  const castCount = stylex.props(s.castCount)
  const castLink = stylex.props(s.castLink)
  const graphBtn = stylex.props(s.graphBtn)
  const recentSupporting = stylex.props(s.recentSupporting)
  const synopsisCard = stylex.props(s.synopsisCard)
  const synopsisBody = stylex.props(s.synopsisBody)
  const premiseBody = stylex.props(s.premiseBody)
  const premiseLabel = stylex.props(s.premiseLabel)
  const empty = stylex.props(s.empty)
  const frameLink = stylex.props(s.frameLink)
  const summaryCard = stylex.props(s.summaryCard)
  const summaryRow = stylex.props(s.summaryRow)
  const summaryNum = stylex.props(s.summaryNum)

  return (
    <>
      {/* Gói P · P1: lập kế hoạch giấu cả hai ô (Main.dc.html: chưa có chương nào để nộp/duyệt). */}
      {!snapshot.plan && (
        <div {...tiles}>
          <a className={['card', 'hov-row', tile.className].filter(Boolean).join(' ')} style={tile.style}
            href={`#${paths.reader(project, story)}`}>
            <Icon name="document-text-bulk" size={16} style={{ color: 'var(--color-accent)' }} />
            <span {...tileLabel}>{t.workspace.latestCommit}</span>
            <span className={['num', tileValue.className].filter(Boolean).join(' ')} style={tileValue.style} title={commit}>
              {commit || '—'}
            </span>
          </a>
          <a className={['card', 'hov-row', tile.className].filter(Boolean).join(' ')} style={tile.style}
            href={`#${paths.reader(project, story)}`}>
            <Icon name="shield-tick-bulk" size={16} style={{ color: 'var(--color-warn)' }} />
            <span {...tileLabel}>{t.workspace.latestReview}</span>
            <span className={tileValue.className} style={tileValue.style} title={review}>
              {review || '—'}
            </span>
          </a>
        </div>
      )}

      <div className={['card', castCard.className].filter(Boolean).join(' ')} style={castCard.style}>
        <div {...castHead}>
          <CardTitle icon={<Icon name="profile-2user" size={15} />}>{t.workspace.cast}</CardTitle>
          <span {...castHeadRight}>
            {snapshot.supportingCount > 0 && (
              <span className={['num', castCount.className].filter(Boolean).join(' ')} style={castCount.style}>
                +{snapshot.supportingCount}
              </span>
            )}
            {!mobile && (
              <button type="button" className={graphBtn.className} style={graphBtn.style}
                aria-label={t.workspace.openBrain} title={t.workspace.openBrain} onClick={() => open(null)}>
                <Icon name="routing-2" size={13} />
                {t.workspace.graph}
              </button>
            )}
            {/* P2: “Tất cả” mở Não truyện ở dạng Danh sách, không dẫn về Dự án & truyện. */}
            <a className={castLink.className} style={{ ...castLink.style, ...fg3 }}
              href={`#${paths.brain(project, story)}?view=list`}>{t.workspace.allCast}</a>
          </span>
        </div>
        {cast.length === 0 && <EmptyState icon="profile-2user" title={t.workspace.noCast} />}
        {cast.map((character) => (
          <CastRow key={character.name} character={character} brain={data} address={address.data ?? null}
            chapter={chapter} viewed={viewed} onOpen={open} />
        ))}
        {cast.length > 0 && <KindLegend />}
        {(snapshot.recentSupporting ?? []).length > 0 && (
          <div {...recentSupporting}>
            {(snapshot.recentSupporting ?? []).map((entry) => splitCastLabel(entry).name).join(' · ')}
          </div>
        )}
      </div>

      {data && chapter > 0 && <ChangeCard brain={data} chapter={chapter} onOpen={open} />}

      <div className={['card', synopsisCard.className].filter(Boolean).join(' ')} style={synopsisCard.style}>
        <CardTitle icon={<Icon name="book-1" size={15} />} right={
          <a className={frameLink.className} style={{ ...frameLink.style, ...fg3 }}
            href={`#${paths.story(project, story)}`} title={t.workspace.frameLinkTitle}>{t.workspace.frameLink}</a>
        }>{t.workspace.synopsis}</CardTitle>
        <div {...synopsisBody}>
          {synopsis.length > 0
            ? <ChapterContent blocks={synopsis} unmatchedRevisions={[]} showDiff={false} />
            : <span {...empty}>—</span>}
        </div>
        {/* Main.dc.html: “Tiền đề · <tag>” là một dòng, không phải khối nhiều mục. */}
        {premiseLine !== '' && (
          <div {...premiseBody}>
            <span {...premiseLabel}>{t.workspace.premise} · </span>
            {premiseLine}
          </div>
        )}
      </div>

      {/* Gói P · P1: lập kế hoạch vẫn giữ thẻ, hiện dòng “Chưa có chương nào…”. */}
      {(snapshot.plan || (snapshot.recentSummaries ?? []).length > 0) && (
        <div className={['card', summaryCard.className].filter(Boolean).join(' ')} style={summaryCard.style}>
          <CardTitle icon={<Icon name="clock" size={15} />}>{t.workspace.recentSummaries}</CardTitle>
          {(snapshot.recentSummaries ?? []).length === 0 && (
            <div {...stylex.props(s.summaryHint)}>{t.plan.noSummaries}</div>
          )}
          {(snapshot.recentSummaries ?? []).map((summary, index) => {
            const text = translateHost(summary)
            const at = summaryChapter(summary)
            // Main.dc.html: tiền tố “Chương N · ” mờ, phần tóm tắt sáng; cả dòng mở Trình đọc.
            const body = at ? text.replace(/^(?:Chương|第)\s*\d+\s*[:·]\s*/, '') : text
            const label = <><span {...summaryNum}>Chương {at} · </span>{body}</>
            // P2: mỗi dòng mở đúng chương đó trong Trình đọc; dòng không rõ số chương giữ dạng chữ.
            return at
              ? <a key={index} className={['hov-row', summaryRow.className].filter(Boolean).join(' ')}
                style={{ ...summaryRow.style, ...fg2 }}
                href={`#${paths.reader(project, story, at)}`} title={text}>{label}</a>
              : <div key={index} className={summaryRow.className} style={summaryRow.style} title={text}>{text}</div>
          })}
        </div>
      )}
    </>
  )
}

/** Một hàng nhân vật: nút mở Não truyện chọn sẵn người đó; rê/focus hiện thẻ nhỏ `role="tooltip"`. */
function CastRow({ character, brain, address, chapter, viewed, onOpen }: {
  character: BrainEntity
  brain: Brain | null
  address: AddressTerms | null
  chapter: number
  /** Chương mới nhất đã nộp — xét “chưa lộ” của avatar (Gói S). */
  viewed: number
  onOpen: (target: string) => void
}) {
  const [peek, setPeek] = useState(false)
  const tipId = useId()
  const { name, role } = character
  const kind: CharacterKind = character.kind ?? 'support'
  const hidden = isKindHidden(character.revealAt, viewed)
  const meta = KIND_META[kind]
  const entity = brain?.entities.find((item) => item.id === name || item.name === name) ?? null
  const rels = brain && entity && chapter > 0 ? relsAt(brain, entity.id, chapter).slice(0, 2) : []
  const xh = entity && address && chapter > 0
    ? addressRowsAt(address, entity, chapter).find((row) => row.from === entity.name || row.from === entity.id)
    : undefined
  const showPeek = peek && entity !== null
  const row = stylex.props(s.castRow)
  const rowText = stylex.props(s.rowText)
  const rowName = stylex.props(s.rowName)
  const rowRole = stylex.props(s.rowRole)
  const peekCard = stylex.props(s.peek)
  const peekHead = stylex.props(s.peekHead)
  const peekKind = stylex.props(s.peekKind)
  const peekRel = stylex.props(s.peekRel)
  const peekLine = stylex.props(s.peekLine)
  const peekLabel = stylex.props(s.peekLabel)
  const peekOther = stylex.props(s.peekOther)
  const peekSince = stylex.props(s.peekSince)
  const peekXh = stylex.props(s.peekXh)
  const peekHint = stylex.props(s.peekHint)
  return (
    <button type="button" className={row.className} style={row.style}
      aria-describedby={showPeek ? tipId : undefined}
      onMouseEnter={() => setPeek(true)} onMouseLeave={() => setPeek(false)}
      onFocus={() => setPeek(true)} onBlur={() => setPeek(false)}
      onClick={() => onOpen(entity?.id ?? name)}>
      <CharAvatar name={name} ini={character.ini} kind={kind} hidden={hidden} />
      <span {...rowText}>
        <span {...rowName} title={name}>{name}</span>
        {role && <span {...rowRole} title={role}>{role}</span>}
      </span>
      {showPeek && (
        <span role="tooltip" id={tipId} className={peekCard.className} style={peekCard.style}>
          <span {...peekHead}>
            {t.workspace.peekAtChapter(chapter)}
            {/* 04 › “Nhãn loại” — thẻ rê nói rõ loại vai, kèm “· chưa lộ” khi chưa tới chương lộ. */}
            <span {...peekKind} style={{ color: meta.fg }}>{kindLabel(kind, hidden)}</span>
          </span>
          {rels.map((rel) => (
            <span key={rel.id} {...peekRel}>
              <span aria-hidden="true" className={peekLine.className}
                style={{ ...peekLine.style, backgroundColor: rel.color }} />
              <span {...peekLabel} style={{ color: rel.color }}>{rel.label}</span>
              <span {...peekOther}>{rel.otherName}</span>
              <span {...peekSince}>{t.brain.sinceChapter(rel.sinceChapter)}</span>
            </span>
          ))}
          {xh && (
            <span {...peekXh}>
              {t.workspace.peekXh(xh.to, xh.call, xh.self)}
            </span>
          )}
          <span {...peekHint}>{t.workspace.peekOpen}</span>
        </span>
      )}
    </button>
  )
}

/** Quan hệ bắt đầu hoặc đổi ở chương đang xem; cùng bộ lọc với đồ thị. */
function ChangeCard({ brain, chapter, onOpen }: {
  brain: Brain
  chapter: number
  onOpen: (target: string) => void
}) {
  const changes = useMemo(() => changesAt(brain, chapter), [brain, chapter])
  const card = stylex.props(s.changeCard)
  const head = stylex.props(s.changeHead)
  const count = stylex.props(s.changeCount)
  const row = stylex.props(s.changeRow)
  const tag = stylex.props(s.changeTag)
  const text = stylex.props(s.changeText)
  const none = stylex.props(s.changeEmpty)
  const action = stylex.props(s.changeAction)
  return (
    <div className={['card', card.className].filter(Boolean).join(' ')} style={card.style}>
      <div {...head}>
        <CardTitle icon={<Icon name="refresh-2" size={15} />}>{t.workspace.changedTitle(chapter)}</CardTitle>
        {changes.length > 0 && <span {...count}>{t.workspace.changedCount(changes.length)}</span>}
      </div>
      {changes.length === 0 && <span {...none}>{t.workspace.changedNone}</span>}
      {changes.map((change) => (
        <div key={change.id} {...row}>
          <span aria-hidden="true" {...tag}>{change.changed ? t.workspace.changedChanged : t.workspace.changedNew}</span>
          <span {...text}>
            <span style={{ color: 'var(--color-fg)' }}>{change.who}</span>
            <span style={{ color: change.color }}> · {change.rel} · </span>
            <span style={{ color: 'var(--color-fg)' }}>{change.to}</span>
          </span>
        </div>
      ))}
      {/* Main.dc.html: nút luôn hiện — chưa có thay đổi thì mở Não truyện ở chương này. */}
      <button type="button" className={action.className} style={action.style}
        onClick={() => onOpen(changes[0]?.entity ?? brain.entities[0]?.id ?? '')}>{t.workspace.changedView}</button>
    </div>
  )
}
