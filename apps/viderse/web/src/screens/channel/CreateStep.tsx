// Bước Tạo kênh (artboard CreateChannel/Onboard › step=create): chọn ≥ 1 đối
// thủ đang theo dõi làm nguồn (bắt buộc), theo dõi & chọn đối thủ mới ngay tại
// đây, tên/link YouTube tuỳ chọn, ngôn ngữ đoán từ đối thủ tham chiếu, thẻ
// "Cách kênh chạy" (Mẫu chuẩn + nhịp). Nút Tạo kênh khoá (aria-disabled + lý do)
// tới khi chọn được một đối thủ. Mức tự chủ (§20) không có trong đợt này.
import { useState } from 'react'
import { Field, Icon, toast } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { ActionButton, ChannelAvatar } from '../../components'
import { t } from '../../i18n/vi'
import { navigate, paths } from '../../router'
import { keys } from '../../data/keys'
import { useResource } from '../../data/useResource'
import { addRival, listRivals, type Rival } from '../../api/rivals'
import { createChannel } from '../../api/channels'
import { listTemplates } from '../../api/library'
import { addRivalError } from '../explore/AddRivalForm'
import { fmtSubs } from '../explore/fmt'
import { channelTint } from '../../lib/channelTint'
import { PaceStepper } from './PaceStepper'

const num = new Intl.NumberFormat('vi-VN')

export function CreateStep({ firstRun, onCancel, onCreated }: {
  firstRun: boolean
  onCancel: () => void
  onCreated: (channelId: string) => void
}) {
  const tt = t.channels.create
  const rivalsRes = useResource(keys.rivals(), ({ signal }) => listRivals(signal))
  const templates = useResource(keys.templates(), ({ signal }) => listTemplates(signal))
  const rivals = rivalsRes.data?.rivals ?? []
  const [picked, setPicked] = useState<string[]>([])
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [pace, setPace] = useState(5)
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
    <div className="vch-new">
      <div className="vch-new__main">
        <div className="vch-crumb">
          <a href={`#${paths.workbench()}`} onClick={(event) => { event.preventDefault(); onCancel() }}>
            <Icon name="arrow-r" size={13} style={{ transform: 'rotate(180deg)' }} />
            {tt.breadcrumbHome}
          </a>
          <span aria-hidden="true">/</span>
          <span>{tt.title}</span>
        </div>
        <h1>{firstRun ? t.channels.onboard.title : tt.title}</h1>
        <p className="vch-intro">{firstRun ? t.channels.onboard.intro : tt.intro}</p>

        <section className="vch-card vch-card--flush" aria-label={tt.rivalsTitle}>
          <div className="vch-cardhead">
            <h2><Icon name="radar" size={16} />{tt.rivalsTitle} <span style={{ color: 'var(--vv-warn-ink)', fontWeight: 400, fontSize: 12.5 }}>{tt.required}</span></h2>
            <span className="vch-row__count num">
              {rivals.length === 0 ? tt.noFollowed : tt.pickedLine(picked.length, rivals.length)}
            </span>
          </div>
          {rivals.length === 0 && (
            <div className="vch-empty" style={{ margin: '0 var(--vv-s-14) var(--vv-s-10)' }} role="status">
              <b>{tt.noFollowed}</b>
              <span className="vch-hint">{tt.noFollowedBody}</span>
            </div>
          )}
          <div className="vch-picklist" role="group" aria-label={tt.rivalsTitle}>
            {rivals.map((rival) => {
              const checked = picked.includes(rival.id)
              const isRef = picked[0] === rival.id
              return (
                <button
                  key={rival.id}
                  type="button"
                  role="checkbox"
                  aria-checked={checked}
                  className="vch-pick"
                  onClick={() => toggle(rival.id)}
                >
                  <span className="vch-pickbox" data-on={checked || undefined} aria-hidden="true">
                    {checked && <Icon name="tick" size={12} />}
                  </span>
                  <ChannelAvatar
                    name={rival.title}
                    src={rival.avatarUrl ?? undefined}
                    color={{ bg: channelTint(rival.id) }}
                    size={32}
                    style={{ fontSize: 11, fontWeight: 700 }}
                  />
                  <span className="vch-pick__text">
                    <span className="vch-pick__name">
                      <span title={rival.title}>{rival.title}</span>
                      {isRef && <span className="vch-refbadge" title={tt.refTitle}>{tt.refBadge}</span>}
                    </span>
                    <span className="vch-pick__meta">
                      {rival.subscriberCount != null ? `${fmtSubs(rival.subscriberCount)} người theo dõi · ` : ''}
                      {num.format(rival.videoCount)} video đã quét
                    </span>
                  </span>
                </button>
              )
            })}
          </div>
          <div className="vch-addrow">
            <AddRivalPicked
              onAdded={(rival) => {
                setPicked((prev) => prev.includes(rival.id) ? prev : [...prev, rival.id])
                void invalidate(keys.rivals())
              }}
            />
            <a href={`#${paths.explore()}`}>{tt.exploreLink}</a>
          </div>
        </section>

        <div className="vch-grid2">
          <section className="vch-card" aria-label={tt.infoTitle}>
            <div className="vch-cardhead" style={{ padding: 0 }}>
              <h2><Icon name="channel" size={16} />{tt.infoTitle}</h2>
            </div>
            <div className="vch-field">
              <span className="vch-field__row">
                <label className="vch-field__label" htmlFor="vch-new-name">{tt.name} <span style={{ color: 'var(--vv-fg-note)' }}>{tt.optional}</span></label>
              </span>
              <Field
                id="vch-new-name"
                value={name}
                placeholder={tt.namePlaceholder}
                maxLength={200}
                onChange={(event) => setName(event.target.value)}
              />
            </div>
            <div className="vch-field">
              <span className="vch-field__row">
                <label className="vch-field__label" htmlFor="vch-new-url">{tt.url} <span style={{ color: 'var(--vv-fg-note)' }}>{tt.optional}</span></label>
              </span>
              <Field
                id="vch-new-url"
                type="url"
                value={url}
                placeholder={tt.urlPlaceholder}
                maxLength={2048}
                onChange={(event) => setUrl(event.target.value)}
              />
            </div>
            <div className="vch-field">
              <span className="vch-reading">
                <Icon name="global" size={15} className="vch-ic" />
                <span className="vch-row__text">{tt.language}</span>
                <b style={{ fontSize: 13, fontWeight: 500 }}>{reference ? guessLanguage(reference) : '—'}</b>
              </span>
              <p className="vch-field__hint">{tt.languageHint}</p>
            </div>
          </section>

          <section className="vch-card" aria-label={tt.runTitle}>
            <div className="vch-cardhead" style={{ padding: 0 }}>
              <h2><Icon name="layer" size={16} />{tt.runTitle}</h2>
            </div>
            <div className="vch-runrow">
              <span className="vch-runrow__text">
                <span className="vch-runrow__label">{tt.template}</span>
                <span className="vch-runrow__sub">{tt.templateSub}</span>
              </span>
              <span className="vch-runrow__sub" title={tt.templateTitle}>
                {templates.data?.items.find((template) => template.name.toLowerCase().includes('mẫu chuẩn'))?.name ?? tt.template}
              </span>
            </div>
            <div className="vch-runrow">
              <a className="vch-chip" href={`#${paths.library('templates')}`}>{tt.chooseTemplate}</a>
            </div>
            <div className="vch-runrow">
              <span className="vch-runrow__text">
                <span className="vch-runrow__label">{tt.pace}</span>
                <span className="vch-runrow__sub">{tt.paceSub}</span>
              </span>
              <PaceStepper value={pace} onSave={setPace} label={tt.pace} />
            </div>
          </section>
        </div>

        {error && <p className="vch-field__error" role="alert">{error}</p>}
      </div>

      <div className="vch-foot">
        <span className={canCreate ? 'vch-foot__note' : 'vch-foot__note vch-foot__note--blocked'} aria-live="polite">
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
      <span className="sr-only"><label htmlFor="vch-new-rival">{tt.addLabel}</label></span>
      <input
        id="vch-new-rival"
        className="vch-taginput"
        style={{ flex: '1 1 200px', width: 'auto', height: 'var(--vv-s-34)' }}
        value={value}
        placeholder={tt.addPlaceholder}
        onChange={(event) => setValue(event.target.value)}
      />
      <ActionButton size="sm" type="submit" disabled={pending || !value.trim()} pending={pending}>
        {tt.addSubmit}
      </ActionButton>
      {error && <span className="vch-field__error" role="alert" style={{ flexBasis: '100%' }}>{error}</span>}
    </form>
  )
}
