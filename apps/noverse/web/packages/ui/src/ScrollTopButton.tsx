import { useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type RefObject } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, kit, scrollTop } from './tokens.stylex'
import { getKitLabels } from './labels'
import { Icon } from './Icon'

/** One-screen / half-screen hysteresis; short content never installs a scroll listener. */
export function useScrollTop(ref: RefObject<HTMLElement | null>) {
  const [state, setState] = useState({ visible: false, progress: 0, eligible: false })
  const watching = useRef<HTMLElement | null>(null)
  const cleanup = useRef<(() => void) | null>(null)
  useLayoutEffect(() => () => { cleanup.current?.(); watching.current = null }, [])
  useLayoutEffect(() => {
    const element = ref.current
    if (watching.current === element) return
    cleanup.current?.()
    watching.current = element
    if (!element) return
    let frame = 0
    let listening = false
    const measure = () => {
      frame = 0
      const height = element.clientHeight
      const eligible = height > 0 && element.scrollHeight >= height * 3
      if (eligible !== listening) {
        if (eligible) element.addEventListener('scroll', schedule, { passive: true })
        else element.removeEventListener('scroll', schedule)
        listening = eligible
      }
      const progress = Math.min(1, Math.max(0, element.scrollTop / Math.max(1, element.scrollHeight - height)))
      setState((previous) => {
        const visible = eligible && (previous.visible ? element.scrollTop >= height / 2 : element.scrollTop > height)
        return previous.visible === visible && previous.progress === progress && previous.eligible === eligible
          ? previous : { visible, progress, eligible }
      })
    }
    const schedule = () => { if (!frame) frame = requestAnimationFrame(measure) }
    const resize = new ResizeObserver(schedule)
    const observeContent = () => {
      resize.disconnect()
      resize.observe(element)
      for (const child of element.children) resize.observe(child)
      schedule()
    }
    const mutations = new MutationObserver(observeContent)
    mutations.observe(element, { childList: true, subtree: true, characterData: true })
    observeContent()
    cleanup.current = () => {
      cancelAnimationFrame(frame)
      resize.disconnect()
      mutations.disconnect()
      element.removeEventListener('scroll', schedule)
    }
  })
  return state
}

const REDUCE = '@media (prefers-reduced-motion: reduce)'
const styles = stylex.create({
  button: {
    position: 'absolute', right: 20, bottom: 16, zIndex: 15, width: 40, height: 40, padding: 0,
    borderWidth: 0, borderRadius: '50%', backgroundColor: { default: scrollTop['--scroll-top-bg'], ':hover': scrollTop['--scroll-top-hover'] },
    backdropFilter: 'blur(12px)', color: color['--color-fg'], display: 'flex', alignItems: 'center', justifyContent: 'center',
    boxShadow: scrollTop['--scroll-top-shadow'], cursor: 'pointer', opacity: 1, visibility: 'visible',
    transform: { default: 'none', ':active': 'scale(.94)' },
    outline: { default: 'none', ':focus-visible': `2px solid ${color['--color-accent']}` }, outlineOffset: 3,
    transition: { default: 'opacity 180ms ease-out, transform 180ms cubic-bezier(.2,.8,.2,1), background-color 120ms ease, visibility 0s', [REDUCE]: 'none' },
  },
  hidden: {
    opacity: 0, transform: 'translateY(8px) scale(.9)', visibility: 'hidden', pointerEvents: 'none',
    transition: { default: 'opacity 120ms ease-in, transform 120ms ease-in, visibility 0s 120ms', [REDUCE]: 'none' },
  },
  mobile: {
    width: 44, height: 44, right: 16,
    transition: { default: 'opacity 180ms ease-out, transform 180ms cubic-bezier(.2,.8,.2,1), bottom .38s cubic-bezier(.2,.9,.25,1), visibility 0s', [REDUCE]: 'none' },
  },
  mobileHidden: {
    transition: { default: 'opacity 120ms ease-in, transform 120ms ease-in, bottom .38s cubic-bezier(.2,.9,.25,1), visibility 0s 120ms', [REDUCE]: 'none' },
  },
  ring: { position: 'absolute', inset: 0, borderRadius: '50%', maskImage: scrollTop['--scroll-top-mask'], pointerEvents: 'none' },
  icon: { position: 'relative' },
  tip: {
    position: 'absolute', right: 'calc(100% + 10px)', top: '50%', transform: 'translateY(-50%)', height: 28,
    paddingInline: 10, borderRadius: 8, backgroundColor: color['--color-surface-6'], boxShadow: scrollTop['--scroll-top-tip-shadow'],
    color: color['--color-fg'], fontSize: 12, fontWeight: 500, whiteSpace: 'nowrap', display: 'flex', alignItems: 'center',
    gap: 8, opacity: 0, pointerEvents: 'none', transition: { default: 'opacity 120ms ease', [REDUCE]: 'none' },
  },
  tipOn: { opacity: 1 },
  hint: { color: color['--color-fg-4'], fontWeight: 400 },
})

/** Global overlays also suppress desktop mounts (portalled menus/dialogs live outside the target). */
function useScrollTopObscured() {
  const [obscured, setObscured] = useState(false)
  const previous = useRef(false)
  useEffect(() => {
    const measure = () => {
      const overlay = [...document.querySelectorAll<HTMLElement>('[aria-modal="true"], [role="menu"], [role="listbox"]')]
        .some((node) => node.getClientRects().length > 0 && !node.closest('[hidden], [inert], [aria-hidden="true"]'))
      const selection = window.getSelection()
      const keyboard = !!window.visualViewport && window.innerHeight - window.visualViewport.height - window.visualViewport.offsetTop > 100
      const next = overlay || keyboard || !!selection && !selection.isCollapsed
      if (next !== previous.current) { previous.current = next; setObscured(next) }
    }
    const observer = new MutationObserver(measure)
    observer.observe(document.body, { subtree: true, childList: true, attributes: true, attributeFilter: ['hidden', 'inert', 'aria-hidden', 'aria-expanded'] })
    document.addEventListener('selectionchange', measure)
    window.visualViewport?.addEventListener('resize', measure)
    measure()
    return () => { observer.disconnect(); document.removeEventListener('selectionchange', measure); window.visualViewport?.removeEventListener('resize', measure) }
  }, [])
  return obscured
}

export type ScrollTopButtonProps = {
  target: RefObject<HTMLElement | null>
  focusTarget?: RefObject<HTMLElement | null>
  label: string
  progress?: boolean
  size: 'md' | 'lg'
  /** True while the mobile reader's bars are hidden. */
  hideWithBars?: boolean
  hidden?: boolean
  style?: CSSProperties
}

export function ScrollTopButton({ target, focusTarget, label, progress: ring = false, size, hideWithBars = false, hidden = false, style }: ScrollTopButtonProps) {
  const { visible, progress, eligible } = useScrollTop(target)
  const obscured = useScrollTopObscured()
  const [tip, setTip] = useState(false)
  const [returning, setReturning] = useState(false)
  const limit = useRef<number | undefined>(undefined)
  useEffect(() => () => window.clearTimeout(limit.current), [])
  useEffect(() => {
    if (!returning) return
    const element = target.current
    const interrupted = () => { window.clearTimeout(limit.current); setReturning(false) }
    const completed = () => { if (element?.scrollTop === 0) setReturning(false) }
    element?.addEventListener('scrollend', completed)
    element?.addEventListener('wheel', interrupted, { passive: true })
    element?.addEventListener('touchstart', interrupted, { passive: true })
    return () => { window.clearTimeout(limit.current); element?.removeEventListener('scrollend', completed); element?.removeEventListener('wheel', interrupted); element?.removeEventListener('touchstart', interrupted) }
  }, [returning, target])
  // Browser scrollend is not available everywhere; reaching the top also completes the return.
  useEffect(() => { if (progress === 0) setReturning(false) }, [progress])
  if (!eligible) return null
  const show = visible && !returning && !hidden && !hideWithBars && !obscured
  const percent = Math.round(progress * 100)
  const sx = stylex.props(styles.button, size === 'lg' && styles.mobile, !show && styles.hidden, !show && size === 'lg' && styles.mobileHidden)
  return <button type="button" data-scroll-top="" className={sx.className} style={{ ...sx.style, ...style }}
    aria-label={ring ? `${label}, ${getKitLabels().scrollTop.read(percent)}` : label} aria-hidden={!show || undefined} tabIndex={show ? 0 : -1}
    onMouseEnter={() => setTip(true)} onMouseLeave={() => setTip(false)} onFocus={() => setTip(true)} onBlur={() => setTip(false)}
    onClick={() => {
      const element = target.current
      if (!element) return
      const reduced = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
      if (element.scrollTop > element.clientHeight * 6) element.scrollTop = element.clientHeight
      setReturning(true)
      // Bound native smooth scrolling without replacing its platform easing.
      window.clearTimeout(limit.current)
      if (!reduced) limit.current = window.setTimeout(() => {
        if (element.scrollTop > 0) element.scrollTo({ top: 0, behavior: 'instant' })
      }, 400)
      element.scrollTo({ top: 0, behavior: reduced ? 'auto' : 'smooth' })
      const heading = focusTarget?.current ?? element
      if (!heading.hasAttribute('tabindex')) heading.tabIndex = -1
      heading.focus({ preventScroll: true })
    }}>
    {ring && <span aria-hidden="true" {...stylex.props(styles.ring)} style={{ background: `conic-gradient(${color['--color-accent']} ${progress * 360}deg, ${kit['--kit-wash-12']} 0)` }} />}
    <Icon name="arrow-up-2" size={size === 'lg' ? 20 : 18} {...stylex.props(styles.icon)} />
    {size !== 'lg' && <span aria-hidden="true" {...stylex.props(styles.tip, tip && styles.tipOn)}>{label}<span {...stylex.props(styles.hint)}>{ring && `${getKitLabels().scrollTop.read(percent)} · `}Home</span></span>}
  </button>
}
