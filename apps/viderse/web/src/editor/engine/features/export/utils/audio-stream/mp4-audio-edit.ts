import type { TemporaryExportOutput } from '../export-output-target'
import type { AudioTimingDiagnostics } from './encoder-priming'

interface Box { start: number; end: number; header: number; type: string }
const encoder = new TextEncoder()
const decoder = new TextDecoder()
function boxes(bytes: Uint8Array, start = 0, end = bytes.length): Box[] {
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength)
  const result: Box[] = []
  while (start < end) {
    if (end - start < 8) throw new Error('Truncated MP4 box')
    let length = view.getUint32(start)
    const header = length === 1 ? 16 : 8
    if (length === 1) {
      if (end - start < 16) throw new Error('Truncated extended MP4 box')
      length = Number(view.getBigUint64(start + 8))
    } else if (length === 0) length = end - start
    if (!Number.isSafeInteger(length) || length < header || start + length > end) throw new Error('Invalid MP4 box size')
    result.push({ start, end: start + length, header, type: decoder.decode(bytes.subarray(start + 4, start + 8)) })
    start += length
  }
  return result
}
function required(entries: Box[], type: string): Box {
  const found = entries.find(entry => entry.type === type)
  if (!found) throw new Error(`MP4 missing ${type} box`)
  return found
}
function join(parts: Uint8Array[]): Uint8Array<ArrayBuffer> {
  const out = new Uint8Array(parts.reduce((size, part) => size + part.length, 0))
  let offset = 0
  for (const part of parts) { out.set(part, offset); offset += part.length }
  return out
}
function container(type: string, parts: Uint8Array[]): Uint8Array<ArrayBuffer> {
  const data = join(parts)
  const out = new Uint8Array(8 + data.length)
  new DataView(out.buffer).setUint32(0, out.length)
  out.set(encoder.encode(type), 4)
  out.set(data, 8)
  return out
}
function durationOffset(bytes: Uint8Array, box: Box): number {
  const v1 = bytes[box.start + box.header] === 1
  return box.start + box.header + (box.type === 'tkhd' ? (v1 ? 28 : 20) : (v1 ? 24 : 16))
}
function readDuration(bytes: Uint8Array, box: Box): number {
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength)
  const offset = durationOffset(bytes, box)
  return bytes[box.start + box.header] === 1 ? Number(view.getBigUint64(offset)) : view.getUint32(offset)
}
function writeDuration(bytes: Uint8Array, box: Box, duration: number): void {
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength)
  const offset = durationOffset(bytes, box)
  if (bytes[box.start + box.header] === 1) view.setBigUint64(offset, BigInt(duration))
  else {
    if (duration > 0xffffffff) throw new Error('MP4 duration exceeds version-zero field')
    view.setUint32(offset, duration)
  }
}
function timescale(bytes: Uint8Array, box: Box): number {
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength)
  return view.getUint32(box.start + box.header + (bytes[box.start + box.header] === 1 ? 20 : 12))
}

/** Preserve compressed packets, signalling head priming and tail padding only through presentation metadata. */
export function editAudioPrimingMoov(moov: Uint8Array, durationSeconds: number, delayFrames: number, sampleRate: number): Uint8Array<ArrayBuffer> {
  const root = required(boxes(moov), 'moov')
  const children = boxes(moov, root.start + root.header, root.end)
  const movieHeader = required(children, 'mvhd')
  const movieScale = timescale(moov, movieHeader)
  const contentDuration = Math.round(durationSeconds * movieScale)
  let audioTracks = 0
  let movieDuration = 0
  const parts = children.map(child => {
    if (child.type !== 'trak') return moov.slice(child.start, child.end)
    const track = boxes(moov, child.start + child.header, child.end)
    const media = required(track, 'mdia')
    const mediaChildren = boxes(moov, media.start + media.header, media.end)
    const handler = required(mediaChildren, 'hdlr')
    const isAudio = decoder.decode(moov.subarray(handler.start + handler.header + 8, handler.start + handler.header + 12)) === 'soun'
    const trackHeader = required(track, 'tkhd')
    if (!isAudio) {
      movieDuration = Math.max(movieDuration, readDuration(moov, trackHeader))
      return moov.slice(child.start, child.end)
    }
    audioTracks++
    const mediaScale = timescale(moov, required(mediaChildren, 'mdhd'))
    const mediaTime = Math.round(delayFrames * mediaScale / sampleRate)
    const edit = new Uint8Array(28)
    const view = new DataView(edit.buffer)
    edit[0] = 1 // version-one elst supports long exports and signed media time
    view.setUint32(4, 1)
    view.setBigUint64(8, BigInt(contentDuration))
    view.setBigInt64(16, BigInt(mediaTime))
    view.setUint16(24, 1)
    const edts = container('edts', [container('elst', [edit])])
    const rewritten = track.filter(entry => entry.type !== 'edts').map(entry => {
      const data = moov.slice(entry.start, entry.end)
      if (entry.type === 'tkhd') writeDuration(data, { ...entry, start: 0, end: data.length }, contentDuration)
      return data
    })
    rewritten.splice(1, 0, edts)
    movieDuration = Math.max(movieDuration, contentDuration)
    return container('trak', rewritten)
  })
  if (audioTracks !== 1) throw new Error(`Expected one mixed audio track, found ${audioTracks}`)
  const result = container('moov', parts)
  const rewrittenHeader = required(boxes(result, 8), 'mvhd')
  writeDuration(result, rewrittenHeader, movieDuration)
  return result
}

async function findMoov(blob: Blob): Promise<{ position: number; length: number }> {
  let position = 0
  while (position < blob.size) {
    const data = new Uint8Array(await blob.slice(position, position + 16).arrayBuffer())
    if (data.length < 8) throw new Error('Truncated MP4 top-level box')
    const view = new DataView(data.buffer)
    const size = view.getUint32(0)
    const length = size === 1 ? Number(view.getBigUint64(8)) : size === 0 ? blob.size - position : size
    if (!Number.isSafeInteger(length) || length < (size === 1 ? 16 : 8) || position + length > blob.size) throw new Error('Invalid MP4 top-level size')
    if (decoder.decode(data.subarray(4, 8)) === 'moov') return { position, length }
    position += length
  }
  throw new Error('MP4 contains no moov box')
}

/** Metadata-only patch: invalidate the old moov and append its replacement; mdat offsets remain unchanged for either layout. */
export async function applyAudioPrimingEdit<T extends { blob: Blob; temporaryOutput?: TemporaryExportOutput }>(
  completed: T, durationSeconds: number, timing: AudioTimingDiagnostics,
): Promise<T> {
  if (timing.status !== 'compensated') return completed
  const { position, length } = await findMoov(completed.blob)
  const oldMoov = new Uint8Array(await completed.blob.slice(position, position + length).arrayBuffer())
  const moov = editAudioPrimingMoov(oldMoov, durationSeconds, timing.delayFrames, timing.sampleRate)
  const free = encoder.encode('free')
  if (completed.temporaryOutput) {
    const root = await navigator.storage.getDirectory()
    const directory = await root.getDirectoryHandle(completed.temporaryOutput.directory)
    const handle = await directory.getFileHandle(completed.temporaryOutput.fileName)
    const writable = await handle.createWritable({ keepExistingData: true })
    try {
      await writable.write({ type: 'write', position: position + 4, data: free })
      await writable.write({ type: 'write', position: completed.blob.size, data: moov })
      await writable.close()
    } catch (error) { await writable.abort().catch(() => undefined); throw error }
    return { ...completed, blob: new Blob([await handle.getFile()], { type: completed.blob.type }) }
  }
  return { ...completed, blob: new Blob([completed.blob.slice(0, position + 4), free, completed.blob.slice(position + 8), moov], { type: completed.blob.type }) }
}
