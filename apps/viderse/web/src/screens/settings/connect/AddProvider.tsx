import * as stylex from '@stylexjs/stylex'
import { styles as settingsStyles, connAddRowMarker, connAddFieldMarker, connAddMMarker, connSheetPrimaryMarker } from '../settings.stylex'
// AddProvider — thẻ/sheet "Thêm nhà cung cấp" (artboard Settings conn=add,
// MSettings sheet=add): lưới 8 nhà cung cấp (4 cột máy tính, 2 cột điện thoại),
// ô khoá API (Ollama: địa chỉ; Tương thích OpenAI: cả hai), nút "Kết nối và
// lấy model". "Kết nối" → POST /api/ai/providers → thẻ mới lên đầu (conn=added);
// 401/422 auth_failed → thẻ cam ở đầu danh sách (conn=failed).
import { useState } from 'react'
import { Button, Icon } from '@noverse/ui'
import { Field } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { toast } from '@noverse/ui'
import { addProvider, type AiProvider } from '../../../api/ai'
import { keys } from '../../../data/keys'
import { aiCopy } from '../../../i18n/ai'
import { t } from '../../../i18n/settings'
import { existingCardFor } from './order'
import { fieldOf, reasonOf } from './ProviderCard'

const c = aiCopy.connect

export type AddKind = {
  kind: string
  label: string
  auth: 'key' | 'url' | 'both'
  /** Chú thích dưới tên trên máy tính (artboard Settings conn=add). */
  sub: string
  /** Chú thích ngắn trên điện thoại (artboard MSettings sheet=add). */
  subMobile: string
}

/** 8 loại của design (ChatGPT/Grok có thẻ quản lý sẵn trong danh sách). */
export const ADD_KINDS: readonly AddKind[] = [
  { kind: 'openai', label: 'OpenAI', auth: 'key', sub: 'Khoá API · chữ, vẽ ảnh', subMobile: 'Chữ, vẽ ảnh' },
  { kind: 'gemini', label: 'Gemini', auth: 'key', sub: 'Khoá API · chữ, đọc ảnh, vẽ ảnh', subMobile: 'Chữ, đọc ảnh, vẽ ảnh' },
  { kind: 'openrouter', label: 'OpenRouter', auth: 'key', sub: 'Một khoá, hàng trăm model', subMobile: 'Hàng trăm model' },
  { kind: 'deepseek', label: 'DeepSeek', auth: 'key', sub: 'Khoá API · chữ', subMobile: 'Chữ' },
  { kind: 'xai', label: 'xAI', auth: 'key', sub: 'Khoá API · chữ, vẽ ảnh', subMobile: 'Chữ, vẽ ảnh' },
  { kind: 'anthropic', label: 'Anthropic', auth: 'key', sub: 'Khoá API · chữ, đọc ảnh', subMobile: 'Chữ, đọc ảnh' },
  { kind: 'ollama', label: 'Ollama', auth: 'url', sub: 'Chạy trên máy này', subMobile: 'Trên máy' },
  { kind: 'openai_compat', label: 'Tương thích OpenAI', auth: 'both', sub: 'Địa chỉ riêng + khoá', subMobile: 'Địa chỉ riêng' },
]

export function AddProviderBody({ onAdded, onReveal, onDone, mobile, providers }: {
  onAdded: (id: string) => void
  /** Chọn lại nguồn đã có thẻ: đóng panel, cuộn tới thẻ đó và mở Đổi khoá. */
  onReveal: (id: string) => void
  /** Đóng sheet (điện thoại); desktop để trống — thẻ gập lại bằng nút Đóng. */
  onDone?: () => void
  /** bare = nội dung sheet (không khung thẻ), cỡ chữ của MSettings. */
  mobile?: boolean
  /** Danh sách thẻ hiện có — loại đã có thẻ hiện mờ “Đã kết nối” và dẫn tới thẻ đó. */
  providers: readonly AiProvider[]
}) {
  const takenBy = (kind: string) => existingCardFor(providers, kind)
  const firstOpen = () => {
    const preferred = ADD_KINDS.find((k) => k.kind === 'openrouter' && !takenBy(k.kind))
    return (preferred ?? ADD_KINDS.find((k) => !takenBy(k.kind)))?.kind ?? ''
  }
  const [kind, setKind] = useState(firstOpen)
  const [key, setKey] = useState('')
  const [url, setUrl] = useState('')
  const [busy, setBusy] = useState(false)

  const picked = ADD_KINDS.find((k) => k.kind === kind)
  const needsKey = picked != null && picked.auth !== 'url'
  const needsUrl = picked != null && picked.auth !== 'key'
  const canSubmit = picked != null
    && (!needsKey || key.trim().length > 0)
    && (!needsUrl || url.trim().length > 0)

  async function connect() {
    if (!picked || busy) return
    setBusy(true)
    try {
      const provider = await addProvider({
        kind: picked.kind,
        ...(needsKey ? { apiKey: key.trim() } : {}),
        ...(needsUrl ? { baseUrl: url.trim() } : {}),
      })
      await invalidate(keys.aiProviders())
      onAdded(provider.id)
      onDone?.()
    } catch (error) {
      await invalidate(keys.aiProviders())
      const id = fieldOf(error, 'id')
      if (reasonOf(error) === 'duplicate' && id) {
        // Thẻ cùng loại/địa chỉ đã có (tab khác vừa thêm): dẫn tới thẻ đó.
        toast.err(error instanceof Error ? error.message : t.errors.failed)
        onReveal(id)
        onDone?.()
        return
      }
      // auth_failed (401): thẻ đã được tạo với status auth_failed — body có id
      // để UI đưa thẳng thẻ cam lên đầu (conn=failed).
      if (id) onAdded(id)
      toast.err(error instanceof Error ? error.message : t.errors.failed)
    } finally {
      setBusy(false)
    }
  }

  const name = picked?.label ?? 'nhà cung cấp'
  const tiles = ADD_KINDS.map((k) => {
    const existing = takenBy(k.kind)
    const done = existing != null
    return (
      <button
        key={k.kind}
        type="button"
        role="radio"
        aria-checked={kind === k.kind}
        title={done ? c.alreadyConnected(k.label) : undefined}
        className={`conn-tile ${stylex.props(settingsStyles.connTile, done && settingsStyles.connTileTaken).className}`}
        onClick={() => {
          if (existing == null) { setKind(k.kind); return }
          onReveal(existing)
          onDone?.()
        }}
      >
        <span className={`conn-tile__name ${stylex.props(settingsStyles.connTileName).className}`}>{k.label}</span>
        <span className={`conn-tile__sub ${stylex.props(settingsStyles.connTileSub).className}`}>{done ? c.connected : (mobile ? k.subMobile : k.sub)}</span>
      </button>
    )
  })

  const submit = (
    <Button size="compact" variant="primary" className="conn-act" xstyle={[settingsStyles.connAct, settingsStyles.addProviderSubmit]} pending={busy} pendingLabel={c.connecting} onClick={() => void connect()} disabled={!canSubmit}>
      {c.connectFetch}
    </Button>
  )

  /** Desktop: nút Kết nối đứng cạnh ô nhập. Điện thoại: một nút full-width
   *  dưới sheet (artboard MSettings sheet=add) nên ô nhập chiếm trọn hàng. */
  const fields = (inlineSubmit: boolean) => (
    <>
      {needsKey && (
        <div className={`conn-add__row ${stylex.props(connAddRowMarker, settingsStyles.connAddRow).className}`}>
          <label className={`conn-add__field ${stylex.props(connAddFieldMarker, settingsStyles.connAddField).className}`}>
            <span className={`conn-add__label ${stylex.props(settingsStyles.connAddLabel).className}`}>{c.keyLabelOf(name)}</span>
            <Field
              type="password"
              autoComplete="off"
              xstyle={settingsStyles.providerInput}
              value={key}
              onChange={(e) => setKey(e.target.value)}
              placeholder="sk-…"
              aria-label={c.keyLabelOf(name)}
            />
          </label>
          {inlineSubmit && !needsUrl && submit}
        </div>
      )}
      {needsUrl && (
        <div className={`conn-add__row ${stylex.props(connAddRowMarker, settingsStyles.connAddRow).className}`}>
          <label className={`conn-add__field ${stylex.props(connAddFieldMarker, settingsStyles.connAddField).className}`}>
            <span className={`conn-add__label ${stylex.props(settingsStyles.connAddLabel).className}`}>{c.addressLabelOf(name)}</span>
            <Field
              xstyle={settingsStyles.providerInput}
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder={c.addressPlaceholder}
              aria-label={c.addressLabelOf(name)}
            />
          </label>
          {inlineSubmit && submit}
        </div>
      )}
    </>
  )

  if (mobile) {
    return (
      <>
        <div className={`conn-add conn-add--m ${stylex.props(connAddMMarker, settingsStyles.connAdd, settingsStyles.connAddM).className}`}>
          <div className={`conn-add__grid ${stylex.props(settingsStyles.connAddGrid).className}`} role="radiogroup" aria-label={c.addTitle}>{tiles}</div>
          {fields(false)}
          <span className={`conn-add__hint ${stylex.props(settingsStyles.connAddHint).className}`}>{c.addHintMobile}</span>
        </div>
        <div className={`conn-sheet-primary ${stylex.props(connSheetPrimaryMarker, settingsStyles.connSheetPrimary).className}`}>
          <Button size="xl" variant="primary" pending={busy} pendingLabel={c.connecting} onClick={() => void connect()} disabled={!canSubmit} xstyle={[settingsStyles.ConnSheetPrimary_]}>
            {c.connectFetch}
          </Button>
        </div>
      </>
    )
  }

  return (
    <div className={`conn-add ${stylex.props(settingsStyles.connAdd).className}`}>
      <div className={`conn-add__head ${stylex.props(settingsStyles.connAddHead).className}`}>
        <h2 className={`conn-add__title ${stylex.props(settingsStyles.connAddTitle).className}`}>{c.addTitle}</h2>
        {onDone && (
          <Button size="compact" variant="icon" aria-label={c.close} onClick={onDone} icon={<Icon name="close" size={15} />} />
        )}
      </div>
      <div className={`conn-add__grid ${stylex.props(settingsStyles.connAddGrid).className}`} role="radiogroup" aria-label={c.addTitle}>{tiles}</div>
      {fields(true)}
      <span className={`conn-add__hint ${stylex.props(settingsStyles.connAddHint).className}`}>
        <Icon name="key" size={14} className="conn-ic" />
        {c.addHint}
      </span>
    </div>
  )
}
