import * as stylex from '@stylexjs/stylex'
import { styles as settingsStyles, connDirtyMarker, connSecMarker, connMMarker } from '../settings.stylex'
// Kết nối AI — mục Cài đặt (artboard Settings conn=list|add|added|models|manual|
// failed + grok=*, MSettings screen=connect/sheet=add|models).
//   • danh sách thẻ nhà cung cấp (avatar, pill loại + năng lực, dòng phụ, hạn
//     mức, cụm nút, danh mục model inline) — mỗi thẻ một <ProviderCard>.
//   • nút dashed “Thêm nhà cung cấp — …” mở panel/sheet thêm.
//   • thanh chưa lưu dính đầu mục: chọn model là bản nháp cục bộ, Lưu PATCH
//     selected từng provider đổi, Huỷ trả nháp về server.
// SSE 'ai' → invalidate ['ai'] → danh sách tự làm mới.
import { useMemo, useState } from 'react'
import { Button, Icon, Sheet, toast } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { useMediaQuery } from '../../../lib/useMediaQuery'
import { useResource } from '../../../data/useResource'
import { keys } from '../../../data/keys'
import { listProviders, updateProvider, type AiModel, type AiProvider } from '../../../api/ai'
import { ApiError } from '../../../api/client'
import { aiCopy, clockOf } from '../../../i18n/ai'
import { navigate } from '../../../router'
import { t } from '../../../i18n/settings'
import { AddProviderBody } from './AddProvider'
import { ModelCatalog, catalogSource } from './ModelCatalog'
import { orderProviders } from './order'
import { ProviderCard, reasonOf } from './ProviderCard'
import { YoutubeDataKeyCard } from './YoutubeDataKeyCard'

const c = aiCopy.connect

type DraftMap = Map<string, AiModel[]>

/** Bản nháp chọn của một provider từ selected[] — id mất dấu giữ placeholder
 *  caps RỖNG (không bịa capability); catalog thật đổ vào khi mở trang. */
function draftOf(p: AiProvider): AiModel[] {
  return p.selected.map((id) => ({ id, caps: [] }))
}

export function ConnectSection() {
  const onAssign = () => navigate('/settings?section=agents')
  const mobile = useMediaQuery('(max-width: 767px)')
  const providers = useResource(keys.aiProviders(), ({ signal }) => listProviders(signal))
  const [drafts, setDrafts] = useState<DraftMap | null>(null)
  const [newlyAdded, setNewlyAdded] = useState<string | null>(null)
  /** Thẻ vừa kết nối chưa lưu lần nào — tự thân nó là “1 thay đổi chưa lưu”
   *  (artboard conn=added hiện thanh Huỷ/Lưu kể cả khi chưa tick model nào);
   *  Lưu = ghi nháp xuống, Huỷ = bỏ nháp về server. */
  const [unsavedNew, setUnsavedNew] = useState<Set<string>>(new Set())
  /** Nguồn vừa được chọn lại ở panel Thêm: thẻ đó cuộn vào tầm nhìn và mở Đổi khoá. */
  const [reveal, setReveal] = useState<{ id: string; nonce: number } | null>(null)
  const [addOpen, setAddOpen] = useState(false)
  const [addSheet, setAddSheet] = useState(false)
  const [modelsSheet, setModelsSheet] = useState<AiProvider | null>(null)
  const [saving, setSaving] = useState(false)
  const list = providers.data ?? []
  const offline = providers.error instanceof ApiError && providers.error.status === 0

  // Draft theo provider; server-side selected làm mốc — manual thêm vào kèm
  // name/caps thật trả về từ models/check.
  const draftFor = (p: AiProvider): AiModel[] => drafts?.get(p.id) ?? draftOf(p)
  // Thẻ mới bị xoá/refetch mất khỏi danh sách thì không tính nữa — tránh
  // thanh “chưa lưu” treo mãi vì một id không còn trên dây.
  const unsavedNewActive = useMemo(
    () => new Set([...unsavedNew].filter((id) => list.some((p) => p.id === id))),
    [unsavedNew, list],
  )
  const dirty = drafts != null || unsavedNewActive.size > 0
  // Số THẺ có thay đổi (artboard: “1 thay đổi chưa lưu”), không phải số thẻ
  // đang có bản nháp — bản nháp khởi tạo cho mọi thẻ ngay khi mở danh mục.
  // Thẻ vừa kết nối chưa lưu tự thân nó cũng là một thay đổi.
  const changed = useMemo(() => {
    const ids = new Set(unsavedNewActive)
    for (const p of list) {
      const cur = drafts?.get(p.id)
      if (!cur) continue
      const saved = draftOf(p)
      if (cur.length !== saved.length || cur.some((m, i) => m.id !== saved[i]?.id)) ids.add(p.id)
    }
    return ids.size
  }, [drafts, unsavedNewActive, list])

  function toggleModel(p: AiProvider, id: string, on: boolean, meta?: Partial<AiModel>) {
    setDrafts((prev) => {
      const next = new Map(prev ?? new Map(list.map((x) => [x.id, draftOf(x)])))
      const cur = new Map((next.get(p.id) ?? draftOf(p)).map((m) => [m.id, m]))
      if (on) cur.set(id, { id, caps: meta?.caps ?? [], manual: meta?.manual, contextK: meta?.contextK })
      else cur.delete(id)
      next.set(p.id, [...cur.values()])
      return next
    })
  }

  function onAdded(id: string) {
    setNewlyAdded(id)
    setUnsavedNew((prev) => new Set(prev).add(id))
    setAddOpen(false)
    setAddSheet(false)
  }

  function onReveal(id: string) {
    setReveal((prev) => ({ id, nonce: (prev?.nonce ?? 0) + 1 }))
    setAddOpen(false)
    setAddSheet(false)
  }

  async function save() {
    if (saving || (!drafts && unsavedNewActive.size === 0)) return
    setSaving(true)
    try {
      for (const p of list) {
        const draft = drafts?.get(p.id) ?? draftOf(p)
        const ids = draft.map((m) => m.id)
        // Thẻ mới chưa lưu: Lưu vẫn ghi nháp xuống server (kể cả danh sách
        // rỗng — “lưu lần kết nối này”), nhưng bỏ qua thẻ cũ không đổi.
        if (ids.join('\n') === p.selected.join('\n') && !unsavedNew.has(p.id)) continue
        await updateProvider(p.id, { selected: ids })
      }
      await invalidate(keys.aiProviders())
      setDrafts(null)
      setUnsavedNew(new Set())
    } catch (error) {
      if (reasonOf(error) === 'auth_failed') toast.err(t.errors.auth_failed('nhà cung cấp'))
      else toast.err(error instanceof Error ? error.message : t.errors.failed)
    } finally {
      setSaving(false)
    }
  }

  /** Huỷ: bỏ cả nháp lẫn cờ “thẻ mới chưa lưu” — thẻ mới quay về trạng thái
   *  server (chọn mặc định, thường là rỗng) như chưa từng lưu nháp. */
  function cancel() {
    setDrafts(null)
    setUnsavedNew(new Set())
  }

  // Thẻ lỗi lên đầu, còn lại theo thứ tự thêm (cũ trước); thẻ vừa thêm đứng
  // trên cùng tới khi rời trang (state này mất khi màn unmount).
  const ordered = useMemo(() => orderProviders(list, newlyAdded), [list, newlyAdded])

  const sheetProvider = modelsSheet && (list.find((p) => p.id === modelsSheet.id) ?? modelsSheet)
  const sheetDraft = sheetProvider ? draftFor(sheetProvider) : []

  // Thanh chưa lưu dính đầu mục — artboard đặt nó TRƯỚC danh sách để luôn
  // thấy khi cuộn (desktop) và khi đóng sheet (điện thoại).
  const dirtyBar = dirty && (
    <div className={`conn-dirty ${stylex.props(connDirtyMarker, settingsStyles.connDirty).className}`} role="status">
      <Icon name="info" size={15} style={{ color: 'var(--color-warn)', flex: '0 0 auto' }} />
      <span className={`conn-dirty__text ${stylex.props(settingsStyles.connDirtyText).className}`}>{c.dirty(changed)}</span>
      <Button className="conn-act conn-act--quiet" xstyle={[settingsStyles.connActQuiet, settingsStyles.connAct]} size="compact" variant="quiet" onClick={cancel}>{c.cancel}</Button>
      <Button className="conn-act" xstyle={[settingsStyles.connAct]} size="compact" variant="primary" pending={saving} onClick={() => void save()}>{c.save}</Button>
    </div>
  )

  if (mobile) {
    return (
      <div className={`conn-m ${stylex.props(connMMarker, settingsStyles.connM).className}`}>
        <p className={`conn-m__lede ${stylex.props(settingsStyles.connMLede).className}`}>{c.mobileLede}</p>
        {offline && <div className={`conn-fail ${stylex.props(settingsStyles.connFail).className}`} role="status">{c.offline}</div>}
        {providers.error != null && !offline && <div className={`conn-fail ${stylex.props(settingsStyles.connFail).className}`} role="alert">{c.loadFail}</div>}
        {providers.data === undefined && !providers.error && <div className={`conn-note-line ${stylex.props(settingsStyles.connNoteLine).className}`}>{c.loading}</div>}
        {ordered.map((p) => (
          <ProviderCard
            key={p.id}
            provider={p}
            draft={draftFor(p)}
            mobile
            isNew={p.id === newlyAdded}
            revealKey={reveal?.id === p.id ? reveal.nonce : undefined}
            onToggleModel={toggleModel}
            onOpenModels={(prov) => setModelsSheet(prov)}
            onAssign={onAssign}
          />
        ))}
        {providers.data && list.length === 0 && <div className={`conn-note-line ${stylex.props(settingsStyles.connNoteLine).className}`}>{c.empty}</div>}
        <YoutubeDataKeyCard />

        {dirtyBar}

        <button type="button" className={`conn-addbtn conn-m__addbtn ${stylex.props(settingsStyles.connAddbtn, settingsStyles.connMAddbtn).className}`} onClick={() => setAddSheet(true)}>
          <Icon name="add" size={15} />
          {c.addOpenShort}
        </button>

        <Sheet open={addSheet} onClose={() => setAddSheet(false)} title={c.addTitle} subtitle={c.addSubMobile} size="md" homeIndicator height="auto">
          <AddProviderBody mobile onAdded={onAdded} onReveal={onReveal} onDone={() => setAddSheet(false)} providers={list} />
        </Sheet>

        <Sheet
          open={sheetProvider != null}
          onClose={() => setModelsSheet(null)}
          title={sheetProvider ? c.modelsTitle(sheetProvider.label) : c.addTitle}
          size="md"
          homeIndicator
          height="auto"
          subtitle={sheetProvider
            ? (sheetProvider.catalogAt
              ? c.modelsSub(catalogSource(sheetProvider), clockOf(sheetProvider.catalogAt), sheetProvider.catalogCount)
              : c.modelsSubDefault)
            : undefined}
        >
          {sheetProvider && (
            <>
              <ModelCatalog
                provider={sheetProvider}
                draft={sheetDraft}
                variant="sheet"
                onToggle={(id, on, meta) => toggleModel(sheetProvider, id, on, meta)}
              />
              <div className={`conn-sheet-foot ${stylex.props(settingsStyles.connSheetFoot).className}`}>{c.sheetSum(sheetDraft.length)}</div>
            </>
          )}
        </Sheet>
      </div>
    )
  }

  return (
    <div className={`conn-sec ${stylex.props(connSecMarker, settingsStyles.connSec).className}`}>
      {offline && <div className={`conn-fail ${stylex.props(settingsStyles.connFail).className}`} role="status">{c.offline}</div>}
      {providers.error != null && !offline && <div className={`conn-fail ${stylex.props(settingsStyles.connFail).className}`} role="alert">{c.loadFail}</div>}
      {providers.data === undefined && !providers.error && <div className={`conn-note-line ${stylex.props(settingsStyles.connNoteLine).className}`}>{c.loading}</div>}

      {dirtyBar}

      <div className={`conn-list ${stylex.props(settingsStyles.connList).className}`}>
        {ordered.map((p) => (
          <ProviderCard
            key={p.id}
            provider={p}
            draft={draftFor(p)}
            isNew={p.id === newlyAdded}
            revealKey={reveal?.id === p.id ? reveal.nonce : undefined}
            onToggleModel={toggleModel}
            onAssign={onAssign}
          />
        ))}
        {providers.data && list.length === 0 && <div className={`conn-note-line ${stylex.props(settingsStyles.connNoteLine).className}`}>{c.empty}</div>}
        <YoutubeDataKeyCard />
      </div>

      {addOpen ? (
        <section className={`conn-card ${stylex.props(settingsStyles.card, settingsStyles.connCard).className}`}>
          <AddProviderBody onAdded={onAdded} onReveal={onReveal} onDone={() => setAddOpen(false)} providers={list} />
        </section>
      ) : (
        <button type="button" className={`conn-addbtn ${stylex.props(settingsStyles.connAddbtn).className}`} onClick={() => setAddOpen(true)}>
          <Icon name="add" size={15} />
          {c.addOpen}
        </button>
      )}

    </div>
  )
}

// Re-export cho SettingsWebA (màn setup dùng ChatGPTLoginAction).
export { ChatGPTLoginAction } from './ChatGPTLogin'

export default ConnectSection
