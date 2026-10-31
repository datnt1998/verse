import { z } from 'zod'
const finite = () => z.number().finite()
export const noiseProfileSchema = z.object({
  frequencyBins: z.array(finite().nonnegative()).min(2).max(8193),
  magnitudes: z.array(finite().nonnegative()).min(2).max(8193),
  standardDeviations: z.array(finite().nonnegative()).min(2).max(8193),
  sampleRate: finite().int().min(8000).max(192000),
  fftSize: finite().int().min(256).max(16384).refine(value => (value & (value - 1)) === 0),
}).strict().superRefine((profile, ctx) => {
  const bins = profile.fftSize / 2 + 1
  if (profile.frequencyBins.length !== bins || profile.magnitudes.length !== bins || profile.standardDeviations.length !== bins) {
    ctx.addIssue({ code: 'custom', message: 'Số dải tần của mẫu nhiễu không khớp FFT' })
  }
  for (let i = 0; i < profile.frequencyBins.length; i++) {
    if (Math.abs(profile.frequencyBins[i]! - i * profile.sampleRate / profile.fftSize) > 1e-6) {
      ctx.addIssue({ code: 'custom', message: 'Dải tần của mẫu nhiễu không hợp lệ' })
      break
    }
  }
})
const unit = () => finite().min(0).max(1)
export const compressorParamsSchema = z.object({
  threshold: finite().min(-60).max(0), ratio: finite().min(1).max(20),
  attack: finite().min(0.001).max(1), release: finite().min(0.01).max(3),
  knee: finite().min(0).max(40), makeupGain: finite().min(0).max(24),
}).strict()
export const noiseReductionParamsSchema = z.object({
  threshold: finite().min(-60).max(0), reduction: unit(), smoothing: unit(), profile: noiseProfileSchema.optional(),
}).strict()
export const reverbParamsSchema = z.object({
  roomSize: unit(), damping: unit(), wetLevel: unit(), dryLevel: unit(), preDelay: finite().min(0).max(100),
}).strict()
export const delayParamsSchema = z.object({ time: finite().min(0).max(2), feedback: finite().min(0).max(0.95), wetLevel: unit() }).strict()
const identity = { id: z.string().min(1), enabled: z.boolean() }
export const audioFxEffectSchema = z.discriminatedUnion('type', [
  z.object({ ...identity, type: z.literal('compressor'), params: compressorParamsSchema }).strict(),
  z.object({ ...identity, type: z.literal('noiseReduction'), params: noiseReductionParamsSchema }).strict(),
  z.object({ ...identity, type: z.literal('reverb'), params: reverbParamsSchema }).strict(),
  z.object({ ...identity, type: z.literal('delay'), params: delayParamsSchema }).strict(),
]).superRefine((effect, ctx) => {
  if (effect.type === 'noiseReduction' && effect.enabled && !effect.params.profile) {
    ctx.addIssue({ code: 'custom', message: 'Cần lấy mẫu nhiễu trước khi bật giảm nhiễu' })
  }
})
export const audioFxParamsPatchSchema = z.object({
  threshold: finite().optional(), ratio: finite().optional(), attack: finite().optional(), release: finite().optional(),
  knee: finite().optional(), makeupGain: finite().optional(), reduction: finite().optional(), smoothing: finite().optional(),
  profile: noiseProfileSchema.optional(), roomSize: finite().optional(), damping: finite().optional(),
  wetLevel: finite().optional(), dryLevel: finite().optional(), preDelay: finite().optional(),
  time: finite().optional(), feedback: finite().optional(),
}).strict()
