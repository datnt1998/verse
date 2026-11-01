// Thẻ “việc viết đã dừng” + dải “đang viết bằng model dự phòng” trên Bàn viết
// (Main.dc.html pe.stop / pe.info, MWork.dc.html). Chỉ nói loại lỗi máy chủ đã phân loại:
// quota là tạm thời, không bao giờ trình bày như hết hạn gói.
import { useEffect, useState } from 'react'
import { useSession } from '../../api/session'
import type { ConfigResponse, Snapshot } from '../../api/types'
import { swapProvider } from '../../api/client'
import { appConfigDoc, useAppConfig } from '../../data/appModels'
import { roleModel } from '../settings/lib/config'
import { issueAction, issueLevel, issueTech, issueText, issueTitle, quotaRetryText, retryClock, type ProviderIssueKind } from '../settings/lib/providerError'
import { providerName } from '../settings/sections/connectLogic'
import { getConfigModels } from '../../api/client'
import { useResource } from '../../data/useResource'
import { navigate, paths } from '../../router'
import { Button, Icon } from '@noverse/ui'
import { toast } from '@noverse/ui'

/** Quota tự hết hạn: đồng hồ chỉ chạy khi thật sự có mốc thử lại. */
function useNow(active: boolean, ms = 30_000): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (!active) return
    const timer = window.setInterval(() => setNow(Date.now()), ms)
    return () => window.clearInterval(timer)
  }, [active, ms])
  return now
}

export function ProviderStopCard({ snapshot, isRunning, mobile = false, onResume }: {
  snapshot: Snapshot | null
  isRunning: boolean
  mobile?: boolean
  onResume?: () => void
}) {
  const { user } = useSession()
  const admin = user?.role === 'admin'
  const error = snapshot?.providerError ?? null
  const fallback = snapshot?.fallback ?? null
  const config = useAppConfig(!!error || !!fallback?.active)
  const doc = appConfigDoc(config.data)
  const chapter = snapshot?.inProgressChapter || snapshot?.currentChapter || 0
  const now = useNow(error?.kind === 'quota')
  const [pick, setPick] = useState<string>('')
  const [busy, setBusy] = useState(false)
  const [done, setDone] = useState<string>('')
  const [tech, setTech] = useState(false)

  const stopped = !isRunning && !!error
  useEffect(() => {
    if (stopped || (!isRunning && !error)) {
      setDone('')
      setPick('')
      setTech(false)
    }
  }, [stopped, isRunning, error?.provider, error?.kind, error?.retryAt, error?.message])
  const badName = error ? providerName(error.provider, null) : ''
  const kind = (error?.kind ?? 'key') as ProviderIssueKind
  const rolesUsing = error
    ? (['default', 'architect', 'writer', 'editor'] as const).filter((role) => roleModel(doc, role).provider === error.provider)
    : []

  // Lựa chọn thay thế: nhà cung cấp khác, không lỗi, có model — xếp theo thứ tự cấu hình.
  const options = Object.entries(doc.providers ?? {})
    .filter(([provider, entry]) => provider !== error?.provider && !errorOf(config.data, provider) && (entry.models ?? []).length > 0)
    .map(([provider, entry]) => ({ provider, label: providerName(provider, null), model: entry.models?.[0]?.name ?? '' }))
  const chosen = options.find((option) => option.provider === pick) ?? options[0]
  const catalog = useResource(['app-model-catalog', chosen ? `${chosen.provider}/${chosen.model}` : ''], async () => {
    if (!chosen) return null
    return (await getConfigModels(chosen.provider, [chosen.model])).models.find((entry) => entry.name === chosen.model) ?? null
  }, { enabled: !!chosen })

  if (!snapshot) return null

  const swapAndContinue = async () => {
    if (!chosen || !config.data || !error || busy) return
    setBusy(true)
    try {
      const result = await swapProvider(error.provider, chosen.provider, chosen.model, config.data.version)
      setDone(`Đã chuyển ${result.swapped.length} trợ lý sang ${chosen.label} · ${chosen.model} cho cả app.`)
      void config.refresh()
      if (result.warning) toast(result.warning, 'warn')
      onResume?.()
    } catch (cause) {
      toast(cause instanceof Error ? cause.message : 'Không chuyển được nhà cung cấp — thử lại.', 'err')
    } finally {
      setBusy(false)
    }
  }
  const openFix = () => navigate(paths.settings('connect'))

  return <>
    {!!fallback?.active && (
      <div role="status" style={{
        display: 'flex', alignItems: 'center', gap: 10, padding: mobile ? '10px 12px' : '9px 12px', borderRadius: 11,
        background: 'color-mix(in srgb, var(--color-warn) 6%, transparent)', border: '1px solid color-mix(in srgb, var(--color-warn) 22%, transparent)',
        fontSize: 12.5, color: 'var(--color-fg-2)', lineHeight: 1.5,
      }}>
        <Icon name="tick-circle" size={15} style={{ color: 'var(--color-warn)', flex: 'none' }} />
        <span style={{ flex: 1, minWidth: 0 }}>
          Đang viết bằng model dự phòng <b className="mono" style={{ fontWeight: 500 }}>{fallback.model}</b> vì {providerName(fallback.fromProvider, null)} {fallback.reason === 'quota' ? 'tạm hết lượt' : fallback.reason === 'key' ? 'không dùng được khoá' : 'hết hạn gói'}. Gia hạn xong, Noverse tự quay lại model chính.
        </span>
        <button type="button" onClick={() => navigate(paths.settings('models'))}
          style={{ flex: 'none', height: 26, padding: '0 9px', borderRadius: 7, border: '1px solid var(--line-4)', background: 'transparent', color: 'var(--color-fg-2)', fontSize: 12 }}>
          Trợ lý AI ›
        </button>
      </div>
    )}
    {stopped && error && (
      <section role="alert" aria-label="Việc viết đã dừng" style={{
        display: 'flex', flexDirection: 'column', gap: 10, padding: mobile ? '12px 13px' : '14px 16px', borderRadius: 13,
        background: kind === 'quota' ? 'color-mix(in srgb, var(--color-warn) 5%, transparent)' : 'color-mix(in srgb, var(--color-err) 5%, transparent)',
        border: `1px solid ${kind === 'quota' ? 'color-mix(in srgb, var(--color-warn) 28%, transparent)' : 'color-mix(in srgb, var(--color-err) 30%, transparent)'}`,
      }}>
        <div style={{ display: 'flex', alignItems: 'flex-start', gap: 10 }}>
          <span aria-hidden="true" style={{ flex: 'none', width: 32, height: 32, borderRadius: 9, display: 'flex', alignItems: 'center', justifyContent: 'center', background: `color-mix(in srgb, ${kind === 'quota' ? 'var(--color-warn)' : 'var(--color-err)'} 12%, transparent)`, color: kind === 'quota' ? 'var(--color-warn)' : 'var(--color-err)' }}>
            <Icon name="warning-2" size={16} />
          </span>
          <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 3 }}>
            <span style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
              <b style={{ fontSize: 14.5, fontWeight: 600 }}>
                {kind === 'quota' ? `${badName} tạm hết lượt — tự viết tiếp lúc ${retryClock(error.retryAt) || 'sau ít phút'}` : issueTitle(badName, kind).replace(/\.$/, '') + ' — viết tiếp bằng nhà cung cấp khác'}
              </b>
              <span style={{ height: 20, padding: '0 7px', borderRadius: 6, fontSize: 11.5, fontWeight: 500, display: 'inline-flex', alignItems: 'center', background: `color-mix(in srgb, ${kind === 'quota' ? 'var(--color-warn)' : 'var(--color-err)'} 14%, transparent)`, color: kind === 'quota' ? 'var(--color-warn)' : 'var(--color-err)' }}>{issueLevel(kind)}</span>
            </span>
            <span style={{ fontSize: 13, color: 'var(--color-fg-3)', lineHeight: 1.55 }}>
              {kind === 'quota'
                ? `Nhà cung cấp tạm giới hạn số lần gọi. Người viết dừng ở bước lưu chương ${chapter || '?'}, phần đã viết được giữ và tự chạy lại ${quotaRetryText(error.retryAt, now).replace(/^Noverse /, '')}`
                : `${issueText(kind)} Người viết dừng ở bước lưu chương ${chapter || '?'}; phần đã viết được giữ.${rolesUsing.length > 0 ? ` ${rolesUsing.length} trợ lý đang dùng ${badName} sẽ chuyển theo.` : ''}`}
            </span>
          </span>
        </div>
        {admin && options.length > 0 && (
          <div role="radiogroup" aria-label="Viết tiếp bằng" style={{ marginLeft: mobile ? 0 : 42, display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            {options.map((option) => {
              const on = chosen?.provider === option.provider
              return (
                <button key={option.provider} type="button" role="radio" aria-checked={on} onClick={() => setPick(option.provider)}
                  style={{
                    flex: mobile ? '1 1 100%' : '0 1 auto', minWidth: 0, display: 'flex', alignItems: 'center', gap: 9, padding: '8px 11px', borderRadius: 10,
                    border: `1px solid ${on ? 'color-mix(in srgb, var(--color-accent) 45%, transparent)' : 'var(--line-2)'}`,
                    background: on ? 'color-mix(in srgb, var(--color-accent) 8%, transparent)' : 'transparent', color: 'var(--color-fg)', textAlign: 'left',
                  }}>
                  <span aria-hidden="true" style={{ width: 15, height: 15, borderRadius: '50%', border: `1.5px solid ${on ? 'var(--color-accent)' : 'var(--line-5)'}`, display: 'flex', alignItems: 'center', justifyContent: 'center', flex: 'none' }}>
                    {on && <span style={{ width: 7, height: 7, borderRadius: '50%', background: 'var(--color-accent)' }} />}
                  </span>
                  <span style={{ minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.35 }}>
                    <span style={{ fontSize: 13 }}>{option.label}<span style={{ color: 'var(--color-fg-4)' }}> · </span><span className="mono" style={{ fontSize: 12 }}>{option.model}</span></span>
                    <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{catalog.data && option.provider === chosen?.provider ? `Ngữ cảnh ${Math.round((catalog.data.contextWindow ?? 0) / 1000)}K` : 'Nhà cung cấp khác'}</span>
                  </span>
                </button>
              )
            })}
          </div>
        )}
        <div style={{ marginLeft: mobile ? 0 : 42, display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
          {admin && (
            <Button size={mobile ? 'lg' : 'sm'} variant="primary" style={mobile ? { width: '100%', minHeight: 48 } : undefined} disabled={!chosen || busy} onClick={() => void swapAndContinue()}>
              <Icon name="play" size={13} />{kind === 'quota' ? 'Đổi và viết tiếp ngay' : 'Đổi và viết tiếp'}
            </Button>
          )}
          {admin && !mobile && kind !== 'quota' && (
            <Button size="sm" onClick={openFix}>{issueAction(kind)} {badName}</Button>
          )}
          {!admin && kind !== 'quota' && (
            <span style={{ fontSize: 12.5, color: 'var(--color-fg-4)' }}>Nhờ quản trị đổi nhà cung cấp</span>
          )}
          <span style={{ flex: 1 }} />
          <Button size="sm" variant="quiet" aria-expanded={tech} onClick={() => setTech((open) => !open)}>Chi tiết kỹ thuật</Button>
        </div>
        <span style={{ marginLeft: mobile ? 0 : 42, fontSize: 12, color: 'var(--color-fg-4)' }}>
          {kind === 'quota' ? `hoặc chờ, tự chạy lại lúc ${retryClock(error.retryAt) || 'sau ít phút'}` : mobile ? 'Đổi lại ở Thêm › Model trợ lý. Gia hạn rồi đăng nhập lại nhà cung cấp trên máy tính.' : 'Đổi lại được ở Tuỳ chỉnh › Trợ lý AI'}
        </span>
        {tech && <div className="mono" style={{ marginLeft: mobile ? 0 : 42, padding: '8px 10px', borderRadius: 8, background: 'var(--color-bg)', border: '1px solid var(--line-1)', fontSize: 11.5, color: 'var(--color-fg-3)' }}>{issueTech(error.provider, { kind, message: error.message, retryAt: error.retryAt })}</div>}
      </section>
    )}
    {!!done && isRunning && !error && (
      <div role="status" style={{
        display: 'flex', alignItems: 'center', gap: 10, padding: '9px 12px', borderRadius: 11,
        background: 'color-mix(in srgb, var(--color-ok) 6%, transparent)', border: '1px solid color-mix(in srgb, var(--color-ok) 20%, transparent)',
        fontSize: 12.5, color: 'var(--color-fg-2)', lineHeight: 1.5,
      }}>
        <Icon name="tick-circle" size={15} style={{ color: 'var(--color-ok)', flex: 'none' }} />
        <span style={{ flex: 1, minWidth: 0 }}>{done}{chapter > 0 ? ` Đang viết tiếp chương ${chapter}.` : ''}</span>
        <button type="button" onClick={() => navigate(paths.settings('models'))} style={{ flex: 'none', height: 26, padding: '0 9px', borderRadius: 7, border: '1px solid var(--line-4)', background: 'transparent', color: 'var(--color-fg-2)', fontSize: 12 }}>Trợ lý AI ›</button>
      </div>
    )}
  </>
}

/** Lỗi của một nhà cung cấp trong cấu hình chung — chỉ để loại khỏi danh sách thay thế. */
function errorOf(response: ConfigResponse | null | undefined, provider: string): boolean {
  if (!response) return false
  return !!response.accounts?.[provider]?.expired || !!response.providerErrors?.[provider]
}
