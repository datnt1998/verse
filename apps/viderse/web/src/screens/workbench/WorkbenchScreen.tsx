// Cổng vào Bàn dựng: chỉ chọn bố cục theo bề rộng. Màn máy tính và màn điện
// thoại là hai chunk riêng, nên điện thoại không tải WorkbenchPanels và máy
// tính không tải màn điện thoại.
import { Suspense, lazy, useEffect } from 'react'
import { useMediaQuery } from '../../lib/useMediaQuery'
import type { Route } from '../../router'
import { navigate, paths } from '../../router'
import { getQueue, getQueueItem } from '../../api/clone'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import { Button } from '@noverse/ui'

const WorkbenchDesktop = lazy(() => import('./WorkbenchDesktop').then((m) => ({ default: m.WorkbenchDesktop })))
const MobileWorkbench = lazy(() => import('./mobile/MobileWorkbench').then((m) => ({ default: m.MobileWorkbench })))


export function WorkbenchScreen({ route }: { route: Extract<Route, { name: 'workbench' }> }) {
  const mobile = useMediaQuery('(max-width: 767px)')
  if (route.queueItem && !route.video) return <PendingWorkbench id={route.queueItem} />
  return <Suspense fallback={<div aria-busy="true" />}>{mobile ? <MobileWorkbench route={route} /> : <WorkbenchDesktop route={route} />}</Suspense>
}

function PendingWorkbench({ id }: { id: string }) {
  const target = useResource(['queue', 'item', id], ({ signal }) => getQueueItem(id, signal))
  const queue = useResource(keys.queue(), ({ signal }) => getQueue(signal))
  const refresh = target.refresh
  useEffect(() => {
    const timer = window.setInterval(() => { void refresh() }, 2500)
    return () => window.clearInterval(timer)
  }, [refresh])
  useEffect(() => {
    if (target.data?.jobId) navigate(paths.workbench({ channel: target.data.channelId, video: target.data.jobId }), true)
  }, [target.data])
  const item = queue.data?.items.find((row) => row.id === id)
  return <main aria-label="Video đang chờ" style={{ padding: 'var(--vv-s-24)', color: 'var(--color-fg)', display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-16)' }}>
    <a href={`#${paths.workbench()}`} style={{ minHeight: 44, display: 'inline-flex', alignItems: 'center' }}>← Bàn dựng</a>
    <h1>{item?.title || 'Video đang chờ'}</h1>
    {item?.thumbnailUrl && <img src={item.thumbnailUrl} alt="" style={{ maxWidth: 320, width: '100%', borderRadius: 'var(--radius-card)' }} />}
    <p role="status">{target.error ? 'Không đọc được video đang chờ.' : target.initialLoading ? 'Đang tải video…' : target.data?.jobId ? 'Đang mở video ở Bàn dựng…' : `Đã chọn đúng video trong hàng chờ · ${item?.templateName || 'Đang chờ đến lượt'}`}</p>
    {item && <p>{item.source} · {item.waitReason === 'queue_paused' ? 'Hàng chờ đang tạm dừng' : 'Máy sẽ mở video tại đây khi đến lượt.'}</p>}
    {!!target.error && <Button size="lg" onClick={() => void refresh()}>Thử lại</Button>}
  </main>
}
