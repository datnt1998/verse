// Chọn prompt trong trang template: lưới thẻ bấm-chọn (ảnh cảnh / ảnh bìa) thay
// ô chọn thả xuống, đúng artboard Template; kèm thẻ "Prompt mới" dẫn về kho và
// khối chi tiết của prompt đang chọn.
import { Icon } from '@noverse/ui'
import { vi as t } from '../../../i18n/vi'
import type { ImagePrompt } from '../../../api/library'

export function PromptPicker({ prompts, value, onSelect, usedLabel }: {
  prompts: ImagePrompt[]
  value: string
  onSelect: (id: string) => void
  usedLabel: (count: number) => string
}) {
  const copy = t.library.template
  const selected = prompts.find((prompt) => prompt.id === value)
  return (
    <>
      <div className="vt-picks" role="radiogroup" aria-label={copy.images.promptAria}>
        {prompts.map((prompt) => (
          <button
            key={prompt.id}
            type="button"
            role="radio"
            aria-checked={prompt.id === value}
            aria-label={prompt.name}
            className="vt-pick"
            data-kind={prompt.kind}
            onClick={() => onSelect(prompt.id)}
          >
            <span className="vt-pick-art" data-kind={prompt.kind} aria-hidden="true">
              <span className="vt-pick-glyph" />
              {prompt.builtIn && <span className="vt-pick-chip">{t.library.prompts.builtChip}</span>}
            </span>
            <span className="vt-pick-name">{prompt.name}</span>
            {prompt.id === value && <span className="vt-pick-check" aria-hidden="true"><Icon name="tick" size={12} /></span>}
          </button>
        ))}
        <a className="vt-pick vt-pick--new" href="#/library?tab=prompts&prompt=new">
          <span className="vt-pick-plus" aria-hidden="true"><Icon name="add" size={16} /></span>
          <span className="vt-pick-name">{copy.images.promptNew}</span>
          <span className="vt-hint">{copy.images.promptNewHint}</span>
        </a>
      </div>
      {selected && (
        <div className="vt-pick-detail">
          <div className="vt-pick-detail-head">
            <strong>{selected.name}</strong>
            {selected.composition && <span className="vt-chip">{t.library.prompts.composition[selected.composition]}</span>}
            <a className="vt-link" href={`#/library?tab=prompts&prompt=${selected.builtIn ? 'builtin' : 'mine'}`}>
              {copy.images.promptEdit} <span aria-hidden="true">›</span>
            </a>
          </div>
          <p className="vt-pick-text">{selected.prompt}</p>
          <p className="vt-hint">{usedLabel(selected.usedBy.length)}</p>
        </div>
      )}
    </>
  )
}
