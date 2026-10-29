// Một đoạn của bản đọc — bám Transcript.dc.html (dòng 188–223): số đoạn + nút nghe, chip người nói,
// chip sắc thái / cờ, ba nút hiện khi rê chuột (Đổi người nói · Tách đoạn · Sửa chữ), chữ đã chuẩn hoá,
// khoảng ngừng, “Gốc:”, gợi ý “AI đoán là”, gợi ý tách câu dài.
import { useEffect, useRef, useState, type CSSProperties } from 'react'
import type { AudioCast, AudioSegment } from '../../api/types'
import { Icon } from '@noverse/ui'
import { Spinner } from '@noverse/ui'
import { TextArea } from '@noverse/ui'
import { Button } from '@noverse/ui'
import { segmentParts, speakerName, speakerOf } from './logic'
import { charInitials } from '../../lib/cast'
import { Eq } from './ui'
import { t } from '../../i18n/vi'
import { CharAvatar } from '../../ui/CharAvatar'

export type SegmentRowProps = {
  segment: AudioSegment
  index: number
  cast: AudioCast | null
  writable: boolean
  showDiff: boolean
  selected: boolean
  /** bộ lọc “Cần xem” đang bật mà đoạn này không có chỗ cần xem → mờ 35% */
  dimmed: boolean
  playing: boolean
  loading: boolean
  editing: boolean
  saving?: boolean
  draft: string
  onSelect: () => void
  onPlay: () => void
  onEdit: () => void
  onDraft: (text: string) => void
  onCommit: () => void
  onCancel: () => void
  onSpeaker: (speakerId: string) => void
  onSplit: () => void
  onGuess: (speakerId: string) => void
}

const iconBtn: CSSProperties = {
  width: 26, height: 26, borderRadius: 7, border: 'none', background: 'transparent', color: '#A0A0AB',
  display: 'flex', alignItems: 'center', justifyContent: 'center',
}

export function SegmentRow(props: SegmentRowProps) {
  const { segment, index, cast, writable, showDiff, selected, dimmed, playing, loading, editing, draft } = props
  const single = cast?.mode === 'single'
  const guesses = single ? [] : segment.speakerGuess ?? []
  const textRef = useRef<HTMLTextAreaElement>(null)
  const [menu, setMenu] = useState(false)
  const [splitDismissed, setSplitDismissed] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!editing) return
    const node = textRef.current
    if (!node) return
    node.focus()
    node.setSelectionRange(node.value.length, node.value.length)
  }, [editing])
  useEffect(() => {
    if (!menu) return
    const close = (event: MouseEvent) => { if (!menuRef.current?.contains(event.target as Node)) setMenu(false) }
    document.addEventListener('mousedown', close)
    return () => document.removeEventListener('mousedown', close)
  }, [menu])

  const n = index + 1
  const pending = segment.pending === true
  const redo = segment.audio?.state === 'stale' || segment.changed === true
  const has = segment.audio?.state === 'ready'
  const dot = redo ? '#F6CB8C' : has ? '#A6EDC1' : 'transparent'
  const dotTip = redo ? t.audio.segment.chuDaDoiCanDocLai : has ? t.audio.segment.daCoGiongDungLuon : t.audio.segment.chuaCoGiong
  const playLabel = has && !redo ? t.audio.segment.nghe : t.audio.segment.taoGiongVaNghe
  const canPlay = !pending && (has || writable)
  const unk = segment.speaker === 'unk' && !single
  const name = single && segment.speaker === 'unk' ? t.audio.segment.loiThoai : speakerName(cast, segment.speaker)
  const ini = single && segment.speaker === 'unk' ? '“”' : segment.speaker === 'unk' ? '?' : charInitials(name, speakerOf(cast, segment.speaker)?.ini)
  const flags = segment.flags?.filter((flag) => flag !== 'unknown_speaker') ?? []
  const warnText = (flag: string) => flag === 'long_sentence' ? t.audio.segment.cauDai
    : flag === 'english' ? t.audio.segment.tuTiengAnh
      : flag === 'no_voice' ? t.audio.segment.chuaCoGiongCua(name) : flag
  const canSplit = flags.includes('long_sentence') && selected && !splitDismissed

  return (
    <li className="seg audio-seg" onClick={props.onSelect} data-segment={segment.id}
      aria-current={selected ? 'true' : undefined} aria-busy={props.saving || undefined}
      style={{
        display: 'flex', gap: 12, padding: 10, borderRadius: 12,
        background: redo ? 'rgba(246,203,140,0.05)' : selected ? '#151517' : 'transparent',
        border: `1px solid ${redo ? 'rgba(246,203,140,0.28)' : selected ? 'rgba(201,166,245,0.3)' : 'transparent'}`,
        opacity: pending || dimmed ? 0.35 : 1,
      }}>
      <div style={{ flex: '0 0 30px', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 6, paddingTop: 2 }}>
        <span className="num mono" style={{ fontSize: 11, color: '#5E5E68' }}>{n}</span>
        <span style={{ position: 'relative' }}>
          <span title={dotTip} aria-label={dotTip} style={{
            position: 'absolute', right: -2, top: -2, zIndex: 1, width: 9, height: 9, borderRadius: '50%',
            background: dot, boxShadow: '0 0 0 2px #0B0B0D', display: redo || has ? 'block' : 'none',
          }} />
          <button type="button" className="hov-ghost" disabled={!canPlay}
            onClick={(event) => { event.stopPropagation(); props.onPlay() }}
            aria-label={`${playLabel} đoạn ${n}`}
            style={{
              width: 28, height: 28, borderRadius: '50%', border: '1px solid rgba(255,255,255,0.1)',
              background: playing ? '#EDEDEF' : 'transparent', color: playing ? '#0B0B0D' : '#EDEDEF',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
            }}>
            {playing
              ? <Eq color="currentColor" height={12} style={{ gap: 2 }} />
              : loading
                ? <Spinner size={12} color="#EDEDEF" />
                : <Icon name="play" size={12} />}
          </button>
        </span>
      </div>

      <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 6 }}>
        <div style={{ position: 'relative', display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
          <span style={{
            height: 24, padding: '0 9px 0 3px', display: 'inline-flex', alignItems: 'center', gap: 6, borderRadius: 999,
            background: unk ? 'rgba(246,203,140,0.08)' : '#131315',
            border: `1px solid ${unk ? 'rgba(246,203,140,0.4)' : 'rgba(255,255,255,0.08)'}`,
            fontSize: 12, fontWeight: 500, color: unk ? '#F6CB8C' : '#C9C9D1',
          }}>
            {segment.speaker === 'unk'
              ? <span style={{
                width: 18, height: 18, borderRadius: '50%', background: '#222225', display: 'flex', alignItems: 'center',
                justifyContent: 'center', fontSize: 9, fontWeight: 600, color: '#EDEDEF',
              }}>{ini}</span>
              : <CharAvatar name={name} ini={speakerOf(cast, segment.speaker)?.ini} kind={speakerOf(cast, segment.speaker)?.kind} size={18} />}
            {name}
          </span>
          {segment.tone && (
            <span style={{
              height: 22, padding: '0 8px', display: 'inline-flex', alignItems: 'center', borderRadius: 6,
              background: 'rgba(255,255,255,0.05)', color: '#A0A0AB', fontSize: 11.5,
            }}>{segment.tone}</span>
          )}
          {flags.map((flag) => (
            <span key={flag} className={flag === 'long_sentence' ? 'audio-flag' : undefined} style={{
              height: 22, padding: '0 8px', display: 'inline-flex', alignItems: 'center', gap: 5, borderRadius: 6,
              background: 'rgba(246,203,140,0.1)', color: '#F6CB8C', fontSize: 11.5, borderBottom: 'none',
            }}>
              <Icon name="warning-2" size={12} />{warnText(flag)}
            </span>
          ))}
          <span style={{ flex: 1 }} />
          {!pending && !editing && (
            <span className="segact audio-segact" style={{ display: 'flex', gap: 2 }}>
              {writable && cast && cast.mode === 'cast' && (
                <button type="button" className="hov-ghost" style={iconBtn}
                  aria-label={t.audio.segment.doiNguoiNoi} title={t.audio.segment.doiNguoiNoi}
                  aria-haspopup="menu" aria-expanded={menu}
                  onClick={(event) => { event.stopPropagation(); setMenu((open) => !open) }}>
                  <Icon name="profile-2user" size={14} />
                </button>
              )}
              <button type="button" className="hov-ghost" style={iconBtn} disabled={!writable}
                aria-label={t.audio.segment.tachDoan} title={t.audio.segment.tachDoan}
                onClick={(event) => { event.stopPropagation(); props.onSplit() }}>
                <Icon name="scissor" size={14} />
              </button>
              <button type="button" className="hov-ghost" style={iconBtn} disabled={!writable}
                aria-label={t.audio.segment.suaChu2} title={t.audio.segment.suaChu2}
                onClick={(event) => { event.stopPropagation(); props.onEdit() }}>
                <Icon name="edit-2" size={14} />
              </button>
            </span>
          )}
          {menu && cast && (
            <div ref={menuRef} role="menu" aria-label={t.audio.segment.doiNguoiNoiDoan(n)} onClick={(event) => event.stopPropagation()}
              style={{
                position: 'absolute', top: 'calc(100% + 4px)', right: 0, zIndex: 20, minWidth: 200, padding: 5,
                background: 'var(--color-surface-3)', border: '1px solid var(--line-3)', borderRadius: 12,
                boxShadow: 'var(--dialog-shadow)', display: 'flex', flexDirection: 'column',
              }}>
              {[...cast.speakers.map((item) => ({ id: item.id, name: item.name })), { id: 'unk', name: t.audio.logic.chuaRoNguoiNoi }].map((item) => (
                <button key={item.id} type="button" role="menuitemradio" aria-checked={item.id === segment.speaker} className="hov-ghost"
                  onClick={() => { setMenu(false); if (item.id !== segment.speaker) props.onSpeaker(item.id) }}
                  style={{
                    textAlign: 'left', padding: '7px 9px', borderRadius: 8, fontSize: 12.5, color: '#EDEDEF',
                    background: item.id === segment.speaker ? 'var(--color-surface-6)' : 'transparent',
                  }}>{item.name}</button>
              ))}
            </div>
          )}
        </div>

        {editing ? (
          <div onClick={(event) => event.stopPropagation()}>
            <TextArea ref={textRef} value={draft} onChange={(event) => props.onDraft(event.target.value)}
              disabled={!writable} showCount={false}
              onKeyDown={(event) => {
                if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); props.onCommit() }
                if (event.key === 'Escape') { event.preventDefault(); props.onCancel() }
              }}
              aria-label={t.audio.segment.suaChuDoan(n)} rows={3}
              error={!draft.trim() ? t.audio.segment.noiDungKhongTrong : undefined}
              style={{
                width: '100%', padding: '7px 9px', borderRadius: 8, resize: 'vertical',
                border: '1px solid var(--line-focus)', background: 'var(--color-surface-0)',
                color: 'var(--color-fg)', fontFamily: "'Literata', Georgia, serif", fontSize: 15.5, lineHeight: 1.75,
              }} />
            <div style={{ display: 'flex', gap: 6, marginTop: 6 }}>
              <Button size="sm" variant="primary" disabled={!writable || !draft.trim()} pending={props.saving} onClick={props.onCommit} title={t.audio.segment.luuDoanPhim}>{t.audio.segment.luu}</Button>
              <Button size="sm" variant="ghost" disabled={props.saving} onClick={props.onCancel} title={t.audio.segment.huySuaPhim}>{t.audio.segment.huy}</Button>
            </div>
          </div>
        ) : (
          <p style={{ margin: 0, fontFamily: "'Literata', Georgia, serif", fontSize: 15.5, lineHeight: 1.75, color: pending ? '#5E5E68' : '#DADAE0' }}>
            {segmentParts(segment).map((part, partIndex) => part.normalized ? (
              <span key={partIndex} className="norm audio-norm" style={{ borderRadius: 3 }}
                title={`${part.normalized.from} → ${part.normalized.to}${part.normalized.kind === 'english' ? ` (${t.audio.segment.cachDoc})` : ''}`}>{part.text}</span>
            ) : (
              <span key={partIndex} style={{ borderRadius: 3 }}>{part.text}</span>
            ))}
            {!pending && segment.pauseMs > 0 && (
              <span title={t.audio.segment.ngungSauDoan} style={{
                marginLeft: 8, padding: '1px 6px 1px 4px', borderRadius: 5, background: '#1E1E21', color: '#7E7E8A',
                fontSize: 11, fontFamily: "'Be Vietnam Pro', sans-serif", verticalAlign: 2, display: 'inline-flex', alignItems: 'center', gap: 3,
              }}>
                <Icon name="pause" size={10} />{(segment.pauseMs / 1000).toLocaleString('vi-VN', { minimumFractionDigits: 1 })} s
              </span>
            )}
          </p>
        )}

        {showDiff && segment.source && segment.source !== segment.text && (
          <p style={{
            margin: 0, padding: '6px 10px', borderLeft: '2px solid #2A2A2E', fontFamily: "'Literata', Georgia, serif",
            fontSize: 13.5, lineHeight: 1.6, color: '#7E7E8A',
          }}>{t.audio.segment.goc}{segment.source}</p>
        )}

        {guesses.length > 0 && !pending && (
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap', fontSize: 12.5, color: '#C9C9D1' }}>
            <span>{t.audio.segment.aiDoanLa}</span>
            {guesses.map((guess, guessIndex) => (
              <button key={guess.who} type="button" disabled={!writable}
                onClick={(event) => { event.stopPropagation(); props.onGuess(guess.who) }}
                style={{
                  height: 26, padding: '0 10px', borderRadius: 999, fontSize: 12,
                  border: `1px solid ${guessIndex === 0 ? 'rgba(201,166,245,0.45)' : 'rgba(255,255,255,0.1)'}`,
                  background: guessIndex === 0 ? 'rgba(201,166,245,0.08)' : 'transparent',
                  color: guessIndex === 0 ? '#EDEDEF' : '#C9C9D1',
                }}>{speakerName(cast, guess.who)} · {Math.round(guess.p * 100)}%</button>
            ))}
            <button type="button" disabled={!writable}
              onClick={(event) => { event.stopPropagation(); setMenu(true) }}
              style={{
                height: 26, padding: '0 10px', borderRadius: 999, border: '1px dashed rgba(255,255,255,0.2)',
                background: 'transparent', color: '#A0A0AB', fontSize: 12,
              }}>{t.audio.segment.nguoiKhac}</button>
          </div>
        )}

        {canSplit && (
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12.5, color: '#C9C9D1' }}>
            <span>{t.audio.segment.cauNenNgat(segment.text.split(/\s+/).filter(Boolean).length)}</span>
            <button type="button" disabled={!writable} onClick={(event) => { event.stopPropagation(); props.onSplit() }}
              style={{
                height: 26, padding: '0 10px', borderRadius: 7, border: '1px solid rgba(255,255,255,0.12)', background: 'transparent',
                color: '#EDEDEF', fontSize: 12, display: 'flex', alignItems: 'center', gap: 5,
              }}>
              <Icon name="scissor" size={13} />{t.audio.segment.tachThanh2Doan}
            </button>
            <button type="button" onClick={(event) => { event.stopPropagation(); setSplitDismissed(true) }}
              style={{ height: 26, padding: '0 8px', borderRadius: 7, border: 'none', background: 'transparent', color: '#7E7E8A', fontSize: 12 }}>
              {t.audio.segment.boQua}
            </button>
          </div>
        )}
      </div>
    </li>
  )
}
