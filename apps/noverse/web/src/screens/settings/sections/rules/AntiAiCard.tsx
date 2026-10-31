import { useId, useState } from 'react'
import { Icon, toast } from '@noverse/ui'
import { getConfig, getConfigPresets, getRuleViolations } from '../../../../api/client'
import { keys, useResource } from '../../../../data'
import { DEFAULT_BAN } from './logic'
import { fatigueRows } from './fatigue'
import type { RulesEditor } from './useRulesEditor'
import { findPack, packLabel } from '../../lib/packs'
import './apRules.css'

export function AntiAiCard({ editor, onAddPhrase, mobile = false, disabled = false }: { editor: RulesEditor; onAddPhrase: () => void; mobile?: boolean; disabled?: boolean }) {
  const [open, setOpen] = useState(false)
  const id = useId()
  const { scope, project, story } = editor
  const config = useResource(keys.config(scope, project, story), ({ signal }) => getConfig(scope, project, story, signal))
  const presets = useResource(['config-presets'], ({ signal }) => getConfigPresets(signal), { staleTime: 60000 })
  const pack = findPack(presets.data?.packs, config.data?.effective.style ?? '')
  const violations = useResource([...keys.rules(scope, project, story), 'violations'], ({ signal }) => getRuleViolations(project, story, 3, signal), { enabled: scope === 'story' && !!project && !!story })
  const data = editor.resource.data
  const rows = fatigueRows(scope, data, editor.lists[scope], editor.runtime, editor.lists.project, violations.data?.chapters)
  const changed = rows.filter((row) => row.changed).length
  const bans = data?.forbiddenDefaults ?? DEFAULT_BAN
  const packWords = rows.filter((row) => row.source === 'pack')
  const set = (row: typeof rows[number], value: number) => {
    editor.setFatigue(row.word, value, row.baseline)
    toast(value === row.baseline ? `Đã về ${row.project ? 'mức của dự án' : 'mặc định'} · “${row.word}” tối đa ${value} lần` : `Đã thêm quy tắc của ${scope === 'story' ? 'truyện' : 'dự án'} · “${row.word}” tối đa ${value} lần`)
  }
  return <>{mobile && <div className="ap-mobile-heading">Chống giọng văn máy · mặc định của Noverse</div>}<section className={`ap-aai card${mobile ? ' ap-aai-mobile' : ''}`} aria-label="Chống giọng văn máy">
    <div className="ap-aai-head"><Icon name="shield-security" size={mobile ? 20 : 18} style={{ color: 'var(--kit-info-fg)', flexShrink: 0 }} />
      <span className="ap-aai-copy"><span className="ap-aai-title">{mobile ? 'Luôn bật' : <><b>Chống giọng văn máy</b><span className="ap-tag">Mặc định của Noverse · luôn bật</span></>}</span><span className="ap-meta">4 cụm từ không dùng · 16 từ dễ lặp{!mobile && ' có giới hạn mỗi chương'}{changed > 0 && (mobile ? ` · đã chỉnh ${changed} từ` : ` · ${changed} từ đã chỉnh cho ${scope === 'story' ? 'truyện này' : 'dự án'}`)}</span></span>
      <button type="button" className="ap-button" aria-label={mobile ? open ? 'Thu gọn chống giọng văn máy' : 'Xem & chỉnh chống giọng văn máy' : undefined} onClick={() => setOpen(!open)} aria-controls={id} aria-expanded={open}>{!mobile && (open ? 'Thu gọn' : 'Xem & chỉnh')}<Icon name="arrow-down-2" size={14} style={{ transform: open ? 'rotate(180deg)' : undefined }} /></button>
    </div>
    {open && <div id={id} className="ap-aai-body">
      <div><div className="ap-aai-label">{mobile ? <span className="ap-meta">Từ dễ lặp · tối đa mỗi chương</span> : <><b>Từ dễ lặp</b><span className="ap-meta">tối đa mỗi chương · chương dài hơn khoảng 1.500 chữ được tăng theo tỉ lệ</span></>}</div>
        <div className="ap-fatigue" role="list">{rows.map((row) => <div key={row.word} className="ap-fatigue-row" role="listitem">
          <span className="ap-word"><span>“{row.word}”</span>
            {(row.source === 'learned' || row.source === 'project' || row.source === 'pack') && <span className={`ap-source ap-source-${row.source}`}>{row.source === 'learned' ? 'góp ý' : row.source === 'project' ? 'dự án' : 'gói'}</span>}
            {row.over !== undefined && <span className="ap-source ap-over">vượt ở ch.{row.over}</span>}
          </span>
          {row.changed && <button type="button" className="ap-reset" disabled={disabled || !editor.canEdit} aria-label={`Về ${row.project ? 'mức của dự án' : 'mặc định'} · ${row.baseline} lần · “${row.word}”`} title={`Về ${row.project ? 'mức của dự án' : 'mặc định'} · ${row.baseline} lần`} onClick={() => set(row, row.baseline)}>↺</button>}
          <span className="ap-stepper"><button type="button" disabled={disabled || !editor.canEdit || row.limit <= 0} aria-label={`Siết “${row.word}”`} title="Siết" onClick={() => set(row, Math.max(0, row.limit - 1))}>−</button><span className={`num${row.changed ? ' ap-adjusted' : ''}`}>{row.limit} lần</span><button type="button" disabled={disabled || !editor.canEdit || row.limit >= 9} aria-label={`Nới “${row.word}”`} title="Nới" onClick={() => set(row, Math.min(9, row.limit + 1))}>+</button></span></div>)}</div></div>
      <div className="ap-bans"><div className="ap-aai-label">{mobile ? <span className="ap-meta">Cụm từ không dùng · gặp là bắt sửa</span> : <><b>Cụm từ không dùng</b><span className="ap-meta">gặp là Biên tập viên bắt sửa</span></>}</div><div className="ap-ban-list">{bans.map((word) => <span key={word} className="ap-ban"><Icon name="lock" size={12} />{mobile ? word : `“${word}”`}</span>)}{!mobile && <button type="button" className="ap-template" disabled={disabled || !editor.canEdit} onClick={onAddPhrase}><Icon name="add" size={12} />Thêm cụm của bạn</button>}</div></div>
      {!mobile && <span className="ap-meta ap-aai-foot">{scope === 'story' ? 'Mỗi lần Nới hoặc Siết tạo một quy tắc của truyện trong danh sách trên; xoá quy tắc đó là về mức cũ.' : 'Mỗi lần Nới hoặc Siết tạo một quy tắc của dự án, áp dụng cho mọi truyện trừ truyện đã chỉnh riêng.'}{pack ? packWords.length > 0 ? ` ${packLabel(pack)} đổi ngưỡng của ${packWords.map((row) => `“${row.word}”`).join(' · ')}.` : ` ${packLabel(pack)} dùng đúng các ngưỡng mặc định.` : ''}</span>}
    </div>}
  </section>{mobile && open && <p className="ap-meta" style={{ margin: '10px 30px 0' }}>Mỗi lần chỉnh tạo một quy tắc trong danh sách trên; xoá quy tắc đó là về mức cũ.</p>}</>
}
