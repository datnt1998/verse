// @vitest-environment node
import { expect, it } from 'vitest'
import * as mediabunny from 'mediabunny'
import { measureEncoderPriming } from './encoder-priming'

it('records unavailable calibration instead of guessing an encoder delay and caches the outcome', async () => {
  const config = { codec: 'aac' as const, sampleRate: 48000, numberOfChannels: 2, bitrate: 192000 }
  const first = measureEncoderPriming(mediabunny, config)
  expect(measureEncoderPriming(mediabunny, config)).toBe(first)
  expect(await first).toEqual({ status: 'uncompensated', reason: 'AudioDecoder unavailable for priming calibration' })
  expect(measureEncoderPriming(mediabunny, { ...config, bitrate: 128000 })).not.toBe(first)
})
