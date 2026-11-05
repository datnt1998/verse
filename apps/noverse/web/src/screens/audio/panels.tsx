// Ba tab của cột phải màn Bản đọc — bám Transcript.dc.html (dòng 237–341): Phân vai · Cần xem · Cách đọc.
import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import { navigate, paths } from '../../router'
import type {
  AudioCast, AudioNormalize, AudioPronunciation, AudioSegment, AudioSettings, AudioVoice,
} from '../../api/types'
import { Icon } from '@noverse/ui'
import { Button } from '@noverse/ui'
import { Radio, RadioCard } from '@noverse/ui'
import { Skeleton, Spinner } from '@noverse/ui'
import { Field } from '@noverse/ui'
import { keys, useAction, useResource } from '../../data'
import { useSaveShortcut } from '@noverse/ui'
import { getTtsStatus, previewTtsVoice } from '../../api/client'
import { ttsPreviewURL, voicePreviewSource } from './voicePreview'
import { issueSegments, speakerName } from './logic'
import { speakerRoleLabel } from '../../lib/cast'
import type { PackVoice } from './packVoice'
import { usePreviewAudio } from './useAudio'
import { MiniSwitch } from './ui'
import { t } from '../../i18n/vi'
import { useUnsavedGuard } from '../../ui/useUnsavedGuard'
import { CharAvatar } from '../../ui/CharAvatar'

export type PanelTab = 'cast' | 'issues' | 'dict'

export function RightPanel({ tab, onTab, issues, model, children }: {
  tab: PanelTab; onTab: (tab: PanelTab) => void; issues: number; model: string; children: ReactNode
}) {
  const tabs: [PanelTab, string, number][] = [
    ['cast', t.audio.panels.phanVai2, 0],
    ['issues', t.audio.panels.canXem, issues],
    ['dict', t.audio.panels.cachDoc2, 0],
  ]
  return (
    <aside aria-label={t.audio.panels.giongDoc} style={{
      flex: '0 0 344px', minHeight: 0, borderLeft: '1px solid rgba(255,255,255,0.06)', display: 'flex', flexDirection: 'column',
    }}>
      <div role="tablist" aria-label={t.audio.panels.panel} style={{
        flex: '0 0 auto', display: 'flex', gap: 2, margin: '12px 12px 0', padding: 3, borderRadius: 10,
        background: '#131315', border: '1px solid rgba(255,255,255,0.06)',
      }}>
        {tabs.map(([id, label, count]) => {
          const on = tab === id
          return (
            <button key={id} role="tab" type="button" aria-selected={on} onClick={() => onTab(id)} style={{
              flex: 1, height: 30, border: 'none', borderRadius: 7, background: on ? '#222225' : 'transparent',
              color: on ? '#EDEDEF' : '#A0A0AB', fontSize: 12.5, fontWeight: on ? 600 : 400,
              display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 6,
            }}>
              {label}
              {count > 0 && (
                <span className="num" style={{
                  height: 18, minWidth: 18, padding: '0 5px', borderRadius: 5, background: 'rgba(246,203,140,0.14)',
                  color: '#F6CB8C', fontSize: 11, display: 'flex', alignItems: 'center', justifyContent: 'center',
                }}>{count}</span>
              )}
            </button>
          )
        })}
      </div>
      <div className="scroll" style={{ flex: 1, minHeight: 0, padding: '14px 12px', display: 'flex', flexDirection: 'column', gap: 12 }}>
        {children}
      </div>
      <div style={{
        flex: '0 0 auto', padding: '10px 14px', borderTop: '1px solid rgba(255,255,255,0.06)', display: 'flex',
        alignItems: 'center', gap: 8, fontSize: 12, color: '#7E7E8A',
      }}>
        <Icon name="cpu" size={14} />
        <span style={{ flex: 1 }}>{t.audio.panels.docTrenMay(model)}</span>
        <a href={`#${paths.settings('voice')}`} onClick={(event) => { event.preventDefault(); navigate(paths.settings('voice')) }}
          style={{ color: '#A0A0AB' }}>{t.audio.panels.doi}</a>
      </div>
    </aside>
  )
}

const lblRow: CSSProperties = { display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', padding: '0 4px' }
const dashedBtn: CSSProperties = {
  borderRadius: 10, border: '1px dashed rgba(255,255,255,0.16)', background: 'transparent', color: '#C9C9D1',
  fontSize: 12.5, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 6,
}

function Info({ children, icon = 'info-circle', color }: { children: ReactNode; icon?: string; color?: string }) {
  return (
    <span style={{ padding: '0 4px', fontSize: 12, color: '#7E7E8A', display: 'flex', gap: 6 }}>
      <Icon name={icon} size={13} style={{ marginTop: 2, flex: '0 0 auto', color }} />
      <span>{children}</span>
    </span>
  )
}

// ─── Phân vai ────────────────────────────────────────────────────────────

function ModeCard({ on, title, desc, icon, onClick, disabled }: {
  on: boolean; title: string; desc: string; icon: string; onClick: () => void; disabled: boolean
}) {
  return (
    <RadioCard checked={on} onChange={onClick} disabled={disabled} mark="none" style={{
      display: 'flex', flexDirection: 'column', gap: 4, padding: '10px 12px', borderRadius: 12,
      border: `1px solid ${on ? 'rgba(201,166,245,0.5)' : 'rgba(255,255,255,0.08)'}`,
      background: on ? 'rgba(201,166,245,0.06)' : '#131315', color: '#EDEDEF', textAlign: 'left',
    }}>
      <span style={{ display: 'flex', alignItems: 'center', gap: 7, fontSize: 13, fontWeight: 600 }}>
        <Icon name={icon} size={16} style={{ color: on ? '#C9A6F5' : '#A0A0AB' }} />{title}
      </span>
      <span style={{ fontSize: 11.5, color: '#A0A0AB' }}>{desc}</span>
    </RadioCard>
  )
}

function SwitchRow({ label, hint, on, onChange, disabled, first }: {
  label: string; hint?: string; on: boolean; onChange: (next: boolean) => void; disabled: boolean; first?: boolean
}) {
  return (
    <div style={{
      display: 'flex', alignItems: 'center', gap: 10, padding: '10px 0',
      borderTop: first ? 'none' : '1px solid rgba(255,255,255,0.06)',
    }}>
      <span style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
        <span style={{ fontSize: 12.5 }}>{label}</span>
        {hint && <span style={{ fontSize: 11.5, color: '#7E7E8A' }}>{hint}</span>}
      </span>
      <MiniSwitch on={on} onChange={onChange} label={label} disabled={disabled} />
    </div>
  )
}

const voiceDesc = (voice: AudioVoice, pack: PackVoice | null = null, recommended = false) => voice.builtIn
  ? recommended && pack ? t.audio.pack.goiDeXuat : t.audio.panels.giongCoSan
  : t.audio.pack.banTaiLen(voice.durationSec ?? 0, voice.gender === 'nu' ? ' · nữ' : voice.gender === 'nam' ? ' · nam' : '')

/** D11 — thẻ khoá “Một giọng kể · theo gói” thay hai thẻ Phân vai | Một giọng khi gói chọn một giọng. */
function PackVoiceCard({ pack, styleHref }: { pack: PackVoice; styleHref: string }) {
  return (
    <div className="card" style={{
      borderRadius: 14, padding: 12, display: 'flex', flexDirection: 'column', gap: 6, borderColor: 'var(--kit-ok-22)',
    }}>
      <span style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13.5, fontWeight: 600 }}>
        <Icon name="microphone-2" size={16} style={{ color: 'var(--color-ok)' }} />{t.audio.pack.motGiongKe}
        <span style={{
          height: 20, padding: '0 7px', display: 'flex', alignItems: 'center', borderRadius: 6,
          background: 'var(--tint-ok)', color: 'var(--color-ok)', fontSize: 11.5, fontWeight: 500,
        }}>{t.audio.pack.badge}</span>
      </span>
      <span style={{ fontSize: 12, color: 'var(--color-fg-3)', lineHeight: 1.5 }}>{t.audio.pack.moTa(pack.ten, pack.gender)}</span>
      <a href={`#${styleHref}`} onClick={(event) => { event.preventDefault(); navigate(styleHref) }}
        style={{ fontSize: 12, color: 'var(--color-accent)' }}>{t.audio.pack.doiGoi}</a>
    </div>
  )
}

/** Ô chọn giọng của một vai — dáng như artboard (icon · tên · mũi tên), bấm mở danh sách giọng. */
function VoicePick({ voices, value, name, disabled, onPick, children }: {
  voices: AudioVoice[]; value: string | null; name: string; disabled: boolean
  onPick: (voiceId: string) => void
  children: (open: () => void, expanded: boolean) => ReactNode
}) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!open) return
    const close = (event: MouseEvent) => { if (!ref.current?.contains(event.target as Node)) setOpen(false) }
    document.addEventListener('mousedown', close)
    return () => document.removeEventListener('mousedown', close)
  }, [open])
  return (
    <div ref={ref} style={{ position: 'relative', flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
      {children(() => { if (!disabled) setOpen((current) => !current) }, open)}
      {open && (
        <div role="listbox" aria-label={t.audio.panels.giongCho(name)} style={{
          position: 'absolute', top: 'calc(100% + 4px)', left: 0, right: 0, zIndex: 20, maxHeight: 260, overflowY: 'auto', padding: 5,
          background: 'var(--color-surface-3)', border: '1px solid var(--line-3)', borderRadius: 12, boxShadow: 'var(--dialog-shadow)',
        }}>
          {voices.map((voice) => (
            <button key={voice.id} type="button" role="option" aria-selected={voice.id === value} className="hov-ghost"
              onClick={() => { setOpen(false); if (voice.id !== value) onPick(voice.id) }}
              style={{
                width: '100%', textAlign: 'left', padding: '7px 9px', borderRadius: 8, display: 'flex', flexDirection: 'column',
                background: voice.id === value ? 'var(--color-surface-6)' : 'transparent',
              }}>
              <span style={{ fontSize: 12.5, color: '#EDEDEF' }}>{voice.name}</span>
              <span style={{ fontSize: 11, color: '#7E7E8A' }}>{voiceDesc(voice)}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

export function CastPanel({ cast, voices, lineCounts, writable, pack, styleHref, onCast, onAddVoice }: {
  cast: AudioCast | null; voices: AudioVoice[]
  /** số đoạn của từng người nói trong chương đang mở */
  lineCounts: Record<string, number>
  writable: boolean
  /** D11 — gói của truyện chọn một giọng kể: khoá hai thẻ cách giao giọng. */
  pack: PackVoice | null
  styleHref: string
  onCast: (cast: AudioCast) => void
  onAddVoice: () => void
}) {
  const preview = usePreviewAudio()
  const tts = useResource([...keys.tts(), 'status'], ({ signal }) => getTtsStatus(signal))
  const canPreview = (voiceId: string | null) => voicePreviewSource(voices.find((voice) => voice.id === voiceId),
    Boolean(tts.data?.ready && tts.data.capabilities.preview)) !== 'none'
  const sampleAction = useAction(async (id: string, voice: string) => {
    // Giọng có tệp mẫu trong dự án phát thẳng URL đó (giọng tải lên được ưu
    // tiên); preset của máy đọc dùng URL GET phát trực tiếp. Truyền thẳng URL
    // để play() chạy ngay trong cử chỉ; chỉ khi không có tuyến nào mới gọi API.
    const found = voices.find((item) => item.id === voice)
    const url = found?.sampleUrl ?? (found?.builtIn ? ttsPreviewURL(voice) : '')
    await preview.play(id, url || (() => previewTtsVoice(voice, t.audio.panels.xinChaoToiLa)))
  })
  if (!cast) return <div role="status" aria-label={t.audio.panels.dangTaiPhanVai} style={{ display: 'grid', gap: 12 }}>
    <Skeleton height={70} /><Skeleton height={90} /><Skeleton height={90} />
  </div>
  // Gói một giọng kể: cả truyện đọc một giọng, nên luôn hiện nhánh “một giọng” (artboard voice=pack).
  const single = pack !== null || cast.mode === 'single'
  const ordered = [...voices.filter((voice) => !voice.builtIn), ...voices.filter((voice) => voice.builtIn)]
  const missing = cast.speakers.filter((speaker) => !speaker.voiceId && !speaker.sameAs)

  const setMode = (mode: AudioCast['mode']) => onCast({ ...cast, mode, voiceMode: mode })
  // Gói một giọng kể: mọi thay đổi giọng đều ghi chế độ `single` để vai.json khớp với gói.
  const setSingle = (patch: Partial<AudioCast['single']>) => onCast({
    ...cast, mode: pack ? 'single' : cast.mode, voiceMode: pack ? 'pack' : cast.mode,
    single: { ...cast.single, ...patch },
  })
  const setSpeaker = (id: string, voiceId: string) => onCast({
    ...cast,
    speakers: cast.speakers.map((speaker) => (speaker.id === id ? { ...speaker, voiceId } : speaker)),
  })
  const sampleButton = (id: string, voiceId: string | null, label: string, size: number, iconSize: number, style?: CSSProperties) => (
    <button type="button" className="hov-ghost" aria-label={label}
      disabled={!canPreview(voiceId) || (sampleAction.busy && preview.loadingId !== id)}
      onClick={() => { if (voiceId) void sampleAction.run(id, voiceId) }}
      style={{
        width: size, height: size, flex: '0 0 auto', borderRadius: '50%', border: '1px solid rgba(255,255,255,0.1)',
        background: 'transparent', color: '#EDEDEF', display: 'flex', alignItems: 'center', justifyContent: 'center', ...style,
      }}>
      {preview.loadingId === id ? <Spinner size={iconSize} color="#EDEDEF" />
        : <Icon name={preview.playingId === id ? 'pause' : 'play'} size={iconSize} />}
    </button>
  )

  return (
    <>
      {pack ? <PackVoiceCard pack={pack} styleHref={styleHref} /> : (
        <div role="radiogroup" aria-label={t.audio.panels.cachGiaoGiong} style={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 6 }}>
          <ModeCard on={!single} title={t.audio.panels.phanVai2} icon="profile-2user"
            desc={t.audio.panels.moiNhanVatMotGiong} disabled={!writable} onClick={() => setMode('cast')} />
          <ModeCard on={single} title={t.audio.panels.motGiong} icon="microphone-2"
            desc={t.audio.panels.motNguoiDocTatCa} disabled={!writable} onClick={() => setMode('single')} />
        </div>
      )}

      {single ? (
        <>
          <div className="card" style={{ borderRadius: 14, padding: 12, display: 'flex', flexDirection: 'column', gap: 10 }}>
            <span className="lbl">{t.audio.panels.giongDocChoCaTruyen}</span>
            <div role="radiogroup" aria-label={t.audio.panels.chonGiong} style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
              {ordered.map((voice) => {
                const on = cast.single.voiceId === voice.id
                // “Giọng có sẵn · gói đề xuất”: giọng dựng sẵn đầu tiên đúng giới tính gói khai
                // (gói không khai giới tính thì giọng đang chọn) — artboard chọn sẵn “Nữ trẻ, ấm”.
                const recommended = pack !== null && voice.builtIn && (pack.gender ? voice.gender === pack.gender : on)
                return (
                  <div key={voice.id} style={{
                    display: 'flex', alignItems: 'center', gap: 8, padding: '6px 8px', borderRadius: 10,
                    border: `1px solid ${on ? 'rgba(201,166,245,0.45)' : 'transparent'}`,
                    background: on ? 'rgba(201,166,245,0.06)' : 'transparent',
                  }}>
                    {sampleButton(voice.id, voice.id, t.audio.panels.nghe(voice.name), 28, 11)}
                    <Radio checked={on} disabled={!writable} dotSize={16} markPosition="end"
                      onChange={() => setSingle({ voiceId: voice.id })}
                      style={{ flex: 1, minWidth: 0, gap: 8, padding: 0, margin: 0 }}>
                      <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
                        <span style={{ fontSize: 12.5, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{voice.name}</span>
                        <span style={{ fontSize: 11, color: '#7E7E8A' }}>{voiceDesc(voice, pack, recommended)}</span>
                      </span>
                    </Radio>
                  </div>
                )
              })}
            </div>
          </div>
          <div className="card" style={{ borderRadius: 14, padding: '4px 12px' }}>
            <SwitchRow first label={t.audio.panels.doiGiongNhe} hint={t.audio.panels.caoHoacTram}
              on={cast.single.dialogueShift} disabled={!writable} onChange={(next) => setSingle({ dialogueShift: next })} />
            <SwitchRow label={t.audio.panels.giuSacThaiTungDoan} hint={t.audio.panels.miaMaiNgot}
              on={cast.single.keepTones} disabled={!writable} onChange={(next) => setSingle({ keepTones: next })} />
            <SwitchRow label={t.audio.panels.ngungLauHon} hint={t.audio.panels.them03Giay}
              on={cast.single.dialoguePauseMs > 0} disabled={!writable}
              onChange={(next) => setSingle({ dialoguePauseMs: next ? 300 : 0 })} />
          </div>
          <Info>{t.audio.panels.nguoiNoiVanDuocGiu}</Info>
        </>
      ) : (
        <>
          <div style={lblRow}>
            <span className="lbl">{t.audio.panels.phanVaiCaTruyen}</span>
            <span style={{ fontSize: 12, color: '#7E7E8A' }}>{missing.length > 0 ? t.audio.panels.vaiChuaCoGiong(missing.length) : t.audio.panels.duGiong}</span>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            {cast.speakers.map((speaker) => {
              const none = !speaker.voiceId && !speaker.sameAs
              const voice = voices.find((item) => item.id === speaker.voiceId)
              return (
                <div key={speaker.id} className="card" style={{
                  borderRadius: 12, padding: '10px 10px 10px 12px', display: 'flex', flexDirection: 'column', gap: 8,
                  borderColor: none ? 'rgba(246,203,140,0.35)' : 'rgba(255,255,255,0.07)',
                }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                    <CharAvatar name={speaker.name} ini={speaker.ini} kind={speaker.kind} size={28} />
                    <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
                      <span style={{ fontSize: 13, fontWeight: 500 }}>{speaker.name}</span>
                      <span style={{ fontSize: 11.5, color: '#7E7E8A' }}>{speakerRoleLabel(speaker.role)}</span>
                    </span>
                    <span className="num" style={{ fontSize: 11.5, color: '#7E7E8A' }}>
                      {none ? '—' : t.audio.panels.nDoan(lineCounts[speaker.id] ?? 0)}
                    </span>
                  </div>
                  {speaker.sameAs && (
                    <span style={{
                      display: 'flex', alignItems: 'center', gap: 6, padding: '6px 8px', borderRadius: 8, background: '#0F0F11',
                      fontSize: 12, color: '#A0A0AB',
                    }}>
                      <Icon name="link-2" size={13} />{t.audio.panels.cungGiong(speaker.sameAs === 'dan' ? t.audio.panels.danTruyenThuong : speakerName(cast, speaker.sameAs))}
                    </span>
                  )}
                  {!speaker.sameAs && !none && (
                    <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                      {sampleButton(speaker.id, speaker.voiceId, t.audio.panels.ngheMauGiong(speaker.name), 30, 12)}
                      <VoicePick voices={ordered} value={speaker.voiceId} name={speaker.name} disabled={!writable}
                        onPick={(id) => setSpeaker(speaker.id, id)}>
                        {(toggle, expanded) => (
                          <button type="button" aria-haspopup="listbox" aria-expanded={expanded} aria-label={t.audio.panels.giongCho(speaker.name)}
                            disabled={!writable} onClick={toggle} style={{
                              height: 30, padding: '0 10px', borderRadius: 8, background: '#0F0F11', border: '1px solid rgba(255,255,255,0.08)',
                              display: 'flex', alignItems: 'center', gap: 8, fontSize: 12.5, color: '#EDEDEF', textAlign: 'left',
                            }}>
                            <Icon name={voice?.builtIn ? 'sound' : 'microphone-2'} size={14} style={{ color: '#A0A0AB' }} />
                            <span style={{ flex: 1, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                              {voice ? (voice.builtIn ? `${voice.name} · ${t.audio.panels.giongCoSanThuong}` : `${voice.name} · ${voice.durationSec ?? 0} giây`) : t.audio.panels.giongDaXoa}
                            </span>
                            <Icon name="arrow-down-2" size={13} style={{ color: '#7E7E8A' }} />
                          </button>
                        )}
                      </VoicePick>
                    </div>
                  )}
                  {none && (
                    <VoicePick voices={ordered} value={null} name={speaker.name} disabled={!writable}
                      onPick={(id) => setSpeaker(speaker.id, id)}>
                      {(toggle, expanded) => (
                        <button type="button" aria-haspopup="listbox" aria-expanded={expanded} disabled={!writable} onClick={toggle} style={{
                          height: 30, borderRadius: 8, border: '1px dashed rgba(246,203,140,0.45)', background: 'rgba(246,203,140,0.05)',
                          color: '#F6CB8C', fontSize: 12.5, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 6,
                        }}>
                          <Icon name="microphone-2" size={14} />{t.audio.panels.chonGiongCho(speaker.name)}
                        </button>
                      )}
                    </VoicePick>
                  )}
                </div>
              )
            })}
          </div>
          <button type="button" className="hov-ghost" onClick={onAddVoice} disabled={!writable} style={{ ...dashedBtn, height: 36 }}>
            <Icon name="add" size={15} />{t.audio.panels.themMauGiong}
          </button>
          <span style={{ padding: '0 4px', fontSize: 12, color: '#7E7E8A' }}>{t.audio.panels.mauGiong1030}</span>
        </>
      )}
    </>
  )
}

// ─── Cần xem ─────────────────────────────────────────────────────────────

type IssueItem = { key: string; icon: string; title: string; desc: string; where: string; onClick: () => void }

export function IssuesPanel({ segments, cast, mode, onJump, onCastTab }: {
  segments: AudioSegment[]; cast: AudioCast | null; mode: AudioCast['mode']
  onJump: (segmentId: string) => void; onCastTab: () => void
}) {
  const issues = issueSegments(segments, mode)
  const autoFixed = segments.reduce((sum, segment) => sum + (segment.normalized?.length ?? 0), 0)
  const items: IssueItem[] = []
  for (const segment of issues) {
    const n = segments.indexOf(segment) + 1
    if (mode !== 'single' && segment.speaker === 'unk') {
      const guess = segment.speakerGuess?.[0]
      items.push({
        key: `unk-${segment.id}`, icon: 'profile-2user', title: t.audio.logic.chuaRoNguoiNoi,
        desc: guess ? t.audio.panels.aiDoanBam(speakerName(cast, guess.who), Math.round(guess.p * 100)) : t.audio.panels.bamDeChon,
        where: t.audio.panels.doanN(n), onClick: () => onJump(segment.id),
      })
    }
    if (segment.flags?.includes('long_sentence')) {
      items.push({
        key: `long-${segment.id}`, icon: 'scissor', title: t.audio.panels.cauDaiTu(segment.text.trim().split(/\s+/).length),
        desc: t.audio.panels.giongDocDeHut, where: t.audio.panels.doanN(n), onClick: () => onJump(segment.id),
      })
    }
  }
  const english = segments.flatMap((segment) => (segment.normalized ?? []).filter((entry) => entry.kind === 'english').map((entry) => ({ segment, entry })))
  if (english.length > 0) {
    const words = [...new Set(english.map(({ entry }) => entry.from))].map((word) => `“${word}”`).join(', ')
    items.push({
      key: 'english', icon: 'translate', title: t.audio.panels.tuTiengAnhTitle(words),
      desc: t.audio.panels.daDocTheoCachDoc, where: t.audio.panels.nCho(english.length), onClick: () => onJump(english[0].segment.id),
    })
  }
  if (mode !== 'single') {
    for (const speaker of cast?.speakers ?? []) {
      if (speaker.voiceId || speaker.sameAs) continue
      const appears = speaker.role.split(' · ')[1]
      items.push({
        key: `voice-${speaker.id}`, icon: 'microphone-2', title: t.audio.panels.chuaCoGiongTitle(speaker.name),
        desc: t.audio.panels.chonTruocKhiTao(appears ? appears.charAt(0).toUpperCase() + appears.slice(1) : ''),
        where: t.audio.panels.phanVaiWhere, onClick: onCastTab,
      })
    }
  }

  return (
    <>
      <span className="lbl" style={{ padding: '0 4px' }}>{t.audio.panels.canXemTruocKhi}</span>
      {items.length === 0 && (
        <div style={{
          padding: '10px 12px', borderRadius: 12, border: '1px solid rgba(255,255,255,0.07)', fontSize: 12, color: '#A6EDC1', display: 'flex', gap: 8,
        }}>
          <Icon name="tick-circle-bulk" size={14} style={{ marginTop: 2 }} />{t.audio.panels.khongConChoNao}
        </div>
      )}
      {items.map((item) => (
        <button key={item.key} type="button" className="card" onClick={item.onClick} style={{
          borderRadius: 12, padding: '10px 12px', display: 'flex', gap: 10, textAlign: 'left', color: '#EDEDEF',
        }}>
          <Icon name={item.icon} size={16} style={{ color: '#F6CB8C', marginTop: 2, flex: '0 0 auto' }} />
          <span style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 2 }}>
            <span style={{ fontSize: 13, fontWeight: 500 }}>{item.title}</span>
            <span style={{ fontSize: 12, color: '#A0A0AB' }}>{item.desc}</span>
          </span>
          <span style={{ fontSize: 11, color: '#7E7E8A' }}>{item.where}</span>
        </button>
      ))}
      {autoFixed > 0 && (
        <div style={{
          padding: '10px 12px', borderRadius: 12, border: '1px solid rgba(255,255,255,0.07)', fontSize: 12, color: '#A0A0AB', display: 'flex', gap: 8,
        }}>
          <Icon name="tick-circle-bulk" size={14} style={{ color: '#A6EDC1', marginTop: 2, flex: '0 0 auto' }} />
          <span>{t.audio.panels.daTuDoiCho(autoFixed)}</span>
        </div>
      )}
    </>
  )
}

// ─── Cách đọc ────────────────────────────────────────────────────────────

const NORMALIZE_ROWS: { key: keyof AudioNormalize; label: string; hint: string }[] = [
  { key: 'numbers', label: t.audio.panels.docSoThanhChu, hint: t.audio.panels.n90Giay },
  { key: 'dates', label: t.audio.panels.ngayGioTien, hint: t.audio.panels.n2140 },
  { key: 'chapterTitles', label: t.audio.panels.docTenChuongO, hint: t.audio.panels.chuongBaHaiChiec },
  { key: 'dropBrackets', label: t.audio.panels.boGhiChuTrong, hint: t.audio.panels.ghiChuBienTap },
]

export function DictPanel({ entries, settings, writable, pending, success, voiceId, onSaveEntries, onDelete, onNormalize }: {
  entries: AudioPronunciation[]; settings: AudioSettings | null; writable: boolean
  pending?: boolean; success?: boolean; voiceId?: string | null
  onSaveEntries: (entries: AudioPronunciation[]) => Promise<boolean>
  onDelete: (entry: AudioPronunciation, index: number) => void
  onNormalize: (normalize: AudioNormalize) => void
}) {
  const [written, setWritten] = useState('')
  const [spoken, setSpoken] = useState('')
  const [adding, setAdding] = useState(false)
  const firstRef = useRef<HTMLInputElement>(null)
  useEffect(() => { if (adding) firstRef.current?.focus() }, [adding])
  const preview = usePreviewAudio()
  const previewAction = useAction(async (entry: AudioPronunciation) => {
    if (!voiceId) throw new Error(t.audio.panels.chonGiongTruocKhiNghe)
    await preview.play(entry.written, () => previewTtsVoice(voiceId, entry.spoken))
  })

  const normalize = settings?.normalize
  const add = async () => {
    if (!writable || !written.trim() || !spoken.trim()) return
    if (!await onSaveEntries([...entries, { written: written.trim(), spoken: spoken.trim() }])) return
    setWritten(''); setSpoken(''); setAdding(false)
  }
  useUnsavedGuard(adding && (!!written.trim() || !!spoken.trim()))
  useSaveShortcut(add, adding && !!written.trim() && !!spoken.trim() && writable)
  const groupOn = (keys: (keyof AudioNormalize)[]) => !!normalize && keys.every((key) => normalize[key])

  return (
    <>
      <div style={lblRow}>
        <span className="lbl">{t.audio.panels.cachDocCaDuAn}</span>
        <span style={{ fontSize: 12, color: '#7E7E8A' }}>{t.audio.panels.nTu(entries.length)}</span>
      </div>
      <span aria-live="polite" className="sr-only">{pending ? t.audio.panels.dangLuuCachDoc : success ? t.audio.panels.daLuu : ''}</span>
      <div className="card" style={{ borderRadius: 12, padding: '4px 12px' }}>
        {entries.length === 0 && <span style={{ display: 'block', fontSize: 12, color: '#7E7E8A', padding: '9px 0' }}>{t.audio.panels.chuaCoTuNao}</span>}
        {entries.map((entry, index) => (
          <div key={`${entry.written}-${index}`} className="audio-dictrow" style={{
            display: 'flex', alignItems: 'center', gap: 8, padding: '9px 0',
            borderTop: index === 0 ? 'none' : '1px solid rgba(255,255,255,0.06)',
          }}>
            <span style={{ flex: 1, minWidth: 0, fontSize: 13 }}>{entry.written}</span>
            <Icon name="arrow-right-2" size={13} style={{ color: '#5E5E68' }} />
            <span style={{ flex: 1, minWidth: 0, fontSize: 13, color: '#A6EDC1' }}>{entry.spoken}</span>
            <button type="button" className="hov-ghost audio-segact" aria-label={t.audio.panels.xoaCachDoc(entry.written)} disabled={!writable}
              onClick={() => onDelete(entry, index)}
              style={{ width: 26, height: 26, borderRadius: '50%', border: 'none', background: 'transparent', color: '#7E7E8A', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <Icon name="minus" size={14} />
            </button>
            <button type="button" className="hov-ghost" aria-label={t.audio.panels.ngheCachDoc(entry.written)}
              onClick={() => void previewAction.run(entry)} disabled={previewAction.busy && preview.loadingId !== entry.written}
              style={{ width: 26, height: 26, borderRadius: '50%', border: 'none', background: 'transparent', color: '#A0A0AB', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              {preview.loadingId === entry.written ? <Spinner size={12} color="#EDEDEF" /> : <Icon name={preview.playingId === entry.written ? 'pause' : 'volume-high'} size={14} />}
            </button>
          </div>
        ))}
      </div>

      {adding && (
        <div style={{ display: 'flex', gap: 4, flexDirection: 'column', padding: 8, borderRadius: 10, background: '#0F0F11', border: '1px solid rgba(255,255,255,0.08)' }}>
          <div style={{ display: 'flex', gap: 4 }}>
            <Field ref={firstRef} value={written} disabled={!writable} onChange={(event) => setWritten(event.target.value)}
              placeholder={t.audio.panels.tuTrongTruyen} aria-label={t.audio.panels.tuTrongTruyen}
              style={{ flex: 1, minWidth: 0, fontSize: 12.5, padding: '4px 6px', borderRadius: 6, border: '1px solid var(--line-3)', background: 'var(--color-surface-0)', color: 'var(--color-fg)' }} />
            <Field value={spoken} disabled={!writable} onChange={(event) => setSpoken(event.target.value)}
              onKeyDown={(event) => { if (event.key === 'Enter') void add() }}
              placeholder={t.audio.panels.docThanh} aria-label={t.audio.panels.docThanh}
              style={{ flex: 1, minWidth: 0, fontSize: 12.5, padding: '4px 6px', borderRadius: 6, border: '1px solid var(--line-3)', background: 'var(--color-surface-0)', color: 'var(--color-fg)' }} />
          </div>
          <div style={{ display: 'flex', gap: 4, justifyContent: 'flex-end' }}>
            <Button size="sm" variant="ghost" onClick={() => { setAdding(false); setWritten(''); setSpoken('') }}>{t.audio.panels.thoi}</Button>
            <Button size="sm" variant="primary" onClick={() => void add()} pending={pending} title={t.audio.panels.luuCachDocPhim} disabled={!writable || !written.trim() || !spoken.trim()}>{t.audio.panels.them}</Button>
          </div>
        </div>
      )}

      <button type="button" className="hov-ghost" onClick={() => setAdding(true)} disabled={!writable} style={{ ...dashedBtn, height: 34 }}>
        <Icon name="add" size={15} />{t.audio.panels.themTu}
      </button>
      <span className="lbl" style={{ padding: '4px 4px 0' }}>{t.audio.panels.tuChuanHoa}</span>
      <div className="card" style={{ borderRadius: 12, padding: '4px 12px' }}>
        {NORMALIZE_ROWS.map((row, index) => {
          const keys: (keyof AudioNormalize)[] = row.key === 'dates' ? ['dates', 'money', 'symbols'] : [row.key]
          return (
            <div key={row.key} style={{
              display: 'flex', alignItems: 'center', gap: 10, padding: '9px 0',
              borderTop: index === 0 ? 'none' : '1px solid rgba(255,255,255,0.06)',
            }}>
              <span style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
                <span style={{ fontSize: 13 }}>{row.label}</span>
                <span style={{ fontSize: 11.5, color: '#7E7E8A' }}>{row.hint}</span>
              </span>
              <MiniSwitch on={groupOn(keys)} label={row.label} disabled={!writable || !normalize} onChange={(next) => {
                if (!normalize) return
                onNormalize({ ...normalize, ...Object.fromEntries(keys.map((key) => [key, next])) })
              }} />
            </div>
          )
        })}
      </div>
    </>
  )
}
