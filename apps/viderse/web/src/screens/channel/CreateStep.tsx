import * as stylex from '@stylexjs/stylex'
import { text } from '../../styles/shared.stylex'
import { channelStyles, channelRootMarker } from './channel.stylex'
// "Cách kênh chạy" đúng 3 hàng: Template mặc định (chọn từ Thư viện, để
// "Mẫu chuẩn · gợi ý" thì máy tự dựng), Tự duyệt ảnh, Video mỗi ngày 1–20 —
// đều gửi ngay trong POST tạo kênh (r44 · V38). Nút Tạo kênh khoá
// (aria-disabled + lý do) tới khi chọn được một đối thủ.
import { useEffect, useRef, useState } from 'react'
import { Button, Checkbox, Field, Icon, MobilePage, Spinner, Switch, toast } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { ActionButton, ChannelAvatar } from '../../components'
import { ResponsiveSelect } from '../../components/responsive-overlays'
import { t } from '../../i18n/channels'
import { navigate, paths } from '../../router'
import { keys } from '../../data/keys'
import { useResource } from '../../data/useResource'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { addRival, listRivals, type Rival } from '../../api/rivals'
import { createChannel } from '../../api/channels'
import { listTemplates, type TemplateSummary } from '../../api/library'
import { addRivalError } from '../explore/AddRivalForm'
import { fmtSubs } from '../explore/fmt'
import { channelTint } from '../../lib/channelTint'
import { PaceStepper } from './PaceStepper'
import { templateLine } from './templateLine'
import { mch } from './mobile/mchannel.stylex'

const num = new Intl.NumberFormat('vi-VN')

export function CreateStep({ firstRun, onCancel, onCreated }: {
  firstRun: boolean
  onCancel: () => void
  onCreated: (channelId: string) => void
}) {
  const tt = t.channels.create
  const rivalsRes = useResource(keys.rivals(), ({ signal }) => listRivals(signal))
  const tplRes = useResource(keys.templates(), ({ signal }) => listTemplates(signal))
  const rivals = rivalsRes.data?.rivals ?? []
  const [picked, setPicked] = useState<string[]>([])
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [perDay, setPerDay] = useState(5)
  const [autoApprove, setAutoApprove] = useState(false)
  /** template mặc định chọn trước; `null` = để "Mẫu chuẩn · gợi ý" */
  const [templateId, setTemplateId] = useState<string | null>(null)
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const narrow = useMediaQuery('(max-width: 767px)')
  const canCreate = picked.length > 0
  const reference = rivals.find((rival) => rival.id === picked[0])

  const toggle = (id: string) => setPicked((prev) => (
    prev.includes(id) ? prev.filter((item) => item !== id) : [...prev, id]
  ))

  const submit = async () => {
    if (!canCreate || pending) return
    setPending(true)
    setError(null)
    try {
      const result = await createChannel({
        name: name.trim() ? name.trim() : undefined,
        youtubeUrl: url.trim() ? url.trim() : undefined,
        competitorIds: picked,
        perDay,
        autoApprove,
        ...(templateId ? { defaultTemplateId: templateId } : {}),
      })
      void invalidate(keys.channels())
      void invalidate(keys.rivals())
      onCreated(result.channel.id)
    } catch (err) {
      const message = err instanceof Error ? err.message : ''
      setError(message ? tt.createError(message) : tt.noteBlocked)
      setPending(false)
    }
  }

  if (narrow) {
    return (
      <MobilePage
        title={tt.title}
        onBack={onCancel}
        footStyle={mch.foot}
        footer={(
          <>
            <span aria-live="polite" {...stylex.props(mch.createFootNote, !canCreate && mch.footNoteBlocked)}>
              <Icon name={canCreate ? 'info' : 'danger'} size={14} />
              {canCreate ? tt.noteReadyM : tt.noteBlocked}
            </span>
            <Button
              variant="primary"
              size="xl"
              block
              disabled={!canCreate}
              pending={pending}
              pendingLabel={tt.submitting}
              title={canCreate ? tt.okTitle : tt.blockedTitle}
              onClick={() => { if (canCreate) void submit() }}
            >
              {tt.submit}
            </Button>
          </>
        )}
      >
        <div {...stylex.props(mch.createBody)}>
          <p {...stylex.props(mch.createIntro)}>{tt.intro}</p>

          <section {...stylex.props(mch.section)} aria-label={tt.rivalsTitle}>
            <div {...stylex.props(mch.sectionHead)}>
              <span {...stylex.props(mch.sec)}>{tt.rivalsTitle} <span {...stylex.props(mch.required)}>{tt.required}</span></span>
              <span className={`num ${stylex.props(mch.pickedLine).className}`}>{rivals.length === 0 ? tt.noFollowed : tt.pickedLineM(picked.length, rivals.length)}</span>
            </div>
            {rivals.length === 0 && (
              <div role="status" {...stylex.props(mch.emptyBox)}>
                <div {...stylex.props(mch.emptyText)}>
                  <b {...stylex.props(mch.emptyTitle)}>{tt.noFollowed}</b>
                  <span {...stylex.props(mch.emptyBody)}>{tt.noFollowedBody}</span>
                </div>
              </div>
            )}
            <div role="group" aria-label={tt.rivalsTitle}>
              {rivals.map((rival) => {
                const checked = picked.includes(rival.id)
                const isRef = picked[0] === rival.id
                return (
                  <Checkbox key={rival.id} checked={checked} onChange={() => toggle(rival.id)} size={22}
                    xstyle={[mch.pick, mch.pickEnd]} label={<span {...stylex.props(mch.pickLabel)}>
                    <ChannelAvatar
                      name={rival.title}
                      src={rival.avatarUrl ?? undefined}
                      color={{ bg: channelTint(rival.id) }}
                      size={36}
                      fontSize={12}
                    />
                    <span {...stylex.props(mch.pickText)}>
                      <span {...stylex.props(mch.pickNameRow)}>
                        <span {...stylex.props(mch.pickName)} title={rival.title}>{rival.title}</span>
                        {isRef && <span {...stylex.props(mch.refBadge)} title={tt.refTitle}>{tt.refBadge}</span>}
                      </span>
                      <span {...stylex.props(mch.pickMeta)}>
                        {rival.subscriberCount != null ? `${fmtSubs(rival.subscriberCount)} người theo dõi · ` : ''}
                        {num.format(rival.videoCount)} video đã quét
                      </span>
                    </span>
                    </span>} />
                )
              })}
            </div>
            <div {...stylex.props(mch.addForm)}>
              <AddRivalPicked
                mobile
                onAdded={(rival) => {
                  setPicked((prev) => prev.includes(rival.id) ? prev : [...prev, rival.id])
                  void invalidate(keys.rivals())
                }}
              />
            </div>
            <p {...stylex.props(mch.refFoot)}>{tt.refFootM}</p>
          </section>

          <section {...stylex.props(mch.infoSection)} aria-label={tt.infoTitle}>
            <div {...stylex.props(mch.sectionHead)}>
              <span {...stylex.props(mch.sec)}>{tt.infoTitle} <span {...stylex.props(mch.optional)}>{tt.optional}</span></span>
            </div>
            <label {...stylex.props(mch.label)} htmlFor="vch-new-name-m">
              {tt.name}
              <Field
                id="vch-new-name-m"
                value={name}
                placeholder={tt.namePlaceholder}
                maxLength={200}
                onChange={(event) => setName(event.target.value)}
                xstyle={mch.field44}
              />
            </label>
            <label {...stylex.props(mch.label)} htmlFor="vch-new-url-m">
              {tt.url}
              <Field
                id="vch-new-url-m"
                type="url"
                value={url}
                placeholder={tt.urlPlaceholder}
                maxLength={2048}
                onChange={(event) => setUrl(event.target.value)}
                xstyle={mch.field44}
              />
            </label>
            <div {...stylex.props(mch.langBox)}>
              <Icon name="global" size={15} />
              <div {...stylex.props(mch.langText)}>
                <div {...stylex.props(mch.langRow)}>
                  <span>{tt.language}</span>
                  <b {...stylex.props(mch.langValue)}>{reference ? guessLanguage(reference) : '—'}</b>
                </div>
                <span {...stylex.props(mch.help)}>{tt.languageHint}</span>
              </div>
            </div>
          </section>

          <section {...stylex.props(mch.runCard)} aria-label={tt.runTitle}>
            <div {...stylex.props(mch.runTitle)}>{tt.runTitle}</div>
            <RunRows
              templates={tplRes.data?.items ?? []}
              loading={tplRes.initialLoading}
              templateId={templateId}
              onTemplateId={setTemplateId}
              autoApprove={autoApprove}
              onAutoApprove={setAutoApprove}
              perDay={perDay}
              onPerDay={setPerDay}
              mobile
            />
          </section>

          {error && <p role="alert" {...stylex.props(mch.error)}>{error}</p>}
        </div>
      </MobilePage>
    )
  }

  return (
    <div className={`vch-new ${stylex.props(channelStyles.new, channelRootMarker).className}`}>
      <div className={`vch-new__main ${stylex.props(channelStyles.newMain).className}`}>
        <div className={`vch-crumb ${stylex.props(channelStyles.crumb).className}`}>
          <a {...stylex.props(channelStyles.crumbLink)} href={`#${paths.workbench()}`} onClick={(event) => { event.preventDefault(); onCancel() }}>
            <Icon name="arrow-r" size={13} style={{ transform: 'rotate(180deg)' }} />
            {tt.breadcrumbHome}
          </a>
          <span aria-hidden="true">/</span>
          <span>{tt.title}</span>
        </div>
        <h1 {...stylex.props(channelStyles.newTitle)}>{firstRun ? t.channels.onboard.title : tt.title}</h1>
        <p className={`vch-intro ${stylex.props(channelStyles.intro).className}`}>{firstRun ? t.channels.onboard.intro : tt.intro}</p>

        <section className={`vch-card vch-card--flush ${stylex.props(channelStyles.card, channelStyles.cardFlush).className}`} aria-label={tt.rivalsTitle}>
          <div className={`vch-cardhead ${stylex.props(channelStyles.cardhead).className}`}>
            <h2 {...stylex.props(channelStyles.cardHeading)}><Icon name="radar" size={16} />{tt.rivalsTitle} <span style={{ color: 'var(--vv-warn-ink)', fontWeight: 400, fontSize: 12.5 }}>{tt.required}</span></h2>
            <span className={`vch-row__count num ${stylex.props(channelStyles.rowCount).className}`}>
              {rivals.length === 0 ? tt.noFollowed : tt.pickedLine(picked.length, rivals.length)}
            </span>
          </div>
          {rivals.length === 0 && (
            <div className={`vch-empty ${stylex.props(channelStyles.empty).className}`} style={{ margin: '0 var(--vv-s-14) var(--vv-s-10)' }} role="status">
              <b {...stylex.props(channelStyles.emptyTitle)}>{tt.noFollowed}</b>
              <span className={`vch-hint ${stylex.props(channelStyles.hint).className}`}>{tt.noFollowedBody}</span>
            </div>
          )}
          <div className={`vch-picklist ${stylex.props(channelStyles.picklist).className}`} role="group" aria-label={tt.rivalsTitle}>
            {rivals.map((rival) => {
              const checked = picked.includes(rival.id)
              const isRef = picked[0] === rival.id
              return (
                <Checkbox key={rival.id} checked={checked} onChange={() => toggle(rival.id)}
                  xstyle={channelStyles.pick} label={<>
                  <ChannelAvatar
                    name={rival.title}
                    src={rival.avatarUrl ?? undefined}
                    color={{ bg: channelTint(rival.id) }}
                    size={32}
                    style={{ fontSize: 11, fontWeight: 700 }}
                  />
                  <span className={`vch-pick__text ${stylex.props(channelStyles.pickText).className}`}>
                    <span className={`vch-pick__name ${stylex.props(channelStyles.pickName).className}`}>
                      <span {...stylex.props(channelStyles.pickNameText)} title={rival.title}>{rival.title}</span>
                      {isRef && <span className={`vch-refbadge ${stylex.props(channelStyles.refbadge, channelStyles.pickNameText).className}`} title={tt.refTitle}>{tt.refBadge}</span>}
                    </span>
                    <span className={`vch-pick__meta ${stylex.props(channelStyles.pickMeta).className}`}>
                      {rival.subscriberCount != null ? `${fmtSubs(rival.subscriberCount)} người theo dõi · ` : ''}
                      {num.format(rival.videoCount)} video đã quét
                    </span>
                  </span>
                  </>} />
              )
            })}
          </div>
          <div className={`vch-addrow ${stylex.props(channelStyles.addrow).className}`}>
            <AddRivalPicked
              onAdded={(rival) => {
                setPicked((prev) => prev.includes(rival.id) ? prev : [...prev, rival.id])
                void invalidate(keys.rivals())
              }}
            />
            <a {...stylex.props(channelStyles.addrowLink)} href={`#${paths.explore()}`}>{tt.exploreLink}</a>
          </div>
        </section>

        <div className={`vch-grid2 ${stylex.props(channelStyles.grid2).className}`}>
          <section className={`vch-card ${stylex.props(channelStyles.card).className}`} aria-label={tt.infoTitle}>
            <div className={`vch-cardhead ${stylex.props(channelStyles.cardhead).className}`} style={{ padding: 0 }}>
              <h2 {...stylex.props(channelStyles.cardHeading)}><Icon name="channel" size={16} />{tt.infoTitle}</h2>
            </div>
            <div className={`vch-field ${stylex.props(channelStyles.field).className}`}>
              <span className={`vch-field__row ${stylex.props(channelStyles.fieldRow).className}`}>
                <label className={`vch-field__label ${stylex.props(channelStyles.fieldLabel).className}`} htmlFor="vch-new-name">{tt.name} <span style={{ color: 'var(--vv-fg-note)' }}>{tt.optional}</span></label>
              </span>
              <Field className={stylex.props(channelStyles.touchTarget).className}
                id="vch-new-name"
                value={name}
                placeholder={tt.namePlaceholder}
                maxLength={200}
                onChange={(event) => setName(event.target.value)}
              />
            </div>
            <div className={`vch-field ${stylex.props(channelStyles.field).className}`}>
              <span className={`vch-field__row ${stylex.props(channelStyles.fieldRow).className}`}>
                <label className={`vch-field__label ${stylex.props(channelStyles.fieldLabel).className}`} htmlFor="vch-new-url">{tt.url} <span style={{ color: 'var(--vv-fg-note)' }}>{tt.optional}</span></label>
              </span>
              <Field className={stylex.props(channelStyles.touchTarget).className}
                id="vch-new-url"
                type="url"
                value={url}
                placeholder={tt.urlPlaceholder}
                maxLength={2048}
                onChange={(event) => setUrl(event.target.value)}
              />
            </div>
            <div className={`vch-field ${stylex.props(channelStyles.field).className}`}>
              <span className={`vch-reading ${stylex.props(channelStyles.reading).className}`}>
                <Icon name="global" size={15} className={`vch-ic ${stylex.props(channelStyles.ic).className}`} />
                <span className={`vch-row__text ${stylex.props(channelStyles.rowText).className}`}>{tt.language}</span>
                <b style={{ fontSize: 13, fontWeight: 500 }}>{reference ? guessLanguage(reference) : '—'}</b>
              </span>
              <p className={`vch-field__hint ${stylex.props(channelStyles.fieldHint).className}`}>{tt.languageHint}</p>
            </div>
          </section>

          <section className={`vch-card ${stylex.props(channelStyles.card).className}`} aria-label={tt.runTitle}>
            <div className={`vch-cardhead ${stylex.props(channelStyles.cardhead).className}`} style={{ padding: 0 }}>
              <h2 {...stylex.props(channelStyles.cardHeading)}><Icon name="layer" size={16} />{tt.runTitle}</h2>
            </div>
            <RunRows
              templates={tplRes.data?.items ?? []}
              loading={tplRes.initialLoading}
              templateId={templateId}
              onTemplateId={setTemplateId}
              autoApprove={autoApprove}
              onAutoApprove={setAutoApprove}
              perDay={perDay}
              onPerDay={setPerDay}
            />
          </section>
        </div>

        {error && <p className={`vch-field__error ${stylex.props(channelStyles.fieldError).className}`} role="alert">{error}</p>}
      </div>

      <div className={`vch-foot ${stylex.props(channelStyles.foot).className}`}>
        <span className={`vch-foot__note${canCreate ? '' : ' vch-foot__note--blocked'} ${stylex.props(channelStyles.footNote, !canCreate && channelStyles.footNoteBlocked).className}`} aria-live="polite">
          <Icon name={canCreate ? 'info' : 'danger'} size={15} />
          {canCreate ? tt.noteReady : tt.noteBlocked}
        </span>
        <ActionButton size="md" onClick={onCancel}>{tt.cancel}</ActionButton>
        <ActionButton
          variant="primary"
          size="md"
          lockedReason={canCreate ? undefined : tt.blockedTitle}
          title={canCreate ? tt.okTitle : tt.blockedTitle}
          pending={pending}
          pendingLabel={tt.submitting}
          onClick={() => { if (canCreate) void submit() }}
        >
          {tt.submit}
        </ActionButton>
      </div>
    </div>
  )
}

/** Ngôn ngữ đoán theo tên đối thủ khi chọn — cùng luật với backend (dấu tiếng Việt → vi). */
function guessLanguage(rival: Rival): string {
  const lower = rival.title.toLowerCase()
  const vietnamese = /[ăâđêôơưàáảãạằắẳẵặầấẩẫậèéẻẽẹềếểễệìíỉĩịòóỏõọồốổỗộờớởỡợùúủũụừứửữựỳýỷỹỵ]/i
  return vietnamese.test(lower) ? t.channels.page.language.vi : t.channels.page.language.en
}

/** Ô theo dõi đối thủ mới: theo dõi xong thêm vào danh sách chọn ngay. */
function AddRivalPicked({ onAdded, mobile = false }: { onAdded: (rival: Rival) => void; mobile?: boolean }) {
  const tt = t.channels.create
  const [value, setValue] = useState('')
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  const submit = async () => {
    const input = value.trim()
    if (pending) return
    if (!input) {
      setError('Dán @handle hoặc link kênh YouTube của đối thủ trước')
      inputRef.current?.focus()
      return
    }
    setPending(true)
    setError(null)
    try {
      const rival = await addRival(input)
      setValue('')
      onAdded(rival)
      toast.ok(rival.title)
    } catch (err) {
      setError(addRivalError(err).message || tt.rivalError)
    } finally {
      setPending(false)
    }
  }

  return (
    <form
      aria-label={tt.addLabel}
      onSubmit={(event) => { event.preventDefault(); void submit() }}
      {...stylex.props(mch.rivalForm, mch.createRivalForm)}
    >
      <span {...stylex.props(text.srOnly)}><label htmlFor="vch-new-rival">{tt.addLabel}</label></span>
      <input
        id="vch-new-rival"
        ref={inputRef}
        aria-invalid={!!error || undefined}
        aria-describedby={error ? 'vch-new-rival-error' : undefined}
        style={error ? { borderColor: 'var(--color-err)' } : undefined}
        className={`vch-taginput ${stylex.props(mobile ? [mch.input44, mch.rivalInputRow] : [channelStyles.taginput, mch.rivalInput]).className}`}
        value={value}
        placeholder={tt.addPlaceholder}
        onChange={(event) => { setValue(event.target.value); setError(null) }}
      />
      <ActionButton size={mobile ? 'lg' : 'sm'} type="submit" disabled={pending} pending={pending}>
        {mobile ? tt.addSubmitM : tt.addSubmit}
      </ActionButton>
      {error && <span id="vch-new-rival-error" className={`vch-field__error ${stylex.props(channelStyles.fieldError).className}`} role="alert" style={{ flexBasis: '100%' }}>{error}</span>}
    </form>
  )
}

/** Ba hàng "Cách kênh chạy" của bước Tạo kênh (artboard CreateChannel/MCreate
 *  › run card): Template chọn từ Thư viện (máy tính nổi dưới nút chọn, điện
 *  thoại mở Sheet; chọn "Mẫu chuẩn · gợi ý" = để máy tự dựng), công tắc Tự
 *  duyệt ảnh, stepper Video mỗi ngày 1–20. */
function RunRows({ templates, loading, templateId, onTemplateId, autoApprove, onAutoApprove, perDay, onPerDay, mobile = false }: {
  templates: TemplateSummary[]
  loading: boolean
  templateId: string | null
  onTemplateId: (id: string | null) => void
  autoApprove: boolean
  onAutoApprove: (on: boolean) => void
  perDay: number
  onPerDay: (pace: number) => void
  mobile?: boolean
}) {
  const tt = t.channels.create
  const options = [
    { value: '', label: tt.templateStandard, description: tt.templateStandardSub },
    ...templates.map((template) => ({ value: template.id, label: template.name, description: templateLine(template) })),
  ]
  const row = mobile ? mch.autoRow : channelStyles.runrow
  const textStyle = mobile ? mch.runText : channelStyles.runrowText
  const labelStyle = mobile ? mch.runLabel : channelStyles.runrowLabel
  const subStyle = mobile ? mch.runSub : channelStyles.runrowSub

  return (
    <>
      <div {...stylex.props(row)}>
        <span {...stylex.props(textStyle)}>
          <span {...stylex.props(labelStyle)}>{tt.template}</span>
          <span {...stylex.props(subStyle)}>{tt.templateSub}</span>
        </span>
        <span {...stylex.props(channelStyles.tplRowPick)}>
          <ResponsiveSelect
            aria-label={tt.chooseTpl}
            value={templateId ?? ''}
            options={options}
            onChange={(id) => onTemplateId(id || null)}
            triggerStyle={mobile ? mch.tplSelectM : channelStyles.tplRowSelect}
            popWidth={360}
            inlineDescription
          />
          {loading && <Spinner size={12} />}
        </span>
      </div>
      <div {...stylex.props(row)}>
        <span {...stylex.props(textStyle)}>
          <span {...stylex.props(labelStyle)}>{tt.approve}</span>
          <span {...stylex.props(subStyle)}>{autoApprove ? tt.approveSubOn : tt.approveSubOff}</span>
        </span>
        <Switch checked={autoApprove} onChange={onAutoApprove} aria-label={tt.approve} />
      </div>
      <div {...stylex.props(row)}>
        <span {...stylex.props(textStyle)}>
          <span {...stylex.props(labelStyle)}>{tt.perDay}</span>
          <span {...stylex.props(subStyle)}>{tt.perDaySub(perDay)}</span>
        </span>
        <PaceStepper value={perDay} onSave={onPerDay} label={tt.perDay} />
      </div>
    </>
  )
}

