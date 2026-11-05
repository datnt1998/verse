import * as stylex from '@stylexjs/stylex'
import { useState } from 'react'
import { Banner, Button, Icon, Spinner, toast } from '@noverse/ui'
import { invalidate, setData } from '@noverse/ui/data'
import { ResponsiveSelect } from '../../components/responsive-overlays'
import { t } from '../../i18n/channels'
import { navigate, paths } from '../../router'
import { keys } from '../../data/keys'
import { useResource } from '../../data/useResource'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { patchChannel, type Channel } from '../../api/channels'
import { listPrompts, listTemplates, type TemplateSummary } from '../../api/library'
import { text } from '../../styles/shared.stylex'
import { templateLine } from './templateLine'
import { channelStyles } from './channel.stylex'
import { mch } from './mobile/mchannel.stylex'
import { TemplateArtwork } from '../library/TemplateArtwork'

// Cùng luật sức khoẻ với thẻ Thư viện; không suy ra URL ảnh từ metadata preview.
const missingBackdrop = (template: TemplateSummary) =>
  template.backdrop.enabled && template.backdrop.channelCount === 0 && template.backdrop.videoCount === 0

export function TemplatesTab({ channel }: { channel: Channel }) {
  const tt = t.channels.templates
  const phone = useMediaQuery('(max-width: 767px)')
  const templates = useResource(keys.templates(), ({ signal }) => listTemplates(signal))
  const prompts = useResource(keys.prompts('', ''), ({ signal }) => listPrompts({}, signal))
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const items = templates.data?.items ?? []
  const current = items.find((template) => template.id === channel.defaultTemplateId)
  const name = channel.name || t.channels.page.noName
  const broken = current ? missingBackdrop(current) : false

  const pick = async (id: string) => {
    const template = items.find((item) => item.id === id)
    if (pending || !template || id === channel.defaultTemplateId) return
    setPending(true)
    setError(null)
    try {
      const next = await patchChannel(channel.id, { defaultTemplateId: id })
      setData(keys.channel(channel.id), next)
      void invalidate(keys.channels())
      toast.ok(phone ? tt.chosenM(template.name) : tt.chosen(name, template.name))
    } catch (err) {
      setError(err instanceof Error ? err.message : tt.saveError)
    } finally { setPending(false) }
  }

  const actions = (
    <div {...stylex.props(phone ? mch.tplActionRow : channelStyles.tplActions)}>
      <ResponsiveSelect
        aria-label={tt.change}
        value={channel.defaultTemplateId}
        optionDescriptions
        options={items.map((template) => ({
          value: template.id,
          label: template.name,
          description: [template.prompts.sceneName, `${template.channels.length} kênh đang dùng`, missingBackdrop(template) && tt.healthFix].filter(Boolean).join(' · '),
          icon: <span {...stylex.props(channelStyles.tplOptionImg, channelStyles.tplThumbEmpty)} aria-hidden="true"><TemplateArtwork template={template} prompts={prompts.data?.items} /></span>,
        }))}
        onChange={(id) => void pick(id)}
        disabled={pending || templates.initialLoading || !templates.data}
        icon={<Icon name="refresh" size={14} />}
        prefix={tt.change}
        valueStyle={text.srOnly}
        triggerStyle={channelStyles.tplRowSelect}
        popWidth={360}
        footer={<Button variant="ghost" icon={<Icon name="folder" size={15} />} onClick={() => navigate(paths.library('templates'))}>{tt.openLibrary}</Button>}
      />
      <Button variant="ghost" size={phone ? 'lg' : 'sm'} xstyle={phone ? channelStyles.touchTarget : undefined}
        onClick={() => navigate(paths.template(channel.defaultTemplateId, 'images'))} icon={<Icon name="edit" size={14} />}>
        {tt.edit}
      </Button>
    </div>
  )
  const health = current && (
    <span {...stylex.props(phone ? mch.tplHealth : channelStyles.tplHealth, phone ? broken ? mch.tplHealthWarn : mch.tplHealthOk : broken ? channelStyles.tplHealthWarn : channelStyles.tplHealthOk)}>
      <span aria-hidden="true" {...stylex.props(phone ? mch.tplHealthDot : channelStyles.tplHealthDot, phone ? broken ? mch.tplHealthDotWarn : mch.tplHealthDotOk : broken ? channelStyles.tplHealthDotWarn : channelStyles.tplHealthDotOk)} />
      {broken ? tt.healthFix : tt.healthOk}
    </span>
  )
  const loadFailure = !!templates.error && !templates.initialLoading && (
    <Banner tone="error" title={tt.loadFail} action={{ label: t.common.retry, onClick: () => void templates.refresh() }} />
  )

  if (phone) return (
    <div {...stylex.props(mch.tplScreen)} aria-label={tt.label}>
      <section {...stylex.props(mch.card16, mch.tplCard)} aria-label={tt.ofChannel(name)}>
        <div {...stylex.props(mch.tplHead)}>
          <span {...stylex.props(mch.tplImg, mch.tplImgEmpty)} aria-hidden="true">{current ? <TemplateArtwork template={current} prompts={prompts.data?.items} /> : <Icon name="layer" size={24} />}</span>
          <span {...stylex.props(mch.tplText)}>
            <span {...stylex.props(mch.tplOf)}>{tt.label}</span>
            <span {...stylex.props(mch.tplName)} title={channel.defaultTemplateName}>{current?.name ?? channel.defaultTemplateName}</span>
            {health}
          </span>
        </div>
        {current && <span {...stylex.props(mch.tplRecipe)}>{templateLine(current)}</span>}
        {broken && <Banner tone="warning" title={tt.fixTitle}>{tt.fixBody}</Banner>}
        {actions}
        {error && <p role="alert" {...stylex.props(mch.error)}>{error}</p>}
        {templates.initialLoading && <span {...stylex.props(mch.tplLoading)}><Spinner size={14} /></span>}
        {loadFailure}
      </section>
      <span {...stylex.props(mch.tplNote)}>{tt.note}</span>
    </div>
  )

  return (
    <section {...stylex.props(channelStyles.tplSection)} aria-label={tt.label}>
      <div {...stylex.props(channelStyles.card, channelStyles.tplCard)}>
        <span {...stylex.props(channelStyles.tplThumb, channelStyles.tplThumbEmpty)} aria-hidden="true">{current ? <TemplateArtwork template={current} prompts={prompts.data?.items} /> : <Icon name="layer" size={24} />}</span>
        <div {...stylex.props(channelStyles.tplCardText)}>
          <span {...stylex.props(channelStyles.tplOf)}>{tt.ofChannel(name)}</span>
          <span {...stylex.props(channelStyles.tplName)} title={channel.defaultTemplateName}>{current?.name ?? channel.defaultTemplateName}</span>
          {current && <span {...stylex.props(channelStyles.tplRecipe)}>{templateLine(current)}</span>}
          {health}
        </div>
        {templates.initialLoading ? <Spinner size={14} /> : actions}
      </div>
      {broken && <Banner tone="warning" title={tt.fixTitle}>{tt.fixBody}</Banner>}
      {error && <p {...stylex.props(channelStyles.fieldError)} role="alert">{error}</p>}
      {loadFailure}
      <div {...stylex.props(channelStyles.tplNote)}><Icon name="info" size={15} xstyle={channelStyles.tplNoteIc} /><span>{tt.note}</span></div>
    </section>
  )
}
