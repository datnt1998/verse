import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from '../libraryTokens.stylex'
// Mục Ảnh bìa (Template.dc.html › lib^cover): prompt ảnh bìa, vẽ thử ảnh thật
// (một ảnh/lần, chạy nền, giữ 12 lần gần nhất) và ảnh bìa mặc định tải từ kho
// media của editor. Prompt có sẵn đứng trước prompt của bạn trong ô chọn.
import { useEffect, useRef, useState } from 'react'
import { flushSync } from 'react-dom'
import { ImageViewer } from '../../../components/image-viewer/ImageViewer'
import { sx as viewerStyles } from '../../../components/image-viewer/image-viewer.stylex'
import { useMediaQuery } from '../../../lib/useMediaQuery'
import { RadioCard, Icon, confirm, useEscapeLayer } from '@noverse/ui'
import { ActionButton } from '../../../components'
import { t } from '../../../i18n/library'
import type { ImagePrompt, Recipe, ThumbnailStyle } from '../../../api/library'
import { uploadRefImage } from '../../../api/images'
import type { CoverTrial, CoverTrialStatus } from '../../../api/cover-trials'
import { useRoute } from '../../../router'
import { clock } from '../fmt'
import { PromptPicker } from './PromptPicker'
import { useCoverTrial } from './useCoverTrial'
import { trialFailureText } from '../../../i18n/errorsCopy'

const MAX_COVER_BYTES = 20 * 1024 * 1024
const COVER_TYPES: Record<string, true> = { 'image/png': true, 'image/jpeg': true, 'image/webp': true }

export function CoverSection({ recipe, prompts, errors, onSave, hideTitle, templateId: templateIdProp }: {
  recipe: Recipe
  prompts: ImagePrompt[]
  errors: Record<string, string>
  onSave: (patch: Partial<Recipe>) => void
  /** Ẩn h2 khi lồng trong màn con có tiêu đề riêng (MTemplate). */
  hideTitle?: boolean
  /** Trang template truyền id; thiếu thì lấy từ route hiện tại. */
  templateId?: string
}) {
  const copy = t.library.template.cover
  const phone = useMediaQuery('(max-width: 767px)')
  const route = useRoute().route
  const templateId = templateIdProp ?? (route.name === 'template' ? route.templateId : '')
  const trial = useCoverTrial(templateId)
  const scenes = recipe.scenes
  const setScenes = (next: Partial<Recipe['scenes']>) => onSave({ scenes: { ...scenes, ...next } })
  const ordered = [...prompts.filter((prompt) => prompt.builtIn), ...prompts.filter((prompt) => !prompt.builtIn)]

  const [pickedJob, setPickedJob] = useState('')
  const [viewing, setViewing] = useState<CoverTrial | null>(null)
  const [mediaError, setMediaError] = useState<string | null>(null)
  const [uploading, setUploading] = useState(false)
  const [dropping, setDropping] = useState(false)
  const fileRef = useRef<HTMLInputElement | null>(null)
  const backRef = useRef<HTMLButtonElement | null>(null)
  const triggerRef = useRef<HTMLButtonElement | null>(null)
  // Hộp xác nhận Vẽ thử có thể còn chờ khi mục Ảnh bìa đã đóng (đổi màn,
  // đóng sheet): chỉ mở lại trình xem khi mục này còn sống.
  const mounted = useRef(true)
  useEffect(() => {
    mounted.current = true
    return () => { mounted.current = false }
  }, [])
  // Giữ ảnh mẫu trong cùng bề mặt chỉnh sửa, không mở thêm lớp modal.
  const previewing = phone && viewing !== null

  useEffect(() => {
    if (phone && viewing) backRef.current?.focus()
  }, [phone, viewing])

  const status = trial.status
  const sources = status?.sources ?? []
  // Hàng ảnh mẫu mới nhất: hộp xác nhận chờ có thể vắt qua một lần làm mới.
  const historyRef = useRef<CoverTrial[]>(status?.history ?? [])
  historyRef.current = status?.history ?? []
  const jobId = pickedJob || sources[0]?.jobId || ''
  const source = sources.find((item) => item.jobId === jobId)
  const costLabel = status?.cost.label || 'AI'
  const running = (status?.history ?? []).find((row) => row.state === 'running')
  const drawLockedReason = running ? copy.trialRunning : !status?.ready ? status?.blockedReason ?? copy.trialLocked : undefined

  const quotaText = (s: CoverTrialStatus | undefined): string => {
    if (!s) return ''
    if (!s.quota) return copy.trialQuotaUnknown
    if (s.quota?.paid) return copy.trialQuota(s.quota.label, s.quota.left, s.quota.total)
    return copy.trialQuotaFree(s.cost.label)
  }

  const draw = async (id: string, title: string): Promise<boolean> => {
    if (!status || drawLockedReason) return false
    const accepted = await confirm({
      title: copy.trialConfirmTitle,
      body: copy.trialConfirmBody(title, status.cost.label, quotaText(status)),
      confirmLabel: copy.trialCost(status.cost.label),
      cancelLabel: t.library.cancel,
      onConfirm: () => undefined,
    })
    if (!accepted) return false
    await trial.start(id)
    return true
  }

  const drawSelected = async () => {
    if (source) await draw(source.jobId, source.title)
  }

  const retry = async (row: CoverTrial) => {
    await draw(row.jobId, row.jobTitle)
  }

  // Vẽ thử từ trình xem ảnh mẫu, theo quy ước SceneImagePanel: đóng trình xem
  // trước rồi mới hỏi (chỉ còn một lớp nhận tương tác). Huỷ hoặc chưa hỏi được
  // thì mở lại đúng ảnh mẫu đó; đã nhận thì giữ đóng và chỉ gửi đúng một lần.
  const drawFromViewer = (sample: CoverTrial) => {
    flushSync(() => setViewing(null))
    void draw(sample.jobId, sample.jobTitle).then((accepted) => {
      if (accepted || !mounted.current) return
      // Ảnh mẫu có thể đã rời 12 lần gần nhất trong lúc hỏi — đừng mở lại.
      const latest = historyRef.current.find((row) => row.id === sample.id)
      if (latest) setViewing(latest)
    })
  }

  const uploadCover = async (file: File | undefined) => {
    if (!file) return
    setMediaError(null)
    if (!COVER_TYPES[file.type]) {
      setMediaError(copy.defaultBadType)
      return
    }
    if (file.size > MAX_COVER_BYTES) {
      setMediaError(copy.defaultTooBig)
      return
    }
    setUploading(true)
    try {
      const id = await uploadRefImage(file)
      setScenes({ coverMediaId: id, coverSource: 'upload' })
    } catch (err) {
      setMediaError(err instanceof Error && err.message ? err.message : copy.defaultUploadFail)
    } finally {
      setUploading(false)
    }
  }

  const removeCover = () => setScenes({ coverMediaId: '', coverSource: 'ai' })

  const openSample = (row: CoverTrial, trigger: HTMLButtonElement) => {
    triggerRef.current = trigger
    setViewing(row)
  }

  const closeSample = () => {
    const trigger = triggerRef.current
    flushSync(() => setViewing(null))
    trigger?.focus()
  }

  // Esc trả về đúng mục Ảnh bìa (không đóng màn/sheet bên ngoài) và trả tiêu
  // điểm về ô ảnh mẫu đã bấm, như bản xem trước của PromptEditor.
  useEscapeLayer(previewing, closeSample)

  return (
    <section className={`vt-sec ${stylex.props(sx.sec, hideTitle && sx.mobileSection).className}`} aria-label={hideTitle ? copy.title : undefined} aria-labelledby={hideTitle ? undefined : 'vt-cover-title'}>
      <div className={`vt-sec-head ${stylex.props(sx.secHead).className}`}>
        {!hideTitle && <h2 {...stylex.props(sx.vtSecHeadH2)} id="vt-cover-title">{copy.title}</h2>}
        <p className={`vt-help ${stylex.props(sx.help).className}`}>{copy.help}</p>
      </div>

      <div className={`vt-card ${stylex.props(sx.card, hideTitle && sx.mobileCard, previewing && sx.settingsHidden).className}`}>
        <div className={`vt-card-head ${stylex.props(sx.cardHead).className}`}>
          <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.promptTitle}</span>
          <span className={`vt-links ${stylex.props(sx.links).className}`}>
            <a className={`vt-link ${stylex.props(sx.link).className}`} href="#/library?tab=prompts">{copy.promptLibrary}</a>
          </span>
        </div>
        <PromptPicker
          kind="cover"
          mobileLayout={hideTitle}
          prompts={ordered}
          value={scenes.coverPromptId}
          onSelect={(id) => setScenes({ coverPromptId: id })}
          usedLabel={copy.promptNote}
        />
        {errors['scenes.coverPromptId'] && <p className={`vt-fielderr ${stylex.props(sx.fielderr).className}`} role="alert">{errors['scenes.coverPromptId']}</p>}
      </div>

      <div className={`vt-card ${stylex.props(sx.card, hideTitle && sx.mobileCard, previewing && sx.settingsHidden).className}`}>
        <div className={`vt-card-head ${stylex.props(sx.cardHead).className}`}>
          <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.trialTitle}</span>
          <span className={`vt-hint ${stylex.props(sx.hint).className}`}>{copy.trialKeep}</span>
        </div>

        <div className={`vt-row ${stylex.props(sx.row).className}`}>
          <label className={`vt-hint ${stylex.props(sx.hint).className}`} htmlFor="vt-cover-trial-source">{copy.trialSource}</label>
          {sources.length > 0 ? (
            <select
              id="vt-cover-trial-source"
              className={`vt-select ${stylex.props(sx.select).className}`}
              aria-label={copy.trialSourceAria}
              value={jobId}
              disabled={trial.running}
              onChange={(event) => setPickedJob(event.currentTarget.value)}
            >
              {sources.map((item) => (
                <option key={item.jobId} value={item.jobId}>
                  {copy.trialSourceOption(item.title, Math.max(1, Math.round(item.durationSec / 60)))}
                </option>
              ))}
            </select>
          ) : (
            <span className={`vt-hint ${stylex.props(sx.hint).className}`}>{trial.loading ? copy.trialRunning : copy.trialLocked}</span>
          )}
        </div>

        {running ? (
          <div className={`vt-row ${stylex.props(sx.row).className}`}>
            <span className={`vt-hint ${stylex.props(sx.progress).className}`} role="status">
              {status?.task?.phase ? copy.trialPhase(status.task.phase) : copy.trialRunning}
            </span>
            <ActionButton variant="outline" icon={<Icon name="close" size={16} />} onClick={() => void trial.cancel(running.id)}>
              {copy.trialCancel}
            </ActionButton>
          </div>
        ) : (
          <div className={`vt-row ${stylex.props(sx.row).className}`}>
            <ActionButton
              variant="outline"
              icon={<Icon name="magicpen" size={16} />}
              pending={trial.loading}
              lockedReason={drawLockedReason}
              onClick={() => void drawSelected()}
            >
              {copy.trialCost(costLabel || 'AI')}
            </ActionButton>
            <p className={`vt-hint ${stylex.props(sx.hint).className}`}>
              {status?.blockedReason ?? (status?.quota?.paid
                ? copy.trialHelp + ' ' + quotaText(status)
                : copy.trialHelp)}
            </p>
          </div>
        )}
        {trial.error && <p className={`vt-fielderr ${stylex.props(sx.fielderr).className}`} role="alert">{trial.error}</p>}

        {(status?.history.length ?? 0) > 0 ? (
          <ul className={`vt-trial-grid ${stylex.props(sx.trials).className}`} aria-label={copy.trialTitle}>
            {status!.history.map((row) => (
              <li key={row.id} className={`vt-trial ${stylex.props(sx.trial).className}`} data-state={row.state}>
                {row.state === 'ready' ? (
                  <button type="button" className={`vt-trial-open ${stylex.props(sx.trialOpen).className}`} onClick={(event) => openSample(row, event.currentTarget)} aria-label={copy.trialView}>
                    <img className={`vt-trial-img ${stylex.props(sx.trialImg).className}`} src={row.imageUrl ?? ''} alt="" loading="lazy" />
                    <span className={`vt-trial-meta ${stylex.props(sx.trialMeta).className}`}>
                      {t.library.prompts.composition[(row.composition as ThumbnailStyle) ?? 'clean'] ?? row.composition} · {clock(row.createdAt)}
                    </span>
                  </button>
                ) : row.state === 'running' ? (
                  <div className={`vt-trial-state ${stylex.props(sx.trialState).className}`}>
                    <span className={`vt-hint ${stylex.props(sx.hint).className}`} role="status">{copy.trialRunning}</span>
                  </div>
                ) : (
                  <div className={`vt-trial-state ${stylex.props(sx.trialState).className}`}>
                    <span className={`vt-hint ${stylex.props(sx.hint).className}`}>{row.state === 'cancelled' ? copy.trialCancelled : trialFailureText(row.error?.code)}</span>
                    <ActionButton size="sm" variant="outline" lockedReason={drawLockedReason} onClick={() => void retry(row)}>{copy.trialRetry}</ActionButton>
                  </div>
                )}
              </li>
            ))}
          </ul>
        ) : (
          <p className={`vt-hint ${stylex.props(sx.hint).className}`}>{copy.trialEmpty}</p>
        )}
      </div>

      <div className={`vt-card ${stylex.props(sx.card, hideTitle && sx.mobileCard, previewing && sx.settingsHidden).className}`}>
        <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.defaultTitle}</span>
        <div
          className={`vt-cover-default ${stylex.props(sx.coverDefault, dropping && sx.coverDefaultDrop).className}`}
          onDragOver={(event) => { event.preventDefault(); setDropping(true) }}
          onDragLeave={() => setDropping(false)}
          onDrop={(event) => {
            event.preventDefault()
            setDropping(false)
            void uploadCover(event.dataTransfer.files[0])
          }}
        >
          {scenes.coverMediaId ? (
            <img className={`vt-cover-thumb ${stylex.props(sx.coverThumb).className}`} src={`/api/editor/media/${encodeURIComponent(scenes.coverMediaId)}`} alt="" />
          ) : (
            <span className={`vt-cover-thumb ${stylex.props(sx.coverThumb).className}`} aria-hidden="true" />
          )}
          <div className={`vt-cover-copy ${stylex.props(sx.coverCopy).className}`}>
            <p {...stylex.props(sx.vtCoverCopyP)}>{scenes.coverMediaId ? copy.defaultSet : copy.defaultNone}</p>
            <p className={`vt-hint ${stylex.props(sx.hint2).className}`}>{dropping ? copy.defaultDropActive : copy.defaultDrop}</p>
          </div>
          <input
            ref={fileRef}
            type="file"
            accept="image/png,image/jpeg,image/webp"
            className={`vt-visually-hidden ${stylex.props(sx.fileInput).className}`}
            onChange={(event) => {
              void uploadCover(event.currentTarget.files?.[0])
              event.currentTarget.value = ''
            }}
          />
          {scenes.coverMediaId ? (
            <ActionButton variant="outline" icon={<Icon name="trash" size={16} />} onClick={removeCover}>{copy.defaultRemove}</ActionButton>
          ) : (
            <ActionButton variant="outline" pending={uploading} onClick={() => fileRef.current?.click()}>{uploading ? copy.defaultUploading : copy.defaultChoose}</ActionButton>
          )}
        </div>
        <p className={`vt-hint ${stylex.props(sx.hint).className}`}>{copy.defaultHelp}</p>
        {mediaError && <p className={`vt-fielderr ${stylex.props(sx.fielderr).className}`} role="alert">{mediaError}</p>}
        {errors['scenes.coverMediaId'] && <p className={`vt-fielderr ${stylex.props(sx.fielderr).className}`} role="alert">{errors['scenes.coverMediaId']}</p>}
      </div>

      <div className={`vt-card ${stylex.props(sx.card, hideTitle && sx.mobileCard, previewing && sx.settingsHidden).className}`}>
        <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.sourceLabel}</span>
        <div role="radiogroup" aria-label={copy.sourceLabel} className={`vt-sources ${stylex.props(sx.sources).className}`}>
          <RadioCard xstyle={[sx.vtSourcesRadioCard]} checked={scenes.coverSource === 'ai'} onChange={() => setScenes({ coverSource: 'ai' })}
            label={copy.sourceAi} description={copy.sourceAiHelp} />
          <RadioCard xstyle={[sx.vtSourcesRadioCard]} checked={scenes.coverSource === 'upload'} onChange={() => setScenes({ coverSource: 'upload' })}
            label={copy.sourceUpload} description={copy.sourceUploadHelp} />
        </div>
      </div>

      {!phone && viewing && <ImageViewer mode="sample" title={viewing.promptName} selectedId={viewing.id}
        stripLabel="Ảnh mẫu của prompt" subtitle="Ảnh bìa · ảnh mẫu của prompt"
        items={(status?.history ?? []).map(row => ({ id: row.id, label: `${row.jobTitle} · ${clock(row.createdAt)}`, url: row.imageUrl, disabled: row.state !== 'ready' || !row.imageUrl }))}
        onSelect={id => { const sample = status?.history.find(row => row.id === id); if (sample) setViewing(sample) }} onClose={() => setViewing(null)}
        side={<><strong {...stylex.props(viewerStyles.sideTitle)}>{viewing.promptName}</strong><p>{viewing.jobTitle} · {clock(viewing.createdAt)}</p><h3 {...stylex.props(viewerStyles.caption)}>Prompt</h3><p>{prompts.find(prompt => prompt.id === viewing.promptId)?.prompt ?? 'Prompt của ảnh mẫu này không còn trong thư viện.'}</p></>}
        footer={<><span>Ảnh mẫu · không thay ảnh trong video</span><ActionButton variant="primary" lockedReason={drawLockedReason} onClick={() => drawFromViewer(viewing)}>{copy.trialCost(costLabel)}</ActionButton></>}
      />}
      {phone && viewing && (
        <div className={`vt-cover-sample ${stylex.props(sx.sample).className}`} role="group" aria-label={viewing.promptName}>
          <ActionButton ref={backRef} variant="outline" xstyle={[sx.sampleBack]} icon={<Icon name="arrow-left-2" size={16} />} onClick={closeSample}>
            {t.library.mobile.backSection(copy.title)}
          </ActionButton>
          <img className={`vt-trial-full ${stylex.props(sx.trialFull).className}`} src={viewing.imageUrl ?? ''} alt={viewing.promptName} />
          <p className={`vt-hint ${stylex.props(sx.hint).className}`}>
            {t.library.prompts.composition[(viewing.composition as ThumbnailStyle) ?? 'clean'] ?? viewing.composition} · {clock(viewing.createdAt)} · {viewing.promptName}
          </p>
        </div>
      )}
    </section>
  )
}

const sx = stylex.create({
  mobileSection: { rowGap: { default: null, '@media (max-width: 767px)': 'var(--vv-s-18)' } },
  mobileCard: {
    paddingTop: { default: null, '@media (max-width: 767px)': 0 },
    paddingRight: { default: null, '@media (max-width: 767px)': 0 },
    paddingBottom: { default: null, '@media (max-width: 767px)': 0 },
    paddingLeft: { default: null, '@media (max-width: 767px)': 0 },
    borderTopWidth: { default: null, '@media (max-width: 767px)': 0 },
    borderRightWidth: { default: null, '@media (max-width: 767px)': 0 },
    borderBottomWidth: { default: null, '@media (max-width: 767px)': 0 },
    borderLeftWidth: { default: null, '@media (max-width: 767px)': 0 },
    borderTopLeftRadius: { default: null, '@media (max-width: 767px)': 0 },
    borderTopRightRadius: { default: null, '@media (max-width: 767px)': 0 },
    borderBottomLeftRadius: { default: null, '@media (max-width: 767px)': 0 },
    borderBottomRightRadius: { default: null, '@media (max-width: 767px)': 0 },
    backgroundColor: { default: null, '@media (max-width: 767px)': 'transparent' },
  },
  sec: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-12)", columnGap: "var(--vv-s-12)", minWidth: 0 },
  // Điện thoại: form vẫn mount, chỉ ẩn khi đang xem ảnh mẫu trong chính mục này.
  settingsHidden: { display: "none" },
  sample: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", minWidth: 0 },
  sampleBack: { alignSelf: "flex-start", minHeight: "var(--vv-s-44)" },
  secHead: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-4)", columnGap: "var(--vv-s-4)" },
  vtSecHeadH2: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-16)", fontWeight: libraryVars.headingWeight },
  help: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.bodySmallFontSize, color: color["--color-fg-3"], lineHeight: libraryVars.helpLineHeight, maxWidth: libraryVars.templateBackdropSectionHelpMaxWidth },
  card: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", paddingTop: "var(--vv-s-14)", paddingRight: "var(--vv-s-14)", paddingBottom: "var(--vv-s-14)", paddingLeft: "var(--vv-s-14)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-12)", borderTopRightRadius: "var(--vv-r-12)", borderBottomRightRadius: "var(--vv-r-12)", borderBottomLeftRadius: "var(--vv-r-12)", backgroundColor: color["--color-card"], backgroundImage: "none", minWidth: 0 },
  cardHead: { display: "flex", alignItems: "baseline", justifyContent: "space-between", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", flexWrap: "wrap" },
  label: {
    fontSize: { default: 'var(--vv-s-14)', '@media (max-width: 767px)': 'var(--vv-s-12)' },
    fontWeight: { default: libraryVars.labelWeight, '@media (max-width: 767px)': 600 },
    color: { default: color['--color-fg'], '@media (max-width: 767px)': 'var(--kit-fg-sub)' },
    letterSpacing: { default: null, '@media (max-width: 767px)': '.06em' },
    textTransform: { default: null, '@media (max-width: 767px)': 'uppercase' },
  },
  links: { display: "inline-flex", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)" },
  link: { color: color["--color-link"], fontSize: "var(--vv-s-12)", textDecoration: { default: "none", ":hover": "underline" }, minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  fielderr: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-12)", color: color["--color-err"] },
  sources: { display: "grid", gridTemplateColumns: libraryVars.templateCoverSectionSourcesGridTemplateColumns, rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)" },
  vtSourcesRadioCard: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  hint: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.captionFontSize, color: color["--color-fg-4"], lineHeight: libraryVars.bodyLineHeight },
  row: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", flexWrap: "wrap" },
  select: { minHeight: "var(--vv-s-34)", maxWidth: "100%", borderTopWidth: "var(--vv-s-1)", borderRightWidth: "var(--vv-s-1)", borderBottomWidth: "var(--vv-s-1)", borderLeftWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderRightStyle: "solid", borderBottomStyle: "solid", borderLeftStyle: "solid", borderTopColor: line["--line-1"], borderRightColor: line["--line-1"], borderBottomColor: line["--line-1"], borderLeftColor: line["--line-1"], borderTopLeftRadius: "var(--vv-r-7)", borderTopRightRadius: "var(--vv-r-7)", borderBottomRightRadius: "var(--vv-r-7)", borderBottomLeftRadius: "var(--vv-r-7)", backgroundColor: color["--color-bg"], color: color["--color-fg"], paddingLeft: "var(--vv-s-8)", paddingRight: "var(--vv-s-8)", fontSize: "var(--vv-s-13)" },
  progress: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.bodySmallFontSize, color: color["--color-fg-2"] },
  trials: { listStyle: "none", marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, paddingTop: 0, paddingRight: 0, paddingBottom: 0, paddingLeft: 0, display: "grid", gridTemplateColumns: "repeat(4, minmax(0, 1fr))", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)" },
  trial: { minWidth: 0, borderTopWidth: "var(--vv-s-1)", borderRightWidth: "var(--vv-s-1)", borderBottomWidth: "var(--vv-s-1)", borderLeftWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderRightStyle: "solid", borderBottomStyle: "solid", borderLeftStyle: "solid", borderTopColor: line["--line-2"], borderRightColor: line["--line-2"], borderBottomColor: line["--line-2"], borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-7)", borderTopRightRadius: "var(--vv-r-7)", borderBottomRightRadius: "var(--vv-r-7)", borderBottomLeftRadius: "var(--vv-r-7)", overflow: "hidden", backgroundColor: "var(--vv-tile-idle)" },
  trialOpen: { display: "flex", flexDirection: "column", width: "100%", paddingTop: 0, paddingRight: 0, paddingBottom: 0, paddingLeft: 0, borderTopWidth: 0, borderRightWidth: 0, borderBottomWidth: 0, borderLeftWidth: 0, background: "none", cursor: "pointer", textAlign: "left" },
  trialImg: { width: "100%", aspectRatio: libraryVars.videoAspectRatio, objectFit: "cover", display: "block" },
  trialMeta: { display: "block", paddingTop: "var(--vv-s-4)", paddingRight: "var(--vv-s-6)", paddingBottom: "var(--vv-s-4)", paddingLeft: "var(--vv-s-6)", fontSize: "var(--vv-s-11)", color: color["--color-fg-3"], whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" },
  trialState: { display: "flex", flexDirection: "column", justifyContent: "center", rowGap: "var(--vv-s-4)", columnGap: "var(--vv-s-4)", paddingTop: "var(--vv-s-8)", paddingRight: "var(--vv-s-8)", paddingBottom: "var(--vv-s-8)", paddingLeft: "var(--vv-s-8)", minHeight: "var(--vv-s-80)" },
  trialFull: { width: "100%", height: "auto", display: "block", borderTopLeftRadius: "var(--vv-r-7)", borderTopRightRadius: "var(--vv-r-7)", borderBottomRightRadius: "var(--vv-r-7)", borderBottomLeftRadius: "var(--vv-r-7)" },
  coverDefault: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-12)", columnGap: "var(--vv-s-12)", flexWrap: "wrap", borderTopWidth: "var(--vv-s-1)", borderRightWidth: "var(--vv-s-1)", borderBottomWidth: "var(--vv-s-1)", borderLeftWidth: "var(--vv-s-1)", borderTopStyle: "dashed", borderRightStyle: "dashed", borderBottomStyle: "dashed", borderLeftStyle: "dashed", borderTopColor: "transparent", borderRightColor: "transparent", borderBottomColor: "transparent", borderLeftColor: "transparent", borderTopLeftRadius: "var(--vv-r-7)", borderTopRightRadius: "var(--vv-r-7)", borderBottomRightRadius: "var(--vv-r-7)", borderBottomLeftRadius: "var(--vv-r-7)" },
  coverDefaultDrop: { borderTopColor: color["--color-link"], borderRightColor: color["--color-link"], borderBottomColor: color["--color-link"], borderLeftColor: color["--color-link"] },
  coverThumb: { flexGrow: 0, flexShrink: 0, flexBasis: "auto", width: libraryVars.templateCoverSectionCoverThumbWidth, aspectRatio: libraryVars.videoAspectRatio, objectFit: "cover", borderTopLeftRadius: "var(--vv-r-7)", borderTopRightRadius: "var(--vv-r-7)", borderBottomRightRadius: "var(--vv-r-7)", borderBottomLeftRadius: "var(--vv-r-7)", backgroundColor: "var(--vv-tile-idle)", backgroundImage: "none" },
  coverCopy: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: libraryVars.templateCoverSectionCoverCopyFlexBasis, minWidth: 0, display: "flex", flexDirection: "column", rowGap: "var(--vv-s-2)", columnGap: "var(--vv-s-2)" },
  vtCoverCopyP: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.bodySmallFontSize, color: color["--color-fg-2"] },
  hint2: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.bodySmallFontSize, color: color["--color-fg-2"], lineHeight: libraryVars.bodyLineHeight },
  fileInput: { position: "absolute", width: "var(--vv-s-1)", height: "var(--vv-s-1)", paddingTop: 0, paddingRight: 0, paddingBottom: 0, paddingLeft: 0, marginTop: -1, marginRight: -1, marginBottom: -1, marginLeft: -1, overflow: "hidden", clip: "rect(0 0 0 0)", whiteSpace: "nowrap", borderTopWidth: 0, borderRightWidth: 0, borderBottomWidth: 0, borderLeftWidth: 0 },
})
