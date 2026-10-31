// M4 › Tuỳ chỉnh truyện (MMore.dc.html · storyset): văn phong, cách chạy, ngân sách theo 3 cấp.
import { useEffect, useState } from 'react'
import { getConfig, getSnapshot, putConfig, setReview } from '../../../api/client'
import { useLibrary } from '../../../api/library'
import { useSession } from '../../../api/session'
import { cache } from '@noverse/ui/data'
import { keys } from '../../../data/keys'
import { useAction } from '../../../data/useAction'
import { useResource } from '../../../data/useResource'
import { projectLabel, storyLabel } from '../../../shell/labels'
import { Icon } from '@noverse/ui'
import { Switch } from '@noverse/ui'
import { configForSave } from '../../settings/lib/config'
import type { OpenTarget, ScopeTarget } from '../../settings/scope/model'
import { canEditTarget, MobileScopeRow, MobileScopeSheet } from './ScopeSheet'
import { ResourceNotice } from './parts'
import { budgetPercent, stepBudget, storySetChangedKeys, storySetConfig, storySetValues, usd, type StorySetDraft } from './storySetLogic'

/** Thanh “N thay đổi · Huỷ · Áp dụng” nổi trên thanh tab. */
export function ApplyBar({ text, onCancel, onApply, busy }: { text: string; onCancel: () => void; onApply: () => void; busy?: boolean }) {
  return <div className="mm-applybar" role="region" aria-label="Thay đổi chưa áp dụng">
    <span>{text}</span>
    <button className="mm-apply-cancel" type="button" disabled={busy} onClick={onCancel}>Huỷ</button>
    <button className="mm-apply-go" type="button" disabled={busy} aria-busy={busy} onClick={onApply}>Áp dụng</button>
  </div>
}

export function MobileStorySet({ project, story, nextChapter, online, toast, onModel, writerModel }: {
  project: string | null; story: string | null; nextChapter: number
  online: boolean; toast: (text: string) => void
  onModel: () => void; writerModel: string
}) {
  // Gói R — đổi chỗ xem bằng dòng phạm vi + bottom sheet, không đổi truyện đang mở ở Bàn viết.
  const [target, setTarget] = useState<ScopeTarget>({
    scope: story ? 'story' : project ? 'project' : 'global', project: project ?? '', story: story ?? '',
  })
  const [scopeOpen, setScopeOpen] = useState(false)
  const { user } = useSession()
  const library = useLibrary()
  const [draft, setDraft] = useState<StorySetDraft>({})
  const key = keys.config(target.scope, target.project, target.story)
  const config = useResource(key, () => getConfig(target.scope, target.project || undefined, target.story || undefined), { enabled: target.scope === 'global' || !!target.project && (target.scope === 'project' || !!target.story) })
  const snapshot = useResource(keys.story(target.project, target.story, 'snapshot'), ({ signal }) => getSnapshot(target.project, target.story, signal), { enabled: target.scope === 'story' && !!target.project && !!target.story })
  useEffect(() => { setDraft({}) }, [target.scope, target.project, target.story])

  // Truyện đang mở ở Bàn viết: giữ tên + chương kế tiếp từ màn trước; xem truyện khác thì lấy từ thư viện.
  const openTarget: OpenTarget = project && story
    ? { project, story, title: storyLabel(library.storyOf(project, story) ?? { story }) }
    : null
  const viewedSummary = target.project && target.story
    ? library.projectDetail(target.project)?.stories.find((item) => item.story === target.story) ?? null
    : null
  const effectiveChapter = target.project === project && target.story === story
    ? nextChapter
    : viewedSummary ? viewedSummary.completedChapters + 1 : nextChapter
  const projectName = projectLabel(library.library?.projects.find((item) => item.id === target.project), target.project)
  const storyTitle = storyLabel(viewedSummary ?? { story: target.story })
  const scope = target.scope
  const canEdit = canEditTarget(user, target) && online
  const effective = config.data?.effective ?? {}
  const values = storySetValues(effective, draft)
  const changed = storySetChangedKeys(effective, draft)
  const spent = snapshot.data?.totalCostUSD ?? 0

  const review = useAction(async (on: boolean) => {
    if (!target.project || !target.story) return
    await setReview(target.project, target.story, on)
    toast(on ? 'Đã bật duyệt từng chương' : 'Đã tắt duyệt từng chương')
  }, {
    invalidate: [key, keys.story(target.project, target.story, 'snapshot')],
    optimistic: (on) => {
      const before = cache.get<NonNullable<typeof config.data>>(key).data
      if (before) cache.setData(key, { ...before, effective: { ...before.effective, advance_mode: on ? 'review' : 'auto' } })
      return () => { cache.restore(key, before) }
    },
  })
  const saveViewed = async (): Promise<boolean> => {
    const current = config.data
    if (!current) return false
    await putConfig(target.scope, configForSave(storySetConfig(current.target.config, current.effective, draft)), target.project || undefined, target.story || undefined, current.version)
    setDraft({})
    return true
  }
  const apply = useAction(async () => {
    if (!await saveViewed()) return
    toast(target.scope === 'story' ? `Đã áp dụng · từ chương ${effectiveChapter}` : 'Đã áp dụng')
  }, { invalidate: [['config']] })

  const toggleReview = () => {
    const on = values.advance !== 'review'
    if (target.scope === 'story') { void review.run(on); return }
    setDraft((old) => ({ ...old, advance: on ? 'review' : 'auto' }))
  }
  const setBudget = (direction: 1 | -1) => setDraft((old) => ({ ...old, budget: stepBudget(storySetValues(effective, old).budget, direction) }))
  const errors = [review.error, apply.error].filter((item) => item != null)

  return <>
    <MobileScopeRow target={target} open={openTarget} projectName={projectName} storyTitle={storyTitle} onOpen={() => setScopeOpen(true)} />
    <MobileScopeSheet open={scopeOpen} onClose={() => setScopeOpen(false)} target={target} openTarget={openTarget}
      dirtyCount={changed.length} onTarget={setTarget} onDiscard={() => setDraft({})}
      onSave={async () => {
        try {
          if (!await saveViewed()) return false
          toast(target.scope === 'story' ? `Đã áp dụng · từ chương ${effectiveChapter}` : 'Đã áp dụng')
          return true
        } catch (error) {
          toast(error instanceof Error ? error.message : 'Không áp dụng được. Vui lòng thử lại.')
          return false
        }
      }} />
    <ResourceNotice resource={config} label="tuỳ chỉnh truyện" />
    {config.data && <>
      {!canEdit && <p className="mm-note">Chỉ người có quyền {scope === 'global' ? 'quản trị' : 'viết'} mới đổi được.</p>}
      <div className="mm-heading">Cách chạy</div>
      <div className="mm-group">
        <div className="mm-row"><span className="mm-copy"><span>Duyệt từng chương</span><span className="mm-sub">{scope === 'story' ? 'Dừng sau mỗi chương chờ bạn duyệt · đổi ngay' : 'Cho truyện mới tạo'}</span></span>
          <Switch size={[51, 31]} checked={values.advance === 'review'} label="Duyệt từng chương" pending={review.busy} disabled={!canEdit} onChange={toggleReview} />
        </div>
        <button className="mm-row" type="button" disabled={!story} onClick={onModel}><span className="mm-copy"><span>Người viết</span><span className="mm-sub">{writerModel}</span></span><Icon name="arrow-right-2" size={16} className="mm-chevron" /></button>
      </div>
      <div className="mm-heading">Ngân sách</div>
      <div className="mm-group">
        {scope === 'story' && <div className="mm-row mm-budget-used"><span><span>Đã dùng</span><span className="mm-num">{usd(spent)} / {values.budget > 0 ? usd(values.budget) : '—'}</span></span><span className="mm-bar"><span style={{ width: budgetPercent(spent, values.budget) }} /></span></div>}
        <div className="mm-row"><span className="mm-fill">Giới hạn mỗi truyện</span>
          <span className="mm-stepper">
            <button className="mm-step" type="button" aria-label="Giảm" disabled={!canEdit} onClick={() => setBudget(-1)}><Icon name="minus" size={16} /></button>
            <span className="mm-num">{usd(values.budget)}</span>
            <button className="mm-step" type="button" aria-label="Tăng" disabled={!canEdit} onClick={() => setBudget(1)}><Icon name="add" size={16} /></button>
          </span>
        </div>
      </div>
      {errors.map((error, index) => <p className="mm-error" role="alert" key={index}>{error instanceof Error ? error.message : 'Không thực hiện được. Vui lòng thử lại.'}</p>)}
      <p className="mm-note">Kết nối AI, xuất tự động và mục Nâng cao chỉnh trên máy tính (quản trị).</p>
    </>}
    {changed.length > 0 && <ApplyBar text={`${changed.length} thay đổi${scope === 'story' ? ` · từ chương ${nextChapter}` : ''}`} busy={apply.busy} onCancel={() => setDraft({})} onApply={() => { void apply.run() }} />}
  </>
}
