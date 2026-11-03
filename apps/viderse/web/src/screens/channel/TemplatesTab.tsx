import * as stylex from '@stylexjs/stylex'
import { channelStyles } from './channel.stylex'
// Tab Template luân phiên (artboard Channel › tab=templates): danh sách có thứ
// tự, đánh dấu "Video tiếp theo" theo con trỏ luân phiên của kênh; đổi thứ tự
// bằng Lên/Xuống (desktop), bỏ khỏi danh sách, thêm từ thư viện, tạo Mẫu chuẩn.
// Mỗi hàng mở trang cài đặt template. Thẻ "sức khoẻ" của artboard cần dữ liệu
// kiểm định template — chưa có nên chỉ hiện recipe thật (tên + dòng phụ).
import { useState } from 'react'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { Button, Dialog, Icon, toast } from '@noverse/ui'
import { ResponsiveActionMenu } from '../../components/responsive-overlays'
import { invalidate, setData } from '@noverse/ui/data'
import { ActionButton } from '../../components'
import { t } from '../../i18n/channels'
import { paths } from '../../router'
import { keys } from '../../data/keys'
import { useResource } from '../../data/useResource'
import { listTemplates } from '../../api/library'
import { setChannelTemplates, type Channel } from '../../api/channels'
import { templateLine } from './templateLine'
import { mch } from './mobile/mchannel.stylex'

export function TemplatesTab({ channel }: { channel: Channel }) {
  const tt = t.channels.templates
  const phone = useMediaQuery('(max-width: 767px)')
  const library = useResource(keys.templates(), ({ signal }) => listTemplates(signal))
  const [open, setOpen] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const apply = async (action: Parameters<typeof setChannelTemplates>[1], done?: string) => {
    setError(null)
    try {
      const { templates } = await setChannelTemplates(channel.id, action)
      setData(keys.channel(channel.id), (prev) => (prev ? { ...prev, templates } : prev))
      void invalidate(keys.channels())
      if (done) toast.ok(done)
    } catch (err) {
      setError(err instanceof Error ? err.message : tt.saveError)
    }
  }

  const nextId = channel.templates.find((template) => template.position === channel.templateCursor)?.id
    ?? channel.templates[0]?.id
  const summaries = new Map((library.data?.items ?? []).map((item) => [item.id, item]))

  if (phone) {
    return (
      <section aria-label={t.channels.page.tabs.templates} {...stylex.props(mch.vSection)}>
        <p {...stylex.props(mch.hint)}>{tt.banner}</p>
        {channel.templates.length === 0 && <p {...stylex.props(mch.hint)}>{tt.empty}</p>}
        <div {...stylex.props(mch.tplList)}>
          {channel.templates.map((template, index) => (
            <div key={template.id} {...stylex.props(mch.tplRow, template.id === nextId && mch.tplRowNext)}>
              <span className={`num ${stylex.props(mch.tplN).className}`}>{index + 1}</span>
              <a href={`#${paths.template(template.id)}`} {...stylex.props(mch.tplThumb)} aria-label={tt.settings}>
                <Icon name="layer" size={18} />
              </a>
              <div {...stylex.props(mch.tplText)}>
                <div {...stylex.props(mch.tplNameRow)}>
                  <a href={`#${paths.template(template.id)}`} title={template.name} {...stylex.props(mch.tplName)}>{template.name}</a>
                  {template.id === nextId && <span {...stylex.props(mch.nextBadge)}>{phone ? tt.nextShort : tt.next}</span>}
                </div>
                <span {...stylex.props(mch.tplRecipe)}>
                  {summaries.get(template.id) ? templateLine(summaries.get(template.id)!) : t.channels.page.tabs.templates}
                </span>
              </div>
              <div {...stylex.props(mch.tplActions)}>
                <Button variant="icon"
                  aria-label={`${tt.moveUp}: ${template.name}`}
                  disabled={index === 0}
                  onClick={() => void apply({ action: 'move', templateId: template.id, position: index - 1 })}>
                  <Icon name="arrow-up" size={15} />
                </Button>
                <ResponsiveActionMenu
                  label={tt.settings}
                  width={224}
                  alwaysVisible
                  trigger={<Icon name="more" size={16} />}
                  items={[
                    { id: 'settings', label: tt.settings, onSelect: () => { window.location.hash = `#${paths.template(template.id)}` } },
                    { id: 'down', label: tt.moveDown, disabled: index === channel.templates.length - 1, onSelect: () => void apply({ action: 'move', templateId: template.id, position: index + 1 }) },
                    { id: 'remove', label: tt.remove, danger: true, onSelect: () => void apply({ action: 'remove', templateId: template.id }, tt.removed) },
                  ]}
                />
              </div>
            </div>
          ))}
        </div>
        {error && <p role="alert" {...stylex.props(mch.error)}>{error}</p>}
        <Button size="lg" block onClick={() => setOpen(true)} icon={<Icon name="add" size={15} />}>{tt.add}</Button>
        <a href={`#${paths.library('templates')}`} {...stylex.props(channelStyles.appliedLink)}>{tt.openLibrary}</a>
        <Button variant="quiet" size="lg" block onClick={() => void apply({ action: 'standard' }, tt.standardDone)}>{tt.standard}</Button>
        <p {...stylex.props(mch.help, mch.centerText)}>{tt.standardHint}</p>

        {open && (
          <TemplatePicker
            channel={channel}
            onClose={() => setOpen(false)}
            onAdd={(templateId) => { void apply({ action: 'add', templateId }, tt.picker.added); setOpen(false) }}
            library={library.data?.items ?? []}
            loading={library.initialLoading}
          />
        )}
      </section>
    )
  }

  return (
    <section className={`vch-card vch-card--flush ${stylex.props(channelStyles.card, channelStyles.cardFlush).className}`} aria-label={t.channels.page.tabs.templates}>
      <p className={`vch-hint ${stylex.props(channelStyles.hint).className}`} style={{ padding: 'var(--vv-s-10) var(--vv-s-14) 0' }}>{tt.banner}</p>
      {channel.templates.length === 0 && (
        <p className={`vch-hint ${stylex.props(channelStyles.hint).className}`} style={{ padding: 'var(--vv-s-10) var(--vv-s-14)' }}>{tt.empty}</p>
      )}
      {channel.templates.map((template, index) => (
        <div className={`vch-tpl ${stylex.props(channelStyles.tpl).className}`} key={template.id}>
          <span className={`vch-tpl__n num ${stylex.props(channelStyles.tplN).className}`} aria-hidden="true">{index + 1}</span>
          <span className={`vch-tpl__text ${stylex.props(channelStyles.tplText).className}`}>
            <span className={`vch-tpl__name ${stylex.props(channelStyles.tplName).className}`}>
              <span {...stylex.props(channelStyles.tplNameLead)} title={template.name}>{template.name}</span>
              {template.id === nextId && <span className={`vch-refbadge ${stylex.props(channelStyles.refbadge, channelStyles.tplNameBadge).className}`}>{phone ? tt.nextShort : tt.next}</span>}
            </span>
            <span className={`vch-tpl__sub ${stylex.props(channelStyles.tplSub).className}`}>
              {summaries.get(template.id) ? templateLine(summaries.get(template.id)!) : t.channels.page.tabs.templates}
            </span>
          </span>
          <div className={`vch-tpl__actions ${stylex.props(channelStyles.tplActions).className}`}>
            <ActionButton
              size="sm"
              aria-label={`${tt.moveUp}: ${template.name}`}
              disabled={index === 0}
              onClick={() => void apply({ action: 'move', templateId: template.id, position: index - 1 })}
              icon={<Icon name="arrow-up" size={14} />}
            >
              {tt.moveUp}
            </ActionButton>
            <ActionButton
              size="sm"
              aria-label={`${tt.moveDown}: ${template.name}`}
              disabled={index === channel.templates.length - 1}
              onClick={() => void apply({ action: 'move', templateId: template.id, position: index + 1 })}
              icon={<Icon name="arrow-down" size={14} />}
            >
              {tt.moveDown}
            </ActionButton>
            <a href={`#${paths.template(template.id)}`} {...stylex.props(channelStyles.appliedLink)}>{tt.settings}</a>
            <Button variant="icon" title={tt.remove} aria-label={`${tt.remove}: ${template.name}`}
              onClick={() => void apply({ action: 'remove', templateId: template.id }, tt.removed)}>
              <Icon name="trash" size={14} />
            </Button>
          </div>
        </div>
      ))}
      {error && <p className={`vch-field__error ${stylex.props(channelStyles.fieldError).className}`} role="alert" style={{ padding: '0 var(--vv-s-14) var(--vv-s-8)' }}>{error}</p>}
      <div className={`vch-actions ${stylex.props(channelStyles.actions).className}`} style={{ padding: 'var(--vv-s-10) var(--vv-s-14) var(--vv-s-14)' }}>
        <ActionButton size="sm" onClick={() => setOpen(true)} icon={<Icon name="layer" size={14} />}>
          {tt.add}
        </ActionButton>
        <ActionButton
          size="sm"
          onClick={() => void apply({ action: 'standard' }, tt.standardDone)}
          icon={<Icon name="add" size={14} />}
        >
          {tt.standard}
        </ActionButton>
        <span className={`vch-hint ${stylex.props(channelStyles.hint).className}`}>{tt.standardHint}</span>
      </div>

      {open && (
        <TemplatePicker
          channel={channel}
          onClose={() => setOpen(false)}
          onAdd={(templateId) => { void apply({ action: 'add', templateId }, tt.picker.added); setOpen(false) }}
          library={library.data?.items ?? []}
          loading={library.initialLoading}
        />
      )}
    </section>
  )
}

function TemplatePicker({ channel, library, loading, onAdd, onClose }: {
  channel: Channel
  library: { id: string; name: string }[]
  loading: boolean
  onAdd: (templateId: string) => void
  onClose: () => void
}) {
  const tt = t.channels.templates.picker
  const used = new Set(channel.templates.map((template) => template.id))
  return (
    <Dialog
      title={tt.title}
      width={520}
      onClose={onClose}
      footer={(
        <>
          <a href={`#${paths.library('templates')}`} style={{ fontSize: 12.5, color: 'var(--color-fg-3)' }}>{tt.openLibrary}</a>
          <span style={{ flex: 1 }} />
          <ActionButton size="md" onClick={onClose}>{t.common.close}</ActionButton>
        </>
      )}
    >
      <p className={`vch-hint ${stylex.props(channelStyles.hint).className}`} style={{ marginBottom: 'var(--vv-s-8)' }}>{tt.hint}</p>
      {loading ? (
        <p className={`vch-hint ${stylex.props(channelStyles.hint).className}`}>{t.common.loading}</p>
      ) : library.length === 0 ? (
        <p className={`vch-hint ${stylex.props(channelStyles.hint).className}`}>{tt.empty}</p>
      ) : (
        <div className={`vch-pickerlist ${stylex.props(channelStyles.pickerlist).className}`}>
          {library.map((template) => {
            const inList = used.has(template.id)
            return (
              <ActionButton key={template.id} block size="lg"
                lockedReason={inList ? tt.inList : undefined}
                onClick={() => { if (!inList) onAdd(template.id) }}>
                <span className={`vch-pickerrow__text ${stylex.props(channelStyles.pickerrowText).className}`}>
                  <span className={`vch-pickerrow__name ${stylex.props(channelStyles.pickerrowName).className}`}>{template.name}</span>
                  <span className={`vch-pickerrow__sub ${stylex.props(channelStyles.pickerrowSub).className}`}>{inList ? tt.inList : t.channels.page.tabs.templates}</span>
                </span>
                {!inList && <Icon name="add" size={15} />}
              </ActionButton>
            )
          })}
        </div>
      )}
    </Dialog>
  )
}
