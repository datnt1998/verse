import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { space as s, corners as r } from '../../styles/viderse.stylex'
import { listPrompts, type PromptKind } from '../../api/library'
import { applyPrompt, getJobImages, planPrompt, uploadCover, type CloneJobView, type PromptChoice, type PromptPlan } from '../../api/workbench'
import { useResource } from '../../data/useResource'

export function PromptMenu({ job, onClose, onRefresh }: { job: CloneJobView; onClose: () => void; onRefresh: () => void }) {
  const [tab, setTab] = useState<PromptKind>('scene')
  const [choice, setChoice] = useState<PromptChoice | null>(null)
  const [error, setError] = useState('')
  const [plan, setPlan] = useState<PromptPlan | null>(null)
  const [busy, setBusy] = useState(false)
  const prompts = useResource(['library', 'prompts', tab, ''], ({ signal }) => listPrompts({ kind: tab }, signal))
  const images = useResource(['workbench', 'images', job.id], ({ signal }) => getJobImages(job.id, signal), { enabled: !job.queueItem })
  const sceneCount = images.data?.items?.filter(item => item.kind === 'scene').length
  const pick = async (next: PromptChoice) => {
    setChoice(next); setPlan(null); setError(''); setBusy(true)
    try { setPlan(await planPrompt(job.id, next)) } catch (err) { setError(err instanceof Error ? err.message : 'Không tính được giá') } finally { setBusy(false) }
  }
  const apply = async () => {
    if (!choice || !plan) return
    setBusy(true); setError('')
    try {
      const revision = job.queueItem ? job.queueItem.overrideRevision : (await getJobImages(job.id)).manifest?.revision
      if (revision === undefined) throw new Error('Chưa có phiên bản ảnh để đổi prompt')
      await applyPrompt(job.id, choice, revision, plan.cost.images)
      onRefresh(); onClose()
    } catch (err) { setError(err instanceof Error ? err.message : 'Không đổi được prompt') } finally { setBusy(false) }
  }
  const cover = async (file: File) => {
    setBusy(true); setError('')
    try {
      const images = await getJobImages(job.id)
      if (!images.manifest) throw new Error('Chưa có bộ ảnh để đổi ảnh bìa')
      await uploadCover(job.id, images.manifest.revision, file)
      onRefresh(); onClose()
    } catch (err) { setError(err instanceof Error ? err.message : 'Không tải được ảnh bìa') } finally { setBusy(false) }
  }
  return <div role="dialog" aria-label="Ảnh của video" {...stylex.props(styles.menu)}>
    <h2 {...stylex.props(styles.heading)}>Ảnh của video</h2><p {...stylex.props(styles.hint)}>Mặc định dùng prompt của template</p>
    <div role="tablist" aria-label="Loại ảnh" {...stylex.props(styles.tabs)}>{(['scene', 'cover'] as const).map(kind => <button key={kind} type="button" role="tab" aria-selected={tab === kind} {...stylex.props(styles.tab, tab === kind && styles.active)} onClick={() => { setTab(kind); setChoice(null); setPlan(null) }}>{kind === 'scene' ? 'Ảnh cảnh' : 'Ảnh bìa'}</button>)}</div>
    <div role="radiogroup" aria-label={tab === 'scene' ? 'Ảnh cảnh' : 'Ảnh bìa'} {...stylex.props(styles.list)}>{prompts.initialLoading ? <p>Đang tải…</p> : prompts.error ? <p role="alert">Không tải được prompt. <button onClick={() => void prompts.refresh()}>Thử lại</button></p> : prompts.data?.items.map(prompt => <button key={prompt.id} type="button" role="radio" aria-checked={choice?.scenePromptId === prompt.id || choice?.coverPromptId === prompt.id} {...stylex.props(styles.row)} onClick={() => void pick(tab === 'scene' ? { scenePromptId: prompt.id, noImages: false } : { coverPromptId: prompt.id })}>{prompt.name}</button>)}{tab === 'scene' && <button type="button" role="radio" aria-checked={choice?.noImages === true} {...stylex.props(styles.row)} onClick={() => void pick({ noImages: true })}>Không tạo ảnh cho video này</button>}</div>
    {tab === 'cover' && <label {...stylex.props(styles.row)}>Dùng ảnh của bạn<input type="file" accept="image/*" disabled={busy} onChange={event => { const file = event.target.files?.[0]; if (file) void cover(file) }} /></label>}
    {plan && <p role="status" {...stylex.props(styles.cost)}>{choice?.noImages ? 'Đổi · không tạo ảnh' : tab === 'scene' ? `Vẽ lại ${plan.redraw.scenes} ảnh cảnh đã vẽ${sceneCount !== undefined ? `; ${Math.max(0, sceneCount - plan.redraw.scenes)} ảnh còn lại vẽ luôn theo prompt mới` : ''}. Tốn ${plan.cost.images} ảnh` : `Vẽ lại ảnh bìa. Tốn ${plan.cost.images} ảnh`}{plan.rebuild ? ' · Dựng lại video' : ''}</p>}{error && <p role="alert">{error}</p>}
    <div {...stylex.props(styles.actions)}><a href="#/library?tab=prompts">Kho prompt ảnh</a><button type="button" onClick={onClose}>Huỷ</button>{choice && <button type="button" disabled={busy || !plan} onClick={() => void apply()}>{choice.noImages ? 'Đổi · không tạo ảnh' : 'Đổi prompt'}</button>}</div>
  </div>
}
const styles = stylex.create({
  menu: { position: 'absolute', zIndex: 40, left: 0, top: '100%', width: 344, padding: s['--vv-s-10'], borderRadius: r['--vv-r-14'], border: '1px solid var(--line-2)', backgroundColor: 'var(--color-surface-3)', color: 'var(--color-fg)', boxShadow: 'var(--dialog-shadow)' },
  heading: { fontSize: 14, margin: 0 }, hint: { fontSize: 12, color: 'var(--color-fg-4)', margin: `${s['--vv-s-4']} 0 ${s['--vv-s-8']}` },
  tabs: { display: 'flex', padding: s['--vv-s-3'], backgroundColor: 'var(--color-surface-1)', borderRadius: r['--vv-r-9'] }, tab: { flex: 1, padding: s['--vv-s-8'], border: 0, borderRadius: r['--vv-r-6'], color: 'var(--color-fg-3)', backgroundColor: 'transparent', cursor: 'pointer' }, active: { backgroundColor: 'var(--color-surface-3)', color: 'var(--color-fg)' },
  list: { maxHeight: 292, overflowY: 'auto', display: 'flex', flexDirection: 'column' }, row: { textAlign: 'left', padding: s['--vv-s-10'], border: 0, borderRadius: r['--vv-r-8'], backgroundColor: 'transparent', color: 'var(--color-fg)', cursor: 'pointer' },
  cost: { padding: s['--vv-s-10'], border: '1px solid var(--line-2)', borderRadius: r['--vv-r-10'], color: 'var(--color-fg-2)', fontSize: 12 }, actions: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: s['--vv-s-8'], fontSize: 12 },
})
