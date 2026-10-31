import * as stylex from '@stylexjs/stylex'
import { EmptyState, Icon, Skeleton } from '@noverse/ui'
import { foundationCopy as c } from '../../../i18n/apFoundation'
import { paths } from '../../../router'
import { groupWorldRules } from './foundation'
import { foundationStyles as s } from './foundation.styles'
import type { FoundationResource } from './useFoundation'

export function AskButton({ children = c.askEdit, onClick, small = false }: { children?: React.ReactNode; onClick: () => void; small?: boolean }) {
  return <button type="button" className={`hov-ghost ${stylex.props(s.button, !small && s.ask).className}`} onClick={onClick}><Icon name="magic-star" size={small ? 13 : 14} style={{ color: 'var(--color-accent)' }} />{children}</button>
}
export function PremiseTab({ foundation, onSteer }: { foundation: FoundationResource; onSteer: (text: string) => void }) {
  if (!foundation.ready) return foundation.premise.error ? <EmptyState icon="warning-2" title={c.loadingPremise} action={<button type="button" onClick={() => void foundation.premise.refresh()}>{c.retry}</button>} /> : <Skeleton height={260} radius={16} />
  const missing = foundation.required.filter((row) => row.missing).length
  const updated = foundation.premise.data?.updatedAt
  return <div {...stylex.props(s.root)}>
    <div {...stylex.props(s.top)}><span {...stylex.props(s.topCopy)}><span {...stylex.props(s.title)}>{c.premise}<span {...stylex.props(s.badge, missing > 0 && s.missingBadge)}>{missing ? c.incomplete(missing, foundation.required.length) : c.complete(foundation.required.length)}</span></span><span {...stylex.props(s.muted)}>{c.premiseIntro}{updated ? ` · cập nhật ${new Date(updated).toLocaleDateString('vi-VN', { day: '2-digit', month: '2-digit' })}` : ''}</span></span><AskButton onClick={() => onSteer(c.editPremise)} /></div>
    <section className={`card ${stylex.props(s.premiseCard).className}`} aria-label="Các mục của tiền đề">
      {foundation.rows.map((row, index) => <div key={row.heading} {...stylex.props(s.premiseRow, index === 0 && s.first)}>
        <span {...stylex.props(s.premiseLabel)}>{row.heading}</span>
        <span {...stylex.props(s.premiseText, row.missing && s.missingText)}>{row.missing ? <><Icon name="warning-2" size={15} style={{ marginTop: 3, flexShrink: 0, color: 'var(--color-warn)' }} />{c.missing}</> : row.text}</span>
        <span>{row.missing && <AskButton small onClick={() => onSteer(c.supplementPremise(row.heading))}>{c.supplement}</AskButton>}</span>
      </div>)}
    </section>
  </div>
}
export function WorldTab({ foundation, project, completed, onSteer }: { foundation: FoundationResource; project: string; completed: number; onSteer: (text: string) => void }) {
  const rules = foundation.world.data
  if (rules === undefined) return foundation.world.error ? <EmptyState icon="warning-2" title={c.loadingWorld} action={<button type="button" onClick={() => void foundation.world.refresh()}>{c.retry}</button>} /> : <Skeleton height={260} radius={16} />
  const groups = groupWorldRules(rules)
  return <div {...stylex.props(s.root)}>
    <div {...stylex.props(s.top)}><span {...stylex.props(s.topCopy)}><span {...stylex.props(s.title)}>{c.world}{rules.length > 0 && <span className={`num ${stylex.props(s.small).className}`}>{rules.length} quy tắc</span>}</span><span {...stylex.props(s.muted)}>{c.worldIntro}</span></span>{rules.length > 0 && <AskButton onClick={() => onSteer(c.editWorld)}>{c.askWorldEdit}</AskButton>}</div>
    {rules.length > 0 ? <>
      <section className={`card ${stylex.props(s.worldCard).className}`} aria-label={c.world}>{groups.map((group) => <div key={group.category} role="group" aria-label={group.label}>
        <div {...stylex.props(s.group)}><Icon name={group.icon} size={14} style={{ color: group.category === 'magic' ? 'var(--color-accent)' : 'var(--kit-info-fg)' }} /><b {...stylex.props(s.groupTitle)}>{group.label}</b><span className="num">{group.rules.length}</span></div>
        {group.rules.map((rule, index) => <div key={`${rule.rule}-${index}`} {...stylex.props(s.worldRow, index === 0 && s.first)}><span {...stylex.props(s.worldRule)}>{rule.rule}</span><span {...stylex.props(s.boundary)}><span {...stylex.props(s.small)}>{c.boundary}</span>{rule.boundary || '—'}</span></div>)}
      </div>)}</section>
      <p {...stylex.props(s.muted)}>{c.worldFooter} <a href={`#${paths.projectBrain(project)}`} {...stylex.props(s.link)}>{c.brain}</a></p>
    </> : <section className={`card ${stylex.props(s.empty).className}`}><Icon name="global" size={26} style={{ color: 'var(--color-fg-4)' }} /><b {...stylex.props(s.emptyTitle)}>{c.worldEmpty}</b><span {...stylex.props(s.emptyCopy)}>{c.worldEmptyHint}{completed > 0 ? ` Kiến trúc sư có thể rút ra từ ${completed} chương đã viết.` : ''}</span><span {...stylex.props(s.emptyAsk)}><AskButton onClick={() => onSteer(c.worldRequest(completed))}>{c.askWorldCreate}</AskButton></span></section>}
  </div>
}
