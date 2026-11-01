// Trạng thái hệ thống (System artboard): loading · not-found · empty-workbench.
// Offline là dải ở shell (shell/OfflineStrip.tsx), còn đây là các khối giữa màn.
// Kiểu: stylex.create bên dưới (trước ở styles/components.css
// .vv-loading*/.vv-emptystage*/.vv-page-empty).
import type { ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { EmptyState, Icon, Spinner } from '@noverse/ui'
import { t } from '../i18n/vi'
import { navigate, paths } from '../router'
import { ActionButton } from './ActionButton'
import { Orb } from './Orb'
import { ASSISTANTS } from './agents'

const sx = stylex.create({
  pageEmpty: { display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '50vh', padding: 'var(--vv-s-26)' },
  loading: { display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-16)', padding: 'var(--vv-s-24) var(--vv-s-26)' },
  loadingGrid: { display: 'flex', gap: 'var(--vv-s-18)' },
  loadingCard: {
    flex: '1 1 0',
    minWidth: 0,
    borderRadius: 'var(--vv-r-14)',
    borderWidth: 1,
    borderStyle: 'solid',
    borderColor: 'var(--line-1)',
    backgroundColor: 'var(--color-surface-1)',
    overflow: 'hidden',
  },
  loadingThumb: { aspectRatio: '16 / 9', backgroundColor: 'var(--color-surface-2)' },
  loadingBody: { padding: 'var(--vv-s-12)', display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-7)' },
  skel: { borderRadius: 'var(--radius-chip)', backgroundColor: 'var(--color-surface-2)' },
  skelDim: { backgroundColor: 'var(--color-surface-1)', borderRadius: 'var(--vv-r-5)' },
  loadingFoot: { display: 'flex', alignItems: 'center', gap: 'var(--vv-s-8)', justifyContent: 'center', color: 'var(--color-fg-4)', fontSize: 13 },
  emptyStage: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    minHeight: '60vh',
    padding: 'var(--vv-s-44) var(--vv-s-26)',
    gap: 'var(--vv-s-14)',
    textAlign: 'center',
  },
  emptyOrbs: {
    position: 'relative',
    width: 200,
    height: 128,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    gap: 'var(--vv-s-12)',
    borderRadius: 'var(--vv-r-22)',
    backgroundImage: 'var(--vv-orbs-bg)',
  },
  emptyTitle: { margin: 0, fontSize: 17, fontWeight: 600 },
  emptyBody: { margin: 0, fontSize: 13, color: 'var(--color-fg-3)', maxWidth: 420 },
  emptyRow: { display: 'flex', gap: 'var(--vv-s-8)', marginTop: 'var(--vv-s-2)' },
})

/** Khung chờ ở giữa màn: 3 khối skeleton + spinner + nhãn đang tải. */
export function SystemLoading({ label = t.system.loadingWorkbench }: { label?: string }) {
  return (
    <div role="status" aria-label={label} {...stylex.props(sx.loading)}>
      <div aria-hidden="true" {...stylex.props(sx.loadingGrid)}>
        {[0, 1, 2].map((i) => (
          <div key={i} {...stylex.props(sx.loadingCard)}>
            <div {...stylex.props(sx.loadingThumb)} />
            <div {...stylex.props(sx.loadingBody)}>
              <div {...stylex.props(sx.skel)} style={{ height: 12, width: i === 0 ? '72%' : '80%' }} />
              <div {...stylex.props(sx.skelDim)} style={{ height: 10, width: '44%' }} />
            </div>
          </div>
        ))}
      </div>
      <div {...stylex.props(sx.loadingFoot)}>
        <Spinner size={13} />
        {label}
      </div>
    </div>
  )
}

/** 404 trong app — link cũ / video bị xoá: kính lúp + lời giải thích + CTA về Bàn dựng. */
export function SystemNotFound({ onRetry }: { onRetry?: () => void }) {
  return (
    <div {...stylex.props(sx.pageEmpty)} style={{ minHeight: '60vh' }}>
      <EmptyState
        variant="frame"
        icon="search-normal-1"
        title={t.system.notFoundTitle}
        hint={t.system.notFoundBody}
        action={
          <>
            <ActionButton variant="primary" onClick={() => navigate(paths.workbench())}>{t.system.notFoundAction}</ActionButton>
            {onRetry && <ActionButton variant="outline" onClick={onRetry}>{t.common.retry}</ActionButton>}
          </>
        }
      />
    </div>
  )
}

/** Trạng thái trống của Bàn dựng: cụm 4 orb trợ lý trên “sân khấu” + lời mời + CTA. */
export function EmptyWorkbench({ action }: { action?: ReactNode }) {
  return (
    <div {...stylex.props(sx.emptyStage)}>
      <div aria-hidden="true" {...stylex.props(sx.emptyOrbs)}>
        {ASSISTANTS.map((assistant) => (
          <Orb key={assistant} of={assistant} size={52} standalone />
        ))}
      </div>
      <h3 {...stylex.props(sx.emptyTitle)}>{t.system.emptyWorkbenchTitle}</h3>
      <p {...stylex.props(sx.emptyBody)}>{t.system.emptyWorkbenchBody}</p>
      {action ?? (
        <div {...stylex.props(sx.emptyRow)}>
          <ActionButton variant="primary" icon={<Icon name="video-add" />} onClick={() => navigate(paths.new())}>
            {t.system.emptyWorkbenchAction}
          </ActionButton>
          <ActionButton variant="quiet" onClick={() => navigate(paths.explore())}>{t.system.emptyWorkbenchAlt}</ActionButton>
        </div>
      )}
    </div>
  )
}
