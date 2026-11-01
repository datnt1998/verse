import * as stylex from '@stylexjs/stylex'
import { styles as settingsStyles, agtHintMarker, agtPillMarker, agtDrawcardBarMarker, agtFootMarker, agtMswitchMarker, agtSheetMarker } from '../settings.stylex'
// Trợ lý AI — mục Cài đặt (artboard Settings section=agents ai=director|
// artist, MSettings screen=agents sheet=director|artist). Wire:
// domain.AgentModels {director{model,effort,fallback}, artist{promptModel,
// draw, fallbackOn, drawFallback}} — lựa chọn là ModelRef {provider, model}.
//   • Một thẻ, bốn hàng: Trinh sát & Kiểm định “Không dùng model” (chạy trên
//     máy); Đạo diễn (model + Mức suy nghĩ + chuỗi dự phòng); Hoạ sĩ (model
//     viết mô tả ảnh cần đọc ảnh, lưới “Vẽ ảnh bằng”, switch vẽ lại + dự phòng).
//   • Điện thoại: bốn hàng gọn (mở sheet để chọn) + “Hoạ sĩ vẽ ảnh bằng” + thẻ
//     switch.
// Lưu: mọi thay đổi ghi vào MỘT bản nháp pending rồi PUT nối tiếp nhau — thay
// đổi mới không bao giờ bị PUT cũ đè lên; caps model đã chọn lấy từ tài nguyên
// dùng chung (filter=selected qua mọi trang), metadata thiếu = "chưa rõ", KHÔNG
// bịa. 422 capability_missing {field: …} → câu inline đúng chỗ.
import { useEffect, useMemo, useRef, useState } from 'react'
import { Button, Icon, RadioCard, RadioDot, Sheet, Switch, toast } from '@noverse/ui'
import { Segmented } from '../../../components/Segmented'
import { invalidate } from '@noverse/ui/data'
import { Orb, assistantName, type Assistant } from '../../../components'
import { navigate } from '../../../router'
import { useMediaQuery } from '../../../lib/useMediaQuery'
import { useResource } from '../../../data/useResource'
import { keys } from '../../../data/keys'
import {
  getAgents, listProviders, listSelectedModels, putAgents,
  type AiAgents, type AiModel, type AiProvider, type ModelRef,
} from '../../../api/ai'
import { aiCopy, clockOf } from '../../../i18n/ai'
import { t } from '../../../i18n/settings'
import { ModelList, ModelPicker, pickedFrom, refId, type Picked } from './ModelPicker'
import { fieldOf, planLabel, reasonOf } from '../connect/ProviderCard'

const a = aiCopy.agents

const EFFORTS = ['auto', 'low', 'medium', 'high'] as const
const EFFORT_LABEL: Record<(typeof EFFORTS)[number], string> = {
  auto: a.effortAuto,
  low: a.effortLow,
  medium: a.effortMedium,
  high: a.effortHigh,
}

type CapError = { field: string; need?: string } | null

function capMessage(err: CapError): string | null {
  if (!err) return null
  if (err.need === 'vision') return a.needVision
  if (err.need === 'image') return a.needImage
  return a.needText
}

const nf = new Intl.NumberFormat('vi-VN', { maximumFractionDigits: 3 })
const usd = (v: number) => `$${nf.format(v)}`

/** Catalog model ĐÃ CHỌN của một provider — tài nguyên dùng chung (SSE 'ai'
 *  invalidate đúng key này), đẩy kết quả lên parent qua onLoaded. */
function SelectedCatalog({ providerId, onLoaded }: {
  providerId: string
  onLoaded: (providerId: string, models: AiModel[] | undefined) => void
}) {
  const res = useResource(
    keys.aiSelectedModels(providerId),
    ({ signal }) => listSelectedModels(providerId, signal),
  )
  useEffect(() => { onLoaded(providerId, res.data) }, [providerId, res.data, onLoaded])
  return null
}

export function AgentsSection() {
  const mobile = useMediaQuery('(max-width: 767px)')
  const agents = useResource(keys.aiAgents(), ({ signal }) => getAgents(signal))
  const providers = useResource(keys.aiProviders(), ({ signal }) => listProviders(signal))
  const [capErr, setCapErr] = useState<CapError>(null)
  const [saving, setSaving] = useState(false)
  const [sheet, setSheet] = useState<null | 'director' | 'artist'>(null)
  const [catalogs, setCatalogs] = useState<Map<string, AiModel[]>>(new Map())
  // Bản nháp pending — nguồn sự thật duy nhất cho PUT nối tiếp.
  const pending = useRef<AiAgents | null>(null)
  const flushing = useRef(false)
  const [, bump] = useState(0)

  const data = agents.data
  const plist = providers.data ?? []
  const view = pending.current ?? data

  const onCatalogLoaded = (id: string, models: AiModel[] | undefined) => {
    setCatalogs((prev) => {
      if (prev.get(id) === models) return prev
      const next = new Map(prev)
      if (models === undefined) next.delete(id)
      else next.set(id, models)
      return next
    })
  }

  const picked = useMemo(() => pickedFrom(plist, (id) => catalogs.get(id)), [plist, catalogs])
  /** Nguồn vẽ = provider có ít nhất một model ảnh đã chọn. */
  const imageSources = useMemo(() => {
    const seen = new Map<string, { model: Picked; provider: AiProvider }>()
    for (const p of picked) {
      if (!p.caps?.includes('image') || seen.has(p.ref.provider)) continue
      const provider = plist.find((x) => x.id === p.ref.provider)
      if (provider) seen.set(p.ref.provider, { model: p, provider })
    }
    return [...seen.values()]
  }, [picked, plist])

  /** PUT nối tiếp từ pending — mỗi vòng gửi snapshot mới nhất; lỗi → huỷ
   *  pending (sửa nối trên bản bị từ chối là sai) và báo lỗi. */
  async function flush() {
    if (flushing.current) return
    flushing.current = true
    setSaving(true)
    try {
      while (pending.current != null) {
        const doc = pending.current
        try {
          await putAgents(doc)
          await invalidate(keys.aiAgents())
          if (pending.current === doc) pending.current = null
        } catch (error) {
          pending.current = null
          void invalidate(keys.aiAgents())
          const reason = reasonOf(error)
          if (reason === 'capability_missing') {
            setCapErr({ field: fieldOf(error, 'field') ?? 'director.model', need: fieldOf(error, 'need') })
          } else if (reason === 'unknown_model') {
            setCapErr({ field: 'director.model', need: 'text' })
          } else {
            toast.err(error instanceof Error ? error.message : t.errors.failed)
          }
          break
        }
      }
    } finally {
      flushing.current = false
      setSaving(false)
      bump((n) => n + 1)
    }
  }

  /** Ghi thay đổi vào pending rồi xả hàng đợi — hai thay đổi liền nhau luôn
   *  được gửi theo đúng thứ tự trong MỘT chuỗi PUT. */
  function mutate(fn: (doc: AiAgents) => AiAgents) {
    const base = pending.current ?? data
    if (!base) return
    pending.current = fn(base)
    setCapErr(null)
    bump((n) => n + 1)
    void flush()
  }

  if (agents.error) {
    return <div className={`agt-sec ${stylex.props(settingsStyles.agtSec).className}`}><div className={`agt-fail ${stylex.props(settingsStyles.agtFail).className}`} role="alert">{a.loadFail}</div></div>
  }
  if (!view) {
    return <div className={`agt-sec ${stylex.props(settingsStyles.agtSec).className}`}><div className={`agt-note ${stylex.props(settingsStyles.agtNote).className}`}>{a.loading}</div></div>
  }

  const directorValue = refId(view.director.model)
  const promptValue = refId(view.artist.promptModel)

  const setDirector = (ref: ModelRef) => mutate((d) => ({ ...d, director: { ...d.director, model: ref } }))
  const setEffort = (effort: string) => mutate((d) => ({ ...d, director: { ...d.director, effort } }))
  const setPrompt = (ref: ModelRef) => mutate((d) => ({ ...d, artist: { ...d.artist, promptModel: ref } }))
  const setDraw = (ref: ModelRef) => mutate((d) => ({ ...d, artist: { ...d.artist, draw: ref } }))
  const setRetry = (fallbackOn: boolean) => mutate((d) => ({ ...d, artist: { ...d.artist, fallbackOn } }))
  const setDirectorFallback = (fallback: ModelRef[]) => mutate((d) => ({ ...d, director: { ...d.director, fallback } }))
  const setDrawFallback = (drawFallback: ModelRef[]) => mutate((d) => ({ ...d, artist: { ...d.artist, drawFallback } }))

  const effortSeg = (
    <Segmented
      size="sm"
      options={EFFORTS.map((e) => ({ id: e, label: EFFORT_LABEL[e] }))}
      value={view.director.effort || 'auto'}
      onChange={setEffort}
      label={a.effort}
    />
  )

  // Dòng trạng thái chỉ hiện khi đang lưu (artboard không có câu thường trực).
  const applied = !capErr && saving ? a.saving : ''

  const directorHead = (
    <div className={`agt-head ${stylex.props(settingsStyles.agtHead).className}`}>
      <Orb of="director" size={22} />
      <div className={`agt-id ${stylex.props(settingsStyles.agtId).className}`}>
        <span className={`agt-name ${stylex.props(settingsStyles.agtName).className}`}>{assistantName('director')}</span>
        <span className={`agt-role ${stylex.props(settingsStyles.agtRole).className}`}>{mobile ? a.roleMobile.director : a.role.director}</span>
      </div>
    </div>
  )

  const artistHead = (
    <div className={`agt-head ${stylex.props(settingsStyles.agtHead).className}`}>
      <Orb of="artist" size={22} />
      <div className={`agt-id ${stylex.props(settingsStyles.agtId).className}`}>
        <span className={`agt-name ${stylex.props(settingsStyles.agtName).className}`}>{mobile ? a.artistMobile : assistantName('artist')}</span>
        <span className={`agt-role ${stylex.props(settingsStyles.agtRole).className}`}>{mobile ? a.roleMobile.artist : a.role.artist}</span>
      </div>
    </div>
  )

  /** Dòng thông tin của thẻ nguồn vẽ: gói/hạn mức từ wire; thẻ không có hạn mức
   *  thì ghi giá mỗi ảnh (“$0,039 mỗi ảnh”). Không ghi giá mỗi video — giá đó
   *  thuộc template của kênh (hỏi đáp #28). */
  const drawMeta = (provider: AiProvider, perImage: number | undefined) => {
    const q = provider.quota
    const period = q?.period === 'week' ? 'week' : 'day'
    const named = a.planName[provider.plan ?? ''] ?? planLabel(provider.plan)
    const plan = provider.auth === 'subscription'
      ? (q ? (named ? a.planSub(named, q.limit, period) : a.planSubGeneric(q.limit, period)) : a.planSubPaid)
      : provider.auth === 'local' ? a.planLocal
        : provider.dailyPaidImageLimit > 0 ? (mobile ? a.planPaidMobile : a.planPaid)(provider.dailyPaidImageLimit) : a.planPaidOff
    let used = ''
    let reset = ''
    let pct = 0
    if (q) {
      used = period === 'week' ? a.usedWeek(q.used, q.limit) : a.usedDay(q.used, q.limit)
      pct = Math.min(100, (q.used / Math.max(1, q.limit)) * 100)
      reset = period === 'week' ? a.resetWeekAt(new Date(q.resetAt).getDay()) : a.resetAt(clockOf(q.resetAt))
    } else {
      used = provider.auth === 'local' ? a.planLocal : perImage ? a.usedPrice(usd(perImage)) : ''
    }
    return { plan, used, reset, pct, hasQuota: !!q }
  }

  /** “ChatGPT · gói Plus” — tên gói thật khi wire có, không thì loại kết nối. */
  const provLine = (p: Picked | undefined) => {
    if (!p) return ''
    const named = planLabel(p.plan)
    return named ? `${p.providerLabel} · gói ${named}` : `${p.providerLabel} · ${a.groupAuth[p.auth]}`
  }

  const drawSources = imageSources.length > 0 ? (
    <div className={`agt-drawgrid ${stylex.props(settingsStyles.agtDrawgrid, imageSources.length === 4 && settingsStyles.agtDrawgridQuad).className}`} role="radiogroup" aria-label={a.drawLabel}>
      {imageSources.map(({ model, provider }) => {
        const on = view.artist.draw.provider === provider.id
        const meta = drawMeta(provider, model.perImage)
        return (
          <RadioCard
            key={provider.id}
            checked={on}
            mark="none"
            tabIndex={on || (!imageSources.some(({ provider: source }) => source.id === view.artist.draw.provider) && provider === imageSources[0].provider) ? 0 : -1}
            onChange={() => setDraw({ provider: provider.id, model: model.ref.model })}
          >
            <span {...stylex.props(settingsStyles.cardContent)}>
            <span className={`agt-drawcard__top ${stylex.props(settingsStyles.agtDrawcardTop).className}`}>
              <RadioDot checked={on} />
              <span className={`agt-drawcard__name ${stylex.props(settingsStyles.agtDrawcardName).className}`}>{provider.label}</span>
              {on && <span className={`agt-default ${stylex.props(settingsStyles.agtDefault).className}`}>{a.default}</span>}
            </span>
            <span className={`agt-drawcard__model ${stylex.props(settingsStyles.agtDrawcardModel).className}`}>{model.ref.model}</span>
            <span className={`agt-drawcard__plan ${stylex.props(settingsStyles.agtDrawcardPlan).className}`}>{meta.plan}</span>
            {meta.hasQuota && (
              <span className={`agt-drawcard__bar ${stylex.props(agtDrawcardBarMarker, settingsStyles.agtDrawcardBar).className}`} aria-hidden="true"><span style={{ width: `${meta.pct}%` }} {...stylex.props(settingsStyles.AgtDrawcardBarSpan)} /></span>
            )}
            <span className={`agt-drawcard__foot ${stylex.props(settingsStyles.agtDrawcardFoot).className}`}>
              <span className={`conn-num ${stylex.props(settingsStyles.connNum).className}`}>{meta.used}</span>
              <span>{meta.reset}</span>
            </span>
            </span>
          </RadioCard>
        )
      })}
    </div>
  ) : (
    <span className={`agt-drawcard__empty ${stylex.props(settingsStyles.agtDrawcardEmpty).className}`}>{a.needLogin('ChatGPT')}</span>
  )

  if (mobile) {
    // Hàng trợ lý: chỉ hàng có model mới mở sheet; hàng Trinh sát/Kiểm định
    // vẫn giữ nguyên độ đậm (aria-disabled, không disabled → không bị làm mờ).
    const row = (of: Assistant, name: string, role: string, value: string, sub: string, onPress?: () => void) => {
      const body = (
        <span {...stylex.props(settingsStyles.agtMrowContent)}>
          <Orb of={of} size={22} />
          <span className={`agt-mrow__id ${stylex.props(settingsStyles.agtMrowId).className}`}>
            <span className={`agt-mrow__name ${stylex.props(settingsStyles.agtMrowName).className}`}>{name}</span>
            <span className={`agt-mrow__role ${stylex.props(settingsStyles.agtMrowRole).className}`}>{role}</span>
          </span>
          <span className={`agt-mrow__val ${stylex.props(settingsStyles.agtMrowVal).className}`}>
            {value ? <span className={`agt-mrow__model ${stylex.props(settingsStyles.agtMrowModel).className}`}>{value}</span> : <span className={`agt-mrow__none ${stylex.props(settingsStyles.agtMrowNone).className}`}>{a.noModel}</span>}
            {sub && <span className={`agt-mrow__prov ${stylex.props(settingsStyles.agtMrowProv).className}`}>{sub}</span>}
          </span>
          {onPress && <Icon name="arrow-r" size={15} className="agt-mrow__chev" xstyle={[settingsStyles.agtMrowChev]} />}
        </span>
      )
      return onPress
        ? <Button key={of} variant="quiet" size="lg" block onClick={onPress} aria-haspopup="dialog">{body}</Button>
        : <div key={of} className={`agt-mrow ${stylex.props(settingsStyles.agtMrow).className}`} aria-disabled="true">{body}</div>
    }

    const directorPicked = picked.find((p) => refId(p.ref) === directorValue)
    const promptPicked = picked.find((p) => refId(p.ref) === promptValue)

    return (
      <div className={`agt-m ${stylex.props(settingsStyles.agtM).className}`}>
        <p className={`agt-m__lede ${stylex.props(settingsStyles.agtMLede).className}`}>{a.mobileLede}</p>
        {plist.filter((p) => p.selected.length > 0).map((p) => (
          <SelectedCatalog key={p.id} providerId={p.id} onLoaded={onCatalogLoaded} />
        ))}

        <section className={`agt-mcard ${stylex.props(settingsStyles.card, settingsStyles.agtMcard).className}`}>
          {row('scout', assistantName('scout'), a.roleMobile.scout, '', '')}
          {row('director', assistantName('director'), a.roleMobile.director, directorPicked?.label ?? '', provLine(directorPicked), () => setSheet('director'))}
          {row('artist', a.artistMobile, a.roleMobile.artist, promptPicked?.label ?? '', provLine(promptPicked), () => setSheet('artist'))}
          {row('qa', assistantName('qa'), a.roleMobile.qa, '', '')}
        </section>

        <div className={`agt-sec-head ${stylex.props(settingsStyles.agtSecHead).className}`}>{a.drawLabelMobile}</div>
        {imageSources.length === 0
          ? <section className={`agt-mcard ${stylex.props(settingsStyles.card, settingsStyles.agtMcard).className}`}><span className={`agt-mrow__none ${stylex.props(settingsStyles.agtMrowNone).className}`} style={{ padding: 'var(--vv-s-14)' }}>{a.needLogin('ChatGPT')}</span></section>
          : <div className={`agt-mdraws ${stylex.props(settingsStyles.agtMdraws).className}`} role="radiogroup" aria-label={a.drawLabelMobile}>{imageSources.map(({ model, provider }) => {
            const on = view.artist.draw.provider === provider.id
            const meta = drawMeta(provider, model.perImage)
            return (
              <RadioCard
                key={provider.id}
                checked={on}
                mark="none"
                tabIndex={on || (!imageSources.some(({ provider: source }) => source.id === view.artist.draw.provider) && provider === imageSources[0].provider) ? 0 : -1}
                onChange={() => setDraw({ provider: provider.id, model: model.ref.model })}
              >
                <span {...stylex.props(settingsStyles.cardContent)}>
                <span className={`agt-mdraw__row ${stylex.props(settingsStyles.agtMdrawRow).className}`}>
                  <RadioDot checked={on} />
                  <span className={`agt-mdraw__name ${stylex.props(settingsStyles.agtMdrawName).className}`}>{provider.label}</span>
                  <span className={`agt-mdraw__model ${stylex.props(settingsStyles.agtMdrawModel).className}`}>{model.ref.model}</span>
                </span>
                <span className={`agt-mdraw__plan ${stylex.props(settingsStyles.agtMdrawPlan).className}`}>{meta.plan}</span>
                <span className={`agt-mdraw__foot ${stylex.props(settingsStyles.agtMdrawFoot).className}`}>
                  <span className={`conn-num ${stylex.props(settingsStyles.connNum).className}`}>{meta.used}</span>
                  <span>{meta.reset}</span>
                </span>
                </span>
              </RadioCard>
            )
          })}</div>}

        <section className={`agt-mswitch ${stylex.props(settingsStyles.card, agtMswitchMarker, settingsStyles.agtMswitch).className}`}>
          <div className={`agt-switchrow ${stylex.props(settingsStyles.agtSwitchrow).className}`}>
            <span className={`agt-switchrow__text ${stylex.props(settingsStyles.agtSwitchrowText).className}`}>
              <span className={`agt-switchrow__label ${stylex.props(settingsStyles.agtSwitchrowLabel).className}`}>{a.retryOther}</span>
              <span className={`agt-switchrow__hint ${stylex.props(settingsStyles.agtSwitchrowHint).className}`}>{a.retryHintMobile}</span>
            </span>
            <Switch checked={view.artist.fallbackOn} onChange={setRetry} disabled={saving} label={a.retryOther} />
          </div>
          {view.artist.fallbackOn && (
            /* Artboard MSettings: điện thoại chỉ ĐỌC chuỗi dự phòng (IA: chuỗi
               và mức suy nghĩ của Đạo diễn/Hoạ sĩ chỉ chỉnh trên máy tính). */
            <FallbackChain
              chain={view.artist.drawFallback}
              picked={picked.filter((s) => s.ref.provider !== view.artist.draw.provider)}
              onChange={setDrawFallback}
              needs="image"
              readOnly
            />
          )}
          {capErr?.field === 'artist.drawFallback' && <span className={`agt-caperr ${stylex.props(settingsStyles.agtCaperr).className}`}>{capMessage(capErr)}</span>}
        </section>

        {applied && <span className={`agt-applied ${stylex.props(settingsStyles.agtApplied).className}`}>{applied}</span>}

        <Sheet
          open={sheet != null}
          onClose={() => setSheet(null)}
          title={sheet === 'director' ? a.directorTitle : a.artistSheetTitle}
          subtitle={sheet === 'director' ? a.sheetSubDirector : a.sheetSubVision}
          size="md"
          homeIndicator
          height="auto"
          xstyle={settingsStyles.agentSheet}
          bodyStyle={settingsStyles.agentSheetBody}
        >
          <div className={`agt-sheet agt-sheet-list ${stylex.props(agtSheetMarker, settingsStyles.agtSheetList).className}`}>
            <ModelList
              variant="sheet"
              picked={picked}
              value={sheet === 'director' ? directorValue : promptValue}
              needs={sheet === 'director' ? 'text' : 'vision'}
              onPick={(p) => { (sheet === 'director' ? setDirector : setPrompt)(p.ref); setSheet(null) }}
            />
          </div>
          {capErr && (capErr.field === 'director.model' || capErr.field === 'artist.promptModel') && (
            <span className={`agt-caperr ${stylex.props(settingsStyles.agtCaperr).className}`} style={{ padding: '0 var(--vv-s-16)' }}>{capMessage(capErr)}</span>
          )}
          <Button variant="quiet" size="lg" onClick={() => { setSheet(null); navigate('/settings?section=connect') }}>{a.pickerFooter}</Button>
        </Sheet>
      </div>
    )
  }

  return (
    <div className={`agt-sec ${stylex.props(settingsStyles.agtSec).className}`}>
      {plist.filter((p) => p.selected.length > 0).map((p) => (
        <SelectedCatalog key={p.id} providerId={p.id} onLoaded={onCatalogLoaded} />
      ))}

      <section className={`agt-card ${stylex.props(settingsStyles.card, settingsStyles.agtCard).className}`}>
        <div className={`agt-row ${stylex.props(settingsStyles.agtRow).className}`}>
          <div className={`agt-head ${stylex.props(settingsStyles.agtHead).className}`}>
            <Orb of="scout" size={22} />
            <div className={`agt-id ${stylex.props(settingsStyles.agtId).className}`}>
              <span className={`agt-name ${stylex.props(settingsStyles.agtName).className}`}>{assistantName('scout')}</span>
              <span className={`agt-role ${stylex.props(settingsStyles.agtRole).className}`}>{a.role.scout}</span>
            </div>
            <span className={`agt-nomodel ${stylex.props(settingsStyles.agtNomodel).className}`}>{a.noModel}</span>
          </div>
        </div>

        <div className={`agt-row ${stylex.props(settingsStyles.agtRow).className}`}>
          {directorHead}
          <div className={`agt-grid ${stylex.props(settingsStyles.agtGrid).className}`}>
            <div className={`agt-field ${stylex.props(settingsStyles.agtField).className}`}>
              <span className={`agt-label ${stylex.props(settingsStyles.agtLabel).className}`}>{a.modelLabel}</span>
              <ModelPicker
                picked={picked}
                value={directorValue}
                needs="text"
                onPick={(p) => setDirector(p.ref)}
                label={a.modelLabel}
                title={a.directorTitle}
                sub={a.pickerSubText}
              />
            </div>
            <div className={`agt-field ${stylex.props(settingsStyles.agtField).className}`}>
              <span className={`agt-label ${stylex.props(settingsStyles.agtLabel).className}`}>{a.effort}</span>
              {effortSeg}
            </div>
          </div>
          <div className={`agt-field ${stylex.props(settingsStyles.agtField).className}`}>
            <span className={`agt-label ${stylex.props(settingsStyles.agtLabel).className}`}>{a.chainDirector}</span>
            <FallbackChain
              chain={view.director.fallback}
              picked={picked.filter((p) => refId(p.ref) !== directorValue)}
              onChange={setDirectorFallback}
              needs="text"
              addLabel={a.chainAddShort}
              addIcon
            />
          </div>
          {capErr?.field === 'director.model' && <span className={`agt-caperr ${stylex.props(settingsStyles.agtCaperr).className}`}>{capMessage(capErr)}</span>}
          {capErr?.field === 'director.fallback' && <span className={`agt-caperr ${stylex.props(settingsStyles.agtCaperr).className}`}>{capMessage(capErr)}</span>}
        </div>

        <div className={`agt-row ${stylex.props(settingsStyles.agtRow).className}`}>
          {artistHead}
          <div className={`agt-grid ${stylex.props(settingsStyles.agtGrid).className}`}>
            <div className={`agt-field ${stylex.props(settingsStyles.agtField).className}`}>
              <span className={`agt-label ${stylex.props(settingsStyles.agtLabel).className}`}>{a.promptModel}</span>
              <ModelPicker
                picked={picked}
                value={promptValue}
                needs="vision"
                onPick={(p) => setPrompt(p.ref)}
                label={a.promptModel}
                title={a.artistSheetTitle}
                sub={a.pickerSubVision}
              />
            </div>
            <div className={`agt-field ${stylex.props(settingsStyles.agtField).className}`}>
              <span className={`agt-hint ${stylex.props(agtHintMarker, settingsStyles.agtHint).className}`}>
                <Icon name="info" size={13} className="agt-ic" xstyle={[settingsStyles.agtIc]} />
                {a.promptHint}
              </span>
            </div>
          </div>
          {capErr?.field === 'artist.promptModel' && <span className={`agt-caperr ${stylex.props(settingsStyles.agtCaperr).className}`}>{capMessage(capErr)}</span>}

          <div className={`agt-field ${stylex.props(settingsStyles.agtField).className}`}>
            <span className={`agt-label ${stylex.props(settingsStyles.agtLabel).className}`}>{a.drawLabel}</span>
            {drawSources}
          </div>
          {capErr?.field === 'artist.draw' && <span className={`agt-caperr ${stylex.props(settingsStyles.agtCaperr).className}`}>{capMessage(capErr)}</span>}

          <div className={`agt-switchrow ${stylex.props(settingsStyles.agtSwitchrow).className}`}>
            <span className={`agt-switchrow__text ${stylex.props(settingsStyles.agtSwitchrowText).className}`}>
              <span className={`agt-switchrow__label ${stylex.props(settingsStyles.agtSwitchrowLabel).className}`}>{a.retryOther}</span>
              <span className={`agt-switchrow__hint ${stylex.props(settingsStyles.agtSwitchrowHint).className}`}>{a.retryHint}</span>
            </span>
            <Switch checked={view.artist.fallbackOn} onChange={setRetry} disabled={saving} label={a.retryOther} />
          </div>

          {view.artist.fallbackOn && (
            <FallbackChain
              chain={view.artist.drawFallback}
              picked={picked.filter((s) => s.ref.provider !== view.artist.draw.provider)}
              onChange={setDrawFallback}
              needs="image"
              addLabel={a.chainAdd}
              addIcon
            />
          )}
          {capErr?.field === 'artist.drawFallback' && <span className={`agt-caperr ${stylex.props(settingsStyles.agtCaperr).className}`}>{capMessage(capErr)}</span>}
        </div>

        <div className={`agt-row ${stylex.props(settingsStyles.agtRow).className}`}>
          <div className={`agt-head ${stylex.props(settingsStyles.agtHead).className}`}>
            <Orb of="qa" size={22} />
            <div className={`agt-id ${stylex.props(settingsStyles.agtId).className}`}>
              <span className={`agt-name ${stylex.props(settingsStyles.agtName).className}`}>{assistantName('qa')}</span>
              <span className={`agt-role ${stylex.props(settingsStyles.agtRole).className}`}>{a.role.qa}</span>
            </div>
            <span className={`agt-nomodel ${stylex.props(settingsStyles.agtNomodel).className}`}>{a.noModel}</span>
          </div>
        </div>

      </section>

      {/* Artboard: câu giải thích nằm NGOÀI thẻ bốn trợ lý, không có vạch chia. */}
      <p className={`agt-foot ${stylex.props(agtFootMarker, settingsStyles.agtFoot).className}`}>
        <Icon name="info" size={15} xstyle={[settingsStyles.AgtFootFirstChild]} />
        <span {...stylex.props(settingsStyles.AgtFootFirstChild)}>{a.footer}</span>
      </p>

      {applied && <span className={`agt-applied ${stylex.props(settingsStyles.agtApplied).className}`}>{applied}</span>}
    </div>
  )
}

/** Chuỗi dự phòng: pill theo thứ tự (× gỡ) + "＋ Thêm nguồn" mở picker. */
function FallbackChain({ chain, picked, onChange, needs, addLabel, addIcon, readOnly }: {
  chain: ModelRef[]
  picked: Picked[]
  onChange: (next: ModelRef[]) => void
  needs?: 'vision' | 'image' | 'text'
  /** Nhãn nút thêm: Đạo diễn “Thêm”, Hoạ sĩ “Thêm nguồn” (artboard). */
  addLabel?: string
  addIcon?: boolean
  /** Điện thoại: chỉ đọc — artboard không có nút thêm/gỡ. */
  readOnly?: boolean
}) {
  const [adding, setAdding] = useState(false)
  const keyset = new Set(chain.map(refId))
  const candidates = picked.filter((p) => !keyset.has(refId(p.ref)))
  return (
    <span className={`agt-chain ${stylex.props(settingsStyles.agtChain).className}`}>
      {chain.map((ref, i) => (
        <span key={refId(ref)} className={`agt-pill ${stylex.props(agtPillMarker, settingsStyles.agtPill).className}`}>
          <span className={`agt-pill__n ${stylex.props(settingsStyles.agtPillN).className}`}>{i + 1}</span>
          <span className={`agt-pill__id ${stylex.props(settingsStyles.agtPillId).className}`}>{ref.model}</span>
          <span className={`agt-pill__prov ${stylex.props(settingsStyles.agtPillProv).className}`}>· {picked.find((p) => refId(p.ref) === refId(ref))?.providerLabel ?? ref.provider}</span>
          {!readOnly && (
            <Button variant="icon" size="sm" aria-label={`${t.common.cancel} ${ref.model}`} onClick={() => onChange(chain.filter((x) => refId(x) !== refId(ref)))}><Icon name="close" size={12} /></Button>
          )}
        </span>
      ))}
      {!readOnly && (adding ? (
        <span className={`agt-addwrap ${stylex.props(settingsStyles.agtAddwrap).className}`}>
          <span className={`agt-addlist ${stylex.props(settingsStyles.agtAddlist).className}`}>
            <ModelList
              picked={candidates}
              value=""
              needs={needs}
              onPick={(p) => { onChange([...chain, p.ref]); setAdding(false) }}
            />
          </span>
          <Button size="lg" variant="quiet" onClick={() => setAdding(false)}>{a.done}</Button>
        </span>
      ) : (
        <Button variant="quiet" size="sm" onClick={() => setAdding(true)} disabled={candidates.length === 0} icon={addIcon ? <Icon name="add" size={12} /> : undefined}>
          {addLabel ?? a.chainAdd}
        </Button>
      ))}
    </span>
  )
}

export default AgentsSection
