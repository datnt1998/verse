import { forceCenter, forceLink, forceManyBody, forceSimulation, type Force, type Simulation, type SimulationNodeDatum } from 'd3-force'
import type { BrainEntity } from '../../api/types'
import { VIEW, pointOnCurve, type ActiveEdge, type Box, type Curve, type GraphBounds, type Point, type Rect } from './model'

export type ForceNode = SimulationNodeDatum & Point & Box & { id: string }
export type ForceGraph = {
  nodes: ForceNode[]
  positions: Map<string, ForceNode>
  simulation: Simulation<ForceNode, undefined>
  releaseAnchors: () => void
  warmupRemaining: number
}

export type ForceWarmupRequest = {
  entities: Pick<BrainEntity, 'id' | 'type'>[]
  edges: ActiveEdge[]
  sizes: Record<string, Box>
  previous: [string, Point][]
  retained: string[]
}
export type ForceWarmupResult = {
  nodes: Pick<ForceNode, 'id' | 'x' | 'y' | 'vx' | 'vy'>[]
  alpha: number
}

/** Include the endpoint itself once, irrespective of repeated or reversed edges. */
export const neighboursOf = (edges: readonly ActiveEdge[], id: string) => {
  const ids = new Set([id])
  for (const { edge } of edges) {
    if (edge.a === id) ids.add(edge.b)
    if (edge.b === id) ids.add(edge.a)
  }
  return ids
}

/** Spatial buckets keep hub-label placement local rather than scanning every
 * card for every candidate on every animation frame. */
export class LabelCollisionIndex {
  private readonly cells = new Map<string, Rect[]>()

  constructor(rects: readonly Rect[]) {
    for (const rect of rects) this.add(rect)
  }

  add(rect: Rect) {
    for (let x = Math.floor(rect[0] / 128); x <= Math.floor(rect[2] / 128); x++) {
      for (let y = Math.floor(rect[1] / 64); y <= Math.floor(rect[3] / 64); y++) {
        const key = `${x}:${y}`
        const bucket = this.cells.get(key)
        if (bucket) bucket.push(rect)
        else this.cells.set(key, [rect])
      }
    }
  }

  overlaps(rect: Rect) {
    for (let x = Math.floor(rect[0] / 128); x <= Math.floor(rect[2] / 128); x++) {
      for (let y = Math.floor(rect[1] / 64); y <= Math.floor(rect[3] / 64); y++) {
        const bucket = this.cells.get(`${x}:${y}`)
        if (bucket) for (const other of bucket) {
          if (rect[0] < other[2] && rect[2] > other[0] && rect[1] < other[3] && rect[3] > other[1]) return true
        }
      }
    }
    return false
  }
}

/** Greedy edge pills: skip rather than displace labels away from the relation. */
export const placeForceLabel = (curve: Curve, width: number, index: LabelCollisionIndex, bounds: GraphBounds) => {
  for (const step of [0.5, 0.35, 0.65, 0.2, 0.8]) {
    const point = pointOnCurve(curve, step)
    for (const dy of [0, -25, 25, -48, 48]) {
      const x = point.x
      const y = point.y + dy
      const rect: Rect = [x - width / 2, y - 10.5, x + width / 2, y + 10.5]
      if (rect[0] < bounds.x || rect[1] < bounds.y || rect[2] > bounds.x + bounds.w || rect[3] > bounds.y + bounds.h) continue
      if (index.overlaps(rect)) continue
      index.add(rect)
      return { x, y, rect }
    }
  }
  return null
}
/** Hard rectangle collision, not circumscribed circles (which waste card space).
 * Integrate the predicted displacement before projection so a tick cannot paint
 * intersecting cards. Retained chapter nodes are anchors; only new nodes settle.
 */
const cardCollide = (anchors: ReadonlyMap<string, Point>): Force<ForceNode, undefined> => {
  let nodes: ForceNode[] = []
  const force: Force<ForceNode, undefined> = () => {
    for (const node of nodes) {
      const anchor = anchors.get(node.id)
      node.x = node.fx ?? anchor?.x ?? node.x + (node.vx ?? 0) * 0.6
      node.y = node.fy ?? anchor?.y ?? node.y + (node.vy ?? 0) * 0.6
      node.vx = 0
      node.vy = 0
    }
    for (let pass = 0; pass < 12; pass++) {
      let hits = 0
      for (let i = 0; i < nodes.length; i++) {
        const a = nodes[i]
        for (let j = i + 1; j < nodes.length; j++) {
          const b = nodes[j]
          const dx = b.x - a.x
          const dy = b.y - a.y
          const ox = (a.w + b.w) / 2 + 16 - Math.abs(dx)
          const oy = (a.h + b.h) / 2 + 16 - Math.abs(dy)
          if (ox <= 0 || oy <= 0) continue
          const aFixed = a.fx != null || anchors.has(a.id)
          const bFixed = b.fx != null || anchors.has(b.id)
          if (aFixed && bFixed) continue
          hits++
          const share = aFixed || bFixed ? 1 : 0.5
          if (ox < oy) {
            const shift = (ox + 0.01) * (dx < 0 ? -1 : 1) * share
            if (!aFixed) a.x -= shift
            if (!bFixed) b.x += shift
          } else {
            const shift = (oy + 0.01) * (dy < 0 ? -1 : 1) * share
            if (!aFixed) a.y -= shift
            if (!bFixed) b.y += shift
          }
        }
      }
      if (!hits) break
    }
  }
  force.initialize = (next) => { nodes = next }
  return force
}

/** Stable ordering + seeded randomness make the first warm paint repeatable.
 * A chapter cutover preserves all known positions, never relays out old cards.
 */
export const createForceGraph = (
  entities: readonly Pick<BrainEntity, 'id' | 'type'>[],
  edges: readonly ActiveEdge[],
  sizes: Readonly<Record<string, Box>>,
  previous: ReadonlyMap<string, Point> = new Map(),
  warmupTicks = 300,
  retainedIds: ReadonlySet<string> = new Set(previous.keys()),
): ForceGraph => {
  const sorted = [...entities].sort((a, b) => a.type.localeCompare(b.type, 'en') || a.id.localeCompare(b.id, 'en'))
  const spacing = Math.max(80, ...sorted.map((entity) => Math.hypot(sizes[entity.id].w + 16, sizes[entity.id].h + 16)))
  let ring = 0
  let slot = 0
  let capacity = 1
  const nodes: ForceNode[] = sorted.map((entity) => {
    const old = previous.get(entity.id)
    if (slot === capacity) {
      ring++; slot = 0
      capacity = Math.max(1, Math.floor(Math.PI / Math.asin(1 / (2 * ring))))
    }
    const angle = slot++ * 2 * Math.PI / capacity
    const radius = ring * spacing
    let x = old?.x ?? VIEW.cx + Math.cos(angle) * radius
    let y = old?.y ?? VIEW.cy + Math.sin(angle) * radius
    if (!old && previous.size) {
      const neighbours = [...neighboursOf(edges, entity.id)].filter((id) => previous.has(id))
      if (neighbours.length) {
        x = neighbours.reduce((sum, id) => sum + previous.get(id)!.x, 0) / neighbours.length + Math.cos(angle) * 100
        y = neighbours.reduce((sum, id) => sum + previous.get(id)!.y, 0) / neighbours.length + Math.sin(angle) * 100
      }
    }
    return { id: entity.id, x, y, vx: 0, vy: 0, ...sizes[entity.id] }
  })
  const positions = new Map(nodes.map((node) => [node.id, node]))
  const anchors = new Map([...previous].filter(([id]) => positions.has(id) && retainedIds.has(id)).map(([id, point]) => [id, { x: point.x, y: point.y }]))
  let seed = 2710
  const links = edges.filter(({ edge }) => positions.has(edge.a) && positions.has(edge.b))
    .map(({ edge }) => ({ source: edge.a, target: edge.b }))
    .sort((a, b) => a.source.localeCompare(b.source) || a.target.localeCompare(b.target))
  const simulation = forceSimulation(nodes).stop()
    .randomSource(() => ((seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0) / 4294967296))
    .force('link', forceLink<ForceNode, { source: string; target: string }>(links).id((node) => node.id).distance(230).strength(0.045))
    .force('charge', forceManyBody<ForceNode>().strength(-650))
    .force('center', forceCenter<ForceNode>(VIEW.cx, VIEW.cy).strength(anchors.size ? 0 : 0.08))
    .force('collide', cardCollide(anchors))
    .alphaDecay(0.025)
  simulation.alpha(anchors.size ? 0.08 : 1).tick(warmupTicks)
  return { nodes, positions, simulation, releaseAnchors: () => anchors.clear(), warmupRemaining: Math.max(0, 300 - warmupTicks) }
}
