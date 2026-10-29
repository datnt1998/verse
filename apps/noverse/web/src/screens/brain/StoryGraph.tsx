// Shared Brain artboard cards; deterministic force layout and direct SVG updates
// keep dragging independent of React rendering the detail/timeline columns.
import { useEffect, useId, useLayoutEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import type { Brain, BrainEdge, BrainEdgeType, BrainEntityType } from '../../api/types'
import { Icon } from '@noverse/ui'
import { useMediaQuery } from '../../shell/AppShell'
import { t } from '../../i18n/vi'
import { brain as brainTokens, line } from '@noverse/ui/tokens.stylex'
import {
  EDGE_META, MAX_ENTITY, MAX_LABEL, MAX_NAME, MAX_NARRATOR, VIEW,
  charHeight, charRadius, charWidth, edgeCurve, edgesAt, entityView, fitText, graphBounds, isNewAt, labelWidth,
  nodeRect, otherWidth, type ActiveEdge, type Curve, type Rect,
} from './model'
import { createForceGraph, LabelCollisionIndex, neighboursOf, placeForceLabel, type ForceNode, type ForceWarmupRequest, type ForceWarmupResult } from './forceGraph'
import './brain.css'
import { badgeGeometry, KindLegend } from '../../ui/CharAvatar'

/** Hình glyph vẽ nét theo loại thực thể (artboard GLY). */
const GLYPH: Partial<Record<BrainEntityType, string>> = {
  obj: 'M8 1.8l5.4 3.1v6.2L8 14.2 2.6 11.1V4.9z M2.6 4.9L8 8l5.4-3.1 M8 8v6.2',
  ev: 'M8 2.2a5.8 5.8 0 1 0 0 11.6A5.8 5.8 0 1 0 8 2.2z M8 4.8V8l2.2 1.4',
  fs: 'M4 14V2.4 M4 3h7.6l-1.8 2.6 1.8 2.6H4',
  place: 'M8 14.2s-4.6-4.1-4.6-7.6a4.6 4.6 0 0 1 9.2 0c0 3.5-4.6 7.6-4.6 7.6z M8 4.9a1.7 1.7 0 1 0 0 3.4a1.7 1.7 0 1 0 0-3.4z',
  rule: 'M8 1.8l5.2 2v4.1c0 3.2-2.3 5.3-5.2 6.3-2.9-1-5.2-3.1-5.2-6.3V3.8z M5.8 8l1.6 1.6 2.9-3',
}

type Measured = { w: number; h: number; name: string; role: string }
type EdgeGeometry = { edge: BrainEdge; index: number; item: ActiveEdge; curve: Curve }

const keyOf = (edge: { a: string; b: string }, index: number) => `${index}:${edge.a}|${edge.b}`
const NO_TYPES: ReadonlySet<BrainEntityType> = new Set()
const NO_EDGES: ReadonlySet<BrainEdgeType> = new Set()
const graphStyles = stylex.create({
  wrap: {
    position: 'relative', flex: '1 1 auto', minHeight: 0, overflow: 'hidden', marginBlock: 0, marginInline: 14,
    borderRadius: 16, borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'],
    background: `radial-gradient(70% 70% at 50% 46%, ${brainTokens['--brain-wrap-1']} 0%, ${brainTokens['--brain-wrap-2']} 60%, ${brainTokens['--brain-wrap-3']} 100%)`,
  },
  compact: { marginTop: 12, marginInline: 12 },
  legend: { position: 'absolute', left: 12, bottom: 12, pointerEvents: 'none', paddingBlock: 6, paddingInline: 11, borderRadius: 999, backgroundColor: brainTokens['--brain-legend-bg'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'], backdropFilter: 'blur(8px)' },
  hint: {
    position: 'absolute', left: 14, bottom: 14, fontSize: 11.5, color: brainTokens['--brain-hint'],
    display: 'flex', alignItems: 'center', gap: 6, pointerEvents: 'none',
  },
  compactHint: { left: 'auto', right: 14, bottom: 16 },
  host: {
    backgroundImage: `radial-gradient(${brainTokens['--brain-dots']} 0.75px, transparent 0.75px)`,
    backgroundSize: '20px 20px',
  },
  accessible: {
    position: 'absolute', width: 1, height: 1, padding: 0, margin: -1,
    overflow: 'hidden', clipPath: 'inset(50%)', whiteSpace: 'nowrap',
  },
})

export function StoryGraph({ brain, chapter, selected, onSelect, hiddenTypes = NO_TYPES, hiddenEdges = NO_EDGES, query = '', compact = false }: {
  brain: Brain
  chapter: number
  selected: string
  onSelect: (id: string) => void
  hiddenTypes?: ReadonlySet<BrainEntityType>
  hiddenEdges?: ReadonlySet<BrainEdgeType>
  query?: string
  /** Gói U: chỉ bỏ điều khiển; cùng dữ liệu, thẻ, cạnh, nhãn và bàn phím. */
  compact?: boolean
}) {
  const graphId = useId().replace(/:/g, '')
  const svgId = (id: string) => `${graphId}-${id}`
  const paint = (id: string) => `url(#${svgId(id)})`
  const reduce = useMediaQuery('(prefers-reduced-motion: reduce)')
  const [hover, setHover] = useState<string | null>(null)
  const [focusOnly, setFocusOnly] = useState(false)
  const drag = useRef<{
    id: number; x: number; y: number; view: [number, number, number, number];
    node: ForceNode | null; offsetX: number; offsetY: number; moved: boolean;
  } | null>(null)
  const pointers = useRef(new Map<number, { x: number; y: number }>())
  const pinch = useRef<{ distance: number; view: [number, number, number, number]; x: number; y: number; zoom: number } | null>(null)
  const suppressClick = useRef(false)
  const positionMemory = useRef(new Map<string, { x: number; y: number }>())
  const chapterMemory = useRef(chapter)
  const retainedIds = useRef(new Set<string>())
  const autoFit = useRef(true)
  const drawRef = useRef<() => void>(() => {})
  const resumeRef = useRef<() => void>(() => {})
  const cancelWarmupRef = useRef<() => void>(() => {})
  const [zoom, setZoom] = useState(1)
  const [measured, setMeasured] = useState<Record<string, Measured>>({})
  const [labelFit, setLabelFit] = useState<Record<string, { text: string; w: number }>>({})
  const [newWidth, setNewWidth] = useState(22)
  const [fontEpoch, setFontEpoch] = useState(0)
  const nodeText = useRef(new Map<string, SVGTextElement>())
  const labelText = useRef(new Map<string, SVGTextElement>())
  const newText = useRef<SVGTextElement | null>(null)
  const svgRef = useRef<SVGSVGElement | null>(null)
  const viewBox = useRef<[number, number, number, number]>([VIEW.x, VIEW.y, VIEW.w, VIEW.h])
  const viewportReady = useRef(false)

  // Chữ đo lại sau khi font tải xong (artboard: fonts.ready + loadingdone → build lại).
  useEffect(() => {
    const fonts = document.fonts
    if (!fonts) return
    const again = () => setFontEpoch((value) => value + 1)
    fonts.addEventListener('loadingdone', again)
    if (fonts.status !== 'loaded') void fonts.ready.then(again)
    return () => fonts.removeEventListener('loadingdone', again)
  }, [])

  const narrator = useMemo(() => brain.entities.find((entity) => entity.type === 'char')?.id ?? null, [brain.entities])
  const sizes = useMemo(() => Object.fromEntries(brain.entities.map((entity) => [entity.id, measured[entity.id] ?? {
    w: entity.type === 'char' ? charWidth(entity.id === narrator, entity.id === narrator ? MAX_NARRATOR : MAX_NAME) : otherWidth(MAX_ENTITY),
    h: entity.type === 'char' ? charHeight(entity.id === narrator) : 32,
  }])), [brain.entities, measured, narrator])
  const knownEntities = useMemo(() => brain.entities.filter((entity) => entity.fromChapter <= chapter), [brain.entities, chapter])
  const active = useMemo(
    () => edgesAt(brain, chapter).filter((item) => !hiddenEdges.has(item.phase.type)),
    [brain, chapter, hiddenEdges],
  )
  const graph = useMemo(() => {
    if (chapterMemory.current !== chapter) {
      retainedIds.current = new Set(positionMemory.current.keys())
      chapterMemory.current = chapter
      autoFit.current = false
    }
    const next = createForceGraph(knownEntities, active, sizes, positionMemory.current, 0, retainedIds.current)
    for (const [id, point] of next.positions) positionMemory.current.set(id, { x: point.x, y: point.y })
    return next
  }, [knownEntities, active, sizes])
  const positions = graph.positions
  const labelGroups = useMemo(() => {
    const grouped = new Map<string, { index: number; label: string; count: number; fresh: boolean }>()
    for (const item of active) {
      const pair = [item.edge.a, item.edge.b].sort()
      const key = JSON.stringify([pair, item.phase.label])
      const group = grouped.get(key)
      if (group) {
        group.count++
        group.fresh ||= item.fresh
      } else {
        grouped.set(key, { index: item.index, label: item.phase.label, count: 1, fresh: item.fresh })
      }
    }
    return new Map([...grouped.values()].map((group) => [group.index, group]))
  }, [active])
  const alive = useMemo(
    () => new Set(brain.entities.filter((entity) => entity.fromChapter <= chapter).map((entity) => entity.id)),
    [brain.entities, chapter],
  )
  const focus = hover && alive.has(hover) ? hover : selected
  const near = useMemo(() => neighboursOf(active, focus), [active, focus])
  const selNear = useMemo(() => neighboursOf(active, selected), [active, selected])
  const visible = useMemo(() => {
    const set = new Set<string>()
    for (const entity of brain.entities) {
      if (!alive.has(entity.id) || hiddenTypes.has(entity.type)) continue
      if (focusOnly && !selNear.has(entity.id)) continue
      set.add(entity.id)
    }
    return set
  }, [alive, brain.entities, focusOnly, hiddenTypes, selNear])

  const box = (id: string) => sizes[id]
  const edges = useMemo<EdgeGeometry[]>(() => active.flatMap((item) => {
    const { edge, index } = item
    const from = positions.get(edge.a)
    const to = positions.get(edge.b)
    if (!from || !to) return []
    return [{ edge, index, item, curve: edgeCurve(from, to, box(edge.a), box(edge.b), index) }]
    // Rebuild curves after the measured card dimensions change.
  }), [positions, active, sizes])
  const canvas = useMemo(() => graphBounds(positions, sizes, edges.map((entry) => entry.curve)), [positions, sizes, edges])
  const maxZoom = Math.max(2, canvas.w / VIEW.w * 2, canvas.h / VIEW.h * 2)

  // ── Đo chữ bằng font thật: bề rộng thẻ + nhãn cắt “…” (tên đầy đủ ở <title>) ──
  useLayoutEffect(() => {
    // Không ghi lại SVG rồi đo từng ký tự: mỗi lần đo ép cả đồ thị bố trí lại.
    // Canvas dùng đúng font đã tải và không phụ thuộc tỉ lệ viewBox giữa hai nơi.
    const context = document.createElement('canvas').getContext('2d')!
    const fonts = new Map<string, { font: string; spacing: string; widths: Map<string, number> }>()
    const measureOf = (element: SVGTextElement) => {
      const key = `${element.getAttribute('font-size')}:${element.getAttribute('font-weight')}:${element.style.letterSpacing}`
      let metrics = fonts.get(key)
      if (!metrics) {
        const font = getComputedStyle(element)
        metrics = { font: `${font.fontWeight} ${font.fontSize} ${font.fontFamily}`, spacing: font.letterSpacing, widths: new Map() }
        fonts.set(key, metrics)
      }
      const current = metrics
      return (text: string) => {
        const cached = current.widths.get(text)
        if (cached != null) return cached
        if (context.font !== current.font) context.font = current.font
        if (context.letterSpacing !== current.spacing) context.letterSpacing = current.spacing
        const width = context.measureText(text).width
        current.widths.set(text, width)
        return width
      }
    }
    const next: Record<string, Measured> = {}
    for (const entity of brain.entities) {
      const view = entityView(entity, chapter, narrator)
      const nameEl = nodeText.current.get(`${entity.id}:name`)
      if (!nameEl) continue
      const roleEl = nodeText.current.get(`${entity.id}:role`)
      const max = view.isChar ? (view.narrator ? MAX_NARRATOR : MAX_NAME) : MAX_ENTITY
      const measureName = measureOf(nameEl)
      const name = fitText(entity.name, max, measureName)
      const nameWidth = measureName(name)
      const measureRole = roleEl ? measureOf(roleEl) : null
      const role = measureRole ? fitText(view.role, max, measureRole) : ''
      const roleWidth = measureRole ? measureRole(role) : 0
      const textWidth = Math.ceil(Math.max(nameWidth, roleWidth))
      next[entity.id] = {
        w: view.isChar ? charWidth(view.narrator, textWidth) : otherWidth(textWidth),
        h: view.isChar ? charHeight(view.narrator) : 32,
        name,
        role,
      }
    }
    const labels: Record<string, { text: string; w: number }> = {}
    brain.edges.forEach((edge, index) => {
      const key = keyOf(edge, index)
      const element = labelText.current.get(key)
      if (!element) return
      const group = labelGroups.get(index)
      if (!group) return
      const suffix = group.count > 1 ? ` ×${group.count}` : ''
      const measure = measureOf(element)
      const text = fitText(group.label, Math.max(0, MAX_LABEL - measure(suffix)), measure) + suffix
      labels[key] = { text, w: measure(text) }
    })
    if (newText.current) setNewWidth(Math.ceil(measureOf(newText.current)(t.brain.newEdge)) || 22)
    setMeasured((current) => {
      const entries = Object.entries(next)
      return Object.keys(current).length === entries.length && entries.every(([id, value]) => {
        const previous = current[id]
        return previous && previous.w === value.w && previous.h === value.h && previous.name === value.name && previous.role === value.role
      }) ? current : next
    })
    setLabelFit((current) => {
      const entries = Object.entries(labels)
      return Object.keys(current).length === entries.length && entries.every(([id, value]) =>
        current[id]?.text === value.text && current[id]?.w === value.w) ? current : labels
    })
  }, [brain.entities, brain.edges, active, chapter, narrator, fontEpoch, labelGroups])

  const applyView = (next: [number, number, number, number]) => {
    viewBox.current = next
    svgRef.current?.setAttribute('viewBox', next.join(' '))
  }
  const fitView = () => {
    autoFit.current = false
    const bounds = graphBounds(positions, sizes)
    applyView([bounds.x, bounds.y, bounds.w, bounds.h])
    setZoom(1)
  }
  const zoomView = (factor: number, clientX?: number, clientY?: number) => {
    autoFit.current = false
    const svg = svgRef.current
    if (!svg) return
    const current = viewBox.current
    const rect = svg.getBoundingClientRect()
    const point = svg.createSVGPoint?.()
    let cx = current[0] + current[2] / 2
    let cy = current[1] + current[3] / 2
    if (point && clientX != null && clientY != null) {
      point.x = clientX; point.y = clientY
      const ctm = svg.getScreenCTM()?.inverse()
      if (ctm) { const world = point.matrixTransform(ctm); cx = world.x; cy = world.y }
    } else if (!rect.width) return
    const nextZoom = Math.max(0.2, Math.min(maxZoom * 4, zoom * factor))
    const ratio = zoom / nextZoom
    applyView([cx + (current[0] - cx) * ratio, cy + (current[1] - cy) * ratio, current[2] * ratio, current[3] * ratio])
    setZoom(nextZoom)
  }

  useLayoutEffect(() => {
    if (!viewportReady.current) {
      const bounds = graphBounds(positions, sizes)
      viewBox.current = [bounds.x, bounds.y, bounds.w, bounds.h]
      viewportReady.current = true
    }
    svgRef.current?.setAttribute('viewBox', viewBox.current.join(' '))
  }, [graph, compact])
  const worldAt = (svg: SVGSVGElement, x: number, y: number) => {
    const point = svg.createSVGPoint()
    point.x = x; point.y = y
    return point.matrixTransform(svg.getScreenCTM()!.inverse())
  }
  const endPointer = (id: number) => {
    pointers.current.delete(id)
    const current = drag.current
    if (current?.id === id) {
      if (current.node) {
        graph.simulation.tick(1)
        current.node.fx = null; current.node.fy = null
        graph.simulation.alphaTarget(0)
        resumeRef.current()
        drawRef.current()
      }
      suppressClick.current = current.moved
      drag.current = null
    }
    if (pointers.current.size < 2 && pinch.current) {
      setZoom(pinch.current.zoom * pinch.current.view[2] / viewBox.current[2])
      pinch.current = null
    }
  }

  const shownKeys = new Set(edges.filter((entry) => visible.has(entry.edge.a) && visible.has(entry.edge.b)).map((entry) => keyOf(entry.edge, entry.index)))
  const highlighted = edges
    .filter((entry) => shownKeys.has(keyOf(entry.edge, entry.index)) && labelGroups.has(entry.index) && entry.item && (entry.edge.a === focus || entry.edge.b === focus))
    .sort((left, right) => (labelFit[keyOf(right.edge, right.index)]?.w ?? 0) - (labelFit[keyOf(left.edge, left.index)]?.w ?? 0))
  const labelled = highlighted
  const nodeBoxes: Rect[] = []
  for (const id of visible) {
    const point = positions.get(id)
    if (point) nodeBoxes.push(nodeRect(point, box(id)))
  }
  const labelIndex = new LabelCollisionIndex(nodeBoxes)
  const labelPlacement = new Map<string, { x: number; y: number; w: number }>()
  for (const entry of labelled) {
    const key = keyOf(entry.edge, entry.index)
    const fit = labelFit[key]
    if (!fit) continue
    const width = labelWidth(fit.w, !!labelGroups.get(entry.index)?.fresh, newWidth)
    const spot = placeForceLabel(entry.curve, width, labelIndex, canvas)
    if (!spot) continue
    labelPlacement.set(key, { x: spot.x, y: spot.y, w: width })
  }

  // Tick updates bypass React; card content remains the existing artboard SVG.
  useLayoutEffect(() => {
    const svg = svgRef.current
    if (!svg) return
    const nodeElements = new Map([...svg.querySelectorAll<SVGGElement>('.bn')].map((element) => [element.dataset.id!, element]))
    const edgeElements = new Map([...svg.querySelectorAll<SVGGElement>('[data-edge]')].map((element) => [Number(element.dataset.edge), element]))
    const labelElements = new Map([...svg.querySelectorAll<SVGGElement>('.bl')].map((element) => [Number(element.dataset.edgeLabel), element]))
    const gradients = new Map([...svg.querySelectorAll<SVGLinearGradientElement>('[data-gradient]')].map((element) => [Number(element.dataset.gradient), element]))
    const previous = new Map<string, { x: number; y: number }>()
    const curves = new Map(edges.map((entry) => [entry.index, entry.curve]))
    const edgeParts = new Map([...edgeElements].map(([index, element]) => [index, {
      paths: [...element.querySelectorAll('path')], circles: [...element.querySelectorAll('circle')],
    }]))
    const draw = () => {
      const bounds = graphBounds(positions, sizes)
      const boxes: Rect[] = []
      const changed = new Set<string>()
      for (const [id, point] of positions) {
        const last = previous.get(id)
        if (!last || Math.abs(last.x - point.x) > 0.001 || Math.abs(last.y - point.y) > 0.001) {
          changed.add(id)
          nodeElements.get(id)?.setAttribute('transform', `translate(${point.x} ${point.y})`)
          if (last) { last.x = point.x; last.y = point.y }
          else previous.set(id, { x: point.x, y: point.y })
        }
        if (visible.has(id)) boxes.push(nodeRect(point, sizes[id]))
        const memory = positionMemory.current.get(id)
        if (memory) { memory.x = point.x; memory.y = point.y }
      }
      if (!changed.size) return
      for (const entry of edges) {
        if (!changed.has(entry.edge.a) && !changed.has(entry.edge.b)) continue
        const a = positions.get(entry.edge.a)!
        const b = positions.get(entry.edge.b)!
        const curve = edgeCurve(a, b, sizes[entry.edge.a], sizes[entry.edge.b], entry.index)
        curves.set(entry.index, curve)
        const parts = edgeParts.get(entry.index)
        if (parts) {
          for (const path of parts.paths) path.setAttribute('d', curve.d)
          parts.circles[0]?.setAttribute('cx', String(curve.ax)); parts.circles[0]?.setAttribute('cy', String(curve.ay))
          parts.circles[1]?.setAttribute('cx', String(curve.bx)); parts.circles[1]?.setAttribute('cy', String(curve.by))
        }
        const gradient = gradients.get(entry.index)
        gradient?.setAttribute('x1', String(curve.ax)); gradient?.setAttribute('y1', String(curve.ay))
        gradient?.setAttribute('x2', String(curve.bx)); gradient?.setAttribute('y2', String(curve.by))
      }
      const labelIndex = new LabelCollisionIndex(boxes)
      const shown = new Set<number>()
      for (const entry of highlighted) {
        const key = keyOf(entry.edge, entry.index)
        const fit = labelFit[key]
        const element = labelElements.get(entry.index)
        if (!fit || !element) continue
        const width = labelWidth(fit.w, !!labelGroups.get(entry.index)?.fresh, newWidth)
        const spot = placeForceLabel(curves.get(entry.index)!, width, labelIndex, bounds)
        if (!spot) continue
        shown.add(entry.index)
        const initial = labelPlacement.get(key)
        const transform = `translate(${spot.x - (initial?.x ?? 0)} ${spot.y - (initial?.y ?? 0)})`
        const rect = spot.rect.join(' ')
        if (element.getAttribute('transform') !== transform) element.setAttribute('transform', transform)
        if (element.dataset.labelRect !== rect) element.setAttribute('data-label-rect', rect)
        if (element.getAttribute('visibility') !== 'visible') {
          element.setAttribute('opacity', '1')
          element.setAttribute('visibility', 'visible')
        }
      }
      for (const [index, element] of labelElements) if (!shown.has(index) && element.getAttribute('visibility') !== 'hidden') {
        element.setAttribute('visibility', 'hidden')
        element.setAttribute('opacity', '0')
        element.removeAttribute('data-label-rect')
      }
      if (autoFit.current) {
        viewBox.current = [bounds.x, bounds.y, bounds.w, bounds.h]
        svg.setAttribute('viewBox', viewBox.current.join(' '))
      }
    }
    drawRef.current = draw
    draw()
  }, [graph, edges, visible, focus, labelFit, labelGroups, newWidth])

  useEffect(() => {
    const svg = svgRef.current
    if (!svg) return
    let inView = true
    let frame = 0
    let running = false
    let worker: Worker | null = null
    const cancelWarmup = () => {
      worker?.terminate()
      worker = null
    }
    const step = () => {
      frame = 0
      if (!running || document.hidden || !inView) return
      graph.simulation.tick(1)
      drawRef.current()
      if (!reduce && graph.simulation.alpha() >= graph.simulation.alphaMin()) {
        frame = requestAnimationFrame(step)
      } else {
        running = false
        autoFit.current = false
        svg.dataset.simulation = 'settled'
      }
    }
    const pause = () => {
      running = false
      cancelAnimationFrame(frame)
      frame = 0
      graph.simulation.stop()
      cancelWarmup()
      svg.dataset.simulation = 'paused'
    }
    const resume = () => {
      if (running || document.hidden || !inView) return
      if (graph.warmupRemaining > 0) {
        if (worker) return
        const next = new Worker(new URL('./forceGraph.worker.ts', import.meta.url), { type: 'module' })
        worker = next
        svg.dataset.simulation = 'warming'
        next.onmessage = ({ data }: MessageEvent<ForceWarmupResult>) => {
          if (worker !== next) return
          cancelWarmup()
          for (const point of data.nodes) Object.assign(positions.get(point.id)!, point)
          graph.warmupRemaining = 0
          graph.simulation.alpha(reduce ? data.alpha : 0.003)
          drawRef.current()
          if (reduce) {
            autoFit.current = false
            svg.dataset.simulation = 'settled'
          } else resume()
        }
        const payload: ForceWarmupRequest = {
          entities: knownEntities.map(({ id, type }) => ({ id, type })),
          edges: active,
          sizes: Object.fromEntries(graph.nodes.map(({ id, w, h }) => [id, { w, h }])),
          previous: graph.nodes.map(({ id, x, y }) => [id, { x, y }]),
          retained: [...retainedIds.current],
        }
        next.postMessage(payload)
        return
      }
      if (graph.simulation.alpha() < graph.simulation.alphaMin()) {
        svg.dataset.simulation = 'settled'
        return
      }
      running = true
      svg.dataset.simulation = 'running'
      frame = requestAnimationFrame(step)
    }
    resumeRef.current = resume
    cancelWarmupRef.current = () => {
      cancelWarmup()
      graph.warmupRemaining = 0
    }
    const visibility = () => { if (document.hidden) pause(); else resume() }
    document.addEventListener('visibilitychange', visibility)
    const observer = typeof IntersectionObserver === 'undefined' ? null : new IntersectionObserver(([entry]) => {
      const opening = !inView && entry.isIntersecting
      inView = entry.isIntersecting
      if (opening && compact) {
        const bounds = graphBounds(positions, sizes)
        viewBox.current = [bounds.x, bounds.y, bounds.w, bounds.h]
        svg.setAttribute('viewBox', viewBox.current.join(' '))
        setZoom(1)
      }
      if (inView) resume(); else pause()
    })
    observer?.observe(svg)
    resume()
    return () => {
      pause()
      observer?.disconnect()
      document.removeEventListener('visibilitychange', visibility)
      if (drag.current?.node) { drag.current.node.fx = null; drag.current.node.fy = null }
      graph.simulation.alphaTarget(0)
      drag.current = null
      pointers.current.clear()
      pinch.current = null
      resumeRef.current = () => {}
      cancelWarmupRef.current = () => {}
    }
  }, [graph, reduce, compact])
  const visibleCount = brain.entities.filter((entity) => alive.has(entity.id) && !hiddenTypes.has(entity.type)).length
  const summary = t.brain.graphSummary(chapter, visibleCount, active.length)
  const lowered = query.trim().toLowerCase()

  const nodeAt = (id: string): ReactNode => {
    const entity = brain.entities.find((item) => item.id === id)
    const point = positions.has(id) ? { x: 0, y: 0 } : null
    if (!entity || !point) return null
    const view = entityView(entity, chapter, narrator)
    const { w, h } = box(id)
    const isSel = id === selected
    const isHover = id === hover
    const isNear = near.has(id)
    const isVisible = visible.has(id)
    const matches = !lowered || entity.name.toLowerCase().includes(lowered)
    const rx = h / 2
    const x0 = point.x - w / 2
    const y0 = point.y - h / 2
    const halo = isSel ? 0.42 : isHover ? 0.22 : isNear && focus !== id ? 0.08 : 0
    const plate = isSel ? view.ring : isHover || isNear ? 'var(--kit-wash-16)' : 'var(--line-3)'
    const label = `${entity.name} · ${view.type.label}${isSel ? t.brain.selectedSuffix : ''}`
    const nameRef = (element: SVGTextElement | null) => {
      if (element) nodeText.current.set(`${id}:name`, element)
      else nodeText.current.delete(`${id}:name`)
    }
    const roleRef = (element: SVGTextElement | null) => {
      if (element) nodeText.current.set(`${id}:role`, element)
      else nodeText.current.delete(`${id}:role`)
    }
    const r = charRadius(view.narrator)
    const badge = view.isChar ? badgeGeometry(r * 2) : null
    const avatarX = x0 + 6 + r
    const textX = avatarX + r + 10
    const glyphX = x0 + 10
    return (
      <g key={id} className="bn" data-id={id} data-highlighted={near.has(id) && visible.has(id) ? 'true' : 'false'}
        transform={`translate(${positions.get(id)!.x} ${positions.get(id)!.y})`} aria-hidden="true"
        aria-label={label}
        style={{ pointerEvents: isVisible ? 'auto' : 'none' }}
        onClick={() => { if (!suppressClick.current) onSelect(id) }}
        onMouseOver={() => { if (isVisible) setHover(id) }}
        onMouseOut={() => setHover((value) => (value === id ? null : value))}>
        <title>{`${entity.name} · ${view.isChar ? view.role : view.type.label}`}</title>
        <g className="bn-body" style={{
          transformOrigin: `${point.x}px ${point.y}px`,
          transform: isVisible ? 'scale(1)' : 'scale(0.7)',
          opacity: isVisible ? 1 : 0,
        }}>
          <rect className="bn-halo" x={x0 - 4} y={y0 - 4} width={w + 8} height={h + 8} rx={rx + 4}
            style={{ fill: view.ring }} opacity={halo} filter={isSel || isHover ? paint('bblur') : undefined} />
          <rect x={x0} y={y0} width={w} height={h} rx={rx} style={{ fill: 'var(--brain-node-back)' }} />
          <g className="bn-cont" style={{ opacity: !matches ? 0.18 : !focus || isNear ? 1 : 0.22 }}>
            <rect x={x0} y={y0} width={w} height={h} rx={rx} style={{ fill: paint('bcard') }}
              stroke={plate} strokeOpacity={isSel ? 0.85 : 1} strokeWidth={1} />
            <rect x={x0} y={y0} width={w} height={h} rx={rx} fill="none" stroke={paint('bcardhi')}
              strokeWidth={1} pointerEvents="none" />
            {view.isChar ? (
              <>
                <circle cx={avatarX} cy={point.y} r={r + 2.5} fill="none" stroke={view.kindMeta.border} strokeOpacity={0.25} strokeWidth={1} />
                {/* Màu theo loại vai (Gói S); chưa lộ thì viền nét đứt như avatar trong danh sách. */}
                <circle cx={avatarX} cy={point.y} r={r} fill={view.kindMeta.background} stroke={view.kindMeta.border}
                  strokeWidth={1.5} strokeDasharray={view.hidden ? '3 2.4' : undefined} />
                <text x={avatarX} y={point.y + (view.narrator ? 4.6 : 4)} textAnchor="middle"
                  fontSize={view.narrator ? 12 : 10.5} fontWeight={700} fillOpacity={0.95}
                  style={{ fill: view.isChar ? view.kindMeta.fg : 'var(--kit-white)', letterSpacing: '0.02em' }}>{view.initials}</text>
                {/* Huy hiệu góc như avatar DOM (artboard `.cav-b`): cùng `badgeGeometry` để không lệch cỡ. */}
                {badge && view.kindMeta.icon && (
                  <g pointerEvents="none">
                    <circle cx={avatarX + badge.dx} cy={point.y + badge.dx} r={badge.size / 2}
                      fill="var(--kind-badge-bg)" stroke={view.kindMeta.border} strokeWidth={badge.border} />
                    <foreignObject x={avatarX + badge.dx - badge.icon / 2} y={point.y + badge.dx - badge.icon / 2}
                      width={badge.icon} height={badge.icon}>
                      <div style={{ width: badge.icon, height: badge.icon, color: view.kindMeta.border }}>
                        <Icon name={view.kindMeta.icon} size={badge.icon} />
                      </div>
                    </foreignObject>
                  </g>
                )}
                <text ref={nameRef} x={textX} y={point.y - (view.narrator ? 2 : 1.5)} fontSize={view.narrator ? 15 : 13.5}
                  fontWeight={600} style={{ fill: isSel ? 'var(--kit-white)' : 'var(--brain-label-fg)' }}>
                  {measured[id]?.name ?? entity.name}
                </text>
                <text ref={roleRef} x={textX} y={point.y + (view.narrator ? 15 : 13.5)} fontSize={view.narrator ? 11.5 : 11}
                  style={{ fill: 'var(--brain-node-sub)' }}>{measured[id]?.role ?? view.role}</text>
              </>
            ) : (
              <>
                <rect x={glyphX - 3} y={point.y - 11} width={22} height={22} rx={7} fill={view.type.color} fillOpacity={0.12} />
                <path d={GLYPH[entity.type] ?? ''} transform={`translate(${glyphX} ${point.y - 8})`} fill="none"
                  stroke={view.type.color} strokeWidth={1.35} strokeLinecap="round" strokeLinejoin="round" />
                <text ref={nameRef} x={glyphX + 16 + 10} y={point.y + 4.4} fontSize={12.5} fontWeight={500}
                  style={{ fill: isSel ? 'var(--kit-white)' : 'var(--brain-node-name)' }}>
                  {measured[id]?.name ?? entity.name}
                </text>
              </>
            )}
          </g>
          <rect className="bn-ring" x={x0 - 3.5} y={y0 - 3.5} width={w + 7} height={h + 7} rx={rx + 3.5} fill="none"
            stroke={view.ring} strokeWidth={1.3} opacity={isSel ? 0.35 : 0} />
          <g opacity={isVisible && isNewAt(entity, chapter) ? 1 : 0} pointerEvents="none">
            <rect x={x0 + w - 22} y={y0 - 8} width={30} height={16} rx={8}
              style={{ fill: 'var(--color-accent)' }} stroke="var(--brain-fade)" strokeWidth={2} />
            <text x={x0 + w - 7} y={y0 + 3.4} textAnchor="middle" fontSize={9.5} fontWeight={700}
              style={{ fill: 'var(--color-bg)' }}>{t.brain.newNode}</text>
          </g>
        </g>
      </g>
    )
  }
  const wrapStyle = stylex.props(graphStyles.wrap, compact && graphStyles.compact)
  const hintStyle = stylex.props(graphStyles.hint, compact && graphStyles.compactHint)
  const hostStyle = stylex.props(graphStyles.host)

  return (
    <div {...wrapStyle} className={`brain-wrap ${wrapStyle.className}`} data-compact={compact || undefined}>
      <div {...hostStyle} className={`brain-host ${hostStyle.className}`}>
        <svg ref={svgRef} viewBox={`${VIEW.x} ${VIEW.y} ${VIEW.w} ${VIEW.h}`} preserveAspectRatio="xMidYMid meet"
          role="presentation" aria-hidden="true" onMouseLeave={() => setHover(null)}
          style={{ touchAction: 'none' }}
          onDoubleClick={(event) => {
            if (!(event.target as Element).closest('[data-id]')) fitView()
          }}
          onPointerDown={(event) => {
            autoFit.current = false
            if (event.button !== 0) return
            const svg = event.currentTarget
            pointers.current.set(event.pointerId, { x: event.clientX, y: event.clientY })
            suppressClick.current = false
            if (pointers.current.size === 2) {
              if (drag.current?.node) { drag.current.node.fx = null; drag.current.node.fy = null }
              graph.simulation.alphaTarget(0)
              drag.current = null
              const [a, b] = [...pointers.current.values()]
              pinch.current = {
                distance: Math.hypot(a.x - b.x, a.y - b.y), view: [...viewBox.current],
                x: (a.x + b.x) / 2, y: (a.y + b.y) / 2, zoom,
              }
            } else {
              const id = (event.target as Element).closest<SVGGElement>('[data-id]')?.dataset.id
              const node = id ? positions.get(id) ?? null : null
              const point = worldAt(svg, event.clientX, event.clientY)
              drag.current = {
                id: event.pointerId, x: event.clientX, y: event.clientY, view: [...viewBox.current],
                node, offsetX: node ? node.x - point.x : 0, offsetY: node ? node.y - point.y : 0, moved: false,
              }
              if (node) {
                cancelWarmupRef.current()
                graph.releaseAnchors()
                retainedIds.current.clear()
                node.fx = node.x; node.fy = node.y
                graph.simulation.alpha(0.07).alphaTarget(reduce ? 0 : 0.035)
                resumeRef.current()
              }
            }
            svg.setPointerCapture(event.pointerId)
          }}
          onPointerMove={(event) => {
            if (!pointers.current.has(event.pointerId)) return
            pointers.current.set(event.pointerId, { x: event.clientX, y: event.clientY })
            const gesture = pinch.current
            if (gesture && pointers.current.size === 2) {
              const [a, b] = [...pointers.current.values()]
              const ratio = Math.max(0.1, Math.min(10, gesture.distance / Math.max(1, Math.hypot(a.x - b.x, a.y - b.y))))
              const rect = event.currentTarget.getBoundingClientRect()
              const scale = Math.max(gesture.view[2] / rect.width, gesture.view[3] / rect.height)
              const cx = gesture.view[0] + gesture.view[2] / 2
              const cy = gesture.view[1] + gesture.view[3] / 2
              applyView([
                cx + (gesture.view[0] - cx) * ratio - ((a.x + b.x) / 2 - gesture.x) * scale,
                cy + (gesture.view[1] - cy) * ratio - ((a.y + b.y) / 2 - gesture.y) * scale,
                gesture.view[2] * ratio, gesture.view[3] * ratio,
              ])
              suppressClick.current = true
              return
            }
            const current = drag.current
            if (!current || current.id !== event.pointerId) return
            if (Math.hypot(event.clientX - current.x, event.clientY - current.y) > 3) current.moved = true
            if (current.node) {
              const point = worldAt(event.currentTarget, event.clientX, event.clientY)
              current.node.fx = point.x + current.offsetX
              current.node.fy = point.y + current.offsetY
              current.node.x = current.node.fx; current.node.y = current.node.fy
              if (reduce) { graph.simulation.tick(1); drawRef.current() }
            } else {
              const rect = event.currentTarget.getBoundingClientRect()
              const scale = Math.max(current.view[2] / rect.width, current.view[3] / rect.height)
              applyView([
                current.view[0] - (event.clientX - current.x) * scale,
                current.view[1] - (event.clientY - current.y) * scale,
                current.view[2], current.view[3],
              ])
            }
          }}
          onPointerUp={(event) => { endPointer(event.pointerId); event.currentTarget.releasePointerCapture(event.pointerId) }}
          onPointerCancel={(event) => endPointer(event.pointerId)}
          onLostPointerCapture={(event) => endPointer(event.pointerId)}
          onWheel={(event) => zoomView(Math.exp(-event.deltaY * 0.002), event.clientX, event.clientY)}>
          <defs>
            <radialGradient id={svgId('bglow')} cx="50%" cy="50%" r="50%">
              <stop offset="0" style={{ stopColor: 'var(--brain-glow)' }} stopOpacity={0.1} />
              <stop offset="0.6" style={{ stopColor: 'var(--brain-glow)' }} stopOpacity={0.025} />
              <stop offset="1" style={{ stopColor: 'var(--brain-glow)' }} stopOpacity={0} />
            </radialGradient>
            <radialGradient id={svgId('bz-fade')} cx="50%" cy="50%" r="62%">
              <stop offset="0.7" style={{ stopColor: 'var(--brain-fade)' }} stopOpacity={0} />
              <stop offset="1" style={{ stopColor: 'var(--brain-fade)' }} stopOpacity={0.55} />
            </radialGradient>
            <filter id={svgId('bblur')} x="-60%" y="-60%" width="220%" height="220%">
              <feGaussianBlur stdDeviation={9} />
            </filter>
            <linearGradient id={svgId('bcard')} x1={0} y1={0} x2={0} y2={1}>
              <stop offset="0" style={{ stopColor: 'var(--brain-card-top)' }} />
              <stop offset="1" style={{ stopColor: 'var(--brain-card-bottom)' }} />
            </linearGradient>
            <linearGradient id={svgId('bcardhi')} x1={0} y1={0} x2={0} y2={1}>
              <stop offset="0" style={{ stopColor: 'var(--kit-white)' }} stopOpacity={0.16} />
              <stop offset="0.5" style={{ stopColor: 'var(--kit-white)' }} stopOpacity={0.05} />
              <stop offset="1" style={{ stopColor: 'var(--kit-white)' }} stopOpacity={0.03} />
            </linearGradient>
            {edges.map((entry) => (
              <linearGradient key={entry.index} data-gradient={entry.index} id={svgId(`bg-${entry.index}`)} gradientUnits="userSpaceOnUse"
                x1={entry.curve.ax} y1={entry.curve.ay} x2={entry.curve.bx} y2={entry.curve.by}>
                <stop offset="0" style={{ stopColor: entry.item ? EDGE_META[entry.item.phase.type].color : 'transparent' }} stopOpacity={0.25} />
                <stop offset="0.5" style={{ stopColor: entry.item ? EDGE_META[entry.item.phase.type].color : 'transparent' }} stopOpacity={1} />
                <stop offset="1" style={{ stopColor: entry.item ? EDGE_META[entry.item.phase.type].color : 'transparent' }} stopOpacity={0.25} />
              </linearGradient>
            ))}
          </defs>
          <ellipse cx={VIEW.cx} cy={VIEW.cy} rx={420} ry={360} fill={paint('bglow')} />

          <g>
            {edges.map((entry) => {
              const key = keyOf(entry.edge, entry.index)
              const meta = entry.item ? EDGE_META[entry.item.phase.type] : null
              const hi = !!entry.item && (entry.edge.a === focus || entry.edge.b === focus)
              return (
                <g key={key} data-edge={entry.index} opacity={shownKeys.has(key) ? 1 : 0} pointerEvents="none">
                  <path className="be" d={entry.curve.d} fill="none" strokeLinecap="round"
                    style={{ stroke: paint(`bg-${entry.index}`) }} strokeWidth={hi ? 1.8 : 1.2}
                    opacity={hi ? 1 : meta?.dashed ? 0.22 : 0.4} strokeDasharray={meta?.dashed ? '3.5 5' : 'none'} />
                  <circle cx={entry.curve.ax} cy={entry.curve.ay} r={2.2} className="be"
                    style={{ fill: meta?.color ?? 'transparent' }} opacity={hi ? 0.95 : 0} />
                  <circle cx={entry.curve.bx} cy={entry.curve.by} r={2.2} className="be"
                    style={{ fill: meta?.color ?? 'transparent' }} opacity={hi ? 0.95 : 0} />
                </g>
              )
            })}
          </g>

          <g>{brain.entities.map((entity) => nodeAt(entity.id))}</g>

          <g>
            {edges.map((entry) => {
              const key = keyOf(entry.edge, entry.index)
              const spot = labelPlacement.get(key)
              const meta = entry.item ? EDGE_META[entry.item.phase.type] : null
              const width = spot?.w ?? (labelFit[key] ? labelWidth(labelFit[key].w, !!labelGroups.get(entry.index)?.fresh, newWidth) : 0)
              const x = spot?.x ?? 0
              const y = spot?.y ?? 0
              const left = x - width / 2
              const fit = labelFit[key]
              const group = labelGroups.get(entry.index)
              const full = group ? group.label + (group.count > 1 ? ` ×${group.count}` : '') : ''
              return (
                <g key={key} className="bl" data-edge-label={entry.index} opacity={spot ? 1 : 0} visibility={spot ? undefined : 'hidden'} pointerEvents="none">
                  <title>{full}</title>
                  <rect x={left} y={y - 10.5} width={width} height={21} rx={10.5}
                    style={{ fill: 'var(--brain-label-bg)', stroke: meta?.color ?? 'transparent', strokeOpacity: 0.45 }} strokeWidth={1} />
                  <circle cx={left + 12} cy={y} r={2.6} style={{ fill: meta?.color ?? 'transparent' }} />
                  <text ref={(element) => {
                    if (element) labelText.current.set(key, element)
                    else labelText.current.delete(key)
                  }} x={left + 21} y={y + 3.9} fontSize={11} fontWeight={500}
                    style={{ fill: 'var(--brain-label-fg)' }}>{fit?.text ?? full}</text>
                  <text ref={entry.index === 0 ? (element) => { newText.current = element } : undefined}
                    x={left + 21 + (fit?.w ?? 0) + 7} y={y + 3.5} fontSize={9.5} fontWeight={700}
                    style={{ fill: 'var(--color-accent)', letterSpacing: '0.04em', display: group?.fresh ? undefined : 'none' }}>{t.brain.newEdge}</text>
                </g>
              )
            })}
          </g>
          <rect x={canvas.x - canvas.w} y={canvas.y - canvas.h} width={canvas.w * 3} height={canvas.h * 3} fill={paint('bz-fade')} pointerEvents="none" />
        </svg>
      </div>
      <ul {...stylex.props(graphStyles.accessible)} aria-label={summary}>
        {knownEntities.filter((entity) => visible.has(entity.id)).map((entity) => (
          <li key={entity.id}>
            <button type="button" data-node-id={entity.id} aria-pressed={selected === entity.id}
              onFocus={() => setHover(entity.id)} onBlur={() => setHover(null)}
              onClick={() => onSelect(entity.id)}
              onKeyDown={(event) => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault()
                  onSelect(entity.id)
                  return
                }
                const direction = event.key === 'ArrowRight' || event.key === 'ArrowDown' ? 1
                  : event.key === 'ArrowLeft' || event.key === 'ArrowUp' ? -1 : 0
                if (!direction) return
                event.preventDefault()
                const buttons = [...event.currentTarget.closest('ul')!.querySelectorAll('button')]
                buttons[(buttons.indexOf(event.currentTarget) + direction + buttons.length) % buttons.length]?.focus()
              }}>{entity.name} · {entityView(entity, chapter, narrator).role}</button>
          </li>
        ))}
      </ul>

      <div className="brain-gcap" aria-hidden="true">
        <b>{t.brain.atChapter(chapter)}</b>
        <span>{t.brain.count(visibleCount, active.length)}</span>
      </div>
      {compact && (
        <div {...stylex.props(graphStyles.legend)}>
          <KindLegend style={{ paddingTop: 0, borderTopWidth: 0, flexWrap: 'wrap', gap: '4px 12px' }} />
        </div>
      )}
      <div {...hintStyle} className={`brain-ghint ${hintStyle.className}`} aria-hidden="true">
        <Icon name="info-circle" size={13} />{compact ? t.brain.compactHint : t.brain.graphHint}
      </div>
      {!compact && <div className="brain-gctl" role="toolbar" aria-label="Điều khiển đồ thị">
        <button type="button" aria-pressed={focusOnly} onClick={() => setFocusOnly((value) => !value)}
          title={t.brain.focusTitle}>
          <Icon name="eye" size={14} />{t.brain.focusOnly}
        </button>
        <span className="sep" />
        <button type="button" onClick={() => zoomView(0.8)} aria-label={t.brain.zoomOut}>
          <Icon name="minus" size={14} />
        </button>
        <button type="button" onClick={fitView} aria-label={t.brain.zoomFit} className="num" style={{ minWidth: 46 }}>
          {`${Math.round(zoom * 100)}%`}
        </button>
        <button type="button" onClick={() => zoomView(1.25)} aria-label={t.brain.zoomIn}>
          <Icon name="add" size={14} />
        </button>
      </div>}
    </div>
  )
}
