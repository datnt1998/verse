// Cột phải màn Duyệt: Phiên bản → Ảnh tham chiếu → Prompt đã gửi → meta →
// Lời kể (artboard Review › aside.ipanel). Prompt hiện đúng văn bản đã gửi,
// giữ nguyên ngôn ngữ gốc; mọi nhãn quanh nó là tiếng Việt.
import { useEffect, useRef, useState, type ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Dropzone, Icon, Segmented, usePopup } from '@noverse/ui'
import type { AiProvider } from '../../api/ai'
import type { ImageQuota, ImageRef, PromptPartKind } from '../../api/images'
import { DevelopEffect } from '../../components'
import {
  EDITABLE_PARTS, PART_LABELS, REF_ROLES, defaultVersion, diffSummary, providerName, rawParams, rawSentText, resetAtLabel, versionMeta, versionNote,
} from './imageView'
import type { ImageEditing } from './useImageEditing'
import type { ReviewItemView } from './imageView'
import { sx } from './review.stylex'

export interface RefCandidate { id: string; url: string; tag: string; label: string; kind: 'image' | 'frame' }

export interface ImageRailProps {
  item: ReviewItemView
  quota?: ImageQuota
  /** Thẻ Kết nối AI — tên hiển thị của nguồn vẽ thay cho id nhà cung cấp. */
  providers?: readonly AiProvider[]
  mode: 'view' | 'raw' | 'edit'
  onMode: (mode: 'view' | 'raw' | 'edit') => void
  selected: number
  onSelectVersion: (n: number) => void
  onUseVersion: (n: number) => void
  edit: ImageEditing
  online: boolean
  mutationBlocked: boolean
  candidates: { images: RefCandidate[]; frames: RefCandidate[] }
  onUpload: (file: File) => void
  uploading?: boolean
  /** Khối Điểm của Kiểm định (đứng trước Phiên bản như artboard). */
  score?: ReactNode
}

export function ImageRail({
  item, quota, providers, mode, onMode, selected, onSelectVersion, onUseVersion, edit, online, mutationBlocked, candidates, onUpload, uploading, score,
}: ImageRailProps) {
  const refPopup = usePopup()
  const { open: refsOpen, setOpen: setRefsOpen } = refPopup
  const [tab, setTab] = useState<'images' | 'frames' | 'upload'>('images')
  const [role, setRole] = useState<ImageRef['role']>('character')
  const [picked, setPicked] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const [diffHidden, setDiffHidden] = useState(false)
  const copyTimer = useRef<number | undefined>(undefined)
  const uploadInput = useRef<HTMLInputElement>(null)
  const refsTrigger = useRef<HTMLButtonElement>(null)
  useEffect(() => () => window.clearTimeout(copyTimer.current), [])
  useEffect(() => { setRefsOpen(false); setPicked(null); setCopied(false); setDiffHidden(false) }, [item.id])
  useEffect(() => {
    if (!refsOpen) return
    const panel = refPopup.root.current?.querySelector<HTMLElement>('[role="dialog"]')
    panel?.querySelector<HTMLButtonElement>('button:not(:disabled)')?.focus()
    return () => {
      const active = document.activeElement
      if (refsTrigger.current?.isConnected && (active === document.body || panel?.contains(active))) {
        refsTrigger.current.focus()
      }
    }
  }, [refsOpen, refPopup.root])

  const liveVersion = item.versions.find((candidate) => candidate.n === selected && candidate.state !== 'cancelled') ?? defaultVersion(item.versions)
  const version = mode === 'edit' || (mode === 'raw' && edit.changedCount > 0) ? edit.base : liveVersion
  const canMutate = online && !mutationBlocked
  useEffect(() => { if (!canMutate) setRefsOpen(false) }, [canMutate, setRefsOpen])
  const drawingVersion = item.versions.find((candidate) => candidate.state === 'drawing')
  const failedVersion = item.versions.find((candidate) => candidate.state === 'failed')
  const refs = mode === 'edit' ? edit.refs : version?.refs ?? []
  const diff = diffSummary(version)
  const editable = EDITABLE_PARTS.map((kind) => ({
    kind,
    part: version?.parts.find((candidate) => candidate.kind === kind),
    changed: edit.changed[kind] !== undefined,
    text: edit.edits[kind] ?? version?.parts.find((candidate) => candidate.kind === kind)?.text ?? '',
  })).filter((entry) => entry.part || entry.kind === 'scene')
  const legacyParts = version?.partsLegacy || !version?.parts.length
  const list = tab === 'frames' ? candidates.frames : candidates.images
  const addLabel = picked ? 'Thêm 1 ảnh' : 'Chọn ảnh để thêm'

  const copy = async () => {
    if (!version) return
    try {
      await navigator.clipboard.writeText(version.sentText)
      setCopied(true)
      copyTimer.current = window.setTimeout(() => setCopied(false), 1600)
    } catch { setCopied(false) }
  }

  const addPicked = () => {
    if (!picked || !canMutate || refs.length >= item.refLimit) return
    const candidate = list.find((entry) => entry.id === picked)
    if (!candidate) return
    edit.addRef(candidate.kind === 'frame'
      ? frameRef(candidate, role)
      : {
          id: `add-${candidate.id}`,
          role,
          url: candidate.url,
          source: { type: 'video_image', label: candidate.label, imageId: candidate.id },
        })
    setRefsOpen(false)
    setPicked(null)
  }

  return (
    <aside aria-label="Ảnh và chỉ dẫn" {...stylex.props(sx.rail, mode === 'edit' ? sx.railEdit : sx.railWide)}>
      <div {...stylex.props(sx.railHead)}>
        <span {...stylex.props(sx.railTitle)}>{item.label}</span>
        <span {...stylex.props(sx.railSub)}>
          {[item.direction, sceneTime(item)].filter(Boolean).join(' · ')}
        </span>
      </div>
      {score}

      <section {...stylex.props(sx.section)}>
        <div {...stylex.props(sx.sectionHead)}>
          <span {...stylex.props(sx.sectionTitle)}>Phiên bản · {item.versions.length}</span>
        </div>
        <div role="group" aria-label="Phiên bản của ảnh" {...stylex.props(sx.versions)}>
          {item.versions.map((entry) => {
            const previous = item.versions.find((candidate) => candidate.n === entry.n - 1)
            const url = entry.url ?? entry.previewUrl ?? (entry.state === 'drawing' ? previous?.url : null) ?? item.legacyUrl
            const on = entry.n === selected
            const cancelled = entry.state === 'cancelled'
            return (
              <button
                key={entry.n}
                type="button"
                aria-pressed={on}
                onClick={() => { if (!cancelled) onSelectVersion(entry.n) }}
                disabled={cancelled}
                aria-label={cancelled
                  ? versionNote(entry)
                  : `Bản ${entry.n} · ${entry.feedback ? `theo góp ý “${entry.feedback}”` : 'lần đầu'}${entry.n === item.usedVersion ? ' · đang dùng' : ''}`}
                title={cancelled
                  ? versionNote(entry)
                  : `${versionNote(entry)}${entry.n === item.usedVersion ? ' · đang dùng' : ''}`}
                {...stylex.props(sx.ver, on && sx.verOn, cancelled && sx.dimmed)}
              >
                {url && <img alt="" src={url} {...stylex.props(sx.verImage)} />}
                {entry.state === 'drawing' && <DevelopEffect state="drawing" baseUrl={url ?? undefined} compact hideCard />}
                <span {...stylex.props(sx.verN)}>{entry.n}</span>
                {entry.n === item.usedVersion && <span aria-hidden="true" {...stylex.props(sx.verUsed)} />}
              </button>
            )
          })}
        </div>
        <span {...stylex.props(sx.verNote)}>
          {drawingVersion
            ? `Đang vẽ bản ${drawingVersion.n}. Bản ${item.usedVersion} vẫn dùng cho video tới khi bản mới xong.`
            : failedVersion && !drawingVersion
              ? `Bản ${failedVersion.n} chưa vẽ được. Bản ${item.usedVersion} vẫn dùng cho video.`
              : versionNote(version)}
        </span>
        {version && version.n !== item.usedVersion && version.state === 'ready' && (
          <Button size="sm" xstyle={sx.useButton} type="button" disabled={!canMutate} onClick={() => onUseVersion(version.n)} >
            Dùng bản {version.n} cho video
          </Button>
        )}
      </section>

      <section ref={refPopup.root} {...stylex.props(sx.section)}>
        <div {...stylex.props(sx.sectionHead)}>
          <span {...stylex.props(sx.sectionTitle)}>Ảnh tham chiếu · {refs.length}/{item.refLimit}</span>
          <Button size="sm" type="button"
          ref={refsTrigger}
          onClick={() => { setRefsOpen((open) => !open); setPicked(null) }}
          aria-label="Thêm"
          title="Thêm"
          aria-haspopup="dialog"
          aria-expanded={refsOpen}
          disabled={!canMutate || mode === 'edit' && refs.length >= item.refLimit} ><Icon name="add" size={14} />Thêm
                    </Button>
        </div>
        {refs.map((ref, index) => (
          <div key={ref.id} {...stylex.props(sx.refCard)}>
            <img alt="" src={ref.url} {...stylex.props(sx.refThumb)} />
            <span {...stylex.props(sx.refText)}>
              <span {...stylex.props(sx.refLine)}>
                <span {...stylex.props(sx.refN)}>{index + 1}</span>
                <b {...stylex.props(sx.refRole)}>{roleLabel(ref.role)}</b>
              </span>
              <span {...stylex.props(sx.refSource)} title={ref.source.label}>{ref.source.label}</span>
            </span>
            {mode === 'edit' && (
              <Button variant="icon" type="button" disabled={!canMutate} onClick={() => edit.toggleRef(ref.id)} aria-label={`Bỏ ảnh tham chiếu ${index + 1}`} ><Icon name="close" size={14} /></Button>
            )}
          </div>
        ))}
        {!refs.length && (
          <span {...stylex.props(sx.verNote)}>Không dùng ảnh tham chiếu. Thêm ảnh để giữ nhân vật hoặc bố cục giống nhau giữa các cảnh.</span>
        )}
        {refsOpen && (
          <div role="dialog" aria-label="Thêm ảnh tham chiếu" {...stylex.props(sx.pop)}>
            <div {...stylex.props(sx.popHead)}>
              <b {...stylex.props(sx.popTitle)}>Thêm ảnh tham chiếu</b>
              <Button variant="icon" type="button" onClick={() => setRefsOpen(false)} aria-label="Đóng" ><Icon name="close" size={14} /></Button>
            </div>
            <Segmented label="Nguồn ảnh tham chiếu" value={tab} onChange={next => { setTab(next as typeof tab); setPicked(null) }} fit="content" options={[{ id: 'images', label: 'Ảnh trong video' }, { id: 'frames', label: 'Khung video gốc' }, { id: 'upload', label: 'Tải lên' }]} />
            {tab === 'upload' ? (
              <>
                <Dropzone label="Tải ảnh tham chiếu" title={uploading ? 'Đang tải ảnh lên…' : 'Kéo ảnh vào đây, hoặc chọn tệp'} hint="PNG, JPG, WebP · tối đa 20 MB" multiple={false} busy={uploading} disabled={!canMutate || uploading} onPick={() => uploadInput.current?.click()} onDrop={files => { if (canMutate && files[0]) onUpload(files[0]) }} />
                <input ref={uploadInput} type="file" accept="image/png,image/jpeg,image/webp" hidden disabled={!canMutate || uploading} onChange={event => { const file = event.target.files?.[0]; if (canMutate && file) onUpload(file); event.target.value = '' }} />
              </>
            ) : (
              <div {...stylex.props(sx.candGrid)}>
                {list.map((candidate) => (
                  <button
                    key={candidate.id}
                    type="button"
                    aria-pressed={picked === candidate.id}
                    onClick={() => setPicked(picked === candidate.id ? null : candidate.id)}
                    aria-label={candidate.label}
                    title={candidate.label}
                    {...stylex.props(sx.cand, picked === candidate.id && sx.candOn)}
                  >
                    <img alt="" src={candidate.url} {...stylex.props(sx.candImage)} />
                    <span {...stylex.props(sx.candTag)}>{candidate.tag}</span>
                  </button>
                ))}
                {!list.length && <span {...stylex.props(sx.verNote)}>{tab === 'frames' ? 'Chưa có khung đối thủ nào đã gửi cho video này.' : 'Video này chưa có ảnh khác.'}</span>}
              </div>
            )}
            <div {...stylex.props(sx.roleRow)}>
              <span {...stylex.props(sx.roleLabel)}>Dùng làm</span>
              <Segmented label="Vai trò của ảnh" value={role} onChange={next => setRole(next as ImageRef['role'])} size="sm" fit="content" options={REF_ROLES.map(option => ({ id: option.role, label: option.label }))} />
            </div>
            <div {...stylex.props(sx.popFoot)}>
              <span {...stylex.props(sx.popFootText)}>{providerName(version?.provider ?? 'chatgpt', providers)} nhận tối đa {item.refLimit} ảnh tham chiếu</span>
              <Button size="sm" type="button" onClick={() => setRefsOpen(false)} >Huỷ</Button>
              <Button size="sm" variant="primary" type="button" onClick={addPicked} aria-disabled={!canMutate || !picked || refs.length >= item.refLimit} >{addLabel}</Button>
            </div>
          </div>
        )}
      </section>

      <section {...stylex.props(sx.section)}>
        <div {...stylex.props(sx.sectionHead)}>
          <span {...stylex.props(sx.sectionTitle)}>Prompt đã gửi</span>
          {mode !== 'edit' && (
            <>
              <Button size="sm" type="button" onClick={() => void copy()} aria-label={copied ? 'Đã chép' : 'Chép'} title={copied ? 'Đã chép' : 'Chép'} ><Icon name="copy" size={14} />{copied ? 'Đã chép' : 'Chép'}</Button>
              <Button size="sm" type="button" onClick={() => onMode('edit')} aria-label="Sửa" title="Sửa" aria-keyshortcuts="E" disabled={!canMutate || legacyParts} ><Icon name="edit" size={14} />Sửa
                            </Button>
            </>
          )}
        </div>
        {diff && mode === 'view' && (
          <div {...stylex.props(sx.diffRow)}>
            <span {...stylex.props(sx.diffNote)}>{diff}</span>
            <Button variant="quiet" size="sm" type="button" aria-pressed={!diffHidden} onClick={() => setDiffHidden((hidden) => !hidden)} >{diffHidden ? 'Xem thay đổi' : 'Ẩn thay đổi'}</Button>
          </div>
        )}
        {mode === 'raw' ? (
          <>
            {/* Nguyên văn đúng từng byte nhà cung cấp nhận: gồm cả hàng rào an toàn và
                chỉ dẫn công cụ — phần không sửa được, không có trong chế độ sửa. */}
            <pre {...stylex.props(sx.raw)}>{rawSentText(version)}</pre>
            {rawParams(version) && <span {...stylex.props(sx.verNote)}>{rawParams(version)}</span>}
          </>
        ) : mode === 'edit' && !legacyParts ? (
          <div>
            {editable.map((entry, index) => (
              <div key={entry.kind} {...stylex.props(sx.seg, index === 0 && sx.segFirst, entry.changed && sx.segChanged)}>
                <span {...stylex.props(sx.segLabel)}>
                  {PART_LABELS[entry.kind]}
                  {entry.part && <span {...stylex.props(sx.segSource)}>· {entry.part.source.label}</span>}
                  <span {...stylex.props(sx.hintSpacer)} />
                  {entry.changed && <span {...stylex.props(sx.segChangedLabel)}>Đã sửa</span>}
                  {entry.changed && (
                    <Button variant="quiet" size="sm" type="button" onClick={() => edit.resetPart(entry.kind)} >Hoàn tác</Button>
                  )}
                </span>
                <label htmlFor={`prompt-${entry.kind}`} {...stylex.props(sx.hiddenLabel)}>{PART_LABELS[entry.kind]}</label>
                <textarea
                  id={`prompt-${entry.kind}`}
                  rows={entry.kind === 'scene' ? 6 : 3}
                  value={entry.text}
                  onChange={(event) => edit.editText(entry.kind as PromptPartKind, event.target.value)}
                  {...stylex.props(sx.segTextarea, entry.changed && sx.segChanged)}
                />
              </div>
            ))}
          </div>
        ) : (
          <div {...stylex.props(sx.promptCard)}>
            {version?.parts.length
              ? version.parts.filter(part => part.kind !== 'learned').map((part, index) => (
                  <div key={`${part.kind}-${index}`} {...stylex.props(sx.seg, index === 0 && sx.segFirst)}>
                    <span {...stylex.props(sx.segLabel)}>
                      {PART_LABELS[part.kind]}
                      <span {...stylex.props(sx.segSource)}>· {part.source.label}</span>
                    </span>
                    <span {...stylex.props(sx.segText)}>
                      {part.kind === 'scene' && version.diffFromPrev && !diffHidden
                        ? version.diffFromPrev.map(([text, kind], at) => kind === 'ins'
                          ? <ins key={at} {...stylex.props(sx.ins)}>{text}</ins>
                          : kind === 'del'
                            ? <del key={at} {...stylex.props(sx.del)}>{text}</del>
                            : <span key={at}>{text}</span>)
                        : part.text}
                    </span>
                  </div>
                ))
              : <pre {...stylex.props(sx.raw)}>{version?.sentText ?? ''}</pre>}
          </div>
        )}
        <div {...stylex.props(sx.metaRow)}>
          <span {...stylex.props(sx.metaText)}>{versionMeta(version, providers)}</span>
          <Button variant="quiet" size="sm" type="button" aria-pressed={mode === 'raw'} onClick={() => onMode(mode === 'raw' ? (edit.changedCount ? 'edit' : 'view') : 'raw')} >{mode === 'raw' ? 'Xem theo phần' : 'Xem nguyên văn gửi máy chủ'}</Button>
        </div>
        {quota && mode === 'edit' && (
          <span {...stylex.props(sx.verNote)}>
            {quota.providers.map((entry) => `${entry.label} còn ${entry.left}/${entry.total} hôm nay`).join(' · ')}
            {quota.providers.some((entry) => entry.resetAt) ? ` · ${resetAtLabel(quota.providers.find((entry) => entry.resetAt)?.resetAt ?? undefined)}` : ''}
          </span>
        )}
      </section>

      <details {...stylex.props(sx.narration)}>
        <summary {...stylex.props(sx.narrationTitle)}>
          <Icon name="doc" size={15} />Lời kể trong cảnh
        </summary>
        <p {...stylex.props(sx.narrationText)}>{item.narration || 'Ảnh bìa không gắn với một đoạn lời kể.'}</p>
      </details>
    </aside>
  )
}

const roleLabel = (role: ImageRef['role']) => REF_ROLES.find((option) => option.role === role)?.label ?? role

const sceneTime = (item: ReviewItemView) => {
  if (item.startSec == null || item.endSec == null) return ''
  const format = (seconds: number) => `${Math.floor(seconds / 60)}:${String(Math.floor(seconds % 60)).padStart(2, '0')}`
  return `${format(item.startSec)}–${format(item.endSec)}`
}

/** Khung đối thủ đã gửi: đủ `videoId`/`atSec` để gửi lại nguyên vẹn. */
function frameRef(candidate: RefCandidate, role: ImageRef['role']): ImageRef {
  const [videoId, atSec] = candidate.id.split('|')
  return {
    id: `add-frame-${candidate.id}`,
    role,
    url: candidate.url,
    source: { type: 'rival_frame', label: candidate.label, videoId, atSec: Number(atSec) },
  }
}
