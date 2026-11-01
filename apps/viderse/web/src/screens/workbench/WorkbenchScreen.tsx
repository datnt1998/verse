// Cổng vào Bàn dựng: chỉ chọn bố cục theo bề rộng. Màn máy tính và màn điện
// thoại là hai chunk riêng, nên điện thoại không tải WorkbenchPanels /
// AssistantComposer (C8) và máy tính không tải màn điện thoại.
import { Suspense, lazy } from 'react'
import { useMediaQuery } from '../../lib/useMediaQuery'
import type { Route } from '../../router'

const WorkbenchDesktop = lazy(() => import('./WorkbenchDesktop').then((m) => ({ default: m.WorkbenchDesktop })))
const MobileWorkbench = lazy(() => import('./mobile/MobileWorkbench').then((m) => ({ default: m.MobileWorkbench })))

export function WorkbenchScreen({ route }: { route: Extract<Route, { name: 'workbench' }> }) {
  const mobile = useMediaQuery('(max-width: 767px)')
  return <Suspense fallback={<div aria-busy="true" />}>{mobile ? <MobileWorkbench route={route} /> : <WorkbenchDesktop route={route} />}</Suspense>
}
