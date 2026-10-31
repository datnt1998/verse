type MediabunnyModule = typeof import('mediabunny')
export type AudioTimingDiagnostics =
  | { status: 'compensated'; delayFrames: number; sampleRate: number }
  | { status: 'uncompensated'; reason: string }

const calibrations = new Map<string, Promise<AudioTimingDiagnostics>>()

/** Measure the actual encoder selected by AudioSampleSource, including WASM fallbacks. */
export function measureEncoderPriming(
  mediabunny: MediabunnyModule,
  config: { codec: 'aac'; sampleRate: number; numberOfChannels: number; bitrate: number },
): Promise<AudioTimingDiagnostics> {
  const key = `${config.codec}:${config.sampleRate}:${config.numberOfChannels}:${config.bitrate}`
  let result = calibrations.get(key)
  if (!result) {
    result = calibrate(mediabunny, config).catch((error: unknown) => ({
      status: 'uncompensated' as const,
      reason: error instanceof Error ? error.message : String(error),
    }))
    calibrations.set(key, result)
  }
  return result
}

async function calibrate(
  M: MediabunnyModule,
  config: { codec: 'aac'; sampleRate: number; numberOfChannels: number; bitrate: number },
): Promise<AudioTimingDiagnostics> {
  if (typeof AudioDecoder === 'undefined') throw new Error('AudioDecoder unavailable for priming calibration')
  const frames = Math.round(config.sampleRate / 4)
  const clickFrame = Math.round(config.sampleRate / 20)
  const packets: { packet: InstanceType<MediabunnyModule['EncodedPacket']>; meta?: EncodedAudioChunkMetadata }[] = []
  const source = new M.AudioSampleSource({ codec: config.codec, bitrate: config.bitrate, onEncodedPacket: (packet, meta) => { packets.push({ packet, meta }) } })
  const output = new M.Output({ format: new M.Mp4OutputFormat({ fastStart: false }), target: new M.NullTarget() })
  output.addAudioTrack(source)
  let decoder: AudioDecoder | undefined
  let decoderError: Error | undefined
  let decodedFrames = 0
  let maxAmplitude = 0
  let peakFrame = -1
  try {
    await output.start()
    const planar = new Float32Array(frames * config.numberOfChannels)
    for (let channel = 0; channel < config.numberOfChannels; channel++) planar[channel * frames + clickFrame] = 0.9
    const sample = new M.AudioSample({ data: planar, format: 'f32-planar', sampleRate: config.sampleRate, numberOfChannels: config.numberOfChannels, timestamp: 0 })
    try { await source.add(sample) } finally { sample.close() }
    source.close()
    await output.finalize()
    const decoderConfig = packets.find(entry => entry.meta?.decoderConfig)?.meta?.decoderConfig
    if (!decoderConfig) throw new Error('Encoder omitted decoder config during priming calibration')
    decoder = new AudioDecoder({
      output: data => {
        try {
          if (data.sampleRate !== config.sampleRate) throw new Error('Calibration decoder changed sample rate')
          const samples = new Float32Array(data.numberOfFrames)
          data.copyTo(samples, { format: 'f32-planar', planeIndex: 0 })
          for (let frame = 0; frame < samples.length; frame++) {
            const amplitude = Math.abs(samples[frame]!)
            if (amplitude > maxAmplitude) { maxAmplitude = amplitude; peakFrame = decodedFrames + frame }
          }
          decodedFrames += samples.length
        } catch (error) { decoderError = error instanceof Error ? error : new Error(String(error)) }
        finally { data.close() }
      },
      error: error => { decoderError = error },
    })
    decoder.configure(decoderConfig)
    for (const entry of packets) decoder.decode(entry.packet.toEncodedAudioChunk())
    await decoder.flush()
    if (decoderError) throw decoderError
    if (maxAmplitude < 0.05 || peakFrame < clickFrame || peakFrame > clickFrame + config.sampleRate / 10) {
      throw new Error('No reliable click onset found during priming calibration')
    }
    return { status: 'compensated', delayFrames: peakFrame - clickFrame, sampleRate: config.sampleRate }
  } finally {
    if (decoder && decoder.state !== 'closed') decoder.close()
    if (output.state === 'started') await output.cancel()
  }
}
