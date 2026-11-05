import * as stylex from '@stylexjs/stylex'
import { color } from '@noverse/ui/tokens.stylex'
import { libraryVars } from '../libraryTokens.stylex'
// Mục Ảnh cảnh (Template.dc.html › lib=images): cách ghép ảnh, prompt ảnh cảnh,
// số ảnh + giá, chuyển động, độ trong suốt, ảnh đối thủ. Mọi điều khiển lưu
// ngay qua `onSave` với khối `scenes` đầy đủ (PATCH thay cả recipe).
import { Icon, NumberStepper, RadioCard, Switch } from '@noverse/ui'
import { Segmented } from '../../../components/Segmented'
import { t } from '../../../i18n/library'
import { num, usd } from '../fmt'
import type { ImagePrompt, PriceEstimate, Recipe, ScenePlacement } from '../../../api/library'
import { SliderField } from './SliderField'
import { PromptPicker } from './PromptPicker'

export function ImagesSection({ recipe, price, prompts, errors, onSave, hideTitle }: {
  recipe: Recipe
  price: PriceEstimate
  prompts: ImagePrompt[]
  errors: Record<string, string>
  onSave: (patch: Partial<Recipe>) => void
  /** Ẩn h2 khi lồng trong màn con có tiêu đề riêng (MTemplate). */
  hideTitle?: boolean
}) {
  const copy = t.library.template.images
  const scenes = recipe.scenes
  const placement: ScenePlacement = scenes.style === 'full' ? 'full' : scenes.framing === 'close' ? 'close' : 'cutout'
  const setScenes = (next: Partial<Recipe['scenes']>) => onSave({ scenes: { ...scenes, ...next } })

  const countText = scenes.images.mode === 'none'
    ? copy.countHelpNone
    : scenes.images.mode === 'per-minutes'
      ? copy.countHelp(num(Math.ceil(42 / Math.max(1, scenes.images.minutes))), num(scenes.images.minutes))
      : copy.countHelp(num(scenes.images.count), num(Math.max(1, Math.round(42 / Math.max(1, scenes.images.count)))))
  const priceText = scenes.images.mode === 'none'
    ? copy.priceNone
    : scenes.images.mode === 'per-minutes'
      ? copy.pricePerMinute(num(scenes.images.minutes))
      : !price.known || price.totalUSD === null
        ? copy.priceUnknown(num(scenes.images.count))
        : copy.priceKnown(num(scenes.images.count), usd(price.totalUSD))

  return (
    <section className={`vt-sec ${stylex.props(sx.sec, hideTitle && sx.mobileSection).className}`} aria-label={hideTitle ? copy.title : undefined} aria-labelledby={hideTitle ? undefined : 'vt-images-title'}>
      <div className={`vt-sec-head ${stylex.props(sx.secHead, hideTitle && sx.mobileIntroHidden).className}`}>
        {!hideTitle && <h2 {...stylex.props(sx.vtSecHeadH2)} id="vt-images-title">{copy.title}</h2>}
        <p className={`vt-help ${stylex.props(sx.help).className}`}>{copy.help}</p>
      </div>

      <div className={`vt-card ${stylex.props(sx.card).className}`}>
        <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.placeTitle}</span>
        <div role="radiogroup" aria-label={copy.placeAria} className={`vt-places ${stylex.props(sx.places).className}`}>
          {(['full', 'cutout', 'close'] as const).map((place) => (
            <RadioCard xstyle={[sx.vtPlacesRadioCard, placement === place && sx.placeSelected]} labelStyle={sx.placeCopy} dotStyle={sx.placeDot} descriptionStyle={sx.placeDescription}
              key={place}
              checked={placement === place}
              onChange={() => setScenes({ style: place === 'full' ? 'full' : 'cutout', framing: place === 'close' ? 'close' : '' })}
              label={<span className={`vt-place ${stylex.props(sx.place).className}`} data-place={place}>
                <span className={`vt-place-art ${stylex.props(sx.placeArt).className}`} aria-hidden="true">
                  <img src={`/illustrations/scene-composition-${place}.jpg`} alt="" {...stylex.props(sx.placeImage)} />
                  {placement === place && <span {...stylex.props(sx.placeTick)}><Icon name="tick" size={12} /></span>}
                </span>
                <span {...stylex.props(sx.placeName)} title={copy.places[place]}>{copy.places[place]}{placement === place && <Icon name="tick" size={14} xstyle={sx.desktopPlaceTick} />}</span>
              </span>}
              description={copy.placeHelp[place]}
            />
          ))}
        </div>
        <p className={`vt-hint ${stylex.props(sx.hint).className}`}><span {...stylex.props(sx.desktopHint)}>{copy.placeHelpTail}</span><span {...stylex.props(sx.mobileHint)}>{copy.placeHelp[placement]}</span></p>
        {errors['scenes.style'] && <p className={`vt-fielderr ${stylex.props(sx.fielderr).className}`} role="alert">{errors['scenes.style']}</p>}
        {errors['scenes.framing'] && <p className={`vt-fielderr ${stylex.props(sx.fielderr).className}`} role="alert">{errors['scenes.framing']}</p>}
      </div>

      <div className={`vt-card ${stylex.props(sx.card).className}`}>
        <div className={`vt-card-head ${stylex.props(sx.cardHead).className}`}>
          <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.promptTitle}</span>
          <span className={`vt-links ${stylex.props(sx.links).className}`}>
            <a className={`vt-link ${stylex.props(sx.link).className}`} href="#/library?tab=prompts">{copy.promptLibrary}</a>
          </span>
        </div>
        <PromptPicker
          kind="scene"
          mobileLayout={hideTitle}
          prompts={prompts}
          value={scenes.scenePromptId}
          onSelect={(id) => setScenes({ scenePromptId: id })}
          usedLabel={copy.promptNote}
        />
        {errors['scenes.scenePromptId'] && <p className={`vt-fielderr ${stylex.props(sx.fielderr).className}`} role="alert">{errors['scenes.scenePromptId']}</p>}
      </div>

      <div className={`vt-card ${stylex.props(sx.card).className}`}>
        <span className={`vt-label ${stylex.props(sx.label).className}`} id="vt-count">{copy.countTitle}</span>
        <Segmented buttonStyle={sx.touchTarget}
          label={copy.countAria}
         
          value={scenes.images.mode}
          onChange={(mode) => setScenes({ images: { ...scenes.images, mode: mode as Recipe['scenes']['images']['mode'] } })}
          options={[
            { id: 'fixed', label: copy.countFixed },
            { id: 'per-minutes', label: copy.countPer },
            { id: 'none', label: copy.countNone },
          ]}
        />
        {scenes.images.mode !== 'none' && (
          <div className={`vt-row ${stylex.props(sx.row).className}`}>
            <span className={`vt-row-note ${stylex.props(sx.rowNote).className}`}>{scenes.images.mode === 'fixed' ? copy.countPreFixed : copy.countPrePer}</span>
            <NumberStepper buttonStyle={sx.touchTarget} inputStyle={sx.touchTarget}
              aria-label={copy.countTitle}
              value={scenes.images.mode === 'fixed' ? scenes.images.count : scenes.images.minutes}
              min={1}
              max={scenes.images.mode === 'fixed' ? 40 : 20}
              onChange={(value) => setScenes({
                images: scenes.images.mode === 'fixed'
                  ? { ...scenes.images, count: value }
                  : { ...scenes.images, minutes: value },
              })}
            />
            <span className={`vt-row-note ${stylex.props(sx.rowNote).className}`}>{scenes.images.mode === 'fixed' ? copy.countPostFixed : copy.countPostPer}</span>
          </div>
        )}
        <p className={`vt-help vt-help--small ${stylex.props(sx.help).className}`}>{countText}</p>
        <p className={`vt-help vt-help--small ${stylex.props(sx.help).className}`}>{priceText}</p>
        {errors['scenes.images.count'] && <p className={`vt-fielderr ${stylex.props(sx.fielderr).className}`} role="alert">{errors['scenes.images.count']}</p>}
        {errors['scenes.images.minutes'] && <p className={`vt-fielderr ${stylex.props(sx.fielderr).className}`} role="alert">{errors['scenes.images.minutes']}</p>}
      </div>

      <div className={`vt-card ${stylex.props(sx.card, hideTitle && sx.mobileSliderInsets).className}`}>
        <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.motionTitle}</span>
        <Segmented buttonStyle={sx.touchTarget}
          label={copy.motionAria}
         
          value={scenes.motion.kind}
          onChange={(kind) => setScenes({ motion: { ...scenes.motion, kind: kind as Recipe['scenes']['motion']['kind'] } })}
          options={[
            { id: 'ken-burns', label: copy.motion['ken-burns'] },
            { id: 'wander', label: copy.motion.wander },
          ]}
        />
        <SliderField
          label={copy.intensity}
          value={scenes.motion.intensity}
          min={0}
          max={1}
          step={0.05}
          formatValue={(value) => `${Math.round(value * 100)}%`}
          onChange={(value) => setScenes({ motion: { ...scenes.motion, intensity: value } })}
        />
        <SliderField
          label={copy.speed}
          value={scenes.motion.speed}
          min={0.5}
          max={2}
          step={0.05}
          formatValue={(value) => `${Number(value.toFixed(2))}×`}
          onChange={(value) => setScenes({ motion: { ...scenes.motion, speed: value } })}
        />
        {errors['scenes.motion'] && <p className={`vt-fielderr ${stylex.props(sx.fielderr).className}`} role="alert">{errors['scenes.motion']}</p>}
      </div>

      <div className={`vt-card ${stylex.props(sx.card, hideTitle && sx.mobileSliderInsets).className}`}>
        <SliderField
          label={copy.opacityTitle}
          value={scenes.sceneOpacity}
          min={0.1}
          max={1}
          step={0.05}
          formatValue={(value) => (value >= 0.999 ? copy.opacityFull : copy.opacityPct(String(Math.round(value * 100))))}
          onChange={(value) => setScenes({ sceneOpacity: value })}
        />
        <p className={`vt-help vt-help--small ${stylex.props(sx.help).className}`}>{copy.opacityHelp}</p>
        {errors['scenes.sceneOpacity'] && <p className={`vt-fielderr ${stylex.props(sx.fielderr).className}`} role="alert">{errors['scenes.sceneOpacity']}</p>}
      </div>

      <div className={`vt-card ${stylex.props(sx.card).className}`}>
        <div className={`vt-switch-row ${stylex.props(sx.switchRow).className}`}>
          <div>
            <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.referenceTitle}</span>
            <p className={`vt-help vt-help--small ${stylex.props(sx.help).className}`}>{copy.referenceHelp}</p>
          </div>
          <Switch xstyle={[sx.touchTarget]} label={copy.referenceTitle} checked={scenes.referenceImages}
            onChange={(on) => setScenes({ referenceImages: on })} />
        </div>
      </div>
    </section>
  )
}

const sx = stylex.create({
  mobileSection: { rowGap: { default: null, '@media (max-width: 767px)': 'var(--vv-s-18)' } },
  mobileIntroHidden: { display: { default: null, '@media (max-width: 767px)': 'none' } },
  mobileSliderInsets: {
    paddingLeft: { default: null, '@media (max-width: 767px)': 'var(--vv-s-14)' },
    paddingRight: { default: null, '@media (max-width: 767px)': 'var(--vv-s-14)' },
  },
  placeCopy: { width: '100%' },
  placeDot: { display: 'none' },
  placeDescription: { display: 'none' },
  placeSelected: { borderColor: color['--color-accent'], backgroundColor: 'var(--vv-accent-6)' },
  placeName: {
    display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 'var(--vv-s-5)',
    padding: { default: '0 var(--vv-s-3)', '@media (max-width: 767px)': 'var(--vv-s-7) var(--vv-s-9) var(--vv-s-8)' },
    overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
  },
  placeTick: {
    display: { default: 'none', '@media (max-width: 767px)': 'flex' },
    position: 'absolute', top: 'var(--vv-s-6)', right: 'var(--vv-s-6)',
    width: 'var(--vv-s-20)', height: 'var(--vv-s-20)', borderRadius: '50%',
    alignItems: 'center', justifyContent: 'center',
    backgroundColor: color['--color-accent'], color: color['--color-accent-fg'],
  },
  desktopPlaceTick: { display: { default: 'inline-flex', '@media (max-width: 767px)': 'none' }, color: color['--color-accent'], flexShrink: 0 },
  desktopHint: { display: { default: null, '@media (max-width: 767px)': 'none' } },
  mobileHint: { display: { default: 'none', '@media (max-width: 767px)': 'inline' } },
  sec: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-24)", minWidth: 0 },
  secHead: { display: "flex", flexDirection: 'column', alignItems: 'flex-start', rowGap: "var(--vv-s-4)", paddingBottom: 'var(--vv-s-6)' },
  vtSecHeadH2: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-18)", fontWeight: libraryVars.headingWeight },
  help: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.bodySmallFontSize, color: color["--color-fg-3"], lineHeight: libraryVars.helpLineHeight, maxWidth: libraryVars.templateBackdropSectionHelpMaxWidth },
  card: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-10)", minWidth: 0 },
  label: {
    fontSize: { default: 'var(--vv-s-14)', '@media (max-width: 767px)': 'var(--vv-s-12)' },
    fontWeight: { default: libraryVars.labelWeight, '@media (max-width: 767px)': 600 },
    color: { default: color['--color-fg'], '@media (max-width: 767px)': 'var(--kit-fg-sub)' },
    letterSpacing: { default: null, '@media (max-width: 767px)': '.06em' },
    textTransform: { default: null, '@media (max-width: 767px)': 'uppercase' },
  },
  places: { display: "grid", gridTemplateColumns: libraryVars.templateImagesSectionPlacesGridTemplateColumns, rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)" },
  vtPlacesRadioCard: {
    minWidth: 0,
    padding: { default: 'var(--vv-s-5) var(--vv-s-5) var(--vv-s-8)', '@media (max-width: 767px)': 0 },
    overflow: 'hidden',
    borderRadius: 'var(--vv-r-12)',
    borderWidth: 'calc(var(--vv-s-3) / 2)',
    backgroundColor: color['--color-surface-1'],
    minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" },
  },
  place: { display: "flex", flexDirection: "column", gap: { default: 'var(--vv-s-6)', '@media (max-width: 767px)': 0 }, fontSize: libraryVars.bodySmallFontSize },
  placeArt: { position: 'relative', display: "block", overflow: 'hidden', aspectRatio: { default: '16 / 9', '@media (max-width: 767px)': '4 / 5' }, borderRadius: { default: 'var(--vv-r-6)', '@media (max-width: 767px)': 0 }, backgroundColor: "var(--vv-tile-idle)" },
  placeImage: { display: 'block', width: '100%', height: '100%', objectFit: 'cover', position: 'absolute', inset: 0 },
  hint: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.captionFontSize, color: color["--color-fg-4"], lineHeight: libraryVars.bodyLineHeight },
  fielderr: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-12)", color: color["--color-err"] },
  cardHead: { display: "flex", alignItems: "baseline", justifyContent: "space-between", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", flexWrap: "wrap" },
  links: { display: "inline-flex", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)" },
  link: { color: color["--color-link"], fontSize: "var(--vv-s-12)", textDecoration: { default: "none", ":hover": "underline" }, minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  touchTarget: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  row: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", flexWrap: "wrap" },
  rowNote: { fontSize: libraryVars.bodySmallFontSize, color: color["--color-fg-3"] },
  switchRow: { display: "flex", alignItems: "center", justifyContent: "space-between", rowGap: "var(--vv-s-12)", columnGap: "var(--vv-s-12)", flexWrap: "wrap" },
})
