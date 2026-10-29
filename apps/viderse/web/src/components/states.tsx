// Trạng thái hệ thống (System artboard): loading · not-found · empty-workbench.
// Offline là dải ở shell (shell/OfflineStrip.tsx), còn đây là các khối giữa màn.
// Kiểu: styles/components.css .vv-loading*/.vv-emptystage*/.vv-page-empty.
import type { ReactNode } from 'react'
import { EmptyState, Icon, Spinner } from '@noverse/ui'
import { t } from '../i18n/vi'
import { navigate, paths } from '../router'
import { ActionButton } from './ActionButton'
import { Orb } from './Orb'
import { ASSISTANTS } from './agents'

/** Khung chờ ở giữa màn: 3 khối skeleton + spinner + nhãn đang tải. */
export function SystemLoading({ label = t.system.loadingWorkbench }: { label?: string }) {
  return (
    <div role="status" aria-label={label} className="vv-loading">
      <div aria-hidden="true" className="vv-loading__grid">
        {[0, 1, 2].map((i) => (
          <div key={i} className="vv-loading__card">
            <div className="vv-loading__thumb" />
            <div className="vv-loading__body">
              <div className="vv-skel" style={{ height: 12, width: i === 0 ? '72%' : '80%' }} />
              <div className="vv-skel--dim" style={{ height: 10, width: '44%' }} />
            </div>
          </div>
        ))}
      </div>
      <div className="vv-loading__foot">
        <Spinner size={13} />
        {label}
      </div>
    </div>
  )
}

/** 404 trong app — link cũ / video bị xoá: kính lúp + lời giải thích + CTA về Bàn dựng. */
export function SystemNotFound({ onRetry }: { onRetry?: () => void }) {
  return (
    <div className="vv-page-empty" style={{ minHeight: '60vh' }}>
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
    <div className="vv-emptystage">
      <div aria-hidden="true" className="vv-emptystage__orbs">
        {ASSISTANTS.map((assistant) => (
          <Orb key={assistant} of={assistant} size={52} standalone />
        ))}
      </div>
      <h3 className="vv-emptystage__title">{t.system.emptyWorkbenchTitle}</h3>
      <p className="vv-emptystage__body">{t.system.emptyWorkbenchBody}</p>
      {action ?? (
        <div className="vv-emptystage__row">
          <ActionButton variant="primary" icon={<Icon name="video-add" />} onClick={() => navigate(paths.new())}>
            {t.system.emptyWorkbenchAction}
          </ActionButton>
          <ActionButton variant="quiet" onClick={() => navigate(paths.explore())}>{t.system.emptyWorkbenchAlt}</ActionButton>
        </div>
      )}
    </div>
  )
}
