import { useEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Dialog, Icon, Spinner, toast } from '@noverse/ui'
import { ApiError, controlStorageMigration, getStorage, putStorage } from '../../../api/client'
import type { StorageCandidate, StorageTest } from '../../../api/storage-settings'
import { setData } from '../../../data'
import { storageCopy as c, storageProviders } from '../../../i18n/vi'
import { bytes, migrating, num, saveAndMigrate } from './logic'
import { candidateFor, providerOf, StorageForm } from './form'
import { MigrationCard, StorageOverview, storageName } from './overview'
import { storageKey, useStorage } from './resource'
import { sx } from './styles'

export function StorageSection({ mobile = false }: { mobile?: boolean }) {
  const resource = useStorage(true)
  const view = resource.data
  const [page, setPage] = useState<'overview' | 'pick' | 'form'>('overview')
  const [draft, setDraft] = useState<StorageCandidate | null>(null)
  const [test, setTest] = useState<StorageTest | null>(null)
  const [confirm, setConfirm] = useState(false)
  const [busy, setBusy] = useState(false)
  const [failure, setFailure] = useState('')
  const [pendingVersion, setPendingVersion] = useState<number | null>(null)
  const actionLock = useRef(false)
  const conflict = () => { setTest(null); setConfirm(false); void resource.refresh() }
  useEffect(() => { if (view?.locked || mobile) { setPage('overview'); setDraft(null); setTest(null) } }, [view?.locked, mobile])
  const reportError = (error: unknown) => {
    const message = error instanceof Error ? error.message : c.actionError
    setFailure(message); toast(message)
    if (error instanceof ApiError && error.status === 409) conflict()
  }
  const confirmSave = async () => {
    if (actionLock.current || !draft || !test?.ok) return
    actionLock.current = true; setBusy(true)
    try {
      // Inventory can change after testing. Never confirm from a cached zero count.
      const latest = await getStorage()
      setData(storageKey, latest)
      if (latest.version !== draft.version || latest.locked || !latest.encryption.available) { setTest(null); throw new Error('Cấu hình kho vừa thay đổi. Vui lòng kiểm tra lại kết nối.') }
      if ((latest.total?.count ?? 0) > 0) setConfirm(true)
      else { actionLock.current = false; await save() }
    } catch (error) { reportError(error) }
    finally { actionLock.current = false; setBusy(false) }
  }
  const save = async () => {
    if (actionLock.current || !draft || !test?.ok || !view?.encryption.available || view.locked) return
    actionLock.current = true; setBusy(true); setFailure('')
    const provider = providerOf(draft.provider).name
    try {
      const result = await saveAndMigrate(draft, test.ok, async (candidate) => {
        const saved = await putStorage(candidate)
        // Saving credentials ends the form's lifetime, independently of migration startup.
        setDraft(null); setTest(null); setConfirm(false); setPage('overview')
        if (saved.needsMigration) setPendingVersion(saved.version)
        return saved
      }, async (version) => { const next = await controlStorageMigration('start', version); setData(storageKey, next); setPendingVersion(null) })
      await resource.refresh()
      toast(result.needsMigration ? `Đang chuyển sang ${provider}` : `Đã chuyển sang ${provider}`)
    } catch (error) { reportError(error); setConfirm(false); setPage('overview'); await resource.refresh() }
    finally { actionLock.current = false; setBusy(false) }
  }
  const control = async (action: 'pause' | 'resume' | 'cancel' | 'start') => {
    if (actionLock.current || !view) return
    actionLock.current = true; setBusy(true); setFailure('')
    try { const next = await controlStorageMigration(action, action === 'start' ? pendingVersion ?? view.version ?? 0 : view.version ?? 0); setData(storageKey, next); setPendingVersion(null); setPage('overview'); setConfirm(false) }
    catch (error) { reportError(error) }
    finally { actionLock.current = false; setBusy(false) }
  }
  if (!view) return <div role={resource.error ? 'alert' : 'status'} {...stylex.props(sx.root)}>{resource.error ? <><p>{c.loadError}</p><button type="button" {...stylex.props(sx.button)} onClick={() => { void resource.refresh() }}>{c.retest}</button></> : <><Spinner size={20} />{c.loading}</>}</div>
  return <div {...stylex.props(sx.root, mobile && sx.mobile)}>
    {!mobile && page === 'overview' && <div {...stylex.props(sx.stack)}><h1 {...stylex.props(sx.title)}>{c.title}</h1><p {...stylex.props(sx.sub)}>{c.intro}</p></div>}
    {(resource.error || failure) && <div role="alert" {...stylex.props(sx.inset, sx.error)}>{failure || c.loadError}</div>}
    {page === 'overview' && <>
      <MigrationCard view={view} mobile={mobile} busy={busy} onControl={(action) => { void control(action) }} />
      {!mobile && view.pending && !migrating(view.migration) && <section {...stylex.props(sx.card, sx.warning)}><b>Kho mới đã lưu, chưa bắt đầu chuyển tệp</b><span {...stylex.props(sx.small)}>App vẫn dùng {storageName(view)}. Bắt đầu chuyển sang {providerOf(view.pending.provider).name} khi sẵn sàng.</span><button type="button" {...stylex.props(sx.button)} disabled={busy} onClick={() => { void control('start') }}>{c.start}</button></section>}
      <StorageOverview view={view} mobile={mobile} onPick={() => setPage('pick')} onRefresh={resource.refresh} />
    </>}
    {!mobile && page === 'pick' && !view.locked && <>
      <button type="button" {...stylex.props(sx.button, sx.ghost)} onClick={() => setPage('overview')}><Icon name="arrow-left-2" size={15} />{c.title}</button>
      <div {...stylex.props(sx.stack)}><h1 {...stylex.props(sx.title)}>{c.pick}</h1><p {...stylex.props(sx.sub)}>{c.unchanged}</p></div>
      <div {...stylex.props(sx.grid, sx.pickGrid)}>{storageProviders.map((provider) => <button type="button" key={provider.id} {...stylex.props(sx.card, sx.pick)} onClick={() => { setDraft(candidateFor(provider.id, view)); setTest(null); setFailure(''); setPage('form') }}><span {...stylex.props(sx.logo)}>{provider.initials}</span><span {...stylex.props(sx.stack, sx.grow)}><span {...stylex.props(sx.row)}><b {...stylex.props(sx.heading)}>{provider.name}</b>{view.active?.provider === provider.id && <span {...stylex.props(sx.chip)}>{c.active}</span>}</span><span {...stylex.props(sx.note)}>{provider.hint}</span><span {...stylex.props(sx.note)}>{c.recommend}: {provider.proxy ? 'gửi qua máy chủ' : 'gửi trực tiếp'}</span></span><Icon name="arrow-right-2" size={16} /></button>)}</div>
    </>}
    {!mobile && page === 'form' && draft && !view.locked && <StorageForm key={draft.provider} view={view} draft={draft} onDraft={setDraft} result={test} onResult={setTest} onSave={() => { void confirmSave() }} onBack={() => { setPage('pick'); setTest(null) }} saving={busy} onConflict={conflict} />}
    {confirm && draft && <Dialog alertDialog title={`Chuyển ${num(view.total?.count ?? 0)} tệp · ${bytes(view.total?.bytes ?? 0)} sang ${providerOf(draft.provider).name}?`} onClose={() => { if (!busy) setConfirm(false) }} footer={<><button type="button" {...stylex.props(sx.button, sx.ghost)} disabled={busy} onClick={() => setConfirm(false)}>{c.cancel}</button><button type="button" {...stylex.props(sx.button, sx.primary)} disabled={busy} onClick={() => { void save() }}>{busy ? <Spinner size={14} /> : c.start}</button></>}><div {...stylex.props(sx.card)}><span {...stylex.props(sx.sub)}>Từ {storageName(view)} · kết nối đã kiểm tra đạt</span>{c.confirmLines.map((text, index) => <div key={text} {...stylex.props(sx.row, sx.small)}><Icon name={['play', 'shield-tick', 'box', 'pause'][index]} size={15} {...stylex.props(sx.okText)} /><span>{text}</span></div>)}{failure && <div role="alert" {...stylex.props(sx.errorText)}>{failure}</div>}{pendingVersion !== null && <button type="button" {...stylex.props(sx.button)} disabled={busy} onClick={() => { void control('start') }}>{c.start}</button>}</div></Dialog>}
  </div>
}
