// “Việc cần làm” (Quality.dc.html › view=todo): Đang chặn chương · Cần bạn xem · Sẽ tự sửa khi viết
// lại (gập sẵn). “Nhờ sửa ở chương này” thêm mục vào ý kiến cho lần viết lại; “Giữ nguyên…” mở ô lý do
// của luồng giữ nguyên GAP-47 (ReviewKeepForm của ReviewIssue).
import { useState } from 'react'
import type { QualityIssue } from '../../api/types'
import { t } from '../../i18n/vi'
import { navigate, paths } from '../../router'
import { Icon } from '@noverse/ui'
import { autoChapters, GROUP_LIMIT, type TodoGroups } from './model'
import { ReviewKeepForm, reviewLevel } from '../../ui/ReviewIssue'

function TodoCard({ issue, kind, project, story, writable, saving, onAskFix, onKeep }: {
  issue: QualityIssue
  kind: 'block' | 'look'
  project: string
  story: string
  writable: boolean
  saving: boolean
  onAskFix: (issue: QualityIssue) => void
  onKeep: (issue: QualityIssue, reason: string) => Promise<boolean>
}) {
  const [asking, setAsking] = useState(false)
  const error = reviewLevel(issue) === 'error'
  return (
    <article className="q-todo" style={{ borderColor: error ? 'var(--quality-todo-err-bd)' : 'var(--line-2)' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
        <span className="q-chip" style={{
          background: error ? 'var(--kit-errf-12)' : 'var(--kit-warn-12)', color: error ? 'var(--kit-err-soft)' : 'var(--kit-warn-soft)',
        }}>{error ? t.review.error : t.review.warning}</span>
        <span className="q-chip" style={{ background: 'var(--kit-wash-5)', color: 'var(--color-fg-2)' }}>{issue.ref}</span>
        <span style={{ fontSize: 12.5, color: 'var(--kit-fg-icon)' }}>{t.quality.refChapter(issue.refName, issue.ch)}</span>
        <span style={{ flex: 1 }} />
        <span style={{ fontSize: 12, color: kind === 'block' ? 'var(--kit-badge-run)' : 'var(--color-fg)' }}>
          {kind === 'block' ? t.quality.blockState(issue.ch) : t.quality.state.open}
        </span>
      </div>
      <span style={{ fontSize: 14, color: 'var(--color-fg)', lineHeight: 1.5 }}>{issue.text}</span>
      {issue.quote !== '' && (
        <span className="q-tq" style={{ borderLeftColor: error ? 'var(--quality-todo-err-q)' : 'var(--quality-todo-warn-q)' }}>“{issue.quote}”</span>
      )}
      {asking && (
        <ReviewKeepForm saving={saving} onCancel={() => setAsking(false)} onSave={async (reason) => {
          if (await onKeep(issue, reason)) setAsking(false)
        }} />
      )}
      {!asking && (
        <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
          {writable && (kind === 'block' ? (
            // Ghi chú cho Người viết đi qua ô ý kiến chương sẵn có ở Trình đọc (không dựng ô mới);
            // `?note=1` đặt con trỏ vào ô bình luận chương (Câu 11).
            <button type="button" className="q-tbtn" onClick={() => navigate(paths.readerNote(project, story, issue.ch))}>{t.quality.addNote}</button>
          ) : (
            <>
              <button type="button" className="q-tbtn pri" disabled={saving} onClick={() => onAskFix(issue)}>{t.quality.askFix}</button>
              <button type="button" className="q-tbtn" disabled={saving} onClick={() => setAsking(true)}>{t.quality.keep}</button>
            </>
          ))}
          <span style={{ flex: 1 }} />
          <a href={`#${paths.reader(project, story, issue.ch)}`} style={{ fontSize: 12.5, color: 'var(--color-fg-3)' }}>{t.quality.openPassage}</a>
        </div>
      )}
    </article>
  )
}

export function Todo({ groups, project, story, writable, savingId, onAskFix, onKeep }: {
  groups: TodoGroups
  project: string
  story: string
  writable: boolean
  /** id mục đang lưu (nhờ sửa / giữ nguyên) */
  savingId: string
  onAskFix: (issue: QualityIssue) => void
  onKeep: (issue: QualityIssue, reason: string) => Promise<boolean>
}) {
  const sections = [
    { kind: 'block' as const, items: groups.block, title: t.quality.groups.block, sub: t.quality.groups.blockSub, dot: 'var(--color-err)' },
    { kind: 'look' as const, items: groups.look, title: t.quality.groups.look, sub: t.quality.groups.lookSub, dot: 'var(--color-warn)' },
  ].filter((section) => section.items.length > 0)
  const autoSub = autoChapters(groups.auto)
  return (
    <div className="scroll" style={{ flex: '1 1 auto', minHeight: 0, padding: '0 24px 24px', display: 'flex', flexDirection: 'column', gap: 18 }}>
      {sections.map((section) => (
        <section key={section.kind} aria-label={section.title} style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          <div style={{ display: 'flex', alignItems: 'baseline', gap: 10 }}>
            <span style={{ width: 8, height: 8, borderRadius: '50%', background: section.dot, flex: 'none', transform: 'translateY(-1px)' }} />
            <h2 style={{ margin: 0, fontSize: 15, fontWeight: 600 }}>{section.title}</h2>
            <span className="num" style={{ fontSize: 12.5, color: 'var(--kit-fg-icon)' }}>{section.items.length}</span>
            <span title={section.sub} style={{
              flex: 1, minWidth: 0, fontSize: 12.5, color: 'var(--kit-fg-icon)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
            }}>{section.sub}</span>
          </div>
          {section.items.slice(0, GROUP_LIMIT[section.kind]).map((issue) => (
            <TodoCard key={issue.id} issue={issue} kind={section.kind} project={project} story={story} writable={writable}
              saving={savingId === issue.id} onAskFix={onAskFix} onKeep={onKeep} />
          ))}
        </section>
      ))}
      {/* Artboard luôn dựng nhóm gập, kể cả khi 0 cảnh báo (level=error). */}
      <details className="q-auto" style={{ borderRadius: 16, border: '1px dashed var(--kit-line-12)' }}>
          <summary style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '12px 14px' }}>
            <Icon name="arrow-right-2" size={14} className="q-chev" style={{ color: 'var(--kit-fg-icon)' }} />
            <span style={{ flex: 1, fontSize: 13.5, color: 'var(--color-fg-2)' }}>
              {t.quality.groups.auto} <span className="num" style={{ color: 'var(--kit-fg-icon)' }}>{t.quality.groups.autoCount(groups.auto.length)}</span>
            </span>
            {autoSub !== '' && <span style={{ fontSize: 12.5, color: 'var(--kit-fg-icon)' }}>{t.quality.groups.autoSub(autoSub)}</span>}
          </summary>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6, padding: '0 14px 14px' }}>
            {groups.auto.map((issue) => (
              <div key={issue.id} style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13 }}>
                <span className="q-chip" style={{ background: 'var(--kit-wash-5)', color: 'var(--color-fg-2)', flex: 'none' }}>{issue.ref}</span>
                <span className="num" style={{ flex: 'none', fontSize: 12, color: 'var(--kit-fg-icon)' }}>Ch. {issue.ch}</span>
                <span title={issue.text} style={{
                  flex: 1, minWidth: 0, color: 'var(--color-fg-3)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
                }}>{issue.text}</span>
              </div>
            ))}
          </div>
      </details>
      {groups.block.length === 0 && groups.look.length === 0 && (
        <div style={{
          padding: 28, borderRadius: 16, border: '1px dashed var(--quality-done-bd)', textAlign: 'center', color: 'var(--color-ok)', fontSize: 14,
        }}>{t.quality.todoDone}</div>
      )}
    </div>
  )
}
