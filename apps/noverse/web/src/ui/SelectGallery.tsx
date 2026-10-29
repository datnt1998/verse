import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import { Kbd } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { KitSelect, RichSelect, Combobox, MultiCombobox, type Option } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { formatContextK } from '../lib/modelContext'

const languages: Option[] = [['vi', 'Tiếng Việt'], ['en', 'English'], ['zh', '中文'], ['ja', '日本語'], ['ko', '한국어']].map(([value, label]) => ({ value, label, code: value }))
const voices: Option[] = [
  ['lan', 'Ngọc Lan', 'Nữ · miền Bắc · ấm, chậm', 'NL', 'linear-gradient(135deg,#F3E8FF,#C9A6F5)'], ['minh', 'Minh Quân', 'Nam · miền Bắc · trầm', 'MQ', 'linear-gradient(135deg,#EEF4FF,#7C93F3)'],
  ['thao', 'Phương Thảo', 'Nữ · miền Nam · trong', 'PT', 'linear-gradient(135deg,#FFF1DC,#EC9548)'], ['bao', 'Gia Bảo', 'Nam · miền Nam · trẻ', 'GB', 'linear-gradient(135deg,#E9FBEF,#5DC1B0)'],
].map(([value, label, description, avatar, avatarColor]) => ({ value, label, description, avatar, avatarColor, group: 'Giọng trên máy' }))
const models: Option[] = [
  ['ChatGPT', 'gpt-6.1-sol', 400000, 'tốt nhất'], ['ChatGPT', 'gpt-6-luna', 400000, 'cân bằng'], ['ChatGPT', 'gpt-5.6-terra', 200000, 'nhanh'],
  ['OpenRouter', 'claude-3.5-sonnet', 200000, 'văn mượt'], ['OpenRouter', 'gemini-2.5-pro', 1000000, 'ngữ cảnh dài'], ['OpenRouter', 'gemini-2.5-flash', 1000000, 'rẻ'],
  ['DeepSeek', 'deepseek-chat', 128000, 'rẻ'],
].map(([group, value, window, note]) => ({ value: String(value), label: String(value), group: String(group), meta: `${formatContextK(Number(window))} · ${note}` }))
const genres: Option[] = [['Ngôn tình', 182], ['Hiện đại', 140], ['Showbiz', 36], ['Trọng sinh', 97], ['Hài hước', 64], ['Cổ trang', 120], ['Xuyên sách', 41], ['Trinh thám', 28], ['Học đường', 52]].map(([label, count]) => ({ value: String(label), label: String(label), meta: `${count} truyện` }))

const triggerStates: { name: string; value: string; fg: string; demo?: string; message?: string; disabled?: boolean }[] = [
  { name: 'Mặc định', value: 'Tiếng Việt', fg: '#EDEDEF' }, { name: 'Chưa chọn', value: 'Chọn ngôn ngữ…', fg: '#7E7E8A' }, { name: 'Rê chuột', value: 'Tiếng Việt', fg: '#EDEDEF', demo: 'hover' },
  { name: 'Đang mở / focus', value: 'Tiếng Việt', fg: '#EDEDEF', demo: 'open' }, { name: 'Lỗi', value: 'Chọn ngôn ngữ…', fg: '#7E7E8A', demo: 'err', message: 'Cần chọn ngôn ngữ' }, { name: 'Vô hiệu', value: 'Tiếng Việt', fg: '#EDEDEF', disabled: true },
]
const cbModes = [['ready', 'Có dữ liệu'], ['loading', 'Đang tải'], ['error', 'Lỗi tải']] as const

const cardStyle: CSSProperties = { borderRadius: 16, padding: 18, display: 'flex', flexDirection: 'column', gap: 10 }

function Card({ title, tech, hint, minHeight, children }: { title: string; tech: string; hint: ReactNode; minHeight: number; children: ReactNode }) {
  return <section className="card" style={{ ...cardStyle, minHeight }}>
    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' }}><span style={{ fontSize: 14, fontWeight: 600 }}>{title}</span><span className="mono" style={{ fontSize: 11.5, color: '#7E7E8A' }}>{tech}</span></div>
    <span style={{ fontSize: 12.5, color: '#8E8E99' }}>{hint}</span>
    {children}
  </section>
}

export function SelectGallery({ initialOpen = 'voice' }: { initialOpen?: string }) {
  const [lang, setLang] = useState('vi')
  const [voice, setVoice] = useState('lan')
  const [playing, setPlaying] = useState<string | null>(null)
  const [model, setModel] = useState('gpt-6-luna')
  const [mode, setMode] = useState<(typeof cbModes)[number][0]>('ready')
  const [modeKey, setModeKey] = useState(0)
  const [selectedGenres, setGenres] = useState(['Ngôn tình', 'Showbiz', 'Hiện đại'])
  const timers = useRef<number[]>([])
  useEffect(() => () => timers.current.forEach(window.clearTimeout), [])
  const later = (run: () => void, ms: number) => { timers.current.push(window.setTimeout(run, ms)) }
  return <div style={{ display: 'flex', flexDirection: 'column', gap: 22 }}>
    <section style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
      <span className="lbl" style={{ padding: '0 4px' }}>Trạng thái ô chọn</span>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(6, minmax(0, 1fr))', gap: 12 }}>
        {triggerStates.map((state) => <div key={state.name} style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          <span style={{ fontSize: 11.5, color: '#8E8E99' }}>{state.name}</span>
          <button className="kit-sel-trg" type="button" data-demo={state.demo} disabled={state.disabled} tabIndex={-1}><span style={{ flex: '1 1 auto', minWidth: 0, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', color: state.fg }}>{state.value}</span><Icon name="arrow-down-2" size={15} className="kit-sel-chev" style={{ color: '#7E7E8A' }} /></button>
          {state.message && <span style={{ fontSize: 11.5, color: '#FFB3BA' }}>{state.message}</span>}
        </div>)}
      </div>
    </section>

    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 20 }}>
      <Card title="Select đơn" tech="role=listbox" minHeight={440} hint="Dưới 7 lựa chọn, không cần tìm. Mục đang chọn có dấu tick.">
        <KitSelect label="Ngôn ngữ truyện" icon={<Icon name="language-square" size={16} />} options={languages} value={lang} onChange={setLang} defaultOpen={initialOpen === 'lang'} style={{ width: 300, marginTop: 6 }} />
      </Card>
      <Card title="Select nhiều thông tin" tech="role=listbox" minHeight={440} hint="Mỗi lựa chọn có ảnh, mô tả và nút phụ (nghe thử). Bấm nút phụ không làm đóng danh sách.">
        <RichSelect label="Giọng người kể" options={voices} value={voice} onChange={setVoice} defaultOpen={initialOpen === 'voice'} playing={playing}
          onPreview={(item) => { setPlaying(item.value); later(() => setPlaying((current) => current === item.value ? null : current), 2200) }}
          onManage={() => toast('Đã chọn Quản lý giọng đọc', 'info')} style={{ width: 340, marginTop: 6 }} />
      </Card>
      <Card title="Combobox có tìm kiếm" tech="role=combobox" minHeight={460} hint="Danh sách dài, nhóm theo nhà cung cấp. Gõ để lọc, phần khớp được tô sáng.">
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ fontSize: 11.5, color: '#7E7E8A' }}>Thử:</span>
          <div role="radiogroup" aria-label="Trạng thái danh sách" style={{ display: 'flex', gap: 3, padding: 3, borderRadius: 9, background: '#0F0F11', border: '1px solid rgba(255,255,255,.06)' }}>
            {cbModes.map(([id, text]) => <button key={id} type="button" role="radio" aria-checked={mode === id} onClick={() => { setMode(id); setModeKey((key) => key + 1) }} style={{ height: 24, padding: '0 9px', border: 'none', borderRadius: 6, background: mode === id ? '#2A2A2E' : 'transparent', color: mode === id ? '#EDEDEF' : '#A0A0AB', fontSize: 11.5, whiteSpace: 'nowrap' }}>{text}</button>)}
          </div>
        </div>
        <Combobox key={modeKey} label="Model cho Người viết" noun="model" placeholder="Tìm model…" showCurrent options={models} value={model} onChange={setModel} defaultOpen={initialOpen === 'model' || modeKey > 0}
          loading={mode === 'loading'} loadError={mode === 'error' ? 'Nhà cung cấp không phản hồi. Model đã chọn vẫn dùng bình thường.' : undefined}
          onRetry={() => { setMode('loading'); later(() => setMode('ready'), 1200) }} onAddProvider={() => toast('Đã chọn Thêm nhà cung cấp', 'info')} style={{ width: 380 }} />
      </Card>
      <Card title="Chọn nhiều (thẻ)" tech="aria-multiselectable" minHeight={460} hint={<>Thẻ đã chọn nằm trong ô. <Kbd>⌫</Kbd> khi ô trống xoá thẻ cuối. Có thể tạo thẻ mới.</>}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6, width: 420, marginTop: 6 }}>
          <MultiCombobox label="Thể loại" placeholder="Chọn hoặc gõ thể loại…" options={genres} value={selectedGenres} onChange={setGenres} defaultOpen={initialOpen === 'genres'} />
          <span style={{ fontSize: 11.5, color: '#7E7E8A' }}>Tối đa 5 thể loại — Kiến trúc sư dùng để chọn giọng và nhịp truyện.</span>
        </div>
      </Card>
    </div>
  </div>
}
