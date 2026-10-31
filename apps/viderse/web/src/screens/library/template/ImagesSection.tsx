// Mục Ảnh cảnh (Template.dc.html › lib=images): cách ghép ảnh, prompt ảnh cảnh,
// số ảnh + giá, chuyển động, độ trong suốt, ảnh đối thủ. Mọi điều khiển lưu
// ngay qua `onSave` với khối `scenes` đầy đủ (PATCH thay cả recipe).
import { NumberStepper, RadioCard, Segmented, Switch } from '@noverse/ui'
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
    <section className="vt-sec" aria-labelledby="vt-images-title">
      <div className="vt-sec-head">
        <h2 id="vt-images-title">{copy.title}</h2>
        <p className="vt-help">{copy.help}</p>
      </div>

      <div className="vt-card">
        <span className="vt-label">{copy.placeTitle}</span>
        <div role="radiogroup" aria-label={copy.placeAria} className="vt-places">
          {(['full', 'cutout', 'close'] as const).map((place) => (
            <RadioCard
              key={place}
              checked={placement === place}
              onChange={() => setScenes({ style: place === 'full' ? 'full' : 'cutout', framing: place === 'close' ? 'close' : '' })}
              label={<span className="vt-place" data-place={place}><span className="vt-place-art" aria-hidden="true" /><span>{copy.places[place]}</span></span>}
              description={copy.placeHelp[place]}
            />
          ))}
        </div>
        <p className="vt-hint">{copy.placeHelpTail}</p>
        {errors['scenes.style'] && <p className="vt-fielderr" role="alert">{errors['scenes.style']}</p>}
        {errors['scenes.framing'] && <p className="vt-fielderr" role="alert">{errors['scenes.framing']}</p>}
      </div>

      <div className="vt-card">
        <div className="vt-card-head">
          <span className="vt-label">{copy.promptTitle}</span>
          <span className="vt-links">
            <a className="vt-link" href="#/library?tab=prompts">{copy.promptLibrary}</a>
          </span>
        </div>
        <PromptPicker
          prompts={prompts}
          value={scenes.scenePromptId}
          onSelect={(id) => setScenes({ scenePromptId: id })}
          usedLabel={copy.promptNote}
        />
        {errors['scenes.scenePromptId'] && <p className="vt-fielderr" role="alert">{errors['scenes.scenePromptId']}</p>}
      </div>

      <div className="vt-card">
        <span className="vt-label" id="vt-count">{copy.countTitle}</span>
        <Segmented
          label={copy.countAria}
          fit="content"
          value={scenes.images.mode}
          onChange={(mode) => setScenes({ images: { ...scenes.images, mode: mode as Recipe['scenes']['images']['mode'] } })}
          options={[
            { id: 'fixed', label: copy.countFixed },
            { id: 'per-minutes', label: copy.countPer },
            { id: 'none', label: copy.countNone },
          ]}
        />
        {scenes.images.mode !== 'none' && (
          <div className="vt-row">
            <span className="vt-row-note">{scenes.images.mode === 'fixed' ? copy.countPreFixed : copy.countPrePer}</span>
            <NumberStepper
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
            <span className="vt-row-note">{scenes.images.mode === 'fixed' ? copy.countPostFixed : copy.countPostPer}</span>
          </div>
        )}
        <p className="vt-help vt-help--small">{countText}</p>
        <p className="vt-help vt-help--small">{priceText}</p>
        {errors['scenes.images.count'] && <p className="vt-fielderr" role="alert">{errors['scenes.images.count']}</p>}
        {errors['scenes.images.minutes'] && <p className="vt-fielderr" role="alert">{errors['scenes.images.minutes']}</p>}
      </div>

      <div className="vt-card">
        <span className="vt-label">{copy.motionTitle}</span>
        <Segmented
          label={copy.motionAria}
          fit="content"
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
        {errors['scenes.motion'] && <p className="vt-fielderr" role="alert">{errors['scenes.motion']}</p>}
      </div>

      <div className="vt-card">
        <SliderField
          label={copy.opacityTitle}
          value={scenes.sceneOpacity}
          min={0.1}
          max={1}
          step={0.05}
          formatValue={(value) => (value >= 0.999 ? copy.opacityFull : copy.opacityPct(String(Math.round(value * 100))))}
          onChange={(value) => setScenes({ sceneOpacity: value })}
        />
        <p className="vt-help vt-help--small">{copy.opacityHelp}</p>
        {errors['scenes.sceneOpacity'] && <p className="vt-fielderr" role="alert">{errors['scenes.sceneOpacity']}</p>}
      </div>

      <div className="vt-card">
        <div className="vt-switch-row">
          <div>
            <span className="vt-label">{copy.referenceTitle}</span>
            <p className="vt-help vt-help--small">{copy.referenceHelp}</p>
          </div>
          <Switch label={copy.referenceTitle} checked={scenes.referenceImages}
            onChange={(on) => setScenes({ referenceImages: on })} />
        </div>
      </div>
    </section>
  )
}
