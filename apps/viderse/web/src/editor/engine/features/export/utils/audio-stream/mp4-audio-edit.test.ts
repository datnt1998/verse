// @vitest-environment node
import { describe, expect, it } from 'vitest'
import { applyAudioPrimingEdit, editAudioPrimingMoov } from './mp4-audio-edit'

function box(type: string, ...parts: Uint8Array[]): Uint8Array<ArrayBuffer> {
  const bytes = new Uint8Array(8 + parts.reduce((length, part) => length + part.length, 0))
  new DataView(bytes.buffer).setUint32(0, bytes.length)
  bytes.set(new TextEncoder().encode(type), 4)
  let offset = 8
  for (const part of parts) { bytes.set(part, offset); offset += part.length }
  return bytes
}
function header(type: 'mvhd' | 'tkhd' | 'mdhd', duration: number, scale: number, version: number): Uint8Array<ArrayBuffer> {
  const payload = new Uint8Array(100)
  payload[0] = version
  const view = new DataView(payload.buffer)
  const durationOffset = type === 'tkhd' ? (version ? 28 : 20) : (version ? 24 : 16)
  if (type !== 'tkhd') view.setUint32(version ? 20 : 12, scale)
  if (version) view.setBigUint64(durationOffset, BigInt(duration))
  else view.setUint32(durationOffset, duration)
  return box(type, payload)
}
function fixture(version = 0): Uint8Array<ArrayBuffer> {
  const handler = (type: string) => { const data = new Uint8Array(24); data.set(new TextEncoder().encode(type), 8); return box('hdlr', data) }
  return box('moov', header('mvhd', 10069, 1000, version),
    box('trak', header('tkhd', 10000, 1000, version), box('mdia', header('mdhd', 300, 30, version), handler('vide'))),
    box('trak', header('tkhd', 10069, 1000, version), box('mdia', header('mdhd', 483328, 48000, version), handler('soun'), box('minf', box('stbl', box('stco', new Uint8Array([0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 32])))))))
}
function locations(bytes: Uint8Array, type: string): number[] {
  const needle = new TextEncoder().encode(type)
  const result: number[] = []
  for (let index = 4; index < bytes.length - 3; index++) if (needle.every((value, off) => bytes[index + off] === value)) result.push(index - 4)
  return result
}
function duration(bytes: Uint8Array, start: number, track: boolean): number {
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength)
  const v1 = bytes[start + 8] === 1
  const offset = start + 8 + (track ? (v1 ? 28 : 20) : (v1 ? 24 : 16))
  return v1 ? Number(view.getBigUint64(offset)) : view.getUint32(offset)
}

describe('AAC presentation edits', () => {
  it.each([0, 1])('signals priming without changing raw media or chunk offsets (header version %i)', version => {
    const original = fixture(version)
    const patched = editAudioPrimingMoov(original, 10, 2112, 48000)
    const view = new DataView(patched.buffer)
    const elst = locations(patched, 'elst')[0]!
    expect(patched[elst + 8]).toBe(1)
    expect(view.getUint32(elst + 12)).toBe(1)
    expect(view.getBigUint64(elst + 16)).toBe(BigInt(10000))
    expect(view.getBigInt64(elst + 24)).toBe(BigInt(2112))
    expect(view.getUint16(elst + 32)).toBe(1)
    expect(locations(patched, 'tkhd').map(start => duration(patched, start, true))).toEqual([10000, 10000])
    expect(duration(patched, locations(patched, 'mvhd')[0]!, false)).toBe(10000)
    expect(duration(patched, locations(patched, 'mdhd')[1]!, false)).toBe(483328)
    const originalOffset = locations(original, 'stco')[0]!
    const newOffset = locations(patched, 'stco')[0]!
    expect(patched.slice(newOffset, newOffset + 20)).toEqual(original.slice(originalOffset, originalOffset + 20))
    expect(editAudioPrimingMoov(patched, 10, 2112, 48000)).toEqual(patched)
  })
  it.each([true, false])('preserves every media byte with fast-start=%s', async fastStart => {
    const moov = fixture()
    const mdat = box('mdat', new Uint8Array([7, 8, 9, 10]))
    const ftyp = box('ftyp', new Uint8Array(16))
    const blob = new Blob(fastStart ? [ftyp, moov, mdat] : [ftyp, mdat, moov], { type: 'video/mp4' })
    const patched = await applyAudioPrimingEdit({ blob }, 10, { status: 'compensated', delayFrames: 2112, sampleRate: 48000 })
    const bytes = new Uint8Array(await patched.blob.arrayBuffer())
    const oldMoovPosition = ftyp.length + (fastStart ? 0 : mdat.length)
    expect(new TextDecoder().decode(bytes.slice(oldMoovPosition + 4, oldMoovPosition + 8))).toBe('free')
    const mediaPosition = ftyp.length + (fastStart ? moov.length : 0)
    expect(bytes.slice(mediaPosition, mediaPosition + mdat.length)).toEqual(mdat)
    expect(locations(bytes, 'moov')).toEqual([blob.size])
  })
  it('does not alter an export when calibration fails', async () => {
    const completed = { blob: new Blob([fixture()]) }
    expect(await applyAudioPrimingEdit(completed, 10, { status: 'uncompensated', reason: 'Decoder unavailable' })).toBe(completed)
  })
})
