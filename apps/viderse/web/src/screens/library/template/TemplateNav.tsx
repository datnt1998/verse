import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from '../libraryTokens.stylex'
// Cột trái trang template (Template.dc.html): “Tất cả template” + 4 mục kèm
// dòng tóm tắt sống. Mobile: cùng danh sách cuộn ngang dưới đầu trang.
import { Icon } from '@noverse/ui'
import type { IconName } from '@noverse/ui'
import { t } from '../../../i18n/library'
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
    <nav className={`vt-nav ${stylex.props(sx.nav).className}`} aria-label={copy.sideAria}>
      <a className={`vt-nav-back ${stylex.props(sx.navBack).className}`} href={`#${paths.library('templates')}`}>
        <span aria-hidden="true">←</span>{copy.back}
      </a>
      <div className={`vt-nav-group ${stylex.props(sx.navGroup).className}`}>
        <span className={`vt-nav-label ${stylex.props(sx.navLabel).className}`}>{copy.sectionGroup}</span>
        {items.map((item) => (
          <a
            key={item.id}
            className={`vt-nav-item ${stylex.props(sx.navItem, ((active === item.id ? 'page' : undefined) === "page") && sx.navItemAriaCurrentPage).className}`}
            href={`#${paths.template(detail.id, item.id)}`}
            aria-current={active === item.id ? 'page' : undefined}
          >
            <Icon name={item.icon} size={15} />
            <span className={`vt-nav-text ${stylex.props(sx.navText).className}`}>
              <span className={`vt-nav-title ${stylex.props(sx.navTitle).className}`}>{item.label}</span>
              <span className={`vt-nav-sum ${stylex.props(sx.navSum).className}`}>{item.sum}</span>
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
    <section className={`vt-health ${stylex.props(sx.health).className}`} aria-label={t.library.template.healthTitle}>
      <div className={`vt-health-head ${stylex.props(sx.healthHead).className}`}>
        <span className={`vt-health-dot ${stylex.props(sx.healthDot, ((warn ? 'warn' : 'ok') === "warn") && sx.healthDotDataStateWarn).className}`} data-state={warn ? 'warn' : 'ok'} aria-hidden="true" />
        <strong>{warn ? copy.limited : copy.ready}</strong>
      </div>
      <ul {...stylex.props(sx.vtHealthUl)}>
        {checks.map((check) => <li key={check}>{check}</li>)}
      </ul>
    </section>
  )
}

const sx = stylex.create({
  nav: { display: { default: "flex", "@media (max-width: 1099px)": "none" }, flexDirection: "column", flexGrow: 0, flexShrink: 0, flexBasis: "var(--vv-s-232)", rowGap: "var(--vv-s-18)", columnGap: "var(--vv-s-18)", paddingTop: "var(--vv-s-22)", paddingRight: "var(--vv-s-14)", paddingBottom: "var(--vv-s-22)", paddingLeft: "var(--vv-s-20)", boxShadow: libraryVars.libraryScreenSideBoxShadow, minHeight: 0, overflowY: "auto" },
  navBack: { display: "inline-flex", alignItems: "center", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", paddingTop: "var(--vv-s-5)", paddingRight: "var(--vv-s-8)", paddingBottom: "var(--vv-s-5)", paddingLeft: "var(--vv-s-8)", borderTopLeftRadius: "var(--vv-r-7)", borderTopRightRadius: "var(--vv-r-7)", borderBottomRightRadius: "var(--vv-r-7)", borderBottomLeftRadius: "var(--vv-r-7)", color: { default: color["--color-fg-3"], ":hover": color["--color-fg"] }, fontSize: libraryVars.bodySmallFontSize, textDecoration: "none", minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" }, backgroundColor: { default: null, ":hover": "var(--vv-hover)" }, backgroundImage: { default: null, ":hover": "none" } },
  navGroup: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-2)", columnGap: "var(--vv-s-2)" },
  navLabel: { paddingTop: 0, paddingRight: "var(--vv-s-8)", paddingBottom: "var(--vv-s-4)", paddingLeft: "var(--vv-s-8)", fontSize: "var(--vv-s-11)", letterSpacing: libraryVars.libraryScreenSideLabelLetterSpacing, textTransform: "uppercase", color: color["--color-fg-4"] },
  navItem: { display: "flex", alignItems: "flex-start", rowGap: "var(--vv-s-9)", columnGap: "var(--vv-s-9)", paddingTop: "var(--vv-s-7)", paddingRight: "var(--vv-s-8)", paddingBottom: "var(--vv-s-7)", paddingLeft: "var(--vv-s-8)", borderTopLeftRadius: "var(--vv-r-8)", borderTopRightRadius: "var(--vv-r-8)", borderBottomRightRadius: "var(--vv-r-8)", borderBottomLeftRadius: "var(--vv-r-8)", color: { default: color["--color-fg-2"], ":hover": color["--color-fg"] }, textDecoration: "none", minWidth: 0, minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" }, backgroundColor: { default: null, ":hover": "var(--vv-hover-row)" }, backgroundImage: { default: null, ":hover": "none" } },
  navItemAriaCurrentPage: { backgroundColor: color["--color-surface-5"], backgroundImage: "none", color: color["--color-fg"] },
  navText: { display: "flex", flexDirection: "column", minWidth: 0 },
  navTitle: { fontSize: "var(--vv-s-13)", fontWeight: libraryVars.labelWeight },
  navSum: { fontSize: libraryVars.captionFontSize, color: color["--color-fg-4"], overflowX: "hidden", overflowY: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" },
  health: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)", paddingTop: "var(--vv-s-12)", paddingRight: "var(--vv-s-12)", paddingBottom: "var(--vv-s-12)", paddingLeft: "var(--vv-s-12)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-12)", borderTopRightRadius: "var(--vv-r-12)", borderBottomRightRadius: "var(--vv-r-12)", borderBottomLeftRadius: "var(--vv-r-12)", backgroundColor: color["--color-surface-1"], backgroundImage: "none", marginTop: "auto" },
  healthHead: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)", fontSize: "var(--vv-s-13)" },
  healthDot: { width: "var(--vv-s-8)", height: "var(--vv-s-8)", borderTopLeftRadius: "50%", borderTopRightRadius: "50%", borderBottomRightRadius: "50%", borderBottomLeftRadius: "50%", backgroundColor: color["--color-ok"], backgroundImage: "none" },
  healthDotDataStateWarn: { backgroundColor: color["--color-warn"], backgroundImage: "none" },
  vtHealthUl: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, paddingLeft: "var(--vv-s-18)", display: "flex", flexDirection: "column", rowGap: "var(--vv-s-4)", columnGap: "var(--vv-s-4)", fontSize: "var(--vv-s-12)", color: color["--color-fg-3"] },
})
