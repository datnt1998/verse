import { createForceGraph, type ForceWarmupRequest, type ForceWarmupResult } from './forceGraph'

self.onmessage = ({ data }: MessageEvent<ForceWarmupRequest>) => {
  const graph = createForceGraph(data.entities, data.edges, data.sizes, new Map(data.previous), 300, new Set(data.retained))
  const result: ForceWarmupResult = {
    nodes: graph.nodes.map(({ id, x, y, vx, vy }) => ({ id, x, y, vx, vy })),
    alpha: graph.simulation.alpha(),
  }
  self.postMessage(result)
  graph.simulation.stop()
}
