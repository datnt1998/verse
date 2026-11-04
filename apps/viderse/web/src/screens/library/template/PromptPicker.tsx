import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from '../libraryTokens.stylex'
// Chọn prompt trong trang template: lưới thẻ bấm-chọn (ảnh cảnh / ảnh bìa) thay
// ô chọn thả xuống, đúng artboard Template; kèm thẻ "Prompt mới" dẫn về kho và
// khối chi tiết của prompt đang chọn.
import { useState } from 'react'
import { Button, Icon } from '@noverse/ui'
import { ImageViewer } from '../../../components/image-viewer/ImageViewer'
import { sx as viewerStyles } from '../../../components/image-viewer/image-viewer.stylex'
import { useMediaQuery } from '../../../lib/useMediaQuery'
import { paths } from '../../../router'
import { t } from '../../../i18n/library'
import type { ImagePrompt } from '../../../api/library'

/** Một ô prompt: vẽ ảnh mẫu thật khi có; không có hoặc tải hỏng thì giữ ô trung tính. */
function PromptPick({ prompt, selected, mobileLayout, onSelect, onPreview }: {
  prompt: ImagePrompt
  selected: boolean
  mobileLayout: boolean
  onSelect: (id: string) => void
  onPreview?: () => void
}) {
  const copy = t.library.template
  // Khoá theo URL: ảnh mẫu mới thay ảnh hỏng thì vẫn hiện.
  const [failedUrl, setFailedUrl] = useState<string | null>(null)
  const sampleUrl = prompt.sampleUrls?.[0]
  const showArt = sampleUrl !== undefined && sampleUrl !== '' && sampleUrl !== failedUrl
  return (
    <div {...stylex.props(sx.pickWrap)}><button
      type="button"
      role="radio"
      aria-checked={selected}
      aria-label={prompt.name}
      className={`vt-pick ${stylex.props(sx.pick, sx.pickFill, mobileLayout && sx.mobilePick, (selected === true) && sx.pickAriaCheckedTrue).className}`}
      data-kind={prompt.kind}
      onClick={() => onSelect(prompt.id)}
    >
      <span className={`vt-pick-art ${stylex.props(sx.pickArt, mobileLayout && sx.mobileArt, ((prompt.kind) === "cover") && sx.pickArtDataKindCover).className}`} data-kind={prompt.kind} aria-hidden="true">
        {showArt && (
          <img className={`vt-pick-img ${stylex.props(sx.pickImg).className}`} src={sampleUrl} alt="" loading="lazy"
            onError={() => setFailedUrl(sampleUrl ?? null)} />
        )}
        {!showArt && <span className={`vt-pick-glyph ${stylex.props(sx.pickGlyph, mobileLayout && sx.mobileHidden).className}`} />}
        {prompt.builtIn && <span className={`vt-pick-chip ${stylex.props(sx.pickChip, mobileLayout && sx.mobileHidden).className}`}>{t.library.prompts.builtChip}</span>}
      </span>
      <span className={`vt-pick-name ${stylex.props(sx.pickName, mobileLayout && sx.mobileName).className}`} title={prompt.name}>{prompt.name}</span>
      {selected && <span className={`vt-pick-check ${stylex.props(sx.pickCheck, mobileLayout && sx.mobileCheck).className}`} aria-hidden="true"><Icon name="tick" size={12} /></span>}
    </button>
      {onPreview && <Button variant="icon" size="sm" aria-label={`Xem lớn ${prompt.name}`} title={showArt ? 'Xem lớn' : 'Chưa có ảnh mẫu cho prompt này'} tooltip={showArt ? 'Xem lớn' : 'Chưa có ảnh mẫu cho prompt này'} disabled={!showArt} onClick={onPreview} xstyle={sx.previewButton}><Icon name="eye" size={14} /></Button>}
    </div>
  )
}

export function PromptPicker({ kind, prompts, value, onSelect, usedLabel, mobileLayout = false }: {
  kind: 'scene' | 'cover'
  prompts: ImagePrompt[]
  value: string
  onSelect: (id: string) => void
  usedLabel: (count: number) => string
  mobileLayout?: boolean
}) {
  const copy = t.library.template[kind === 'cover' ? 'cover' : 'images']
  const selected = prompts.find((prompt) => prompt.id === value)
  const phone = useMediaQuery('(max-width: 767px)')
  const [previewId, setPreviewId] = useState<string | null>(null)
  const preview = prompts.find(prompt => prompt.id === previewId)
  const previewItems = prompts.filter(prompt => prompt.kind === preview?.kind).map(prompt => ({ id: prompt.id, label: prompt.name, url: prompt.sampleUrls?.[0], disabled: !prompt.sampleUrls?.[0] }))
  return (
    <>
      <div className={`vt-picks ${stylex.props(sx.picks).className}`} role="radiogroup" aria-label={copy.promptAria}>
        {prompts.map((prompt) => (
          <PromptPick key={prompt.id} prompt={prompt} selected={prompt.id === value} mobileLayout={mobileLayout} onSelect={onSelect} onPreview={phone ? undefined : () => setPreviewId(prompt.id)} />
        ))}
        <a className={`vt-pick vt-pick--new ${stylex.props(sx.pickPickNew, mobileLayout && sx.mobileHidden).className}`} href="#/library?tab=prompts&prompt=new">
          <span className={`vt-pick-plus ${stylex.props(sx.pickPlus).className}`} aria-hidden="true"><Icon name="add" size={16} /></span>
          <span className={`vt-pick-name ${stylex.props(sx.pickName).className}`}>{copy.promptNew}</span>
          <span className={`vt-hint ${stylex.props(sx.hint).className}`}>{copy.promptNewHint}</span>
        </a>
      </div>
      {selected && (
        <div className={`vt-pick-detail ${stylex.props(sx.pickDetail).className}`}>
          <div className={`vt-pick-detail-head ${stylex.props(sx.pickDetailHead).className}`}>
            <strong>{selected.name}</strong>
            {selected.composition && <span className={`vt-chip ${stylex.props(sx.chip).className}`}>{t.library.prompts.composition[selected.composition]}</span>}
            <a className={`vt-link ${stylex.props(sx.link).className}`} href={`#/library?tab=prompts&prompt=${selected.builtIn ? 'builtin' : 'mine'}`}>
              {copy.promptEdit} <span aria-hidden="true">›</span>
            </a>
          </div>
          <p className={`vt-pick-text ${stylex.props(sx.pickText).className}`}>{selected.prompt}</p>
          <p className={`vt-hint ${stylex.props(sx.hint).className}`}>{usedLabel(selected.usedBy.length)}</p>
        </div>
      )}
      {!phone && preview && <ImageViewer mode="sample" title={preview.name} items={previewItems} selectedId={preview.id} onSelect={setPreviewId} onClose={() => setPreviewId(null)}
        subtitle={`${preview.kind === 'scene' ? 'Prompt ảnh cảnh' : 'Prompt ảnh bìa'} · ảnh mẫu vẽ bằng prompt này`}
        side={<><strong {...stylex.props(viewerStyles.sideTitle)}>{preview.name}</strong><h3 {...stylex.props(viewerStyles.caption)}>Prompt</h3><p>{preview.prompt}</p><h3 {...stylex.props(viewerStyles.caption)}>Đang dùng ở</h3>{preview.usedBy.length ? preview.usedBy.map(template => <a key={template.id} href={`#${paths.template(template.id)}`} {...stylex.props(viewerStyles.sideLink)}>{template.name}</a>) : <p>Chưa có template nào dùng prompt này.</p>}<a href={`#${paths.library('prompts', preview.builtIn ? 'builtin' : 'mine')}`} {...stylex.props(viewerStyles.sideLink)}>Mở trong kho prompt</a></>}
        footer={<><span>← → xem mẫu khác · Esc đóng · đổi prompt không vẽ lại video đã xong</span>{preview.id === value ? <span>Template đang dùng</span> : <Button variant="primary" onClick={() => { onSelect(preview.id); setPreviewId(null) }}>Dùng prompt này</Button>}</>}
      />}
    </>
  )
}

const sx = stylex.create({
  pickWrap: { position: 'relative', minWidth: 0 },
  pickFill: { width: '100%' },
  previewButton: { position: 'absolute', left: 'var(--vv-s-10)', top: 'var(--vv-s-10)', backgroundColor: 'var(--vv-thumb-veil)', color: color['--color-fg'] },
  mobilePick: {
    paddingTop: { default: null, '@media (max-width: 767px)': 0 },
    paddingRight: { default: null, '@media (max-width: 767px)': 0 },
    paddingBottom: { default: null, '@media (max-width: 767px)': 0 },
    paddingLeft: { default: null, '@media (max-width: 767px)': 0 },
    rowGap: { default: null, '@media (max-width: 767px)': 0 },
    overflow: { default: null, '@media (max-width: 767px)': 'hidden' },
    borderTopLeftRadius: { default: null, '@media (max-width: 767px)': 'var(--vv-r-12)' },
    borderTopRightRadius: { default: null, '@media (max-width: 767px)': 'var(--vv-r-12)' },
    borderBottomLeftRadius: { default: null, '@media (max-width: 767px)': 'var(--vv-r-12)' },
    borderBottomRightRadius: { default: null, '@media (max-width: 767px)': 'var(--vv-r-12)' },
    borderTopWidth: { default: null, '@media (max-width: 767px)': 'calc(var(--vv-s-3) / 2)' },
    borderRightWidth: { default: null, '@media (max-width: 767px)': 'calc(var(--vv-s-3) / 2)' },
    borderBottomWidth: { default: null, '@media (max-width: 767px)': 'calc(var(--vv-s-3) / 2)' },
    borderLeftWidth: { default: null, '@media (max-width: 767px)': 'calc(var(--vv-s-3) / 2)' },
    backgroundColor: { default: null, '@media (max-width: 767px)': color['--color-surface-1'] },
  },
  mobileArt: {
    height: { default: null, '@media (max-width: 767px)': 'auto' },
    aspectRatio: { default: null, '@media (max-width: 767px)': '16 / 9' },
    borderTopLeftRadius: { default: null, '@media (max-width: 767px)': 0 },
    borderTopRightRadius: { default: null, '@media (max-width: 767px)': 0 },
    borderBottomLeftRadius: { default: null, '@media (max-width: 767px)': 0 },
    borderBottomRightRadius: { default: null, '@media (max-width: 767px)': 0 },
  },
  mobileHidden: { display: { default: null, '@media (max-width: 767px)': 'none' } },
  mobileName: {
    padding: { default: null, '@media (max-width: 767px)': 'var(--vv-s-7) var(--vv-s-9) var(--vv-s-8)' },
    fontSize: { default: null, '@media (max-width: 767px)': 'var(--vv-s-13)' },
  },
  mobileCheck: {
    width: { default: null, '@media (max-width: 767px)': 'var(--vv-s-20)' },
    height: { default: null, '@media (max-width: 767px)': 'var(--vv-s-20)' },
    top: { default: null, '@media (max-width: 767px)': 'var(--vv-s-6)' },
    right: { default: null, '@media (max-width: 767px)': 'var(--vv-s-6)' },
  },
  picks: { display: "grid", gridTemplateColumns: { default: libraryVars.templatePromptPickerPicksGridTemplateColumns, "@media (max-width: 1099px)": libraryVars.promptEditorSamplesGridTemplateColumns }, rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)" },
  pick: { position: "relative", display: "flex", flexDirection: "column", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", paddingTop: "var(--vv-s-6)", paddingRight: "var(--vv-s-6)", paddingBottom: "var(--vv-s-6)", paddingLeft: "var(--vv-s-6)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: { default: line["--line-3"], ":hover": line["--line-4"] }, borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: { default: line["--line-3"], ":hover": line["--line-4"] }, borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: { default: line["--line-3"], ":hover": line["--line-4"] }, borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: { default: line["--line-3"], ":hover": line["--line-4"] }, borderTopLeftRadius: "var(--vv-r-10)", borderTopRightRadius: "var(--vv-r-10)", borderBottomRightRadius: "var(--vv-r-10)", borderBottomLeftRadius: "var(--vv-r-10)", backgroundColor: color["--color-surface-1"], backgroundImage: "none", color: color["--color-fg-2"], fontFamily: "inherit", fontSize: "var(--vv-s-12)", fontStyle: "inherit", fontWeight: "inherit", fontStretch: "inherit", fontVariant: "inherit", lineHeight: "inherit", textAlign: "left", cursor: "pointer", minWidth: 0, minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" }, outlineWidth: { default: null, ":focus-visible": "var(--vv-s-2)" }, outlineStyle: { default: null, ":focus-visible": "solid" }, outlineColor: { default: null, ":focus-visible": color["--color-accent"] }, outlineOffset: { default: null, ":focus-visible": "var(--vv-s-1)" } },
  pickAriaCheckedTrue: { borderTopColor: color["--color-accent"], borderRightColor: color["--color-accent"], borderBottomColor: color["--color-accent"], borderLeftColor: color["--color-accent"], color: color["--color-fg"] },
  pickArt: { position: "relative", display: "block", height: libraryVars.templatePromptPickerPickArtHeight, borderTopLeftRadius: "var(--vv-r-6)", borderTopRightRadius: "var(--vv-r-6)", borderBottomRightRadius: "var(--vv-r-6)", borderBottomLeftRadius: "var(--vv-r-6)", backgroundColor: "transparent", backgroundImage: libraryVars.templatePromptPickerPickArtBackgroundImage, overflowX: "hidden", overflowY: "hidden" },
  pickArtDataKindCover: { backgroundColor: "transparent", backgroundImage: libraryVars.templatePromptPickerPickArtDataKindCoverBackgroundImage },
  pickImg: { position: "absolute", top: 0, right: 0, bottom: 0, left: 0, width: "100%", height: "100%", objectFit: "cover", display: "block" },
  pickGlyph: { position: "absolute", left: "50%", bottom: "var(--vv-s-6)", width: "var(--vv-s-14)", height: "var(--vv-s-26)", transform: libraryVars.templatePromptPickerPickGlyphTransform, borderTopLeftRadius: libraryVars.templatePromptPickerPickGlyphBorderTopLeftRadius, borderTopRightRadius: libraryVars.templatePromptPickerPickGlyphBorderTopLeftRadius, borderBottomRightRadius: libraryVars.templatePreviewPanePreviewArtTop, borderBottomLeftRadius: libraryVars.templatePreviewPanePreviewArtTop, backgroundColor: color["--color-surface-6"], backgroundImage: "none", opacity: libraryVars.templatePromptPickerPickGlyphOpacity },
  pickChip: { position: "absolute", top: "var(--vv-s-4)", left: "var(--vv-s-4)", paddingTop: 0, paddingRight: "var(--vv-s-5)", paddingBottom: 0, paddingLeft: "var(--vv-s-5)", borderTopLeftRadius: "var(--vv-r-pill)", borderTopRightRadius: "var(--vv-r-pill)", borderBottomRightRadius: "var(--vv-r-pill)", borderBottomLeftRadius: "var(--vv-r-pill)", backgroundColor: color["--color-surface-6"], backgroundImage: "none", color: color["--color-fg-3"], fontSize: libraryVars.badgeFontSize },
  pickName: { overflowX: "hidden", overflowY: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" },
  pickCheck: { position: "absolute", top: "var(--vv-s-4)", right: "var(--vv-s-4)", display: "inline-flex", alignItems: "center", justifyContent: "center", width: "var(--vv-s-16)", height: "var(--vv-s-16)", borderTopLeftRadius: "50%", borderTopRightRadius: "50%", borderBottomRightRadius: "50%", borderBottomLeftRadius: "50%", backgroundColor: color["--color-accent"], backgroundImage: "none", color: color["--color-accent-fg"] },
  pickPickNew: { position: "relative", display: "flex", flexDirection: "column", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", paddingTop: "var(--vv-s-6)", paddingRight: "var(--vv-s-6)", paddingBottom: "var(--vv-s-6)", paddingLeft: "var(--vv-s-6)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "dashed", borderTopColor: { default: line["--line-3"], ":hover": line["--line-4"] }, borderRightWidth: "var(--vv-s-1)", borderRightStyle: "dashed", borderRightColor: { default: line["--line-3"], ":hover": line["--line-4"] }, borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "dashed", borderBottomColor: { default: line["--line-3"], ":hover": line["--line-4"] }, borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "dashed", borderLeftColor: { default: line["--line-3"], ":hover": line["--line-4"] }, borderTopLeftRadius: "var(--vv-r-10)", borderTopRightRadius: "var(--vv-r-10)", borderBottomRightRadius: "var(--vv-r-10)", borderBottomLeftRadius: "var(--vv-r-10)", backgroundColor: color["--color-surface-1"], backgroundImage: "none", color: color["--color-fg-3"], fontFamily: "inherit", fontSize: "var(--vv-s-12)", fontStyle: "inherit", fontWeight: "inherit", fontStretch: "inherit", fontVariant: "inherit", lineHeight: "inherit", textAlign: "center", cursor: "pointer", minWidth: 0, alignItems: "center", justifyContent: "center", minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": libraryVars.templatePromptPickerPickMinHeight }, outlineWidth: { default: null, ":focus-visible": "var(--vv-s-2)" }, outlineStyle: { default: null, ":focus-visible": "solid" }, outlineColor: { default: null, ":focus-visible": color["--color-accent"] }, outlineOffset: { default: null, ":focus-visible": "var(--vv-s-1)" } },
  pickPlus: { display: "inline-flex" },
  hint: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.captionFontSize, color: color["--color-fg-4"], lineHeight: libraryVars.bodyLineHeight },
  pickDetail: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", paddingTop: "var(--vv-s-10)", paddingRight: "var(--vv-s-10)", paddingBottom: "var(--vv-s-10)", paddingLeft: "var(--vv-s-10)", marginTop: "var(--vv-s-8)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-10)", borderTopRightRadius: "var(--vv-r-10)", borderBottomRightRadius: "var(--vv-r-10)", borderBottomLeftRadius: "var(--vv-r-10)", backgroundColor: color["--color-surface-1"], backgroundImage: "none" },
  pickDetailHead: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)", fontSize: libraryVars.bodySmallFontSize },
  chip: { paddingTop: "var(--vv-s-2)", paddingRight: "var(--vv-s-7)", paddingBottom: "var(--vv-s-2)", paddingLeft: "var(--vv-s-7)", borderTopLeftRadius: "var(--vv-r-pill)", borderTopRightRadius: "var(--vv-r-pill)", borderBottomRightRadius: "var(--vv-r-pill)", borderBottomLeftRadius: "var(--vv-r-pill)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], fontSize: libraryVars.captionFontSize, color: color["--color-fg-3"], whiteSpace: "nowrap" },
  link: { color: color["--color-link"], fontSize: "var(--vv-s-12)", textDecoration: { default: "none", ":hover": "underline" }, marginLeft: "auto", minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  pickText: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-12)", color: color["--color-fg-3"], lineHeight: libraryVars.bodyLineHeight, display: "-webkit-box", WebkitLineClamp: libraryVars.templatePromptPickerPickTextWebkitLineClamp, WebkitBoxOrient: "vertical", overflowX: "hidden", overflowY: "hidden" },
})
