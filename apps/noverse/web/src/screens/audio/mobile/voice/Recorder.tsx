import { useEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Icon } from '@noverse/ui'
import { mobileVoiceCopy as c } from '../../../../i18n/vi'
import { s } from './styles'

/** MediaRecorder produces webm on Chromium; convert the actual decoded signal to an accepted WAV. */
export function recordingWav(buffer: AudioBuffer): File {
  const frames = Math.min(buffer.length, buffer.sampleRate * 30)
  const data = new ArrayBuffer(44 + frames * 2), view = new DataView(data)
  const text = (at: number, value: string) => { for (let i = 0; i < value.length; i++) view.setUint8(at + i, value.charCodeAt(i)) }
  text(0, 'RIFF'); view.setUint32(4, 36 + frames * 2, true); text(8, 'WAVE'); text(12, 'fmt ')
  view.setUint32(16, 16, true); view.setUint16(20, 1, true); view.setUint16(22, 1, true)
  view.setUint32(24, buffer.sampleRate, true); view.setUint32(28, buffer.sampleRate * 2, true)
  view.setUint16(32, 2, true); view.setUint16(34, 16, true); text(36, 'data'); view.setUint32(40, frames * 2, true)
  const channels = Array.from({ length: buffer.numberOfChannels }, (_, i) => buffer.getChannelData(i))
  for (let i = 0; i < frames; i++) {
    let value = 0
    for (const channel of channels) value += channel[i] / channels.length
    view.setInt16(44 + i * 2, Math.round(Math.max(-1, Math.min(1, value)) * (value < 0 ? 32768 : 32767)), true)
  }
  return new File([data], 'mau-giong.wav', { type: 'audio/wav' })
}

export function Recorder({ onBack, onUse, busy }: { onBack: () => void; onUse: (file: File) => Promise<void>; busy: boolean }) {
  const [recording, setRecording] = useState(false), [seconds, setSeconds] = useState(0)
  const [file, setFile] = useState<File | null>(null), [error, setError] = useState(''), [decoding, setDecoding] = useState(false)
  const [levels, setLevels] = useState<number[]>([])
  const input = useRef<HTMLInputElement>(null), recorder = useRef<MediaRecorder | null>(null)
  const media = useRef<MediaStream | null>(null), context = useRef<AudioContext | null>(null)
  const alive = useRef(true), starting = useRef(false), timer = useRef<number | undefined>(undefined)
  const clear = () => { clearInterval(timer.current); media.current?.getTracks().forEach(track => track.stop()); media.current = null; void context.current?.close(); context.current = null }
  useEffect(() => { alive.current = true; return () => { alive.current = false; if (recorder.current?.state === 'recording') recorder.current.stop(); clear() } }, [])
  const decode = async (blob: Blob, original?: File) => {
    setDecoding(true)
    const audio = new AudioContext()
    try {
      const buffer = await audio.decodeAudioData(await blob.arrayBuffer())
      if (!alive.current) return
      const duration = original ? buffer.duration : Math.min(30, buffer.duration)
      setSeconds(duration)
      if (duration > 30 || duration < 10) { setFile(null); if (duration > 30) setError(c.sampleNote); return }
      setFile(original ?? recordingWav(buffer))
    } catch { if (alive.current) setError(c.fileError) }
    finally { await audio.close(); if (alive.current) setDecoding(false) }
  }
  const toggle = async () => {
    if (recorder.current?.state === 'recording') { recorder.current.stop(); return }
    if (starting.current) return
    starting.current = true; setError(''); setFile(null); setSeconds(0); setLevels([])
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
      if (!alive.current) { stream.getTracks().forEach(track => track.stop()); return }
      media.current = stream
      const audio = new AudioContext(); context.current = audio
      const analyser = audio.createAnalyser(); analyser.fftSize = 256; audio.createMediaStreamSource(stream).connect(analyser)
      const samples = new Uint8Array(analyser.fftSize), chunks: Blob[] = []
      const next = new MediaRecorder(stream); recorder.current = next
      next.ondataavailable = event => { if (event.data.size) chunks.push(event.data) }
      next.onerror = () => { clear(); if (alive.current) { setRecording(false); setError(c.micError) } }
      next.onstop = () => { clear(); if (!alive.current) return; setRecording(false); void decode(new Blob(chunks, { type: next.mimeType })) }
      next.start(250); setRecording(true)
      const started = performance.now()
      timer.current = window.setInterval(() => {
        const elapsed = Math.min(30, (performance.now() - started) / 1000); setSeconds(elapsed)
        analyser.getByteTimeDomainData(samples)
        let energy = 0; for (const sample of samples) energy += ((sample - 128) / 128) ** 2
        setLevels(previous => [...previous.slice(-39), Math.min(1, Math.sqrt(energy / samples.length) * 4)])
        if (elapsed >= 30 && next.state === 'recording') next.stop()
      }, 100)
    } catch { clear(); setError(c.micError) }
    finally { starting.current = false }
  }
  const ready = !!file && seconds >= 10 && seconds <= 30 && !recording && !decoding && !busy
  const hint = recording ? seconds < 10 ? c.recShort : c.recEnough : seconds > 0 ? seconds < 10 ? c.tooShort : c.recorded(seconds) : c.recIdle
  return <div {...stylex.props(s.content)} style={{ gap: 14, paddingTop: 14 }}>
    <header {...stylex.props(s.header)}><button {...stylex.props(s.icon)} onClick={onBack} aria-label={c.back} disabled={busy}><Icon name="arrow-left-2" size={17} /></button><h2 {...stylex.props(s.title)}>{c.recTitle}</h2></header>
    <p {...stylex.props(s.quote)}>{c.passage}</p><span {...stylex.props(s.subtitle, s.centered)} style={{ lineHeight: 1.5 }}>{c.recAdvice}</span>
    <div {...stylex.props(s.record)}><button {...stylex.props(s.recordButton, recording && s.recording)} onClick={() => { void toggle() }} disabled={busy || decoding} aria-label={recording ? c.stop : c.start}><Icon name={recording ? 'stop' : 'microphone-2'} size={34} /></button>
      <span {...stylex.props(s.time)}>0:{String(Math.floor(seconds)).padStart(2, '0')} <span {...stylex.props(s.explanation)}>/ 0:30</span></span>
      <div {...stylex.props(s.wave)} aria-hidden="true">{Array.from({ length: 40 }, (_, i) => <i key={i} {...stylex.props(s.bar)} style={{ height: `${Math.max(12, (levels[i] ?? 0) * 100)}%`, opacity: levels[i] === undefined ? .14 : 1 }} />)}</div>
      <span {...stylex.props(s.small, seconds > 0 && seconds < 10 && !recording && s.error)} style={{ lineHeight: 1.5 }} role="status">{hint}</span>
    </div>
    {error && <p {...stylex.props(s.error)} role="alert">{error}</p>}
    <div {...stylex.props(s.fileActions)}><button {...stylex.props(s.primary, s.ghost)} disabled={recording || busy || decoding} onClick={() => input.current?.click()}><Icon name="folder-open" size={16} />{c.chooseFile}</button><button {...stylex.props(s.primary, !ready && s.disabled)} disabled={!ready} onClick={() => { if (file) void onUse(file) }}>{c.useSample}</button></div>
    <input {...stylex.props(s.hidden)} type="file" ref={input} accept=".mp3,.wav,.m4a" onChange={event => { const picked = event.target.files?.[0]; event.target.value = ''; if (!picked) return; setError(''); setFile(null); if (!/\.(mp3|wav|m4a)$/i.test(picked.name)) { setError(c.fileError); return }; void decode(picked, picked) }} />
    <span {...stylex.props(s.explanation, s.centered)} style={{ fontSize: 12 }}>{c.consent}</span>
  </div>
}
