// @vitest-environment node

import { describe, expect, it } from 'vitest'
import type { ResolvedTransform } from '@editor/types/transform'
import type { MotionModifier } from '@editor/types/motion'
import { DEFAULT_MOTION_GENERATOR_SETTINGS } from './motion-generator'
import {
  applyMotionModifiers,
  createMotionModifier,
  evaluateMotionModifiers,
  getMotionModifierSettings,
  updateMotionModifierSettings,
  type MotionModifierEvalContext,
} from './motion-modifier-eval'

const resting: ResolvedTransform = {
  x: 100,
  y: 200,
  width: 400,
  height: 300,
  anchorX: 200,
  anchorY: 150,
  rotation: 0,
  opacity: 0.9,
  cornerRadius: 0,
}

function ctx(overrides: Partial<MotionModifierEvalContext> = {}): MotionModifierEvalContext {
  return { frame: 15, fps: 30, frameWidth: 1920, frameHeight: 1080, ...overrides }
}

function modifier(overrides: Partial<MotionModifier> = {}): MotionModifier {
  return {
    id: 'm1',
    type: 'float-drift',
    enabled: true,
    amplitude: 1,
    frequency: 0.625,
    phaseFrames: 0,
    seed: 1,
    ...overrides,
  }
}

describe('motion modifier evaluation', () => {
  it('is frame-rate independent at equal wall-clock time', () => {
    // 0.5s in: frame 15 @30fps vs frame 30 @60fps must produce the same offset.
    const at30 = evaluateMotionModifiers([modifier()], ctx({ frame: 15, fps: 30 }))
    const at60 = evaluateMotionModifiers([modifier()], ctx({ frame: 30, fps: 60 }))

    expect(at60.dx).toBeCloseTo(at30.dx, 6)
    expect(at60.dy).toBeCloseTo(at30.dy, 6)
    expect(at60.dRotation).toBeCloseTo(at30.dRotation, 6)
  })

  it('scales position offset linearly with amplitude', () => {
    const weak = evaluateMotionModifiers([modifier({ amplitude: 0.5 })], ctx())
    const strong = evaluateMotionModifiers([modifier({ amplitude: 1 })], ctx())

    // Same phase, double amplitude -> double offset.
    expect(strong.dx).toBeCloseTo(weak.dx * 2, 6)
    expect(strong.dy).toBeCloseTo(weak.dy * 2, 6)
  })

  it('sums contributions from multiple modifiers additively', () => {
    const a = modifier({ id: 'a', type: 'float-drift' })
    const b = modifier({ id: 'b', type: 'micro-shake', frequency: 8, seed: 3 })

    const combined = evaluateMotionModifiers([a, b], ctx())
    const onlyA = evaluateMotionModifiers([a], ctx())
    const onlyB = evaluateMotionModifiers([b], ctx())

    expect(combined.dx).toBeCloseTo(onlyA.dx + onlyB.dx, 6)
    expect(combined.dy).toBeCloseTo(onlyA.dy + onlyB.dy, 6)
  })

  it('ignores disabled and zero-amplitude modifiers', () => {
    const disabled = evaluateMotionModifiers([modifier({ enabled: false })], ctx())
    const muted = evaluateMotionModifiers([modifier({ amplitude: 0 })], ctx())

    for (const c of [disabled, muted]) {
      expect(c.dx).toBe(0)
      expect(c.dy).toBe(0)
      expect(c.dRotation).toBe(0)
      expect(c.scaleWidth).toBe(1)
      expect(c.scaleHeight).toBe(1)
    }
  })

  it('returns the same transform reference when nothing applies (fast path)', () => {
    expect(applyMotionModifiers(resting, undefined, ctx())).toBe(resting)
    expect(applyMotionModifiers(resting, [], ctx())).toBe(resting)
    expect(applyMotionModifiers(resting, [modifier({ enabled: false })], ctx())).toBe(resting)
  })

  it('layers drift as additive position/rotation deltas around rest', () => {
    const result = applyMotionModifiers(resting, [modifier()], ctx())

    // Offsets are bounded by the derived amplitudes; box/opacity untouched by drift.
    expect(Math.abs(result.x - resting.x)).toBeGreaterThan(0)
    expect(Math.abs(result.x - resting.x)).toBeLessThanOrEqual(18)
    expect(result.width).toBe(resting.width)
    expect(result.opacity).toBe(resting.opacity)
  })

  it('breath-pulse rescales the box and nudges opacity, clamped to [0,1]', () => {
    const breath = modifier({ type: 'breath-pulse', frequency: 0.55 })
    // Pick a frame near the wave peak so the scale clearly deviates from 1.
    const result = applyMotionModifiers({ ...resting, opacity: 1 }, [breath], ctx({ frame: 13 }))

    expect(result.width).not.toBe(resting.width)
    expect(result.opacity).toBeLessThanOrEqual(1)
    expect(result.opacity).toBeGreaterThanOrEqual(0)
  })

  it('micro-shake is deterministic for a given seed and frame', () => {
    const shake = modifier({ type: 'micro-shake', frequency: 8, seed: 5 })
    const first = evaluateMotionModifiers([shake], ctx({ frame: 9 }))
    const second = evaluateMotionModifiers([shake], ctx({ frame: 9 }))

    expect(first).toEqual(second)
  })

  it('createMotionModifier staggers phase and varies seed by item index', () => {
    const settings = { ...DEFAULT_MOTION_GENERATOR_SETTINGS, staggerFrames: 4 }
    const first = createMotionModifier('float-drift', settings, 0)
    const second = createMotionModifier('float-drift', settings, 2)

    expect(first.phaseFrames).toBe(0)
    expect(second.phaseFrames).toBe(8)
    expect(second.seed).not.toBe(first.seed)
    expect(first.amplitude).toBe(settings.intensityScale)
    expect(first.version).toBe(2)
    expect(first.channelGains).toEqual({ x: 1, y: 1, rotation: 1 })
  })

  it('independently tunes or mutes typed transform channels', () => {
    const xOnly = modifier({ channelGains: { x: 1, y: 0, rotation: 0 } })
    const result = evaluateMotionModifiers([xOnly], ctx())

    expect(result.dx).not.toBe(0)
    expect(result.dy).toBe(0)
    expect(result.dRotation).toBe(0)

    const doubledX = evaluateMotionModifiers(
      [modifier({ channelGains: { x: 2, y: 0, rotation: 0 } })],
      ctx(),
    )
    expect(doubledX.dx).toBeCloseTo(result.dx * 2, 6)
  })

  it('keeps legacy v1 modifiers at full gain when channel controls are absent', () => {
    const legacy = modifier()
    const explicitV2 = modifier({
      version: 2,
      channelGains: { x: 1, y: 1, rotation: 1 },
    })

    expect(evaluateMotionModifiers([legacy], ctx())).toEqual(
      evaluateMotionModifiers([explicitV2], ctx()),
    )
  })

  it('round-trips generator settings through create/get (frequency is duration inverse)', () => {
    const settings = { ...DEFAULT_MOTION_GENERATOR_SETTINGS, intensityScale: 1.4, durationScale: 2 }
    const mod = createMotionModifier('float-drift', settings)
    const recovered = getMotionModifierSettings(mod)
    expect(recovered.intensityScale).toBeCloseTo(1.4, 6)
    expect(recovered.durationScale).toBeCloseTo(2, 6)
  })

  it('updateMotionModifierSettings tunes amplitude/frequency but keeps identity', () => {
    const mod = createMotionModifier('breath-pulse', DEFAULT_MOTION_GENERATOR_SETTINGS, 3)
    const next = updateMotionModifierSettings(mod, { intensityScale: 0.5, durationScale: 0.5 })

    expect(next.id).toBe(mod.id) // same instance — editing, not replacing
    expect(next.seed).toBe(mod.seed)
    expect(next.amplitude).toBeCloseTo(0.5, 6)
    // durationScale 0.5 → twice as fast as the 1.0 baseline.
    const baseline = createMotionModifier('breath-pulse', DEFAULT_MOTION_GENERATOR_SETTINGS)
    expect(next.frequency).toBeCloseTo(baseline.frequency * 2, 6)
    // A partial edit leaves the untouched field alone.
    const intensityOnly = updateMotionModifierSettings(mod, { intensityScale: 1 })
    expect(intensityOnly.frequency).toBe(mod.frequency)

    const channelOnly = updateMotionModifierSettings(mod, {
      channelGains: { opacity: 0 },
    })
    expect(channelOnly.version).toBe(2)
    expect(channelOnly.channelGains).toMatchObject({ width: 1, height: 1, opacity: 0 })
  })

  it('sway oscillates rotation only, peaking near a quarter period', () => {
    const sway = createMotionModifier('sway', DEFAULT_MOTION_GENERATOR_SETTINGS)
    // freq 0.5Hz -> 2s period -> quarter at 0.5s = frame 15 @30fps, sin=1.
    const peak = evaluateMotionModifiers([sway], ctx({ frame: 15 }))
    const start = evaluateMotionModifiers([sway], ctx({ frame: 0 }))
    expect(peak.dRotation).toBeCloseTo(4, 3) // ±4° at full intensity
    expect(start.dRotation).toBeCloseTo(0, 6)
    expect(peak.dx).toBe(0) // rotation only
    expect(peak.scaleWidth).toBe(1)
  })

  it('spin accumulates rotation continuously (not an oscillation)', () => {
    const spin = createMotionModifier('spin', DEFAULT_MOTION_GENERATOR_SETTINGS)
    const r0 = evaluateMotionModifiers([spin], ctx({ frame: 0 })).dRotation
    const r1 = evaluateMotionModifiers([spin], ctx({ frame: 30 })).dRotation
    const r2 = evaluateMotionModifiers([spin], ctx({ frame: 60 })).dRotation
    expect(r0).toBe(0)
    expect(r1).toBeGreaterThan(0)
    expect(r2).toBeGreaterThan(r1) // keeps growing, doesn't return
    // freq 0.3 rev/s -> 108°/s; at 1s ≈ 108°.
    expect(r1).toBeCloseTo(108, 3)
  })

  it('createMotionModifier slows oscillation as duration scale grows', () => {
    const fast = createMotionModifier('float-drift', {
      ...DEFAULT_MOTION_GENERATOR_SETTINGS,
      durationScale: 1,
    })
    const slow = createMotionModifier('float-drift', {
      ...DEFAULT_MOTION_GENERATOR_SETTINGS,
      durationScale: 2,
    })

    expect(slow.frequency).toBeCloseTo(fast.frequency / 2, 6)
  })
})

describe('wander modifier', () => {
  const wander = (overrides: Partial<MotionModifier> = {}): MotionModifier =>
    modifier({ type: 'wander', frequency: 0.25, seed: 7, ...overrides })

  it('is deterministic for a given seed and frame', () => {
    const first = evaluateMotionModifiers([wander()], ctx({ frame: 47 }))
    const second = evaluateMotionModifiers([wander()], ctx({ frame: 47 }))

    expect(first).toEqual(second)
  })

  it('follows different paths for different seeds', () => {
    let divergence = 0
    for (let frame = 0; frame < 90; frame += 1) {
      const a = evaluateMotionModifiers([wander({ seed: 7 })], ctx({ frame }))
      const b = evaluateMotionModifiers([wander({ seed: 42 })], ctx({ frame }))
      divergence += Math.abs(a.dx - b.dx) + Math.abs(a.dy - b.dy)
    }

    expect(divergence).toBeGreaterThan(1)
  })

  it('stays inside amplitude and scale bounds over two hours at 30fps', () => {
    // ctx() frames at 1920x1080: x amp = 23.04px, y amp = 12.96px, scale ±0.025.
    const xAmp = 1920 * 0.012
    const yAmp = 1080 * 0.012
    const item = wander({ amplitude: 1 })
    const frames = 2 * 60 * 60 * 30

    // Track the extremes and assert once: 216k frames of per-frame expect() calls
    // dominate the runtime under load.
    let maxDx = 0
    let maxDy = 0
    let minScale = 1
    let maxScale = 1
    let maxVelocity = 0
    let maxAcceleration = 0
    let previous = evaluateMotionModifiers([item], ctx({ frame: 0 }))
    let previousVelocity = 0
    for (let frame = 1; frame < frames; frame += 1) {
      const c = evaluateMotionModifiers([item], ctx({ frame }))
      maxDx = Math.max(maxDx, Math.abs(c.dx))
      maxDy = Math.max(maxDy, Math.abs(c.dy))
      minScale = Math.min(minScale, c.scaleWidth)
      maxScale = Math.max(maxScale, c.scaleWidth)
      const velocity = c.dx - previous.dx
      maxVelocity = Math.max(maxVelocity, Math.abs(velocity))
      if (frame > 1) maxAcceleration = Math.max(maxAcceleration, Math.abs(velocity - previousVelocity))
      previousVelocity = velocity
      previous = c
    }
    expect(maxDx).toBeLessThanOrEqual(xAmp)
    expect(maxDy).toBeLessThanOrEqual(yAmp)
    expect(minScale).toBeGreaterThanOrEqual(0.975)
    expect(maxScale).toBeLessThanOrEqual(1.025)
    // Per-frame velocity is bounded well below the amplitude: no jumps.
    expect(maxVelocity).toBeLessThan(2)
    // Per-frame acceleration stays under half a pixel: no kinks or stops.
    expect(maxAcceleration).toBeLessThan(0.5)
  })

  it('keeps moving within every sampled window (never rests)', () => {
    const item = wander({ seed: 11 })
    for (let windowStart = 0; windowStart < 4; windowStart += 1) {
      const start = windowStart * 1200
      let min = Number.POSITIVE_INFINITY
      let max = Number.NEGATIVE_INFINITY
      for (let frame = start; frame < start + 240; frame += 1) {
        const { dx } = evaluateMotionModifiers([item], ctx({ frame }))
        min = Math.min(min, dx)
        max = Math.max(max, dx)
      }
      expect(max - min).toBeGreaterThan(0.5)
    }
  })

  it('applies the same scale factor to width and height', () => {
    const item = wander({ channelGains: { x: 1, y: 1, width: 1, height: 1 } })
    for (const frame of [0, 33, 97, 211]) {
      const c = evaluateMotionModifiers([item], ctx({ frame }))
      expect(c.scaleWidth).toBe(c.scaleHeight)
    }
  })

  it('mutes only the x channel when its gain is zero', () => {
    const item = wander({ channelGains: { x: 0, y: 1, width: 1, height: 1 } })
    const c = evaluateMotionModifiers([item], ctx({ frame: 61 }))

    expect(c.dx).toBe(0)
    expect(c.dy).not.toBe(0)
    expect(c.scaleWidth).not.toBe(1)
    expect(c.scaleHeight).not.toBe(1)
  })

  it('shifts the path with phaseFrames like the other modifiers', () => {
    const shifted = evaluateMotionModifiers([wander({ phaseFrames: 30 })], ctx({ frame: 10 }))
    const ahead = evaluateMotionModifiers([wander()], ctx({ frame: 40 }))

    expect(shifted.dx).toBeCloseTo(ahead.dx, 10)
    expect(shifted.dy).toBeCloseTo(ahead.dy, 10)
    expect(shifted.scaleWidth).toBeCloseTo(ahead.scaleWidth, 10)
  })

  it('creates a 0.25Hz wander modifier with x/y/width/height gains', () => {
    const item = createMotionModifier('wander', DEFAULT_MOTION_GENERATOR_SETTINGS)

    expect(item.frequency).toBeCloseTo(0.25, 6)
    expect(item.channelGains).toEqual({ x: 1, y: 1, width: 1, height: 1 })
  })
})
