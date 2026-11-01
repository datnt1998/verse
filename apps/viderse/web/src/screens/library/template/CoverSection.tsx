import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from '../libraryTokens.stylex'
// Mục Ảnh bìa (Template.dc.html › lib^cover): prompt ảnh bìa, vẽ thử (khoá —
// cần video đã làm xong), ảnh bìa mặc định (khoá — chưa có kho media), cách lấy
// ảnh bìa. Prompt có sẵn đứng trước prompt của bạn trong ô chọn.
import { RadioCard, Icon } from '@noverse/ui'
import { ActionButton } from '../../../components'
import { t } from '../../../i18n/library'
import type { ImagePrompt, Recipe } from '../../../api/library'
import { PromptPicker } from './PromptPicker'

export function CoverSection({ recipe, prompts, errors, onSave, hideTitle }: {
  recipe: Recipe
  prompts: ImagePrompt[]
  errors: Record<string, string>
  onSave: (patch: Partial<Recipe>) => void
  /** Ẩn h2 khi lồng trong màn con có tiêu đề riêng (MTemplate). */
  hideTitle?: boolean
}) {
  const copy = t.library.template.cover
  const scenes = recipe.scenes
  const setScenes = (next: Partial<Recipe['scenes']>) => onSave({ scenes: { ...scenes, ...next } })
  const ordered = [...prompts.filter((prompt) => prompt.builtIn), ...prompts.filter((prompt) => !prompt.builtIn)]

  return (
    <section className={`vt-sec ${stylex.props(sx.sec).className}`} aria-labelledby="vt-cover-title">
      <div className={`vt-sec-head ${stylex.props(sx.secHead).className}`}>
        {!hideTitle && <h2 {...stylex.props(sx.vtSecHeadH2)} id="vt-cover-title">{copy.title}</h2>}
        <p className={`vt-help ${stylex.props(sx.help).className}`}>{copy.help}</p>
      </div>

      <div className={`vt-card ${stylex.props(sx.card).className}`}>
        <div className={`vt-card-head ${stylex.props(sx.cardHead).className}`}>
          <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.promptTitle}</span>
          <span className={`vt-links ${stylex.props(sx.links).className}`}>
            <a className={`vt-link ${stylex.props(sx.link).className}`} href="#/library?tab=prompts">{copy.promptLibrary}</a>
          </span>
        </div>
        <PromptPicker
          prompts={ordered}
          value={scenes.coverPromptId}
          onSelect={(id) => setScenes({ coverPromptId: id })}
          usedLabel={copy.promptNote}
        />
        {errors['scenes.coverPromptId'] && <p className={`vt-fielderr ${stylex.props(sx.fielderr).className}`} role="alert">{errors['scenes.coverPromptId']}</p>}
      </div>

      <div className={`vt-card ${stylex.props(sx.card).className}`}>
        <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.sourceLabel}</span>
        <div role="radiogroup" aria-label={copy.sourceLabel} className={`vt-sources ${stylex.props(sx.sources).className}`}>
          <RadioCard xstyle={[sx.vtSourcesRadioCard]} checked={scenes.coverSource === 'ai'} onChange={() => setScenes({ coverSource: 'ai' })}
            label={copy.sourceAi} description={copy.sourceAiHelp} />
          <RadioCard xstyle={[sx.vtSourcesRadioCard]} checked={scenes.coverSource === 'upload'} onChange={() => setScenes({ coverSource: 'upload' })}
            label={copy.sourceUpload} description={copy.sourceUploadHelp} />
        </div>
      </div>

      <div className={`vt-card ${stylex.props(sx.card).className}`}>
        <div className={`vt-card-head ${stylex.props(sx.cardHead).className}`}>
          <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.trialTitle}</span>
          <span className={`vt-hint ${stylex.props(sx.hint).className}`}>{copy.trialKeep}</span>
        </div>
        <p className={`vt-hint ${stylex.props(sx.hint).className}`}>{copy.trialEmpty}</p>
        <div className={`vt-row ${stylex.props(sx.row).className}`}>
          <ActionButton xstyle={[sx.vtSourcesRadioCard]} variant="outline" icon={<Icon name="magicpen" size={16} />} lockedReason={copy.trialLocked}>{copy.trialDraw}</ActionButton>
          <p className={`vt-hint ${stylex.props(sx.hint).className}`}>{copy.trialHelp}</p>
        </div>
      </div>

      <div className={`vt-card ${stylex.props(sx.card).className}`}>
        <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.defaultTitle}</span>
        <div className={`vt-cover-default ${stylex.props(sx.coverDefault).className}`}>
          <span className={`vt-cover-thumb ${stylex.props(sx.coverThumb).className}`} aria-hidden="true" />
          <div className={`vt-cover-copy ${stylex.props(sx.coverCopy).className}`}>
            <p {...stylex.props(sx.vtCoverCopyP)}>{copy.defaultNone}</p>
            <p className={`vt-hint ${stylex.props(sx.hint2).className}`}>{copy.defaultDrop}</p>
          </div>
          <ActionButton xstyle={[sx.vtSourcesRadioCard]} variant="outline" lockedReason={copy.defaultLocked}>{copy.defaultChoose}</ActionButton>
        </div>
        <p className={`vt-hint ${stylex.props(sx.hint).className}`}>{copy.defaultHelp}</p>
        {errors['scenes.coverMediaId'] && <p className={`vt-fielderr ${stylex.props(sx.fielderr).className}`} role="alert">{errors['scenes.coverMediaId']}</p>}
      </div>
    </section>
  )
}

const sx = stylex.create({
  sec: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-12)", columnGap: "var(--vv-s-12)", minWidth: 0 },
  secHead: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-4)", columnGap: "var(--vv-s-4)" },
  vtSecHeadH2: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-16)", fontWeight: libraryVars.headingWeight },
  help: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.bodySmallFontSize, color: color["--color-fg-3"], lineHeight: libraryVars.helpLineHeight, maxWidth: libraryVars.templateBackdropSectionHelpMaxWidth },
  card: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", paddingTop: "var(--vv-s-14)", paddingRight: "var(--vv-s-14)", paddingBottom: "var(--vv-s-14)", paddingLeft: "var(--vv-s-14)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-12)", borderTopRightRadius: "var(--vv-r-12)", borderBottomRightRadius: "var(--vv-r-12)", borderBottomLeftRadius: "var(--vv-r-12)", backgroundColor: color["--color-card"], backgroundImage: "none", minWidth: 0 },
  cardHead: { display: "flex", alignItems: "baseline", justifyContent: "space-between", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", flexWrap: "wrap" },
  label: { fontSize: "var(--vv-s-14)", fontWeight: libraryVars.labelWeight, color: color["--color-fg"] },
  links: { display: "inline-flex", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)" },
  link: { color: color["--color-link"], fontSize: "var(--vv-s-12)", textDecoration: { default: "none", ":hover": "underline" }, minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  fielderr: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-12)", color: color["--color-err"] },
  sources: { display: "grid", gridTemplateColumns: libraryVars.templateCoverSectionSourcesGridTemplateColumns, rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)" },
  vtSourcesRadioCard: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  hint: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.captionFontSize, color: color["--color-fg-4"], lineHeight: libraryVars.bodyLineHeight },
  row: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", flexWrap: "wrap" },
  coverDefault: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-12)", columnGap: "var(--vv-s-12)", flexWrap: "wrap" },
  coverThumb: { flexGrow: 0, flexShrink: 0, flexBasis: "auto", width: libraryVars.templateCoverSectionCoverThumbWidth, aspectRatio: libraryVars.videoAspectRatio, borderTopLeftRadius: "var(--vv-r-7)", borderTopRightRadius: "var(--vv-r-7)", borderBottomRightRadius: "var(--vv-r-7)", borderBottomLeftRadius: "var(--vv-r-7)", backgroundColor: "var(--vv-tile-idle)", backgroundImage: "none" },
  coverCopy: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: libraryVars.templateCoverSectionCoverCopyFlexBasis, minWidth: 0, display: "flex", flexDirection: "column", rowGap: "var(--vv-s-2)", columnGap: "var(--vv-s-2)" },
  vtCoverCopyP: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.bodySmallFontSize, color: color["--color-fg-2"] },
  hint2: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.bodySmallFontSize, color: color["--color-fg-2"], lineHeight: libraryVars.bodyLineHeight },
})
