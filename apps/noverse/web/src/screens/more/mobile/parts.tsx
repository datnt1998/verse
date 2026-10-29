import type { Resource } from '../../../data/useResource'

export function ResourceNotice({ resource, label }: { resource: Resource<unknown>; label: string }) {
  if (resource.error) return <div className="mm-error" role="alert">
    <span>{resource.data !== undefined ? 'Không tải lại được' : `Không tải được ${label}`} · {resource.error instanceof Error ? resource.error.message : 'Vui lòng thử lại.'}</span>
    <button type="button" onClick={() => { void resource.refresh() }}>Thử lại</button>
  </div>
  if (resource.data === undefined) return <div className="mm-skeleton" role="status" aria-label={`Đang tải ${label}…`}><span /><span /><span /></div>
  if (resource.refreshing) return <div className="mm-refresh" role="status" aria-label={`Đang cập nhật ${label}`} />
  return null
}
