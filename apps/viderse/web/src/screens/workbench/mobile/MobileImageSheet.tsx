// Sheet Ảnh của video (artboard MMain › detail): xem ảnh lớn, dải ảnh, chọn
// phiên bản đã vẽ và đọc prompt hiệu lực. Dữ liệu từ GET /api/clone/:id/images.
import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Sheet } from '@noverse/ui'
import type { JobImage, JobImageList } from '../../../api/workbench'
import { screenWorkbench } from '../../../i18n/screens'
import { imageLabel, imagePrompt, imageUrl, imageVersion } from './mworkbench-logic'
import { mw } from './mworkbench.stylex'

const c = screenWorkbench.mobile
const sheet = c.imageSheet

export function MobileImageSheet({ open, jobId, initialId, images, loading, error, onClose, onChangePrompt, onRetry }: {
  open: boolean
  jobId: string
  initialId?: string
  images: JobImageList | undefined
  loading: boolean
  error: unknown
  onClose: () => void
  onChangePrompt: () => void
  onRetry: () => void
}) {
  return open ? <OpenImageSheet jobId={jobId} initialId={initialId} images={images} loading={loading} error={error} onClose={onClose} onChangePrompt={onChangePrompt} onRetry={onRetry} /> : null
}

function OpenImageSheet({ jobId, initialId, images, loading, error, onClose, onChangePrompt, onRetry }: {
  jobId: string
  initialId?: string
  images: JobImageList | undefined
  loading: boolean
  error: unknown
  onClose: () => void
  onChangePrompt: () => void
  onRetry: () => void
}) {
  const items = images?.items ?? []
  const [currentId, setCurrentId] = useState<string | undefined>(initialId ?? items[0]?.id)
  const [versions, setVersions] = useState<Record<string, number>>({})
  const current = items.find((item) => item.id === currentId) ?? items[0]
  const chosen = current ? versions[current.id] : undefined
  const src = current ? imageUrl(current, chosen) : undefined
  const manifest = images?.manifest
  const usedPrompt = current ? imagePrompt(current, chosen) : undefined

  return <Sheet open onClose={onClose} label={sheet.title} size="md" height="auto">
    <div {...stylex.props(mw.sheetBody)}>
      <h2 {...stylex.props(mw.cardTitle)}>{sheet.title}</h2>
      {loading && !images && <p role="status" {...stylex.props(mw.notice)}>Đang tải…</p>}
      {error != null && !images && <div role="alert" {...stylex.props(mw.error)}>{screenWorkbench.mobile.loadError} <Button size="sm" variant="quiet" onClick={onRetry}>{screenWorkbench.mobile.retry}</Button></div>}
      {images && items.length === 0 && <p {...stylex.props(mw.notice)}>{sheet.empty}</p>}
      {current && <>
        {src ? <img {...stylex.props(mw.viewer)} src={src} alt={imageLabel(current, items.indexOf(current))} /> : <div {...stylex.props(mw.viewer)} role="img" aria-label={imageLabel(current, items.indexOf(current))} />}
        <div {...stylex.props(mw.filmstrip)} role="listbox" aria-label={sheet.title} onKeyDown={(event) => {
          const index = items.findIndex((item) => item.id === current.id)
          const next = event.key === 'ArrowRight' ? (index + 1) % items.length
            : event.key === 'ArrowLeft' ? (index + items.length - 1) % items.length
            : event.key === 'Home' ? 0 : event.key === 'End' ? items.length - 1 : -1
          if (next < 0) return
          event.preventDefault()
          setCurrentId(items[next].id)
          event.currentTarget.querySelectorAll<HTMLElement>('[role="option"]')[next]?.focus()
        }}>
          {items.map((item, index) => <ImageThumb key={item.id} item={item} index={index} selected={item.id === current.id} onPick={() => setCurrentId(item.id)} />)}
        </div>
        <div {...stylex.props(mw.versionRow)} role="group" aria-label={sheet.versions}>
          {(current.versions ?? []).map((version) => <Button key={version.version} size="sm" variant={imageVersion(current, chosen) === version.version ? 'primary' : 'ghost'} aria-pressed={imageVersion(current, chosen) === version.version} onClick={() => setVersions((all) => ({ ...all, [current.id]: version.version }))}>v{version.version}</Button>)}
        </div>
        <div {...stylex.props(mw.field)} aria-live="polite">
          <strong {...stylex.props(mw.cardHint)}>{c.imagePromptUsed} · v{imageVersion(current, chosen)}</strong>
          <p {...stylex.props(mw.promptText)}>{usedPrompt ?? c.imagePromptUnknown}</p>
        </div>
        <div {...stylex.props(mw.promptLine)}>
          {manifest && <>{c.promptScene(manifest.scenePrompt.name)}{manifest.scenePrompt.overridden && <span {...stylex.props(mw.promptTag)}> · {c.promptOverride}</span>}<br />{c.promptCover(manifest.coverPrompt.name)}{manifest.coverPrompt.overridden && <span {...stylex.props(mw.promptTag)}> · {c.promptOverride}</span>}</>}
          {!manifest && 'Chưa có prompt hiệu lực cho video này.'}
        </div>
      </>}
      <div {...stylex.props(mw.planFooter)}>
        <Button variant="ghost" size="lg" onClick={onChangePrompt}>{sheet.changePrompt}</Button>
        <a {...stylex.props(mw.moreLink)} href={`#/review?video=${encodeURIComponent(jobId)}`}>{sheet.review} ›</a>
      </div>
      <Button variant="quiet" size="sm" onClick={onClose}>{sheet.close}</Button>
    </div>
  </Sheet>
}

function ImageThumb({ item, index, selected, onPick }: { item: JobImage; index: number; selected: boolean; onPick: () => void }) {
  const src = imageUrl(item)
  return <button type="button" role="option" aria-selected={selected} tabIndex={selected ? 0 : -1} aria-label={imageLabel(item, index)} {...stylex.props(mw.filmThumb, selected && mw.filmThumbOn)} onClick={onPick}>
    {src ? <img {...stylex.props(mw.filmImage)} src={src} alt="" /> : null}
  </button>
}
