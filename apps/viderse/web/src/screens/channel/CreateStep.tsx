import * as stylex from '@stylexjs/stylex'
import { text } from '../../styles/shared.stylex'
import { channelStyles, channelRootMarker } from './channel.stylex'
// Bước Tạo kênh (artboard CreateChannel/Onboard › step=create): chọn ≥ 1 đối
// thủ đang theo dõi làm nguồn (bắt buộc), theo dõi & chọn đối thủ mới ngay tại
// đây, tên/link YouTube tuỳ chọn, ngôn ngữ đoán từ đối thủ tham chiếu, thẻ
// "Cách kênh chạy" (Mẫu chuẩn + nhịp). Nút Tạo kênh khoá (aria-disabled + lý do)
// tới khi chọn được một đối thủ. Mức tự chủ (§20) không có trong đợt này.
import { useState } from 'react'
import { Field, Icon, toast } from '@noverse/ui'
import { Segmented } from '../../components/Segmented'
import { invalidate } from '@noverse/ui/data'
import { ActionButton, ChannelAvatar, Orb } from '../../components'
import { t } from '../../i18n/vi'
import { navigate, paths } from '../../router'
import { keys } from '../../data/keys'
import { useResource } from '../../data/useResource'
import { addRival, listRivals, type Rival } from '../../api/rivals'
import { createChannel } from '../../api/channels'
import { DEFAULT_AUTONOMY, applyPreset, presetIds, type PresetId } from '../../api/autonomy'
import { listTemplates } from '../../api/library'
import { addRivalError } from '../explore/AddRivalForm'
import { fmtSubs } from '../explore/fmt'
import { channelTint } from '../../lib/channelTint'
import { PaceStepper } from './PaceStepper'
import { autonomyStyles } from '../../components/autonomy/autonomy.stylex'

const num = new Intl.NumberFormat('vi-VN')

export function CreateStep({ firstRun, onCancel, onCreated }: {
  firstRun: boolean
  onCancel: () => void
  onCreated: (channelId: string) => void
}) {
  const tt = t.channels.create
  const ta = t.autonomy
  const rivalsRes = useResource(keys.rivals(), ({ signal }) => listRivals(signal))
  const templates = useResource(keys.templates(), ({ signal }) => listTemplates(signal))
  const rivals = rivalsRes.data?.rivals ?? []
  const [picked, setPicked] = useState<string[]>([])
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [pace, setPace] = useState(5)
  const [aut, setAut] = useState<PresetId>('balanced')
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)

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
        pacePerDay: pace,
        autonomy: applyPreset(DEFAULT_AUTONOMY, aut),
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
                <button
                  key={rival.id}
                  type="button"
                  role="checkbox"
                  aria-checked={checked}
                  className={`vch-pick ${stylex.props(channelStyles.pick).className}`}
                  onClick={() => toggle(rival.id)}
                >
                  <span className={`vch-pickbox ${stylex.props(channelStyles.pickbox).className}`} data-on={checked || undefined} aria-hidden="true">
                    {checked && <Icon name="tick" size={12} />}
                  </span>
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
                </button>
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
            <div className={`vch-runrow ${stylex.props(channelStyles.runrow).className}`}>
              <span className={`vch-runrow__text ${stylex.props(channelStyles.runrowText).className}`}>
                <span className={`vch-runrow__label ${stylex.props(channelStyles.runrowLabel).className}`}>{tt.template}</span>
                <span className={`vch-runrow__sub ${stylex.props(channelStyles.runrowSub).className}`}>{tt.templateSub}</span>
              </span>
              <span className={`vch-runrow__sub ${stylex.props(channelStyles.runrowSub).className}`} title={tt.templateTitle}>
                {templates.data?.items.find((template) => template.name.toLowerCase().includes('mẫu chuẩn'))?.name ?? tt.template}
              </span>
            </div>
            <div className={`vch-runrow ${stylex.props(channelStyles.runrow).className}`}>
              <a className={`vch-chip ${stylex.props(channelStyles.chip).className}`} href={`#${paths.library('templates')}`}>{tt.chooseTemplate}</a>
            </div>
            <div className={`vch-runrow ${stylex.props(channelStyles.runrow, autonomyStyles.autRow).className}`}>
              <span {...stylex.props(autonomyStyles.orbsSm)} aria-hidden="true">
                <Orb of="qa" size={11} className={stylex.props(autonomyStyles.orbQa).className} />
                <Orb of="artist" size={11} className={stylex.props(autonomyStyles.orbArtist).className} />
              </span>
              <span className={`vch-runrow__text ${stylex.props(channelStyles.runrowText).className}`}>
                <span className={`vch-runrow__label ${stylex.props(channelStyles.runrowLabel).className}`}>{ta.title}</span>
                <span className={`vch-runrow__sub ${stylex.props(channelStyles.runrowSub).className}`}>{ta.createSub[aut]}</span>
              </span>
              <Segmented
                size="sm"
               
                buttonStyle={autonomyStyles.segSm}
                label={ta.title}
                value={aut}
                options={presetIds.map((id) => ({ id, label: ta.presets[id], style: { paddingInline: 12 } }))}
                onChange={(id) => setAut(id as PresetId)}
              />
            </div>
            <div className={`vch-runrow ${stylex.props(channelStyles.runrow, aut === 'careful' && autonomyStyles.dim).className}`}>
              <span className={`vch-runrow__text ${stylex.props(channelStyles.runrowText).className}`}>
                <span className={`vch-runrow__label ${stylex.props(channelStyles.runrowLabel).className}`}>{tt.pace}</span>
                <span className={`vch-runrow__sub ${stylex.props(channelStyles.runrowSub).className}`}>{tt.paceSub}</span>
              </span>
              <PaceStepper value={pace} onSave={setPace} label={tt.pace} />
            </div>
          </section>
        </div>

        {error && <p className={`vch-field__error ${stylex.props(channelStyles.fieldError).className}`} role="alert">{error}</p>}
      </div>

      <div className={`vch-foot ${stylex.props(channelStyles.foot).className}`}>
        <span className={`vch-foot__note${canCreate ? '' : ' vch-foot__note--blocked'} ${stylex.props(channelStyles.footNote, !canCreate && channelStyles.footNoteBlocked).className}`} aria-live="polite">
          <Icon name={canCreate ? 'info' : 'danger'} size={15} />
          {canCreate ? tt.noteReady : tt.noteBlocked}
        </span>
        <ActionButton className={stylex.props(channelStyles.touchTarget).className} size="md" onClick={onCancel}>{tt.cancel}</ActionButton>
        <ActionButton className={stylex.props(channelStyles.touchTarget).className}
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
function AddRivalPicked({ onAdded }: { onAdded: (rival: Rival) => void }) {
  const tt = t.channels.create
  const [value, setValue] = useState('')
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const submit = async () => {
    const input = value.trim()
    if (!input || pending) return
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
      style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 'var(--vv-s-8)', flex: '1 1 260px' }}
    >
      <span {...stylex.props(text.srOnly)}><label htmlFor="vch-new-rival">{tt.addLabel}</label></span>
      <input
        id="vch-new-rival"
        className={`vch-taginput ${stylex.props(channelStyles.taginput).className}`}
        style={{ flex: '1 1 200px', width: 'auto', height: 'var(--vv-s-34)' }}
        value={value}
        placeholder={tt.addPlaceholder}
        onChange={(event) => setValue(event.target.value)}
      />
      <ActionButton className={stylex.props(channelStyles.touchTarget).className} size="sm" type="submit" disabled={pending || !value.trim()} pending={pending}>
        {tt.addSubmit}
      </ActionButton>
      {error && <span className={`vch-field__error ${stylex.props(channelStyles.fieldError).className}`} role="alert" style={{ flexBasis: '100%' }}>{error}</span>}
    </form>
  )
}
