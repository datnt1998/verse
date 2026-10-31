// Cột trái trang template (Template.dc.html): “Tất cả template” + 4 mục kèm
// dòng tóm tắt sống. Mobile: cùng danh sách cuộn ngang dưới đầu trang.
import { Icon } from '@noverse/ui'
import type { IconName } from '@noverse/ui'
import { vi as t } from '../../../i18n/vi'
import { paths, type TemplateSection } from '../../../router'
import { num } from '../fmt'
import type { TemplateDetail } from '../../../api/library'
import { healthWarn, tplHealthChecks } from '../TemplateScreen'
import type { Recipe } from '../../../api/library'

export function sectionSums(detail: TemplateDetail, overlayNames: string[] = []) {
  const recipe = detail.recipe
  const place = recipe.scenes.style === 'full'
    ? t.library.templates.style.full
    : recipe.scenes.framing === 'close' ? t.library.templates.style.close : t.library.templates.style.cutout
  const images = detail.images.known ? num(detail.images.perVideo ?? detail.images.count) : '?'
  const backdrop = backdropParts(detail)
  const layers = overlayNames.length > 0
    ? t.library.templates.backdropLayers(overlayNames)
    : ''
  const linked = backdrop ? [backdrop, layers].filter(Boolean).join(' · ') : layers
  return {
    images: `${place} · ${images} · ${detail.prompts.sceneName}`,
    cover: detail.prompts.coverName,
    backdrop: linked || t.library.template.health.noBackdrop,
    output: `${recipe.output.compressExport ? t.library.template.output.compressed.title : t.library.template.output.original.title} · ${detail.channels.length}`,
  }
}

function backdropParts(detail: TemplateDetail): string {
  const backdrop = detail.recipe.backdrop
  return backdrop ? t.library.templates.backdrop(backdrop.channels.length, backdrop.fixedVideoIds.length) : ''
}

export function TemplateNav({ detail, active, overlayNames }: { detail: TemplateDetail; active: TemplateSection; overlayNames?: string[] }) {
  const copy = t.library.template
  const sums = sectionSums(detail, overlayNames)
  const items: { id: TemplateSection; label: string; icon: IconName; sum: string }[] = [
    { id: 'images', label: copy.sections.images.label, icon: 'gallery', sum: sums.images },
    { id: 'cover', label: copy.sections.cover.label, icon: 'magicpen', sum: sums.cover },
    { id: 'backdrop', label: copy.sections.backdrop.label, icon: 'video-play', sum: sums.backdrop },
    { id: 'output', label: copy.sections.output.label, icon: 'export', sum: sums.output },
  ]
  return (
    <nav className="vt-nav" aria-label={copy.sideAria}>
      <a className="vt-nav-back" href={`#${paths.library('templates')}`}>
        <span aria-hidden="true">←</span>{copy.back}
      </a>
      <div className="vt-nav-group">
        <span className="vt-nav-label">{copy.sectionGroup}</span>
        {items.map((item) => (
          <a
            key={item.id}
            className="vt-nav-item"
            href={`#${paths.template(detail.id, item.id)}`}
            aria-current={active === item.id ? 'page' : undefined}
          >
            <Icon name={item.icon} size={15} />
            <span className="vt-nav-text">
              <span className="vt-nav-title">{item.label}</span>
              <span className="vt-nav-sum">{item.sum}</span>
            </span>
          </a>
        ))}
      </div>
      <HealthCard detail={detail} />
    </nav>
  )
}

function HealthCard({ detail }: { detail: TemplateDetail }) {
  const copy = t.library.template.health
  const checks = tplHealthChecks(detail)
  const warn = healthWarn(detail)
  return (
    <section className="vt-health" aria-label={t.library.template.healthTitle}>
      <div className="vt-health-head">
        <span className="vt-health-dot" data-state={warn ? 'warn' : 'ok'} aria-hidden="true" />
        <strong>{warn ? copy.limited : copy.ready}</strong>
      </div>
      <ul>
        {checks.map((check) => <li key={check}>{check}</li>)}
      </ul>
    </section>
  )
}
