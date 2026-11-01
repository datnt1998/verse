// “Có gì mới” (Settings.dc.html › modal `wnOpen`, mở từ chân thanh bên, banner, menu tài khoản, Nâng cao).
// Dữ liệu: `GET /api/system`. Nhật ký thay đổi từng bản chưa có nguồn từ máy chủ nên hộp chỉ hiện
// bản đang dùng và bản mới nhất — không tự bịa nội dung từng bản.
import { Dialog } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { buildStamp, hasNewVersion, latestLine, releaseRows, RELEASES_URL, useVersionInfo, versionText } from './version'

function VersionRow({ version, date, tone, label }: { version: string; date: string; tone: 'new' | 'current'; label: string }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
      <b className="num" style={{ fontSize: 14, fontWeight: 600 }}>Noverse {version}</b>
      {date && <span className="num" style={{ fontSize: 12.5, color: 'var(--color-fg-4)' }}>{date}</span>}
      <span style={{
        height: 20, padding: '0 7px', borderRadius: 6, fontSize: 11, display: 'inline-flex', alignItems: 'center',
        background: tone === 'new' ? 'var(--kit-up-badge-bg)' : 'var(--line-1)',
        color: tone === 'new' ? 'var(--kit-up-badge-fg)' : 'var(--color-fg-3)',
      }}>{label}</span>
    </div>
  )
}

export function WhatsNewDialog({ onClose }: { onClose: () => void }) {
  const info = useVersionInfo()
  const current = versionText(info?.version)
  const latest = latestLine(info)
  const newer = hasNewVersion(info)
  const rows = releaseRows(info?.releases, current)

  const download = () => {
    window.open(RELEASES_URL, '_blank', 'noopener')
    toast.ok(`Đã mở trang tải bản ${latest}`)
    onClose()
  }

  return (
    <Dialog padBody={false} width={560} padX={22} onClose={onClose} style={{ borderRadius: 20 }}
      icon={<Icon name="refresh-2" size={18} style={{ color: 'var(--kit-info-fg)' }} />}
      title={(
        <span style={{ display: 'flex', alignItems: 'center', gap: 10, width: '100%' }}>
          <span>Có gì mới</span>
          {current && <span style={{ marginLeft: 'auto', fontSize: 12.5, fontWeight: 400, color: 'var(--color-fg-4)' }}>Đang dùng {current}</span>}
        </span>
      )}
      footer={(
        <>
          <button type="button" className="hov-ghost" onClick={onClose} style={{
            height: 36, padding: '0 14px', borderRadius: 10, border: 'none', background: 'transparent',
            color: 'var(--color-fg-3)', fontSize: 13,
          }}>Đóng</button>
          {newer && (
            <button type="button" className="hov-primary" onClick={download} style={{
              height: 36, padding: '0 16px', borderRadius: 10, border: 'none', background: 'var(--color-fg)',
              color: 'var(--color-bg)', fontSize: 13, fontWeight: 600,
            }}>Tải bản {latest}</button>
          )}
        </>
      )}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 14, padding: '0 22px 16px' }}>
        {rows.length === 0 ? (
          <section style={{ display: 'flex', flexDirection: 'column', gap: 8, paddingTop: 4 }}>
            <VersionRow version={current || '—'} date={buildStamp(info?.built).split(' ')[0] ?? ''} tone="current" label="Đang dùng" />
          </section>
        ) : rows.map((release, index) => (
          <section key={`${release.version}/${index}`} style={{
            display: 'flex', flexDirection: 'column', gap: 8, paddingTop: index === 0 ? 4 : 12,
            borderTop: index === 0 ? undefined : '1px solid var(--line-1)',
          }}>
            <VersionRow version={release.version || '—'} date={release.date}
              tone={release.current ? 'current' : 'new'} label={release.current ? 'Đang dùng' : 'Mới'} />
            {release.items.map((item, at) => (
              <div key={at} style={{ display: 'flex', gap: 10, fontSize: 13, color: 'var(--color-fg-2)', lineHeight: 1.5 }}>
                {item.kind && (
                  <span style={{
                    height: 20, padding: '0 7px', borderRadius: 6, fontSize: 11, flex: '0 0 auto', marginTop: 1,
                    display: 'inline-flex', alignItems: 'center',
                    background: item.kind === 'Mới' ? 'var(--tint-ok)' : 'var(--tint-warn)',
                    color: item.kind === 'Mới' ? 'var(--color-ok)' : 'var(--color-warn)',
                  }}>{item.kind}</span>
                )}
                <span>{item.text}</span>
              </div>
            ))}
          </section>
        ))}
        {newer && (
          <div style={{
            display: 'flex', gap: 10, alignItems: 'flex-start', padding: '10px 12px', borderRadius: 12,
            background: 'var(--color-scope-bg)', border: '1px solid var(--line-2)',
          }}>
            <Icon name="info-circle" size={15} style={{ color: 'var(--color-fg-3)', marginTop: 2 }} />
            <span style={{ fontSize: 12.5, color: 'var(--color-fg-3)', lineHeight: 1.55 }}>
              Cài bản mới trên máy chủ rồi mở lại Noverse. Truyện đang viết tự tiếp tục từ chỗ dừng; chương mới ghi phiên bản mới.
            </span>
          </div>
        )}
      </div>
    </Dialog>
  )
}
