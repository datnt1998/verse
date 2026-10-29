import { useEffect, useState } from 'react'
import { Button } from '@noverse/ui'
import { ConflictBar } from '@noverse/ui'
import { Skeleton } from '@noverse/ui'
import type { AudioData } from './useAudio'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'

/** Phản hồi chung cho các panel audio trên cùng một màn. */
export function AudioFeedback({ data, project, story }: { data: AudioData; project: string; story: string }) {
  return <>
    <PanelRefreshBar resourceKeys={[[ 'story', project, story, 'audio' ], [ 'project', project, 'voices' ]]} updatedAt={data.updatedAt} />
    {data.error && data.volume && <div role="alert" style={{ padding: '7px 16px', fontSize: 12, color: 'var(--color-warn)', display: 'flex', alignItems: 'center', gap: 8 }}><span>Không tải lại được · {data.error}</span><Button size="sm" onClick={data.refresh}>Thử lại</Button></div>}
    {data.conflict && <div style={{ padding: '8px 16px' }}><ConflictBar onView={data.viewCurrent} onOverwrite={data.overwrite} pending={data.savePending} /></div>}
    {(data.savePending || data.saveSuccess) && <div role="status" aria-live="polite" style={{ padding: '4px 16px', fontSize: 11.5, color: data.saveSuccess ? 'var(--color-ok)' : 'var(--color-fg-4)' }}>{data.saveSuccess ? '✓ Đã lưu' : 'Đang lưu…'}</div>}
  </>
}

/** Khung xương giữ nguyên ba cột của phòng đọc và phòng thu khi tải lần đầu. */
export function AudioSkeleton({ sidebar = true }: { sidebar?: boolean }) {
  const [slow, setSlow] = useState(false)
  useEffect(() => { const timer = window.setTimeout(() => setSlow(true), 8000); return () => window.clearTimeout(timer) }, [])
  return <div role="status" aria-label="Đang tải audio" style={{ display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0 }}>
    <div style={{ padding: 16, display: 'flex', gap: 12, borderBottom: '1px solid var(--line-1)' }}><Skeleton width={150} height={28} /><Skeleton width={410} height={28} /></div>
    <div style={{ display: 'flex', gap: 20, padding: 16, flex: 1 }}>
      {sidebar && <div style={{ width: 220, display: 'flex', flexDirection: 'column', gap: 18 }}>{Array.from({ length: 8 }, (_, index) => <Skeleton key={index} height={42} />)}</div>}
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 18 }}><Skeleton height={70} />{Array.from({ length: 6 }, (_, index) => <Skeleton key={index} height={64} />)}</div>
      <div style={{ width: 312, display: 'flex', flexDirection: 'column', gap: 16 }}><Skeleton height={210} /><Skeleton height={140} /></div>
    </div>
    {slow && <span style={{ padding: '8px 16px', fontSize: 12, color: 'var(--color-fg-4)' }}>Đang chậm hơn bình thường…</span>}
  </div>
}
