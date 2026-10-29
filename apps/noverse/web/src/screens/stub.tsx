// Helper chung cho các screen stub của đợt 1 (F0). Wave-2 thay thế từng màn
// bằng bản đầy đủ theo artboard; giữ nguyên tên export + props (xem README.md).
import type { ReactNode } from 'react'
import { EmptyState } from '@noverse/ui'

/** Vùng nội dung trung tâm trong AppShell body */
export function StubPane({ children }: { children: ReactNode }) {
  return (
    <main className="scroll" style={{ flex: 1, minWidth: 0, minHeight: 0, padding: 20, display: 'flex', flexDirection: 'column', gap: 12 }}>
      {children}
    </main>
  )
}

export function StubEmpty({ icon, title, hint, action }: { icon?: string; title: string; hint?: string; action?: ReactNode }) {
  return (
    <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      <EmptyState icon={icon} title={title} hint={hint} action={action} />
    </div>
  )
}
