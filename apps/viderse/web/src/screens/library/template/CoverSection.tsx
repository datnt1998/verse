// Mục Ảnh bìa (Template.dc.html › lib^cover): prompt ảnh bìa, vẽ thử (khoá —
// cần video đã làm xong), ảnh bìa mặc định (khoá — chưa có kho media), cách lấy
// ảnh bìa. Prompt có sẵn đứng trước prompt của bạn trong ô chọn.
import { RadioCard, Icon } from '@noverse/ui'
import { ActionButton } from '../../../components'
import { vi as t } from '../../../i18n/vi'
import type { ImagePrompt, Recipe } from '../../../api/library'
import { PromptPicker } from './PromptPicker'

export function CoverSection({ recipe, prompts, errors, onSave }: {
  recipe: Recipe
  prompts: ImagePrompt[]
  errors: Record<string, string>
  onSave: (patch: Partial<Recipe>) => void
}) {
  const copy = t.library.template.cover
  const scenes = recipe.scenes
  const setScenes = (next: Partial<Recipe['scenes']>) => onSave({ scenes: { ...scenes, ...next } })
  const ordered = [...prompts.filter((prompt) => prompt.builtIn), ...prompts.filter((prompt) => !prompt.builtIn)]

  return (
    <section className="vt-sec" aria-labelledby="vt-cover-title">
      <div className="vt-sec-head">
        <h2 id="vt-cover-title">{copy.title}</h2>
        <p className="vt-help">{copy.help}</p>
      </div>

      <div className="vt-card">
        <div className="vt-card-head">
          <span className="vt-label">{copy.promptTitle}</span>
          <span className="vt-links">
            <a className="vt-link" href="#/library?tab=prompts">{copy.promptLibrary}</a>
          </span>
        </div>
        <PromptPicker
          prompts={ordered}
          value={scenes.coverPromptId}
          onSelect={(id) => setScenes({ coverPromptId: id })}
          usedLabel={copy.promptNote}
        />
        {errors['scenes.coverPromptId'] && <p className="vt-fielderr" role="alert">{errors['scenes.coverPromptId']}</p>}
      </div>

      <div className="vt-card">
        <span className="vt-label">{copy.sourceLabel}</span>
        <div role="radiogroup" aria-label={copy.sourceLabel} className="vt-sources">
          <RadioCard checked={scenes.coverSource === 'ai'} onChange={() => setScenes({ coverSource: 'ai' })}
            label={copy.sourceAi} description={copy.sourceAiHelp} />
          <RadioCard checked={scenes.coverSource === 'upload'} onChange={() => setScenes({ coverSource: 'upload' })}
            label={copy.sourceUpload} description={copy.sourceUploadHelp} />
        </div>
      </div>

      <div className="vt-card">
        <div className="vt-card-head">
          <span className="vt-label">{copy.trialTitle}</span>
          <span className="vt-hint">{copy.trialKeep}</span>
        </div>
        <p className="vt-hint">{copy.trialEmpty}</p>
        <div className="vt-row">
          <ActionButton variant="outline" icon={<Icon name="magicpen" size={16} />} lockedReason={copy.trialLocked}>{copy.trialDraw}</ActionButton>
          <p className="vt-hint">{copy.trialHelp}</p>
        </div>
      </div>

      <div className="vt-card">
        <span className="vt-label">{copy.defaultTitle}</span>
        <div className="vt-cover-default">
          <span className="vt-cover-thumb" aria-hidden="true" />
          <div className="vt-cover-copy">
            <p>{copy.defaultNone}</p>
            <p className="vt-hint">{copy.defaultDrop}</p>
          </div>
          <ActionButton variant="outline" lockedReason={copy.defaultLocked}>{copy.defaultChoose}</ActionButton>
        </div>
        <p className="vt-hint">{copy.defaultHelp}</p>
        {errors['scenes.coverMediaId'] && <p className="vt-fielderr" role="alert">{errors['scenes.coverMediaId']}</p>}
      </div>
    </section>
  )
}
