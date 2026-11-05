import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Icon } from '@noverse/ui'
import type { ImagePrompt, TemplateSummary } from '../../api/library'
import { t } from '../../i18n/library'

/** Minh hoạ bằng ảnh mẫu của chỉ dẫn đang gắn, không giả làm bản xem thử đã dựng. */
export function TemplateArtwork({ template, prompts, xstyle }: {
  template: TemplateSummary
  prompts?: readonly ImagePrompt[]
  xstyle?: stylex.StyleXStyles
}) {
  const prompt = template.images.mode === 'none' ? undefined : prompts?.find(item => item.id === template.prompts.sceneId)
  const url = prompt?.sampleUrls?.[0]
  const [failedUrl, setFailedUrl] = useState<string>()
  return url && failedUrl !== url
    ? <img src={url} alt={t.library.templates.artworkAlt(prompt!.name)} title={t.library.templates.artworkAlt(prompt!.name)} loading="lazy" onError={() => setFailedUrl(url)} {...stylex.props(sx.image, xstyle)} />
    : <span role="img" aria-label={t.library.templates.artworkMissing} title={t.library.templates.artworkMissing} {...stylex.props(sx.empty, xstyle)}><Icon name="layer" size={24} /></span>
}

const sx = stylex.create({
  image: { display: 'block', width: '100%', height: '100%', objectFit: 'cover', borderRadius: 'inherit' },
  empty: { display: 'flex', width: '100%', height: '100%', alignItems: 'center', justifyContent: 'center', borderRadius: 'inherit' },
})
