import * as stylex from '@stylexjs/stylex'
import { box } from '../../../styles/shared.stylex'
import { styles as settingsStyles, stoHeadMRowMarker, stoLocActMarker, stoPickMMarker, stoModeListMarker, stoDrivesMarker, stoPickBodyMarker, stoJobsMarker, stoPickMarker, stoBrowMarker, stoConfirmMarker, driveMarker, checksMarker, checkMarker, isFailMarker, isWarnMarker, moveMMarker, moveGroupsMarker, usageBarMarker, usageLegendMarker, stoBannerFixMarker, cleanlistMarker, pillMarker } from '../settings.stylex'
// Cài đặt › Lưu trữ (V22 + MSettings store=*): thẻ thư mục dự án (ổ, đường dẫn
// mono cắt giữa, Mở trong Finder, Đổi thư mục), khối chọn tại chỗ (store=pick:
// thư mục mới + gợi ý ổ + hai lựa chọn dữ liệu + checklist + CTA), tiến độ
// chuyển theo nhóm qua SSE `activity`, banner xong/offline, biểu đồ dung lượng
// và dọn dẹp với xác nhận cam inline cho mọi thao tác xoá.
//
// Hộp chọn thư mục: trình duyệt không trả đường dẫn thật (hỏi đáp #10) nên
// “Chọn thư mục…” mở danh sách thư mục do máy chủ liệt kê; chọn xong quay về
// khối chọn tại chỗ, đúng artboard Settings · store=pick. Điện thoại chỉ đổi
// được sang ổ đang cắm (sheet, MSettings · store=pick|lowspace|cloud).
import { useEffect, useRef, useState } from 'react'
import { Banner, Button, Dialog, Icon, ProgressBar, RadioCard, Sheet, Spinner, useAsyncAction } from '@noverse/ui'
import { Segmented } from '../../../components/Segmented'
import { useResource } from '../../../data/useResource'
import { invalidate } from '@noverse/ui/data'
import { keys } from '../../../data/keys'
import { useMediaQuery } from '../../../lib/useMediaQuery'
import { useDebounced } from '../../../lib/useDebounced'
import { t } from '../../../i18n/settings'
import {
  browseStorage, cancelStorageMove, checkStoragePath, deletePreviousStorage, dismissStorageMove,
  getStorage, getStorageReclaim, moveStorage, openStorageInFinder, recheckStorage,
  reclaimStorage, retryStorageMove, useDefaultForNew,
  type ReclaimItem, type StorageCheckResult, type StorageMoveFailure, type StorageStatus, type StorageVolume,
} from '../../../api/storage'
import { getActivity, type BackgroundTask } from '../../../api/activity'
import { ActionButton } from '../../../components/ActionButton'
import { fill, fmtBytes, fmtDay, fmtEta, fmtGB, fmtTime } from '../fmt'
import { PathText } from '../../../components/PathText'
import { getServerStorage } from '../../../api/storageServer'
import { ServerStorageSection } from './ServerStorageSection'
import { CleanupCard } from './CleanupCard'

// Lỗi có `code` của Storage (volume_gone, preview_required, not_found…) nằm
// trong payload của ApiError; mã khác/không có thì trả null.
function errorCode(err: unknown): string | null {
  const payload = (err as { payload?: unknown } | null)?.payload
  return payload && typeof payload === 'object' && 'code' in payload && typeof payload.code === 'string' ? payload.code : null
}

/** Dung lượng ổ theo artboard: từ 1000 GB đổi sang TB (“1,62 TB”). */
const fmtSpace = (bytes: number | null | undefined): string => {
  if (bytes == null) return ''
  const gb = bytes / 1e9
  return gb >= 1000 ? `${(gb / 1000).toLocaleString('vi-VN', { maximumFractionDigits: 2 })} TB` : fmtBytes(bytes)
}

/** Câu chỗ trống của ổ đang giữ thư mục dự án, luôn kèm tên ổ (#79):
 *  “Samsung T7 còn trống 1,54 TB” · ổ rút: “Samsung T7 chưa cắm · chưa đọc
 *  được chỗ trống” — không thay bằng số của ổ trong máy. */
const freeLine = (volume: StorageVolume, online: boolean, mobile: boolean): string => {
  if (!online) return fill(mobile ? t.storage.usage.freeOfflineM : t.storage.usage.freeOffline, { volume: volume.name })
  return fill(mobile ? t.storage.usage.freeM : t.storage.usage.free, { volume: volume.name, free: fmtSpace(volume.freeBytes) })
}

/** Loại ổ + tình trạng ở dòng vị trí: “Ổ ngoài USB · APFS · còn 1,54 TB”,
 *  ổ rút “Ổ ngoài USB · APFS · chưa cắm”. */
const kindText = (volume: StorageVolume, online: boolean): string => {
  const kind = kindLabel(volume)
  if (!online) {
    if (volume.kind === 'internal') return fill(t.storage.project.kindUnreadable, { kind })
    return volume.fs
      ? fill(t.storage.project.kindFsOffline, { kind, fs: volume.fs })
      : fill(t.storage.project.kindOffline, { kind })
  }
  const free = fmtSpace(volume.freeBytes)
  return volume.kind === 'internal'
    ? fill(t.storage.project.kindFree, { kind, free })
    : fill(t.storage.project.kindFsFree, { kind, fs: volume.fs, free })
}

/** Chỉ phần số GB kiểu Việt Nam — câu của artboard tự thêm “GB”; fmtGB thuần
 *  sẽ in “0,2 MB GB” với giá trị nhỏ hơn 1 GB. */
const gbOnly = (bytes: number): string => (bytes / 1e9).toLocaleString('vi-VN', { maximumFractionDigits: 1 })

/** Loại ổ (“Ổ ngoài USB”): ổ ngoài kèm cổng nối khi máy chủ biết. */
const kindLabel = (v: StorageVolume): string => {
  const kind = t.storage.project.kinds[v.kind] ?? v.kind
  return v.kind === 'external' && v.bus ? `${kind} ${v.bus}` : kind
}

/** Hàng checklist của bước kiểm tra thư mục mới. */
function Checks({ checks }: { checks: StorageCheckResult['checks'] }) {
  return (
    <div className={`vv-checks ${stylex.props(checksMarker, settingsStyles.checks).className}`}>
      {checks.map((c) => (
        <span key={c.kind} className={`vv-check ${stylex.props(checkMarker, ((c.status !== 'ok') && ((c.status) === "warn")) && isWarnMarker, ((c.status !== 'ok') && ((c.status) === "fail")) && isFailMarker, settingsStyles.check).className}`}>
          <Icon name={c.status === 'ok' ? 'tick-b' : c.status === 'warn' ? 'info' : 'danger'} size={15} className="vv-ic" xstyle={[settingsStyles.ic, c.status === 'ok' ? settingsStyles.icIsOk : c.status === 'warn' ? settingsStyles.icIsWarn : settingsStyles.icIsFail]} />
          <span {...stylex.props((!(c.status === 'ok')) && settingsStyles.checkT)}>{c.text}</span>
        </span>
      ))}
    </div>
  )
}

/**
 * Ba dòng checklist của artboard: chỗ trống (hoặc “cùng ổ”), ghi được, hệ tệp.
 * Server trả thêm dòng `cloud` khi đạt và `space` khi đã có `same_volume` —
 * hai dòng đó lặp ý nên bỏ khi không phải lỗi.
 */
function artboardChecks(checks: StorageCheckResult['checks']): StorageCheckResult['checks'] {
  const same = checks.some((c) => c.kind === 'same_volume')
  return checks.filter((c) => {
    if (c.kind === 'cloud' && c.status !== 'fail') return false
    if (c.kind === 'space' && same) return false
    return true
  })
}

const BROWSE_PAGE = 100

/** Hộp chọn thư mục phía server (hỏi đáp #10): roots = nhà + ổ đang cắm.
 *  Danh sách hiện theo cửa sổ 100 dòng, "Tải thêm" mở rộng dần — thư mục
 *  nghìn mục vẫn không nhồi quá nhiều DOM. */
function FolderBrowser({ path, onPick }: { path?: string; onPick: (path: string) => void }) {
  const res = useResource(keys.storageBrowse(path ?? ''), ({ signal }) => browseStorage(path || undefined, signal))
  const [filter, setFilter] = useState('')
  const [page, setPage] = useState(0)
  const needle = useDebounced(filter).trim().toLowerCase()
  const data = res.data
  // Đổi thư mục thì về trang đầu.
  useEffect(() => { setPage(0); setFilter('') }, [path])
  if (res.status === 'loading' && !data) return <Spinner size={16} />
  const allDirs = data?.dirs ?? []
  const shownDirs = needle ? allDirs.filter((d) => d.name.toLowerCase().includes(needle)) : allDirs
  // Cửa sổ trượt ≤ BROWSE_PAGE dòng — không đếm DOM vượt 100 kể cả thư mục
  // nghìn mục; nút Trước/Sau cho phép với tới mọi thư mục.
  const pageCount = Math.max(1, Math.ceil(shownDirs.length / BROWSE_PAGE))
  const safePage = Math.min(page, pageCount - 1)
  const visible = shownDirs.slice(safePage * BROWSE_PAGE, (safePage + 1) * BROWSE_PAGE)
  return (
    <div className={`vv-sto__browse ${stylex.props(settingsStyles.stoBrowse).className}`}>
      <div className={`vv-drives ${stylex.props(settingsStyles.drives).className}`} role="group" aria-label={t.storage.picker.roots}>
        {data?.roots.map((r) => (
          <button key={r.path} type="button" className={`vv-drive ${stylex.props(driveMarker, settingsStyles.drive).className}`} aria-pressed={r.path === data.path}
            onClick={() => onPick(r.path)} title={r.path}>
            {r.name}
          </button>
        ))}
        {data?.path && data.path !== '/' ? (
          <button type="button" className={`vv-drive ${stylex.props(driveMarker, settingsStyles.drive).className}`} onClick={() => onPick(data.path.replace(/\/[^/]*$/, '') || '/')}>
            <Icon name="arrow-up" size={13} />{t.storage.picker.back}
          </button>
        ) : null}
      </div>
      <div className={`vv-fieldbox ${stylex.props(settingsStyles.fieldbox).className}`}>
        <Icon name="folder" size={16} className="vv-ic" xstyle={[settingsStyles.ic]} />
        <PathText path={data?.path ?? ''} className="vv-mono" xstyle={settingsStyles.mono} />
      </div>
      <label className={`vv-fieldbox ${stylex.props(settingsStyles.fieldbox).className}`}>
        <Icon name="search" size={14} className="vv-ic" xstyle={[settingsStyles.ic]} />
        <input
          type="search" value={filter} aria-label={t.storage.picker.filter}
          placeholder={t.storage.picker.filterPlaceholder}
          onChange={(e) => { setFilter(e.target.value); setPage(0) }}
          className={`vv-sto__search ${stylex.props(settingsStyles.stoSearch).className}`}
        />
      </label>
      <div className={`vv-cleanlist ${stylex.props(cleanlistMarker, settingsStyles.cleanlist).className}`} role="list" style={{ maxHeight: 'var(--vv-s-80)', overflowY: 'auto' }}>
        {shownDirs.length === 0 ? (
          <span {...stylex.props(settingsStyles.VvCleanlistSpan)}><span className={`vv-lb ${stylex.props(settingsStyles.lb).className}`}>{needle ? t.storage.picker.noMatch : t.storage.picker.empty}</span></span>
        ) : visible.map((d) => (
          <button key={d.path} type="button" className={`vv-sto__brow ${stylex.props(stoBrowMarker, settingsStyles.stoBrow).className}`} onClick={() => onPick(d.path)}>
            <Icon name="folder" size={15} className="vv-ic" xstyle={[settingsStyles.ic]} />
            <span className={`vv-sto__browName ${stylex.props(settingsStyles.stoBrowName).className}`}>{d.name}</span>
            <Icon name="arrow-r" size={13} className="vv-ic" xstyle={[settingsStyles.ic]} />
          </button>
        ))}
      </div>
      {pageCount > 1 ? (
        <div className={`vv-sto__pager ${stylex.props(settingsStyles.stoPager).className}`}>
          <ActionButton variant="quiet" size="sm" disabled={safePage <= 0} onClick={() => setPage((p) => p - 1)}>
            {t.storage.picker.prevPage}
          </ActionButton>
          <span className={`vv-num vv-sto__page ${stylex.props(settingsStyles.stoPage, settingsStyles.num).className}`}>
            {fill(t.storage.picker.pageOf, { page: safePage + 1, pages: pageCount })}
          </span>
          <ActionButton variant="quiet" size="sm" disabled={safePage >= pageCount - 1} onClick={() => setPage((p) => p + 1)}>
            {t.storage.picker.nextPage}
          </ActionButton>
        </div>
      ) : null}
      {res.status === 'error' ? <span {...stylex.props(box.errline)}>{(res.error as Error | undefined)?.message}</span> : null}
      <div className={`vv-sto__end ${stylex.props(settingsStyles.stoEnd).className}`}>
        <ActionButton variant="outline" size="sm" disabled={!data?.path} onClick={() => data && onPick(`use:${data.path}`)}>
          {t.storage.picker.useThis}
        </ActionButton>
      </div>
    </div>
  )
}

/** Lý do khoá nút đi tiếp — câu đầu tiên không đạt, hoặc gợi ý khi chỉ cảnh báo. */
function failReason(check: StorageCheckResult | null): string {
  if (!check || check.ok) return ''
  return check.checks.find((c) => c.status === 'fail')?.text ?? check.checks.find((c) => c.status === 'warn')?.text ?? ''
}

/** Câu khôi phục của artboard thay cho lỗi thô: chọn ổ khác / ngoài đám mây. */
function recoveryHint(check: StorageCheckResult | null, mode: 'move' | 'new_only', cloudName?: string): string {
  if (!check || check.ok) return ''
  const fail = check.checks.find((c) => c.status === 'fail')?.kind
  if (fail === 'cloud' && cloudName) return fill(t.storage.picker.cloudHint, { name: cloudName })
  return mode === 'move' ? t.storage.picker.lowspaceHint : failReason(check)
}

type PickProps = {
  data: StorageStatus
  chosen: string
  onChosen: (path: string) => void
  onBrowse: () => void
  onCancel: () => void
  onMoved: () => void
  mobile?: boolean
  jobs: number
}

/** Khối chọn thư mục tại chỗ (Settings · store=pick, MSettings sheet) — checklist
 *  khoá-kèm-lý-do + hai lựa chọn dữ liệu + CTA. */
function PickPanel({ data, chosen, onChosen, onBrowse, onCancel, onMoved, mobile, jobs }: PickProps) {
  const [mode, setMode] = useState<'move' | 'new_only'>('move')
  const [check, setCheck] = useState<StorageCheckResult | null>(null)
  const [checkError, setCheckError] = useState<string | null>(null)
  const [checking, setChecking] = useState(false)
  // Mỗi lần đổi ổ/chế độ là một yêu cầu mới: yêu cầu cũ tới trễ bị loại (không
  // lật lại checklist/CTA của ổ trước) và không bị nhịp busy của action nuốt —
  // useAsyncAction thả lặng yêu cầu thứ hai khi đang bận.
  const checkSeq = useRef(0)
  useEffect(() => {
    const id = ++checkSeq.current
    setCheck(null)
    setCheckError(null)
    setChecking(true)
    checkStoragePath(chosen, mode)
      .then((res) => { if (id === checkSeq.current) setCheck(res) })
      .catch((err) => {
        if (id !== checkSeq.current) return
        // 404 volume_gone: /Volumes/<x> chưa cắm — hiện lời “cắm lại” thay cho
        // bảng kiểm tra (bảng sẽ nói sai là “đủ chỗ”); lỗi khác CTA vẫn khoá.
        if (errorCode(err) === 'volume_gone') setCheckError(t.storage.picker.volumeGone)
      })
      .finally(() => { if (id === checkSeq.current) setChecking(false) })
  }, [chosen, mode])
  const go = useAsyncAction(async () => {
    const res = await moveStorage(chosen, mode)
    // done=true: new_only hoặc đã ở đích — không có task nền, không có SSE.
    if (res) { if (res.done) invalidate(keys.storage()); onMoved() }
  })

  const reason = failReason(check)
  const size = fmtGB(data.projects.usedBytes)
  const eta = check?.etaSec ? fmtEta(check.etaSec) : ''
  const modes = [
    {
      id: 'move',
      label: fill(t.storage.picker.modes.move.label, { size }),
      hint: eta ? fill(t.storage.picker.modes.move.hint, { eta }) : t.storage.picker.modes.move.hintNoEta,
    },
    {
      id: 'new_only',
      label: t.storage.picker.modes.new_only.label,
      hint: fill(t.storage.picker.modes.new_only.hint, { size, volume: data.projects.volume.name }),
    },
  ]
  const submit = mode === 'move' ? t.storage.picker.submitMove : t.storage.picker.submitNew
  const submitLabel = submit.long
  const cloudName = data.suggestions.find((s) => s.path === chosen)?.volume.name
  const chosenCloud = data.suggestions.some((s) => s.volume.kind === 'cloud_sync' && s.path === chosen)
  const hint = recoveryHint(check, mode, cloudName)

  if (mobile) {
    return (
      <div className={`vv-sto__pick vv-sto__pick--m ${stylex.props(stoPickMarker, stoPickMMarker, settingsStyles.stoPick, settingsStyles.stoPickM).className}`}>
        <div className={`vv-sto__pickBody ${stylex.props(stoPickBodyMarker, settingsStyles.stoPickBody).className}`}>
          <div role="radiogroup" aria-label={t.storage.picker.roots} className={`vv-sto__drives ${stylex.props(stoDrivesMarker, settingsStyles.stoDrives).className}`}>
            {data.suggestions.map((s) => (
              <RadioCard key={s.path} checked={chosen === s.path} onChange={() => onChosen(s.path)} size="md" dotSize={22}
                xstyle={settingsStyles.storageDriveCard} labelStyle={settingsStyles.storageDriveLabel}
                descriptionStyle={settingsStyles.storageDriveDescription} dotStyle={settingsStyles.storageDriveDot}
                label={s.volume.name}
                description={s.volume.kind === 'cloud_sync'
                  ? t.storage.picker.cloudRow
                  : fill(t.storage.picker.drive, { kind: kindLabel(s.volume), fs: s.volume.fs, free: fmtSpace(s.volume.freeBytes) })}
                data-path={s.path} />
            ))}
          </div>
          <span className={`vv-help ${stylex.props(settingsStyles.help).className}`}>{t.storage.picker.mobileAnyway}</span>
          <Segmented label={t.storage.picker.modesLabel} value={mode} onChange={(v) => setMode(v as 'move' | 'new_only')} size={40} radius={10}
            options={[
              { id: 'move', label: fill(t.storage.picker.segMove, { size }) },
              { id: 'new_only', label: t.storage.picker.segNew },
            ]} />
          
          {check ? <Checks checks={artboardChecks(check.checks)} /> : checking ? <Spinner size={14} /> : null}
          {checkError ? <span className={`vv-err ${stylex.props(settingsStyles.err).className}`}>{checkError}</span> : null}
        </div>
        <div className={`vv-sto__go vv-sto__go--m ${stylex.props(settingsStyles.stoGoM, settingsStyles.stoGo).className}`}>
          {hint ? <span className={`vv-err ${stylex.props(settingsStyles.err).className}`}>{hint}</span> : null}
          <Button variant="primary" size="xl" className="vv-sto__goBtn" xstyle={[settingsStyles.stoGoBtn]} pending={go.pending} disabled={!check?.ok} onClick={() => void go.run()}>{submitLabel}</Button>
        </div>
      </div>
    )
  }

  return (
    <div className={`vv-sto__pick ${stylex.props(stoPickMarker, settingsStyles.stoPick).className}`}>
      <div className={`vv-sto__newdir ${stylex.props(settingsStyles.stoNewdir).className}`}>
        <span className={`vv-label ${stylex.props(settingsStyles.label).className}`} id="lbl-newdir">{t.storage.picker.newDirLabel}</span>
        <div className={`vv-fieldbox ${stylex.props(settingsStyles.fieldbox).className}`} role="group" aria-labelledby="lbl-newdir">
          <Icon name="folder" size={16} className="vv-ic" xstyle={[settingsStyles.ic]} />
          <PathText path={chosen} className={`vv-mono vv-sto__pickPath ${stylex.props(settingsStyles.stoPickPath, settingsStyles.mono, (chosenCloud) && settingsStyles.stoPickPathIsBad).className}`} />
          <ActionButton variant="outline" size="sm" onClick={onBrowse}>{t.storage.picker.browse}</ActionButton>
        </div>
        <span className={`vv-help ${stylex.props(settingsStyles.help).className}`}>{t.storage.picker.browseHelp}</span>
        <div className={`vv-sto__pills ${stylex.props(settingsStyles.stoPills).className}`}>
          <span className={`vv-help ${stylex.props(settingsStyles.help).className}`}>{t.storage.picker.drivesLabel}</span>
          {data.suggestions.map((s) => (
            <button key={s.path} type="button" className={`vv-pill ${stylex.props(pillMarker, settingsStyles.pill).className}`} aria-pressed={chosen === s.path}
              title={s.volume.kind === 'cloud_sync' ? undefined : `${s.volume.fs} · còn ${fmtSpace(s.volume.freeBytes)}`}
              onClick={() => onChosen(s.path)}>
              {s.volume.name}{s.volume.kind === 'cloud_sync' ? null : <span className={`vv-num vv-sto__pillFree ${stylex.props(settingsStyles.stoPillFree, settingsStyles.num).className}`}>còn {fmtSpace(s.volume.freeBytes)}</span>}
            </button>
          ))}
        </div>
        {chosenCloud ? <span className={`vv-err ${stylex.props(settingsStyles.err).className}`}>{t.storage.picker.cloudWarn}</span> : null}
      </div>
      <div className={`vv-sto__modes ${stylex.props(settingsStyles.stoModes).className}`}>
        <span className={`vv-label ${stylex.props(settingsStyles.label).className}`}>{t.storage.picker.modesLabel}</span>
        <div role="radiogroup" aria-label={t.storage.picker.modesLabel} className={`vv-sto__modeList ${stylex.props(stoModeListMarker, settingsStyles.stoModeList).className}`}>
          {modes.map((m) => (
            <RadioCard key={m.id} checked={mode === m.id} onChange={() => setMode(m.id as 'move' | 'new_only')}
              xstyle={settingsStyles.storageModeCard}
              label={m.label} description={m.hint} />
          ))}
        </div>
      </div>
      {check ? <Checks checks={artboardChecks(check.checks)} /> : checking ? <Spinner size={14} /> : null}
      {checkError ? <span className={`vv-err ${stylex.props(settingsStyles.err).className}`}>{checkError}</span> : null}
      
      <div className={`vv-sto__jobs ${stylex.props(stoJobsMarker, settingsStyles.stoJobs).className}`}>
        <Icon name="info" size={15} className="vv-ic" xstyle={[settingsStyles.ic]} />
        <span>{jobs > 0 ? fill(t.storage.picker.jobsNote, { n: jobs }) : t.storage.picker.jobsNoteZero}</span>
      </div>
      <div className={`vv-sto__go ${stylex.props(settingsStyles.stoGo).className}`}>
        <span className={`vv-err vv-sto__reason ${stylex.props(settingsStyles.stoReason, settingsStyles.err, (!(hint)) && settingsStyles.stoReasonIsEmpty).className}`}>{hint}</span>
        <ActionButton variant="outline" onClick={onCancel}>{t.storage.picker.cancel}</ActionButton>
        <ActionButton variant="primary" pending={go.pending}
          disabled={!check?.ok} disabledReason={hint || t.common.loading}
          onClick={() => void go.run()}>
          {submitLabel}
        </ActionButton>
      </div>
    </div>
  )
}

/** Thẻ tiến độ chuyển thư mục (store=moving) — từ task storage_move của /api/activity. */
function MoveProgress({ task, jobs, volume }: { task: BackgroundTask; jobs: number; volume: string }) {
  const [confirmCancel, setConfirmCancel] = useState(false)
  const cancel = useAsyncAction(async () => { await cancelStorageMove(task.id); invalidate(keys.storage()) })
  const groups = task.progress?.groups ?? []
  const pct = task.progress && task.progress.total > 0 ? Math.round((task.progress.done / task.progress.total) * 100) : 0
  return (
    // role=status + aria-live: phần trăm/nhóm đổi liên tục khi chuyển.
    <div role="status" aria-live="polite" className={`vv-move ${stylex.props(settingsStyles.move).className}`}>
      <div className={`vv-move__head ${stylex.props(settingsStyles.moveHead).className}`}>
        <b className={`vv-move__title ${stylex.props(settingsStyles.moveTitle).className}`}>{task.title || fill(t.storage.moving.title, { volume })}</b>
        <span className={`vv-num vv-move__stats ${stylex.props(settingsStyles.num, settingsStyles.moveStats).className}`}>
          {fill(t.storage.moving.stats, { done: gbOnly(task.progress?.done ?? 0), total: gbOnly(task.progress?.total ?? 0), eta: fmtEta(task.etaSec) })}
        </span>
      </div>
      <ProgressBar value={pct} tone="info" height={6} />
      {groups.length > 0 ? (
        <div className={`vv-move__groups ${stylex.props(moveGroupsMarker, settingsStyles.moveGroups).className}`}>
          {groups.map((g) => {
            const done = g.done >= g.total
            const running = !done && g.done > 0
            return (
              <span key={g.group} {...stylex.props(settingsStyles.VvMoveGroupsSpan)}>
                {done ? <Icon name="tick-b" size={14} className="vv-ic is-ok" xstyle={[settingsStyles.ic, settingsStyles.icIsOk]} />
                  : running ? <Spinner size={12} />
                    : <Icon name="clock" size={14} className="vv-ic is-wait" xstyle={[settingsStyles.ic, settingsStyles.icIsWait]} />}
                <span className={`vv-move__group ${stylex.props(settingsStyles.moveGroup).className}`}>{t.storage.moving.groups[g.group] ?? g.group}</span>
                <span className={`vv-num vv-move__groupN ${stylex.props(settingsStyles.num, settingsStyles.moveGroupN, (done) && settingsStyles.moveGroupNIsDone, ((!(done)) && (running)) && settingsStyles.moveGroupNIsRun).className}`}>
                  {done || running ? `${fmtBytes(g.done)} / ${fmtBytes(g.total)}` : t.storage.moving.waiting}
                </span>
              </span>
            )
          })}
        </div>
      ) : null}
      <span className={`vv-help ${stylex.props(settingsStyles.help).className}`}>{jobs > 0 ? fill(t.storage.moving.note, { n: jobs }) : t.storage.moving.noteNoJobs}</span>
      {confirmCancel ? (
        <div role="alertdialog" className={`vv-sto__confirm ${stylex.props(stoConfirmMarker, settingsStyles.stoConfirm).className}`}>
          <Icon name="warning" size={16} className="vv-ic" xstyle={[settingsStyles.ic]} />
          <span className={`vv-sto__confirmQ ${stylex.props(settingsStyles.stoConfirmQ).className}`}>{t.storage.moving.cancelConfirm}</span>
          <Button variant="quiet" size="compact" onClick={() => setConfirmCancel(false)}>{t.common.cancel}</Button>
          <Button variant="danger" size="compact" pending={cancel.busy} data-confirm="1" onClick={() => void cancel.run()}>{t.storage.moving.cancelShort}</Button>
        </div>
      ) : (
        <div className={`vv-move__foot ${stylex.props(settingsStyles.moveFoot).className}`}>
          <ActionButton variant="quiet" size="sm" onClick={() => setConfirmCancel(true)}>
            {fill(t.storage.moving.cancel, { volume })}
          </ActionButton>
          <span className={`vv-sto__spacer ${stylex.props(settingsStyles.stoSpacer).className}`} />
        </div>
      )}
    </div>
  )
}

/** Tên ổ của một thư mục cũ: khớp gợi ý / vị trí hiện tại, không khớp thì
 *  /Volumes/<tên> (ổ gắn ngoài) hoặc ổ trong máy — câu “Bản cũ ở Macintosh HD…”. */
function volumeNameFor(data: StorageStatus, path: string): string {
  const hit = [
    ...data.suggestions.map((s) => ({ path: s.path, name: s.volume.name })),
    { path: data.projects.path, name: data.projects.volume.name },
  ].find((v) => path === v.path || path.startsWith(v.path === '/' ? '/' : `${v.path.replace(/\/$/, '')}/`))
  return hit?.name
    ?? (/^\/Volumes\/[^/]+/.exec(path)?.[0].split('/')[2]
      ?? data.suggestions.find((s) => s.volume.kind === 'internal')?.volume.name
      ?? data.projects.volume.name)
}

/** Banner xanh “Đã chuyển…” (store=done) với Xoá bản cũ ngay + Chuyển về. */
function DoneBanner({ data, mobile }: { data: StorageStatus; mobile?: boolean }) {
  const prev = data.projects.previous!
  const [confirm, setConfirm] = useState<null | 'delete' | 'back'>(null)
  const act = useAsyncAction(async (kind: 'delete' | 'back') => {
    if (kind === 'delete') { await deletePreviousStorage(); invalidate(keys.storage()) }
    else { await moveStorage(prev.path, 'move'); invalidate(keys.storage()) }
  })
  const volName = data.projects.volume.name
  // Tên ổ của bản cũ để câu “Bản cũ ở Macintosh HD…” đọc được, không phải path:
  // khớp gợi ý/ổ hiện tại trước, không khớp thì lấy tên gắn ổ (/Volumes/<tên>).
  const prevVolume = volumeNameFor(data, prev.path)
  // new_only: bản cũ vẫn là dữ liệu đang đọc — server trả 409 not_deletable,
  // nên không mời xoá (và không mời “chuyển về” vì dữ liệu vốn ở đó).
  const deletable = prev.mode !== 'new_only'
  const title = fill(t.storage.done.title, { size: fmtGB(prev.bytes), volume: volName })
  const sub = prev.deleteAfter
    ? fill(t.storage.done.sub, { path: prevVolume, date: fmtDay(prev.deleteAfter) })
    : fill(t.storage.done.subNoDate, { path: prevVolume })
  // Điện thoại: một câu “<b>Đã chuyển N GB</b>. <phụ>” (MSettings · store=done).
  const mLine = fill(t.storage.done.mTitle, { size: fmtGB(prev.bytes) })
  const mSub = fill(t.storage.done.mSub, { path: prevVolume, date: fmtDay(prev.deleteAfter) })
  return (
    <>
      <Banner tone="success" title={mobile ? '' : title} icon="tick-b"
        iconStyle={settingsStyles.storageBannerIcon} titleStyle={settingsStyles.storageBannerTitle} textStyle={settingsStyles.storageBannerText}
        actions={deletable ? [
          { label: mobile ? t.storage.done.deleteNowM : fill(t.storage.done.deleteNow, { size: fmtGB(prev.bytes) }), variant: 'dsoft', icon: <Icon name="trash" size={14} />, size: mobile ? ('lg' as const) : undefined, onClick: () => setConfirm('delete') },
          ...(mobile ? [] : [{ label: fill(t.storage.done.back, { volume: prevVolume }), variant: 'quiet' as const, onClick: () => setConfirm('back') }]),
        ] : []}>
        {mobile ? <span><b>{mLine}.</b> {mSub}</span> : sub}
      </Banner>
      {confirm ? (
        <div role="alertdialog" className={`vv-sto__confirm ${stylex.props(stoConfirmMarker, settingsStyles.stoConfirm).className}`}>
          <Icon name="warning" size={16} className="vv-ic" xstyle={[settingsStyles.ic]} />
          <span className={`vv-sto__confirmQ ${stylex.props(settingsStyles.stoConfirmQ).className}`}>
            {confirm === 'delete'
              ? fill(t.storage.done.deleteConfirm, { bytes: gbOnly(prev.bytes), path: prev.path })
              : fill(t.storage.done.backConfirm, { path: prev.path })}
          </span>
          <Button variant="quiet" size="compact" onClick={() => setConfirm(null)}>{t.common.cancel}</Button>
          <Button variant="danger" size="compact" pending={act.busy} data-confirm="1"
            onClick={() => void act.run(confirm).then(() => setConfirm(null))}>
            {confirm === 'delete' ? t.storage.done.deleteAction : t.storage.done.backAction}
          </Button>
        </div>
      ) : null}
    </>
  )
}

/** Banner “Video mới lưu ở …” (store=donenew — chọn chỉ lưu video mới). */
const DoneNewBanner = ({ data }: { data: StorageStatus }) => {
  // new_only: không có `previous` (thư mục cũ vẫn là nơi đọc dữ liệu cũ) và
  // không xoá gì — chỉ báo video mới đã đổi chỗ.
  const prev = data.projects.previous?.mode === 'new_only' ? data.projects.previous : undefined
  return (
    <Banner tone="success" title="" icon="tick-b" actions={[]} iconStyle={settingsStyles.storageBannerIcon} titleStyle={settingsStyles.storageBannerTitle} textStyle={settingsStyles.storageBannerText}>
      {fill(prev ? t.storage.done.doneNew : t.storage.done.doneNewNoPrev, {
        volume: data.projects.volume.name,
        bytes: prev ? gbOnly(prev.bytes) : '',
        old: prev ? volumeNameFor(data, prev.path) : '',
      })}
    </Banner>
  )
}


/** Thẻ xoá một nhóm lớn (video gốc / MP4 đã đăng) với xác nhận cam inline. */
function ReclaimCard({ item }: { item: ReclaimItem }) {
  const [ask, setAsk] = useState(false)
  const [gone, setGone] = useState<number | null>(null)
  const run = useAsyncAction(async () => {
    await reclaimStorage(item.id)
    setGone(item.bytes)
    setAsk(false)
    invalidate(keys.storage())
    invalidate(['storage-reclaim'])
  })
  // Pipeline chưa có video nào thì server trả count/bytes = 0 — vẫn hiện thẻ
  // theo design, chỉ khoá nút và nói rõ lý do.
  const empty = item.count <= 0
  if (gone !== null) {
    return (
      <span role="status" className={`vv-sto__gone ${stylex.props(settingsStyles.stoGone).className}`}>
        <Icon name="tick-b" size={16} />{fill(t.storage.reclaim.gone, { bytes: fmtBytes(gone) })}
      </span>
    )
  }
  return (
    <section className={`vv-sto__card ${stylex.props(settingsStyles.card, settingsStyles.stoCard).className}`}>
      <div className={`vv-sto__cardHead ${stylex.props(settingsStyles.stoCardHead).className}`}>
        <div className={`vv-sto__cardText ${stylex.props(settingsStyles.stoCardText).className}`}>
          <h2 className={`vv-sto__h2 ${stylex.props(settingsStyles.stoH2).className}`}>
            {item.title} <span className={`vv-num vv-sto__h2Size ${stylex.props(settingsStyles.stoH2Size, settingsStyles.num).className}`}>· {fmtBytes(item.bytes)}</span>
          </h2>
          <span className={`vv-sto__sub ${stylex.props(settingsStyles.stoSub).className}`}>{empty ? t.storage.reclaim.nothing : item.desc}</span>
        </div>
        {!ask ? (
          <Button variant="warnGhost" size="compact" disabled={empty} title={empty ? t.storage.reclaim.nothing : undefined}
            icon={<Icon name="trash" size={14} />} onClick={() => setAsk(true)}>
            {t.storage.reclaim.actions[item.id] ?? item.action ?? ''}
          </Button>
        ) : null}
      </div>
      {ask ? (
        <div role="alertdialog" className={`vv-sto__confirm ${stylex.props(stoConfirmMarker, settingsStyles.stoConfirm).className}`}>
          <Icon name="warning" size={16} className="vv-ic" xstyle={[settingsStyles.ic]} />
          <span className={`vv-sto__confirmQ ${stylex.props(settingsStyles.stoConfirmQ).className}`}>
            {fill(t.storage.reclaim.confirms[item.id] ?? t.storage.reclaim.confirmAction, { bytes: fmtBytes(item.bytes), n: item.count })}
          </span>
          <Button variant="quiet" size="compact" onClick={() => setAsk(false)}>{t.common.cancel}</Button>
          <Button variant="danger" size="compact" pending={run.busy} data-confirm="1" onClick={() => void run.run()}>
            {fill(t.storage.reclaim.confirmAction, { bytes: fmtBytes(item.bytes) })}
          </Button>
        </div>
      ) : null}
    </section>
  )
}

/** Thẻ lần chuyển dở/hỏng (server báo qua /api/storage move.failed). */
function FailedMoveCard({ failed }: { failed: StorageMoveFailure }) {
  const retry = useAsyncAction(async () => { await retryStorageMove(); invalidate() })
  const dismiss = useAsyncAction(async () => { await dismissStorageMove(); invalidate(keys.storage()) })
  return (
    <section className={`vv-sto__card vv-sto__card--warn ${stylex.props(settingsStyles.card, settingsStyles.stoCard, settingsStyles.stoCardWarn).className}`}>
      <div className={`vv-sto__cardText ${stylex.props(settingsStyles.stoCardText).className}`}>
        <h2 className={`vv-sto__h2 ${stylex.props(settingsStyles.stoH2).className}`}>{t.storage.moveFailed.title}</h2>
        <span className={`vv-sto__sub ${stylex.props(settingsStyles.stoSub).className}`}>{failed.phase}</span>
        <PathText path={failed.target} className={`vv-num vv-sto__failPath ${stylex.props(settingsStyles.num, settingsStyles.stoFailPath).className}`} />
        <div className={`vv-sto__failActs ${stylex.props(settingsStyles.stoFailActs).className}`}>
          <ActionButton variant="outline" size="sm" pending={retry.pending} onClick={() => void retry.run()}>{t.storage.moveFailed.retry}</ActionButton>
          <ActionButton variant="quiet" size="sm" pending={dismiss.pending} onClick={() => void dismiss.run()}>{t.storage.moveFailed.dismiss}</ActionButton>
        </div>
      </div>
    </section>
  )
}

/** Đầu thẻ “Thư mục lưu dự án”: tiêu đề + dung lượng assets bên phải. */
function ProjectHead({ data, mobile }: { data: StorageStatus; mobile?: boolean }) {
  const bytes = fmtBytes(data.projects.usedBytes)
  return (
    <div {...stylex.props((!(mobile)) && settingsStyles.stoHead, (mobile) && settingsStyles.stoHeadM)}>
      <h2 className={`vv-sto__h2--project ${stylex.props((mobile) && settingsStyles.stoH2M, (!(mobile)) && settingsStyles.stoH2, settingsStyles.stoH2Project).className}`}>{t.storage.project.title}</h2>
      <span className={`vv-help ${stylex.props((!(mobile)) && settingsStyles.stoHeadNote, settingsStyles.help).className}`}>
        {fill(mobile ? t.storage.project.assetsM : t.storage.project.assets, { bytes })}
      </span>
    </div>
  )
}

/** Khối vị trí hiện tại (store=here|moving|done|donenew|offline). */
function LocationRow({ data, onPick, finderPending, onFinder, mobile, canChange }: {
  data: StorageStatus
  onPick: () => void
  finderPending: boolean
  onFinder: () => void
  mobile?: boolean
  canChange: boolean
}) {
  const loc = data.projects
  const kind = kindLabel(loc.volume)
  const kindLine = kindText(loc.volume, loc.online)
  if (mobile) {
    return (
      <div className={`vv-sto__locM ${stylex.props(settingsStyles.stoLocM).className}`}>
        <span className={`vv-sto__locIconM ${stylex.props(settingsStyles.stoLocIconM).className}`} aria-hidden="true"><Icon name="folder" size={20} /></span>
        <span className={`vv-sto__locTextM ${stylex.props(settingsStyles.stoLocTextM).className}`}>
          <span className={`vv-sto__locNameM ${stylex.props(settingsStyles.stoLocNameM).className}`}>{loc.volume.name}</span>
          <span className={`vv-sto__locKindM ${stylex.props(settingsStyles.stoLocKindM).className}`}>{kindLine}</span>
          <PathText path={loc.displayPath ?? loc.path} className={`vv-mono vv-sto__locPathM ${stylex.props(settingsStyles.stoLocPathM, settingsStyles.mono).className}`} />
        </span>
        {canChange ? (
          <span className={`vv-sto__locAct ${stylex.props(stoLocActMarker, settingsStyles.stoLocAct).className}`}>
            <ActionButton variant="outline" size="sm" icon={<Icon name="folder" size={16} />} onClick={onPick} xstyle={[settingsStyles.VvStoLocAct_]}>{t.storage.project.changeMobile}</ActionButton>
          </span>
        ) : null}
      </div>
    )
  }
  return (
    <div className={`vv-loc ${stylex.props(settingsStyles.loc).className}`}>
      <span className={`vv-loc__icon ${stylex.props(settingsStyles.locIcon).className}`} aria-hidden="true"><Icon name="folder" size={18} /></span>
      <span className={`vv-sto__grow ${stylex.props(settingsStyles.stoGrow).className}`}>
        <span className={`vv-loc__name ${stylex.props(settingsStyles.locName).className}`}>{loc.volume.name} <span className={`vv-loc__kind ${stylex.props(settingsStyles.locKind).className}`}>· {kindLine}</span></span>
        <PathText path={loc.displayPath ?? loc.path} className={`vv-mono vv-loc__path ${stylex.props(settingsStyles.locPath, settingsStyles.mono).className}`} />
      </span>
      <ActionButton variant="quiet" size="sm" pending={finderPending} onClick={onFinder}>{t.storage.project.finder}</ActionButton>
      {canChange ? <ActionButton variant="outline" size="sm" onClick={onPick}>{t.storage.project.change}</ActionButton> : null}
    </div>
  )
}

export default function StorageSection() {
  const server = useResource(['storage', 'server'], ({ signal }) => getServerStorage(signal))
  if (server.data === undefined) return server.error
    ? <Banner tone="error" title={(server.error as Error).message} actions={[{ label: t.common.retry, onClick: () => void server.refresh() }]} />
    : <Spinner size={16} />
  return server.data ? <ServerStorageSection data={server.data} /> : <LocalStorageSection />
}

function LocalStorageSection() {
  const mobile = useMediaQuery('(max-width: 767px)')
  const res = useResource(keys.storage(), ({ signal }) => getStorage(signal))
  const activity = useResource(keys.activity(), ({ signal }) => getActivity(signal))
  const reclaim = useResource(['storage-reclaim'], ({ signal }) => getStorageReclaim(signal))
  // store=pick: khối chọn tại chỗ; browse = hộp chọn thư mục phía máy chủ.
  const [pick, setPick] = useState<{ open: boolean; chosen: string; browse: boolean }>({ open: false, chosen: '', browse: false })
  const finder = useAsyncAction(async () => { await openStorageInFinder() })
  const recheck = useAsyncAction(async () => { await recheckStorage() })
  const fallback = useAsyncAction(async () => { await useDefaultForNew() })
  const cancelMove = useAsyncAction(async (id: string) => { await cancelStorageMove(id); invalidate(keys.storage()) })

  if (res.status === 'loading' && !res.data) return <Spinner size={16} />
  if (res.status === 'error' && !res.data) {
    return (
      <section className={`vv-sto__card ${stylex.props(settingsStyles.card, settingsStyles.stoCard).className}`}>
        <span {...stylex.props(box.errline)}>
          <span className={`vv-sto__grow ${stylex.props(settingsStyles.stoGrow).className}`}>{(res.error as Error | undefined)?.message ?? ''}</span>
          <ActionButton variant="outline" size="sm" onClick={() => void res.refresh()}>{t.common.retry}</ActionButton>
        </span>
      </section>
    )
  }
  const data = res.data!
  const loc = data.projects
  // Chỉ trạng thái đang chạy mới hiện MoveProgress + khoá Đổi thư mục; 'failed'
  // của activity KHÔNG tính — server báo riêng qua data.move.failed.
  const moveTask = (activity.data?.running ?? []).find((x) =>
    x.kind === 'storage_move' && (x.state === 'running' || x.state === 'waiting' || x.state === 'paused'))
  const moveFailed = data.move?.failed ?? null
  const moving = data.move?.active || Boolean(moveTask)
  const jobs = (activity.data?.running ?? []).filter((x) => x.kind !== 'storage_move').length
  // Chỗ trống luôn của ổ đang giữ thư mục lưu dự án, kèm tên ổ; ổ rút thì ghi
  // “chưa cắm · chưa đọc được chỗ trống” — KHÔNG lấy số của ổ trong máy (#79).
  const freeText = freeLine(loc.volume, loc.online, mobile)
  const totalUsed = loc.usedBytes + data.stayBytes
  const usageGroups = [
    ...data.usage.map((g) => ({ id: g.id, label: t.storage.usage.groups[g.id] ?? g.id, bytes: g.bytes })),
    { id: 'stay', label: t.storage.usage.groups.stay, bytes: data.stayBytes },
  ].filter((g) => g.bytes > 0)
  const items = reclaim.data?.items ?? []
  const canChange = !moving && (loc.online || loc.volume.kind === 'internal')

  // Mở khối chọn: chọn sẵn gợi ý đầu tiên khác thư mục hiện tại (artboard
  // Settings · store=pick mở ra với ổ ngoài đang cắm), không có thì giữ chỗ cũ.
  const openPick = () => {
    const first = data.suggestions.find((s) => s.path !== loc.path) ?? data.suggestions[0]
    setPick({ open: true, chosen: first?.path ?? loc.path, browse: false })
  }

  const pickPanel = pick.open ? (
    <PickPanel data={data} chosen={pick.chosen} jobs={jobs} mobile={mobile}
      onChosen={(path) => setPick((p) => ({ ...p, chosen: path }))}
      onBrowse={() => setPick((p) => ({ ...p, browse: true }))}
      onCancel={() => setPick({ open: false, chosen: '', browse: false })}
      onMoved={() => setPick({ open: false, chosen: '', browse: false })} />
  ) : null

  const browser = pick.open && pick.browse ? (
    mobile ? (
      <Sheet open onClose={() => setPick((p) => ({ ...p, browse: false }))} title={t.storage.picker.title}>
        <FolderBrowser path={pick.chosen} onPick={(p) => {
          if (p.startsWith('use:')) setPick((s) => ({ ...s, chosen: p.slice(4), browse: false }))
          else setPick((s) => ({ ...s, chosen: p }))
        }} />
      </Sheet>
    ) : (
      <Dialog title={t.storage.picker.title} onClose={() => setPick((p) => ({ ...p, browse: false }))} width={640} padBody={false}>
        <FolderBrowser path={pick.chosen} onPick={(p) => {
          if (p.startsWith('use:')) setPick((s) => ({ ...s, chosen: p.slice(4), browse: false }))
          else setPick((s) => ({ ...s, chosen: p }))
        }} />
      </Dialog>
    )
  ) : null

  const offline = !loc.online ? (
    <Banner tone="error" title={loc.volume.kind === 'internal'
      ? t.storage.offline.titleFolder
      : fill(t.storage.offline.title, { name: loc.volume.name })}
      iconStyle={settingsStyles.storageBannerIcon} titleStyle={settingsStyles.storageBannerTitle} textStyle={settingsStyles.storageBannerText}
      actions={[
        { label: t.storage.offline.retry, icon: <Icon name="refresh" size={14} />, variant: 'primary', size: mobile ? ('lg' as const) : undefined, onClick: () => void recheck.run() },
        { label: fill(t.storage.offline.fallback, { volume: 'Macintosh HD' }), variant: 'ghost', size: mobile ? ('lg' as const) : undefined, onClick: () => void fallback.run() },
      ]}>
      {mobile
        ? fill(t.storage.offline.descM, { time: fmtTime(loc.offlineSince), n: jobs })
        : fill(jobs > 0 ? t.storage.offline.desc : t.storage.offline.descNone, { time: fmtTime(loc.offlineSince), n: jobs })}
    </Banner>
  ) : null

  const bannerDone = loc.previous?.mode === 'new_only' || (!loc.previous && loc.lastSwitch?.mode === 'new_only')
    ? <div className={`vv-sto__bannerFix ${stylex.props(stoBannerFixMarker).className}`}><DoneNewBanner data={data} /></div>
    : loc.previous ? <div className={`vv-sto__bannerFix ${stylex.props(stoBannerFixMarker).className}`}><DoneBanner data={data} mobile={mobile} /></div> : null

  if (mobile) {
    return (
      <>
        <p className={`vv-sto__leadM ${stylex.props(settingsStyles.stoLeadM).className}`}>{t.storage.project.ledeM}</p>
        <section className={`vv-sto__card vv-sto__card--m ${stylex.props(settingsStyles.card, settingsStyles.stoCard, settingsStyles.stoCardM).className}`} aria-label={t.storage.project.title}>
          <ProjectHead data={data} mobile />
          {offline ? <div className={`vv-sto__bannerFix ${stylex.props(stoBannerFixMarker).className}`}>{offline}</div> : null}
          <LocationRow data={data} mobile finderPending={finder.pending} onFinder={() => void finder.run()}
            canChange={false} onPick={openPick} />
          {moving && moveTask ? (
            <div role="status" className={`vv-move vv-move--m ${stylex.props(moveMMarker, settingsStyles.move, settingsStyles.moveM).className}`}>
              <b className={`vv-move__title ${stylex.props(settingsStyles.moveTitle).className}`}>{moveTask.title || t.storage.moving.title}</b>
              <span className={`vv-num vv-move__stats ${stylex.props(settingsStyles.num, settingsStyles.moveStats).className}`}>
                {fill(t.storage.moving.stats, { done: gbOnly(moveTask.progress?.done ?? 0), total: gbOnly(moveTask.progress?.total ?? 0), eta: fmtEta(moveTask.etaSec) })}
              </span>
              <ProgressBar value={moveTask.progress && moveTask.progress.total > 0 ? Math.round((moveTask.progress.done / moveTask.progress.total) * 100) : 0} tone="info" height={6} />
              <span className={`vv-help ${stylex.props(settingsStyles.help).className}`}>{t.storage.moving.noteM}</span>
              <ActionButton variant="quiet" size="lg" pending={cancelMove.pending} onClick={() => void cancelMove.run(moveTask.id)}>{t.storage.moving.cancelShort}</ActionButton>
            </div>
          ) : null}
          {!moving && bannerDone ? <div className={`vv-sto__mBanner ${stylex.props(settingsStyles.stoMBanner).className}`}>{bannerDone}</div> : null}
          {!moving && moveFailed ? <FailedMoveCard failed={moveFailed} /> : null}
          {/* “Đổi thư mục lưu” 44px nằm sau kết quả (MSettings: loc → banner → nút). */}
          {!moving && canChange && loc.online ? (
            <span className={`vv-sto__locAct ${stylex.props(stoLocActMarker, settingsStyles.stoLocAct).className}`}>
              <ActionButton variant="outline" size="lg" icon={<Icon name="folder" size={16} />} onClick={openPick} xstyle={[settingsStyles.VvStoLocAct_]}>{t.storage.project.changeMobile}</ActionButton>
            </span>
          ) : null}
        </section>
        <section className={`vv-sto__card vv-sto__card--m ${stylex.props(settingsStyles.card, settingsStyles.stoCard, settingsStyles.stoCardM).className}`} aria-label={t.storage.usage.titleM}>
          <div className={`vv-sto__headM vv-sto__headM--row ${stylex.props(stoHeadMRowMarker, settingsStyles.stoHeadM, settingsStyles.stoHeadMRow).className}`}>
            <span className={`vv-sto__h2M ${stylex.props(settingsStyles.stoH2M).className}`}>{fill(t.storage.usage.titleM, { total: fmtGB(totalUsed) })}</span>
            <span className={`vv-help ${stylex.props(settingsStyles.help).className}`}>{freeText}</span>
          </div>
          <div role="img" aria-label={fill(t.storage.usage.titleM, { total: fmtGB(totalUsed) })} className={`vv-usage__bar vv-usage__bar--m ${stylex.props(usageBarMarker, settingsStyles.usageBar, settingsStyles.usageBarM).className}`}>
            {usageGroups.map((g) => <span key={g.id} {...stylex.props(settingsStyles.VvUsageBarSpan, ((g.id) === "sources") && settingsStyles.chipGSources, ((g.id) === "exports") && settingsStyles.chipGExports, ((g.id) === "editor") && settingsStyles.chipGEditor, ((g.id) === "images") && settingsStyles.chipGImages, ((g.id) === "stay") && settingsStyles.chipGStay, ((g.id) === "other") && settingsStyles.chipGOther)} style={{ flex: `${g.bytes}` }} />)}
          </div>
          {items.map((item) => (
            <MobileReclaim key={item.id} item={item} />
          ))}
        </section>
        {pick.open ? (
          <Sheet open onClose={() => setPick({ open: false, chosen: '', browse: false })} title={t.storage.picker.titleM}
            height="80dvh" style={{ background: 'var(--vv-sheet-bg)' }}>
            <span className={`vv-help vv-sto__hintLine ${stylex.props(settingsStyles.help, settingsStyles.stoHintLine).className}`}>{t.storage.picker.titleMSub}</span>
            {pickPanel}
          </Sheet>
        ) : null}
        {browser}
      </>
    )
  }

  return (
    <>
      <section className={`vv-sto__card ${stylex.props(settingsStyles.card, settingsStyles.stoCard).className}`} aria-labelledby="h-proj">
        <ProjectHead data={data} />
        {offline ? <div className={`vv-sto__bannerFix ${stylex.props(stoBannerFixMarker).className}`}>{offline}</div> : null}
        {/* store=pick thay hẳn dòng vị trí bằng khối chọn (artboard Settings). */}
        {!pick.open ? (
          <LocationRow data={data} finderPending={finder.pending} onFinder={() => void finder.run()}
            canChange={canChange && loc.online} onPick={openPick} />
        ) : null}
        {moving && moveTask ? <MoveProgress task={moveTask} jobs={jobs} volume={loc.volume.name} /> : null}
        {!moving && bannerDone}
        {!moving && moveFailed ? <FailedMoveCard failed={moveFailed} /> : null}
        {/* Chú thích “Luôn ở máy này” chỉ ở store=here — không hiện lúc đang
            chuyển / vừa xong / đang chọn / ổ rút (artboard). */}
        {!pick.open && loc.online && !moving && !bannerDone ? (
          <span className={`vv-help vv-sto__stay ${stylex.props(settingsStyles.stoStay, settingsStyles.help).className}`}>{fill(t.storage.project.stayNote, { bytes: gbOnly(data.stayBytes) })}</span>
        ) : null}
        {pickPanel}
      </section>

      <section className={`vv-sto__card ${stylex.props(settingsStyles.card, settingsStyles.stoCard).className}`} aria-labelledby="h-use">
        <div className={`vv-sto__head ${stylex.props(settingsStyles.stoHead).className}`}>
          <h2 id="h-use" className={`vv-sto__h2 ${stylex.props(settingsStyles.stoH2).className}`}>
            {fill(t.storage.usage.title, { total: fmtGB(totalUsed) })}
          </h2>
          <span className={`vv-num vv-sto__free ${stylex.props(settingsStyles.stoFree, settingsStyles.num).className}`} title={t.storage.usage.freeTitle}>{freeText}</span>
        </div>
        <div role="img" aria-label={fill(t.storage.usage.title, { total: fmtGB(totalUsed) })} className={`vv-usage__bar ${stylex.props(usageBarMarker, settingsStyles.usageBar).className}`}>
          {usageGroups.map((g) => <span key={g.id} {...stylex.props(settingsStyles.VvUsageBarSpan, ((g.id) === "sources") && settingsStyles.chipGSources, ((g.id) === "exports") && settingsStyles.chipGExports, ((g.id) === "editor") && settingsStyles.chipGEditor, ((g.id) === "images") && settingsStyles.chipGImages, ((g.id) === "stay") && settingsStyles.chipGStay, ((g.id) === "other") && settingsStyles.chipGOther)} style={{ flex: `${g.bytes}` }} />)}
        </div>
        <div className={`vv-usage__legend ${stylex.props(usageLegendMarker, settingsStyles.usageLegend).className}`}>
          {usageGroups.map((g) => (
            <span key={g.id} {...stylex.props(settingsStyles.VvUsageLegendSpan)}>
              <span className={`vv-chip ${stylex.props(((g.id) === "sources") && settingsStyles.chipGSources, ((g.id) === "exports") && settingsStyles.chipGExports, ((g.id) === "editor") && settingsStyles.chipGEditor, ((g.id) === "images") && settingsStyles.chipGImages, ((g.id) === "stay") && settingsStyles.chipGStay, ((g.id) === "other") && settingsStyles.chipGOther, settingsStyles.chip).className}`} aria-hidden="true" />
              <span className={`vv-lb ${stylex.props(settingsStyles.lb).className}`}>{g.label}</span>
              <span className={`vv-num ${stylex.props(settingsStyles.num).className}`}>{fmtGB(g.bytes)}</span>
            </span>
          ))}
        </div>
      </section>

      <CleanupCard data={data} />

      {/* Server luôn trả hai nhóm (count/bytes = 0 khi chưa có video) — card hiện mờ. */}
      {items.map((item) => <ReclaimCard key={item.id} item={item} />)}

      {browser}
    </>
  )
}

/** Hàng xoá của điện thoại nằm trong thẻ dung lượng (MSettings · screen=storage). */
function MobileReclaim({ item }: { item: ReclaimItem }) {
  const [gone, setGone] = useState<number | null>(null)
  const run = useAsyncAction(async () => {
    await reclaimStorage(item.id)
    setGone(item.bytes)
    invalidate(keys.storage())
    invalidate(['storage-reclaim'])
  })
  const empty = item.count <= 0
  const note = gone !== null
    ? fill(t.storage.reclaim.gone, { bytes: fmtBytes(gone) })
    : fill(t.storage.reclaim.rowNotes[item.id] ?? '{bytes}', { bytes: fmtBytes(item.bytes) })
  return (
    <div className={`vv-sto__rowM ${stylex.props(settingsStyles.stoRowM).className}`}>
      <span className={`vv-sto__rowTextM ${stylex.props(settingsStyles.stoRowTextM).className}`}>
        <span className={`vv-sto__rowTitleM ${stylex.props(settingsStyles.stoRowTitleM).className}`}>{item.title}</span>
        <span className={`vv-sto__rowNoteM ${stylex.props(settingsStyles.stoRowNoteM, (gone !== null) && settingsStyles.stoRowNoteMIsGone).className}`}>{empty && gone === null ? t.storage.reclaim.nothing : note}</span>
      </span>
      {gone === null ? (
        <ActionButton variant="danger-soft" size="lg" pending={run.pending} disabled={empty} disabledReason={t.storage.reclaim.nothing}
          onClick={() => void run.run()}>{t.storage.reclaim.mobileAction}</ActionButton>
      ) : null}
    </div>
  )
}
