import { useEffect, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { getConfig, getConfigPresets, getProjectVoices, putAudioRoleVoice, uploadAudioVoice } from '../../../../api/client'
import type { AudioCastSpeaker, AudioVoice } from '../../../../api/types'
import { invalidate, keys, setData, useResource } from '../../../../data'
import { mobileVoiceCopy as c } from '../../../../i18n/vi'
import { charInitials } from '../../../../lib/cast'
import { BottomSheet } from '../../../../mobile/primitives'
import { useRoute } from '../../../../router'
import { Icon } from '@noverse/ui'
import { errText, useAudioAccess, useAudioData, usePreviewAudio } from '../../useAudio'
import { packLabel } from '../../../settings/lib/packs'
import { Recorder } from './Recorder'
import { s } from './styles'

export function VoiceSheet({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { route, search } = useRoute()
  if (route.name !== 'audio') return null
  return <VoiceContent key={`${route.project}/${route.story}`} project={route.project} story={route.story} volume={route.volume ?? 1} open={open} onClose={onClose} initial={import.meta.env.MODE === 'mock' ? search.get('mstate') ?? search.get('maudio') : null} />
}

function VoiceContent({ project, story, volume, open, onClose, initial }: { project: string; story: string; volume: number; open: boolean; onClose: () => void; initial: string | null }) {
  const data = useAudioData(project, story, volume), access = useAudioAccess(project, story)
  const config = useResource(keys.config('story', project, story), ({ signal }) => getConfig('story', project, story, signal))
  const presets = useResource(['config-presets'], ({ signal }) => getConfigPresets(signal), { staleTime: 60000 })
  const pack = presets.data?.packs?.find(item => item.id === config.data?.effective.style)
  const [page, setPage] = useState<'voice' | 'pick' | 'record'>(initial === 'genVoicePick' ? 'pick' : initial === 'genVoiceRec' ? 'record' : 'voice')
  const [role, setRole] = useState(initial === 'genVoicePick' || initial === 'genVoiceRec' ? 'nv' : '')
  const [draft, setDraft] = useState<string | null>(null), [redo, setRedo] = useState(false)
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [changed, setChanged] = useState<string[]>([])
  const preview = usePreviewAudio(), cast = data.cast
  const mode = cast?.voiceMode ?? cast?.mode ?? 'cast'
  const narrator = cast?.speakers.find(item => item.id === 'dan')
  const single: AudioCastSpeaker = { id: 'single', name: c.whole, role: '', voiceId: cast?.single.voiceId ?? null, oldSegments: cast?.speakers.reduce((n, speaker) => n + (speaker.oldSegments ?? 0), 0), oldEtaSec: cast?.speakers.reduce((n, speaker) => n + (speaker.oldEtaSec ?? 0), 0) }
  const selected = role === 'single' ? single : cast?.speakers.find(item => item.id === role)
  const current = selected?.sameAs === 'dan' || narrator?.sameAs === selected?.id ? 'same' : selected?.voiceId ?? ''
  const value = draft ?? current
  const voice = (id: string | null | undefined) => data.voices.find(item => item.id === id)
  const voiceLabel = (id: string) => id === 'same' ? c.same : voice(id)?.name ?? c.noVoice
  const close = () => { preview.stop(); setPage('voice'); setRole(''); setDraft(null); setError(''); onClose() }
  useEffect(() => { if (!open) preview.stop() }, [open, preview.stop])
  const play = (item?: AudioVoice) => {
    if (!item) return
    setError('')
    void preview.play(item.id, async () => item.sampleUrl ?? `/api/projects/${encodeURIComponent(project)}/voices/${encodeURIComponent(item.id)}/sample`).catch(cause => setError(errText(cause)))
  }
  const pickRole = (speaker: AudioCastSpeaker) => { preview.stop(); setRole(speaker.id); setDraft(null); setRedo(false); setError(''); setPage('pick') }
  const apply = async () => {
    if (!cast || !selected || busy) return
    if (value === current && !redo) { preview.stop(); setPage('voice'); return }
    setBusy(true); setError('')
    try {
      const saved = await putAudioRoleVoice(project, story, selected.id, { voiceId: value === 'same' ? narrator?.voiceId ?? '' : value, rerenderOld: redo, sameAs: value === 'same' ? 'dan' : '' }, cast.version)
      setData(keys.story(project, story, 'audio', 'cast'), saved)
      await invalidate(keys.story(project, story, 'audio'))
      setChanged(previous => [...previous, selected.id]); preview.stop(); setPage('voice')
    } catch (cause) { setError(errText(cause)) } finally { setBusy(false) }
  }
  const saveMode = async (next: 'cast' | 'single' | 'pack') => {
    if (!cast || busy || !access.writable) return
    setBusy(true); setError(''); preview.stop()
    try { if (!await data.saveCast({ ...cast, voiceMode: next, mode: next === 'single' ? 'single' : 'cast', single: { ...cast.single, voiceId: cast.single.voiceId || narrator?.voiceId || '' } })) setError(c.saveError) }
    catch (cause) { setError(errText(cause)) } finally { setBusy(false) }
  }
  const addSample = async (file: File) => {
    setBusy(true); setError('')
    try {
      const added = await uploadAudioVoice(project, story, c.recTitle, file)
      const voices = await getProjectVoices(project)
      setData(keys.project(project, 'voices'), voices)
      await invalidate(keys.project(project, 'assets'))
      setDraft(added.id); setPage('pick')
    } catch (cause) { setError(errText(cause)) } finally { setBusy(false) }
  }
  const previewButton = (item?: AudioVoice) => <button type="button" {...stylex.props(s.icon, preview.playingId === item?.id && s.playing)} style={page === 'pick' ? { width: 38, height: 38 } : undefined} aria-label={preview.playingId === item?.id ? c.stopSample(item?.name ?? '') : c.play(item?.name ?? '')} disabled={!item} onClick={() => play(item)}>{preview.playingId === item?.id ? <span className="ma-eq" aria-hidden="true"><i /><i /><i /></span> : <Icon name="play" size={14} />}</button>
  const own = data.voices.filter(item => !item.builtIn), builtins = data.voices.filter(item => item.builtIn)
  const sameAllowed = selected?.id !== 'dan' && selected?.id !== 'single' && (selected?.sameAs === 'dan' || narrator?.sameAs === selected?.id || cast?.narratorCharacterId === selected?.id)
  const options = (items: AudioVoice[], mine: boolean) => <div {...stylex.props(s.group, s.inset)} role="radiogroup" aria-label={mine ? c.mine : c.builtin}>
    {mine && sameAllowed && <div {...stylex.props(s.row, s.voiceRow)}>{previewButton(voice(narrator?.voiceId))}<button type="button" {...stylex.props(s.pick)} role="radio" aria-checked={value === 'same'} disabled={!access.writable} onClick={() => { setDraft('same'); play(voice(narrator?.voiceId)) }}><span {...stylex.props(s.column)}><span {...stylex.props(s.name)}>{c.same}</span><small {...stylex.props(s.small)}>{c.sameNote}</small></span><span {...stylex.props(s.radio, value === 'same' && s.selectedRadio)}>{value === 'same' && <i {...stylex.props(s.dot)} />}</span></button></div>}
    {items.map(item => <div key={item.id} {...stylex.props(s.row, s.voiceRow)}>{previewButton(item)}<button type="button" {...stylex.props(s.pick)} role="radio" aria-checked={value === item.id} disabled={!access.writable || busy} onClick={() => { setDraft(item.id); play(item) }}><span {...stylex.props(s.column)}><span {...stylex.props(s.name)} title={item.name}>{item.name}</span><small {...stylex.props(s.small)}>{item.builtIn ? c.builtin : c.uploaded(item.durationSec ?? 0)}{item.gender === 'nu' || item.gender === 'nam' ? c.gender[item.gender] : ''}</small></span><span {...stylex.props(s.radio, value === item.id && s.selectedRadio)}>{value === item.id && <i {...stylex.props(s.dot)} />}</span></button></div>)}
    {mine && <button type="button" {...stylex.props(s.row, s.pick)} style={{ width: '100%', minHeight: 54, padding: '8px 14px', color: 'var(--color-accent)' }} disabled={!access.writable || busy} onClick={() => { preview.stop(); setPage('record') }}><span {...stylex.props(s.icon)} style={{ width: 38, height: 38, border: '1px dashed var(--kit-acc-45)', background: 'transparent' }}><Icon name="microphone-2" size={17} /></span><span {...stylex.props(s.column)}><span {...stylex.props(s.name)}>{c.newSample}</span><small {...stylex.props(s.small)}>{c.sampleNote}</small></span></button>}
  </div>
  const rows = mode === 'single' ? [single] : mode === 'pack' ? narrator ? [narrator] : [] : cast?.speakers ?? []
  return <BottomSheet open={open} onClose={busy ? () => {} : close} label={page === 'record' ? c.recTitle : page === 'pick' ? c.forRole(selected?.name ?? '') : c.title} height={page === 'pick' ? 800 : 'auto'} style={{ paddingBottom: 0, maxHeight: 'calc(100dvh - 28px)' }}>
    {page === 'record' ? <><Recorder onBack={() => { setError(''); setPage('pick') }} onUse={addSample} busy={busy} />{error && <p {...stylex.props(s.error, s.inset)} role="alert">{error}</p>}</> : page === 'voice' ? <div {...stylex.props(s.content)}>
      <header {...stylex.props(s.header)}><button type="button" {...stylex.props(s.icon)} onClick={close} aria-label={c.back}><Icon name="arrow-left-2" size={17} /></button><h2 {...stylex.props(s.title)}>{c.title}</h2></header>
      <div {...stylex.props(s.modeList)} role="radiogroup" aria-label={c.title}>{c.modes.map(([id, label, description]) => <button type="button" key={id} role="radio" aria-checked={mode === id} {...stylex.props(s.mode, mode === id && s.selectedMode)} onClick={() => { void saveMode(id) }} disabled={!cast || !access.writable || busy}><span {...stylex.props(s.radio, mode === id && s.selectedRadio)} style={{ marginTop: 2 }}>{mode === id && <i {...stylex.props(s.dot)} />}</span><span {...stylex.props(s.column)} style={{ gap: 2, lineHeight: 1.4 }}><span {...stylex.props(s.name)}>{id === 'cast' && cast ? c.modeCast(cast.speakers.length) : id === 'pack' && pack ? c.modePack(packLabel(pack)) : label}</span><span {...stylex.props(s.small)} style={{ whiteSpace: 'normal' }}>{description}</span></span></button>)}</div>
      <div {...stylex.props(s.heading)}>{mode === 'cast' ? c.cast : c.used}</div>
      <div {...stylex.props(s.group)}>{rows.map(speaker => { const actual = voice(speaker.sameAs ? cast?.speakers.find(item => item.id === speaker.sameAs)?.voiceId : speaker.voiceId); return <div {...stylex.props(s.row)} key={speaker.id}>{previewButton(actual)}<button type="button" {...stylex.props(s.pick)} aria-label={c.changeFor(speaker.name)} onClick={() => pickRole(speaker)}><span {...stylex.props(s.avatar)}>{speaker.ini ?? charInitials(speaker.name)}</span><span {...stylex.props(s.column)}><span {...stylex.props(s.name)}>{mode === 'pack' ? c.packNarrator : speaker.name}</span><small {...stylex.props(s.small, changed.includes(speaker.id) && s.changed)} title={actual?.name}>{speaker.sameAs === 'dan' ? c.same : actual?.name ?? c.noVoice}{actual && !actual.builtIn ? c.voiceSeconds(actual.durationSec ?? 0) : ''}</small></span><span {...stylex.props(s.small, s.changed)}>{c.change}</span><Icon name="arrow-right-2" size={15} /></button></div> })}{mode === 'pack' && <div {...stylex.props(s.row)}><span {...stylex.props(s.avatar)}>…</span><span {...stylex.props(s.column)}><span {...stylex.props(s.name)}>{c.talk}</span><small {...stylex.props(s.small)}>{c.talkNote}</small></span></div>}</div>
      {(error || data.error) && <p {...stylex.props(s.error)} role="alert">{error || data.error}</p>}<span {...stylex.props(s.explanation)}>{c.explanation}</span><button type="button" {...stylex.props(s.primary)} disabled={busy} onClick={close}>{c.done}</button>
    </div> : <div {...stylex.props(s.content, s.tall)}>
      <header {...stylex.props(s.header, s.insetHeader)}><button type="button" {...stylex.props(s.icon)} disabled={busy} onClick={() => { preview.stop(); setPage('voice') }} aria-label={c.back}><Icon name="arrow-left-2" size={17} /></button><span {...stylex.props(s.column)} style={{ lineHeight: 1.3 }}><h2 {...stylex.props(s.title)}>{c.forRole(selected?.name ?? '')}</h2><small {...stylex.props(s.subtitle)}>{c.old(selected?.oldSegments ?? 0, voiceLabel(current))}</small></span></header>
      <div {...stylex.props(s.scroll)}><div {...stylex.props(s.groupHeading)}>{c.mine}</div>{options(own, true)}<div {...stylex.props(s.groupHeading)}>{c.builtin}</div>{options(builtins, false)}</div>
      <div {...stylex.props(s.bottom)}>{error && <p {...stylex.props(s.error)} role="alert">{error}</p>}{!!selected?.oldSegments && <div {...stylex.props(s.redo)}><span {...stylex.props(s.column)}><span style={{ fontSize: 14.5 }}>{c.redo(selected.oldSegments, selected.name.split(' ').slice(-2).join(' '))}</span><small {...stylex.props(s.small)} style={{ whiteSpace: 'normal', fontSize: 12 }}>{redo ? selected.oldEtaSec ? c.redoEta(selected.oldEtaSec) : c.redoUnknown : c.keepOld}</small></span><button type="button" role="switch" aria-checked={redo} aria-label={c.redoAria} disabled={busy || !access.writable} {...stylex.props(s.switch, redo && s.switchOn)} onClick={() => setRedo(!redo)}><i {...stylex.props(s.knob, redo && s.knobOn)} /></button></div>}<button type="button" {...stylex.props(s.primary, (!value || busy || !access.writable) && s.disabled)} disabled={!value || busy || !access.writable} onClick={() => { void apply() }}>{value !== current ? c.apply(voiceLabel(value)) : c.keep}</button></div>
    </div>}
  </BottomSheet>
}
