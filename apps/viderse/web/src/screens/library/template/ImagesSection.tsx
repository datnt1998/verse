import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from '../libraryTokens.stylex'
// Mục Ảnh cảnh (Template.dc.html › lib=images): cách ghép ảnh, prompt ảnh cảnh,
// số ảnh + giá, chuyển động, độ trong suốt, ảnh đối thủ. Mọi điều khiển lưu
// ngay qua `onSave` với khối `scenes` đầy đủ (PATCH thay cả recipe).
import { NumberStepper, RadioCard, Switch } from '@noverse/ui'
import { Segmented } from '../../../components/Segmented'
import { t } from '../../../i18n/vi'
import { num, usd } from '../fmt'
import type { ImagePrompt, PriceEstimate, Recipe, ScenePlacement } from '../../../api/library'
import { SliderField } from './SliderField'
import { PromptPicker } from './PromptPicker'

export function ImagesSection({ recipe, price, prompts, errors, onSave }: {
  recipe: Recipe
  price: PriceEstimate
  prompts: ImagePrompt[]
  errors: Record<string, string>
  onSave: (patch: Partial<Recipe>) => void
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
    <section className={`vt-sec ${stylex.props(sx.sec).className}`} aria-labelledby="vt-images-title">
      <div className={`vt-sec-head ${stylex.props(sx.secHead).className}`}>
        <h2 {...stylex.props(sx.vtSecHeadH2)} id="vt-images-title">{copy.title}</h2>
        <p className={`vt-help ${stylex.props(sx.help).className}`}>{copy.help}</p>
      </div>

      <div className={`vt-card ${stylex.props(sx.card).className}`}>
        <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.placeTitle}</span>
        <div role="radiogroup" aria-label={copy.placeAria} className={`vt-places ${stylex.props(sx.places).className}`}>
          {(['full', 'cutout', 'close'] as const).map((place) => (
            <RadioCard xstyle={[sx.vtPlacesRadioCard]}
              key={place}
              checked={placement === place}
              onChange={() => setScenes({ style: place === 'full' ? 'full' : 'cutout', framing: place === 'close' ? 'close' : '' })}
              label={<span className={`vt-place ${stylex.props(sx.place).className}`} data-place={place}><span className={`vt-place-art ${stylex.props(sx.placeArt, ((place) === "cutout") && sx.placeArtDataPlaceCutout, ((place) === "close") && sx.placeArtDataPlaceClose).className}`} aria-hidden="true" /><span>{copy.places[place]}</span></span>}
              description={copy.placeHelp[place]}
            />
          ))}
        </div>
        <p className={`vt-hint ${stylex.props(sx.hint).className}`}>{copy.placeHelpTail}</p>
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

      <div className={`vt-card ${stylex.props(sx.card).className}`}>
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

      <div className={`vt-card ${stylex.props(sx.card).className}`}>
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
          <Switch xstyle={[sx.vtPlacesRadioCard]} label={copy.referenceTitle} checked={scenes.referenceImages}
            onChange={(on) => setScenes({ referenceImages: on })} />
        </div>
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
  label: { fontSize: "var(--vv-s-14)", fontWeight: libraryVars.labelWeight, color: color["--color-fg"] },
  places: { display: "grid", gridTemplateColumns: { default: libraryVars.templateImagesSectionPlacesGridTemplateColumns, "@media (max-width: 767px)": libraryVars.overlaysTabGridGridOverlaysGridTemplateColumns3 }, rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)" },
  vtPlacesRadioCard: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  place: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", fontSize: libraryVars.bodySmallFontSize },
  placeArt: { display: "block", height: libraryVars.templateCoverSectionCoverThumbWidth, borderTopLeftRadius: "var(--vv-r-6)", borderTopRightRadius: "var(--vv-r-6)", borderBottomRightRadius: "var(--vv-r-6)", borderBottomLeftRadius: "var(--vv-r-6)", backgroundColor: "var(--vv-tile-idle)", backgroundImage: "none" },
  placeArtDataPlaceCutout: { backgroundColor: "transparent", backgroundImage: libraryVars.templateImagesSectionPlaceArtDataPlaceCutoutBackgroundImage },
  placeArtDataPlaceClose: { backgroundColor: "transparent", backgroundImage: libraryVars.templateImagesSectionPlaceArtDataPlaceCloseBackgroundImage },
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
