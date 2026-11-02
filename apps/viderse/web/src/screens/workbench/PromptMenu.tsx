import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, RadioCard, Segmented, Sheet } from '@noverse/ui'
import { space as s, corners as r } from '../../styles/viderse.stylex'
import { listPrompts, type PromptKind } from '../../api/library'
import { applyPrompt, getJobImages, planPrompt, uploadCover, type CloneJobView, type PromptChoice, type PromptPlan } from '../../api/workbench'
import { useResource } from '../../data/useResource'

/** Menu Ảnh của Bàn dựng: popover trên máy tính, Sheet trên điện thoại. */
export function PromptMenu({ job, onClose, onRefresh, mobile }: { job: CloneJobView; onClose: () => void; onRefresh: () => void; mobile?: boolean }) {
  const [tab, setTab] = useState<PromptKind>('scene')
  const [choice, setChoice] = useState<PromptChoice | null>(null)
  const [error, setError] = useState('')
  const [plan, setPlan] = useState<PromptPlan | null>(null)
  const [busy, setBusy] = useState(false)
  const prompts = useResource(['library', 'prompts', tab, ''], ({ signal }) => listPrompts({ kind: tab }, signal))
  const images = useResource(['workbench', 'images', job.id], ({ signal }) => getJobImages(job.id, signal), { enabled: !job.queueItem })
  const sceneCount = images.data?.items?.filter(item => item.kind === 'scene').length
  const hasRunningScene = images.data?.items?.some(item => item.kind === 'scene' && item.status === 'running') ?? false
  const manifest = images.data?.manifest
  const scenePromptId = job.queueItem ? job.queueItem.scenePromptId : manifest?.scenePrompt.overridden ? manifest.scenePrompt.id : null
  const coverPromptId = job.queueItem ? job.queueItem.coverPromptId : manifest?.coverPrompt.overridden ? manifest.coverPrompt.id : null
  const selected = tab === 'scene'
    ? choice ? choice.noImages ? 'none' : choice.scenePromptId ?? 'template' : job.noImages ? 'none' : scenePromptId ?? 'template'
    : choice ? choice.coverPromptId ?? 'template' : coverPromptId ?? 'template'
  const inheritedTabStop = selected === 'template' || (selected !== 'none' && !prompts.data?.items.some((prompt) => prompt.id === selected))
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
  const body = <>
    <h2 {...stylex.props(styles.heading)}>Ảnh của video</h2><p {...stylex.props(styles.hint)}>Mặc định dùng chỉ dẫn của mẫu video{job.templateName ? ` ${job.templateName}` : ''}. Đổi ở đây chỉ áp dụng cho video này.</p>
    <div {...stylex.props(styles.tabs)}>
      <Segmented value={tab} onChange={(id) => { setTab(id as PromptKind); setChoice(null); setPlan(null); setError('') }} label="Loại ảnh" fit="fill"
        options={[{ id: 'scene', label: 'Ảnh cảnh' }, { id: 'cover', label: 'Ảnh bìa' }]} />
    </div>
    <div role="radiogroup" aria-label={tab === 'scene' ? 'Ảnh cảnh' : 'Ảnh bìa'} {...stylex.props(styles.list, mobile && styles.listMobile)}>
      <RadioCard label="Theo mẫu video" checked={selected === 'template'} disabled={busy} tabIndex={inheritedTabStop ? 0 : -1} xstyle={styles.promptOption} onChange={() => void pick(tab === 'scene' ? { scenePromptId: null, noImages: false } : { coverPromptId: null })} />
      {prompts.initialLoading ? <p>Đang tải…</p> : prompts.error ? <p role="alert">Không tải được chỉ dẫn. <Button size="sm" variant="quiet" onClick={() => void prompts.refresh()}>Thử lại</Button></p> : prompts.data?.items.map((prompt) => <RadioCard key={prompt.id} label={prompt.name} checked={selected === prompt.id} disabled={busy} tabIndex={selected === prompt.id ? 0 : -1} xstyle={styles.promptOption} onChange={() => void pick(tab === 'scene' ? { scenePromptId: prompt.id, noImages: false } : { coverPromptId: prompt.id })} />)}
      {tab === 'scene' && <RadioCard label="Không tạo ảnh cho video này" checked={selected === 'none'} disabled={busy} tabIndex={selected === 'none' ? 0 : -1} xstyle={styles.promptOption} onChange={() => void pick({ noImages: true })} />}
    </div>
    {tab === 'cover' && <label {...stylex.props(styles.row)}>Dùng ảnh của bạn<input type="file" accept="image/*" disabled={busy} onChange={event => { const file = event.target.files?.[0]; if (file) void cover(file) }} /></label>}
    {plan && <p role="status" {...stylex.props(styles.cost)}>{choice?.noImages ? 'Đổi · không tạo ảnh' : plan.enablesImages ? `Bật lại ảnh: vẽ ${plan.redraw.scenes} ảnh cảnh chia đều theo độ dài video, bìa theo mẫu video. Tốn ${plan.cost.images} ảnh` : tab === 'scene' ? `Vẽ lại ${plan.redraw.scenes} ảnh cảnh đã vẽ${hasRunningScene ? ' hoặc đang vẽ' : ''}${sceneCount !== undefined ? `; ${Math.max(0, sceneCount - plan.redraw.scenes)} ảnh còn lại vẽ luôn theo chỉ dẫn mới` : ''}. Tốn ${plan.cost.images} ảnh` : `Vẽ lại ảnh bìa. Tốn ${plan.cost.images} ảnh`}{plan.rebuild ? ' · Dựng lại video' : ''}</p>}{error && <p role="alert">{error}</p>}
    <div {...stylex.props(styles.actions)}><a href="#/library?tab=prompts">Kho chỉ dẫn ảnh</a><span {...stylex.props(styles.actionEnd)}><Button size={mobile ? 'lg' : 'sm'} variant="ghost" onClick={onClose}>Huỷ</Button>{choice && <Button size={mobile ? 'lg' : 'sm'} variant="primary" disabled={busy || !plan} onClick={() => void apply()}>{choice.noImages ? 'Đổi · không tạo ảnh' : 'Đổi chỉ dẫn'}</Button>}</span></div>
  </>
  if (mobile) return <Sheet open onClose={onClose} label="Ảnh của video" size="md" height="auto">
    <div {...stylex.props(styles.sheetBody)}>{body}</div>
  </Sheet>
  return <div role="dialog" aria-label="Ảnh của video" {...stylex.props(styles.menu)}>{body}</div>
}
const styles = stylex.create({
  menu: { position: 'absolute', zIndex: 40, left: s['--vv-s-18'], top: 116, width: 344, height: 498, boxSizing: 'border-box', display: 'flex', flexDirection: 'column', padding: s['--vv-s-10'], borderRadius: r['--vv-r-14'], border: '1px solid var(--line-2)', backgroundColor: 'var(--color-surface-3)', color: 'var(--color-fg)', boxShadow: 'var(--dialog-shadow)' },
  sheetBody: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-8'], minHeight: 0 },
  heading: { fontSize: 14, margin: 0 }, hint: { fontSize: 12, color: 'var(--color-fg-4)', margin: `${s['--vv-s-4']} 0 ${s['--vv-s-8']}` },
  tabs: { display: 'flex' },
  list: { flex: 1, minHeight: 0, overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: s['--vv-s-6'] },
  listMobile: { maxHeight: '46vh', flex: '0 0 auto' },
  promptOption: { flexShrink: 0 },
  row: { textAlign: 'left', padding: s['--vv-s-10'], border: 0, borderRadius: r['--vv-r-8'], backgroundColor: 'transparent', color: 'var(--color-fg)', cursor: 'pointer' },
  cost: { padding: s['--vv-s-10'], border: '1px solid var(--line-2)', borderRadius: r['--vv-r-10'], color: 'var(--color-fg-2)', fontSize: 12 },
  actions: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: s['--vv-s-8'], fontSize: 12 },
  actionEnd: { display: 'inline-flex', alignItems: 'center', gap: s['--vv-s-8'] },
})
