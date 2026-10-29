// Bảng “Xưng hô” + “Lịch sử quan hệ theo chương” của trang nhân vật (Outline.dc.html › person=detail,
// xedit=add · addConfirm · rel · relConfirm). Mọi thao tác sửa đi qua GAP-38, lý do bắt buộc (Câu 20);
// một hộp xác nhận dùng chung cho cả ba thao tác khi “từ chương” chạm chương đã viết (Câu 21).
import { useState, type CSSProperties } from 'react'
import { ApiError, postRelation, putAddressTerm } from '../../../api/client'
import type { AddressApply, AddressTerm, AddressTerms, RelationChange } from '../../../api/types'
import { invalidate, keys } from '../../../data'
import { confirm } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { KitSelect } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { CHIP_NEUTRAL, Chip } from '../bits'
import {
  RELATION_OPTIONS, RELATION_OTHER, addFormNote, affectedChapters, appliedLabel, applyOptions, applyToast,
  confirmBody, confirmTitle, isExpired, otherNames, relationBefore, relationTone, relationsFor, validateRelation,
  validateTerm, type ConfirmInput, type RelationTone,
} from './logic'

/** `.ibtn` của artboard — nút nhỏ trong bảng. */
const IBTN: CSSProperties = {
  height: 28, padding: '0 10px', borderRadius: 8, border: '1px solid var(--line-4)',
  background: 'transparent', color: 'var(--color-fg-2)', fontSize: 12,
  display: 'inline-flex', alignItems: 'center', gap: 6, whiteSpace: 'nowrap',
}
/** `.fin` — ô nhập trong dòng đang sửa. */
const FIN: CSSProperties = {
  height: 30, padding: '0 9px', borderRadius: 8, border: '1px solid var(--kit-acc-45)',
  background: 'var(--color-surface-0)', color: 'var(--color-fg)', font: 'inherit', fontSize: 13,
  width: '100%', minWidth: 0,
}
/** Nhãn nhỏ phía trên ô nhập của form thêm dòng. */
const LABEL: CSSProperties = {
  display: 'flex', flexDirection: 'column', gap: 4, fontSize: 11.5, color: 'var(--kit-fg-icon)',
}
const ROW: CSSProperties = {
  display: 'grid', gridTemplateColumns: 'minmax(0,1.1fr) 92px 82px 92px minmax(0,1.5fr) 64px',
  gap: 10, alignItems: 'center', padding: '9px 14px', fontSize: 13,
}
const REL_ROW: CSSProperties = {
  display: 'grid', gridTemplateColumns: '64px 150px minmax(0,1fr)', gap: 12,
  alignItems: 'start', padding: '8px 0', fontSize: 13,
}
const SAVE: CSSProperties = {
  ...IBTN, background: 'var(--color-fg)', color: 'var(--color-bg)', border: '1px solid var(--color-fg)', fontWeight: 600,
}
const lockedStyle = (locked: boolean): CSSProperties => ({ opacity: locked ? 0.45 : 1, cursor: locked ? 'not-allowed' : 'pointer' })

/** Ba nhóm màu chip quan hệ (Câu 20) — token, không viết mã màu ở đây. */
const REL_CHIP: Record<RelationTone, { background: string; color: string }> = {
  hostile: { background: 'var(--tint-err)', color: 'var(--kit-err-soft)' },
  close: { background: 'var(--tint-ok)', color: 'var(--color-ok)' },
  other: { background: 'var(--frame-info-12)', color: 'var(--color-info)' },
}

type TermDraft = { id?: string; from: string; to: string; call: string; self: string; fromChapter: number; reason: string }
type EditRow = { id: string; call: string; self: string; fromChapter: string; reason: string }
type AddRow = { from: string; to: string; call: string; self: string; fromChapter: string; reason: string }
type RelationForm = { b: string; after: string; other: string; reason: string; fromChapter: string }

export function AddressTable({ project, story, person, names, address }: {
  project: string
  story: string
  person: string
  names: string[]
  address: AddressTerms
}) {
  const [editing, setEditing] = useState<EditRow | null>(null)
  const [adding, setAdding] = useState<AddRow | null>(null)
  const [relation, setRelation] = useState<RelationForm | null>(null)
  const [saving, setSaving] = useState(false)
  const locked = Boolean(address.writing)
  const terms = address.terms.filter((term) => term.from === person || term.to === person)
  const relations = relationsFor(address.relations, person)
  const others = otherNames(names, person)
  const refresh = () => { void invalidate(keys.story(project, story, 'address')) }

  const send = async (term: TermDraft, apply?: AddressApply) => {
    const next = await putAddressTerm(project, story, {
      term: {
        id: term.id, from: term.from, to: term.to, call: term.call.trim(), self: term.self.trim(),
        fromChapter: term.fromChapter, reason: term.reason.trim(),
      },
      ...(apply ? { apply } : {}),
    })
    refresh()
    return next
  }

  /** Một hộp xác nhận cho cả ba thao tác; `run` nhận lựa chọn `next` | `mark`. */
  const askApply = (input: ConfirmInput, run: (apply: AddressApply) => Promise<void>) => confirm({
    tone: 'warn',
    title: confirmTitle(input),
    body: confirmBody(input),
    choices: applyOptions(input),
    defaultChoice: 'mark',
    confirmLabel: 'Lưu và áp dụng',
    pendingLabel: 'Đang lưu…',
    successMessage: (choice) => applyToast(input, choice as AddressApply),
    onConfirm: async (choice) => { await run(choice as AddressApply) },
  })

  const saveTerm = async (term: TermDraft, mode: 'edit' | 'add', oldCall: string) => {
    if (locked || saving) return
    const problem = validateTerm(term)
    if (problem) { toast.err(problem); return }
    const affected = affectedChapters(term.fromChapter, address.latestWritten)
    const ask = (list: number[]) => askApply(
      mode === 'add'
        ? { mode: 'add', affected: list, latestWritten: address.latestWritten, fromName: term.from, toName: term.to, call: term.call.trim(), fromChapter: term.fromChapter }
        : { mode: 'edit', affected: list, latestWritten: address.latestWritten, fromName: term.from, toName: term.to, call: term.call.trim(), fromChapter: term.fromChapter, oldCall },
      async (apply) => { await send(term, apply); setEditing(null); setAdding(null) },
    )
    if (affected.length > 0) { await ask(affected); return }
    setSaving(true)
    try {
      await send(term)
      toast.ok(`${mode === 'add' ? 'Đã thêm dòng xưng hô' : 'Đã lưu xưng hô'} · áp dụng từ chương ${term.fromChapter}`)
      setEditing(null)
      setAdding(null)
    } catch (error) {
      // Máy chủ thấy chương đã viết mà web chưa biết (dữ liệu cũ hơn): hỏi cách áp dụng như khi tự tính.
      // 409 không kèm `affected` (vd. dòng đã hết hiệu lực) thì chỉ báo lỗi của máy chủ.
      if (error instanceof ApiError && error.status === 409) {
        const payload = error.payload as { affected?: number[] } | null
        if (payload?.affected?.length) { await ask(payload.affected); return }
      }
      if (error instanceof ApiError && error.status === 423) refresh()
      toast.err(error instanceof Error ? error.message : 'Không lưu được xưng hô.')
    } finally {
      setSaving(false)
    }
  }

  const saveRelation = async () => {
    if (!relation || locked || saving) return
    const fromChapter = Number(relation.fromChapter)
    const after = (relation.after === RELATION_OTHER ? relation.other : relation.after).trim()
    const problem = validateRelation({ a: person, b: relation.b, after, fromChapter, reason: relation.reason })
    if (problem) { toast.err(problem); return }
    const before = relationBefore(address.relations, person, relation.b, fromChapter)?.after ?? ''
    const commit = async (apply?: AddressApply) => {
      await postRelation(project, story, {
        a: person, b: relation.b, after, reason: relation.reason.trim(), fromChapter,
        ...(apply ? { apply } : {}),
      })
      refresh()
      setRelation(null)
    }
    const ask = () => askApply(
      { mode: 'rel', chapter: fromChapter, latestWritten: address.latestWritten, other: relation.b, before, after },
      async (apply) => { await commit(apply) },
    )
    if (fromChapter <= address.latestWritten) { await ask(); return }
    setSaving(true)
    try {
      await commit()
      toast.ok(`Đã ghi thay đổi quan hệ ở chương ${fromChapter}`)
    } catch (error) {
      if (error instanceof ApiError && error.status === 409) { await ask(); return }
      if (error instanceof ApiError && error.status === 423) refresh()
      toast.err(error instanceof Error ? error.message : 'Không ghi được thay đổi quan hệ.')
    } finally {
      setSaving(false)
    }
  }

  const startEdit = (term: AddressTerm) => {
    if (locked) return
    setAdding(null)
    setEditing({ id: term.id, call: term.call, self: term.self, fromChapter: String(term.fromChapter), reason: term.reason })
  }

  const startAdd = () => {
    if (locked) return
    setEditing(null)
    setAdding({ from: person, to: others[0] ?? person, call: '', self: '', fromChapter: String(address.latestWritten + 1), reason: '' })
  }

  const draft: TermDraft | null = adding
    ? { from: adding.from, to: adding.to, call: adding.call, self: adding.self, fromChapter: Number(adding.fromChapter), reason: adding.reason }
    : null
  const addProblem = draft ? validateTerm(draft) : 'Chưa mở form'
  const beforeChip = relation ? relationBefore(address.relations, person, relation.b, Number(relation.fromChapter)) : null
  const relAfter = relation ? (relation.after === RELATION_OTHER ? relation.other : relation.after).trim() : ''
  const relProblem = relation
    ? validateRelation({ a: person, b: relation.b, after: relAfter, fromChapter: Number(relation.fromChapter), reason: relation.reason })
    : 'Chưa mở form'

  return (
    <>
      <section className="card" aria-labelledby="xh-t" style={{ marginTop: 10, borderRadius: 16, overflow: 'hidden' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '12px 14px' }}>
          <span id="xh-t" style={{ fontSize: 14, fontWeight: 600 }}>Xưng hô</span>
          <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>mỗi dòng một chiều: ai gọi ai · Biên tập viên soát theo bảng này</span>
          <span style={{ flex: 1 }} />
          <button type="button" style={{ ...IBTN, ...lockedStyle(locked) }} disabled={locked} aria-disabled={locked}
            aria-expanded={adding !== null} onClick={startAdd}>
            <Icon name="add" size={13} />Thêm dòng
          </button>
        </div>
        <div style={{ ...ROW, fontSize: 11.5, color: 'var(--color-fg-4)', borderTop: '1px solid var(--line-1)', paddingTop: 8, paddingBottom: 8 }}>
          <span>Chiều</span><span>Gọi là</span><span>Tự xưng</span><span>Áp dụng</span><span>Lý do</span><span />
        </div>
        {adding && (
          <div role="group" aria-label="Thêm dòng xưng hô" style={{
            padding: '12px 14px', display: 'flex', flexDirection: 'column', gap: 10,
            background: 'var(--kit-acc-5)', borderTop: '1px solid var(--kit-acc-25)', borderBottom: '1px solid var(--kit-acc-25)',
          }}>
            <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'flex-end', gap: 10 }}>
              <label style={{ ...LABEL, width: 170 }}>Ai gọi
                <KitSelect aria-label="Ai gọi" value={adding.from} height={30}
                  options={names.map((name) => ({ value: name, label: name }))}
                  onChange={(name) => setAdding({ ...adding, from: name })} />
              </label>
              <Icon name="arrow-right-2" size={13} style={{ color: 'var(--kit-fg-arrow)', marginBottom: 9 }} />
              <label style={{ ...LABEL, width: 170 }}>Gọi ai
                <KitSelect aria-label="Gọi ai" value={adding.to} height={30}
                  options={others.map((name) => ({ value: name, label: name }))}
                  onChange={(name) => setAdding({ ...adding, to: name })} />
              </label>
              <label style={{ ...LABEL, width: 130 }}>Gọi là
                <input style={FIN} value={adding.call} placeholder="chị" onChange={(event) => setAdding({ ...adding, call: event.target.value })} />
              </label>
              <label style={{ ...LABEL, width: 90 }}>Tự xưng
                <input style={FIN} value={adding.self} placeholder="em" onChange={(event) => setAdding({ ...adding, self: event.target.value })} />
              </label>
              <label style={{ ...LABEL, width: 70 }}>Từ chương
                <input className="num" style={FIN} value={adding.fromChapter}
                  onChange={(event) => setAdding({ ...adding, fromChapter: event.target.value.replace(/\D+/g, '').slice(0, 3) })} />
              </label>
            </div>
            <div style={{ display: 'flex', alignItems: 'flex-end', gap: 10 }}>
              <label style={{ ...LABEL, flex: 1 }}>Lý do · bắt buộc
                <input style={FIN} value={adding.reason} placeholder="Vì sao có cách gọi này"
                  onChange={(event) => setAdding({ ...adding, reason: event.target.value })} />
              </label>
              <button type="button" style={{ ...IBTN, height: 30 }} onClick={() => setAdding(null)}>Huỷ</button>
              <button type="button" style={{ ...SAVE, height: 30, ...lockedStyle(locked) }} disabled={locked || saving || Boolean(addProblem)}
                onClick={() => { if (draft) void saveTerm(draft, 'add', '') }}>Thêm dòng</button>
            </div>
            <span style={{ fontSize: 12, color: 'var(--kit-fg-icon)', lineHeight: 1.5 }}>{addFormNote(address.latestWritten)}</span>
          </div>
        )}
        {terms.map((term) => {
          const expired = isExpired(term, address.latestWritten)
          const editingThis = editing?.id === term.id
          const strike = expired ? 'line-through' : 'none'
          return (
            <div key={term.id} style={{
              ...ROW,
              background: editingThis ? 'var(--kit-acc-5)' : 'transparent',
              color: expired ? 'var(--color-fg-4)' : 'var(--color-fg)',
              borderTop: '1px solid var(--kit-wash-5)',
            }}>
              <span style={{ display: 'flex', alignItems: 'center', gap: 6, minWidth: 0 }}>
                <span style={{ whiteSpace: 'nowrap' }} title={term.fromName}>{term.fromName}</span>
                <Icon name="arrow-right-2" size={12} style={{ color: 'var(--kit-fg-arrow)', flex: 'none' }} />
                <span style={{ whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }} title={term.toName}>{term.toName}</span>
              </span>
              {editingThis
                ? <input style={FIN} value={editing.call} aria-label="Gọi là" onChange={(event) => setEditing({ ...editing, call: event.target.value })} />
                : <span style={{ fontWeight: 500, textDecoration: strike }}>“{term.call}”</span>}
              {editingThis
                ? <input style={FIN} value={editing.self} aria-label="Tự xưng" onChange={(event) => setEditing({ ...editing, self: event.target.value })} />
                : <span style={{ textDecoration: strike }}>“{term.self}”</span>}
              {editingThis
                ? <span style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 12, color: 'var(--color-fg-3)' }}>
                    từ ch.<input className="num" style={{ ...FIN, width: 38, padding: '0 6px' }} value={editing.fromChapter} aria-label="Từ chương"
                      onChange={(event) => setEditing({ ...editing, fromChapter: event.target.value.replace(/\D+/g, '').slice(0, 3) })} />
                  </span>
                : <span className="num" style={{ fontSize: 12.5, color: 'var(--color-fg-3)' }}>{appliedLabel(term)}</span>}
              {/* Dòng engine ghi không có lý do: sửa trên web thì lý do bắt buộc, nên ô này thành ô nhập. */}
              {editingThis && !term.reason
                ? <input style={FIN} value={editing.reason} aria-label="Lý do" placeholder="Vì sao đổi cách gọi"
                    onChange={(event) => setEditing({ ...editing, reason: event.target.value })} />
                : <span style={{ fontSize: 12.5, color: 'var(--color-fg-3)', lineHeight: 1.45 }}>{term.reason}</span>}
              <span style={{ display: 'flex', justifyContent: 'flex-end' }}>
                {editingThis
                  ? <button type="button" style={{ ...SAVE, ...lockedStyle(locked) }} disabled={locked || saving}
                      onClick={() => void saveTerm({
                        id: term.id, from: term.from, to: term.to, call: editing.call, self: editing.self,
                        fromChapter: Number(editing.fromChapter), reason: editing.reason,
                      }, 'edit', term.call)}>Lưu</button>
                  : !expired && (
                    <button type="button" style={{ ...IBTN, ...lockedStyle(locked), padding: '0 8px' }} disabled={locked} aria-disabled={locked}
                      aria-label={`Sửa xưng hô ${term.fromName} gọi ${term.toName}`} onClick={() => startEdit(term)}>
                      <Icon name="edit-2" size={13} />
                    </button>
                  )}
              </span>
            </div>
          )
        })}
        {terms.length === 0 && !adding && (
          <p style={{ margin: 0, padding: '9px 14px', fontSize: 12.5, color: 'var(--color-fg-4)', borderTop: '1px solid var(--kit-wash-5)' }}>
            Nhân vật này chưa có dòng xưng hô nào.
          </p>
        )}
      </section>

      <section className="card" aria-labelledby="qh-t" style={{ marginTop: 10, borderRadius: 16, padding: '12px 14px 8px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, paddingBottom: 6 }}>
          <span id="qh-t" style={{ fontSize: 14, fontWeight: 600 }}>Lịch sử quan hệ theo chương</span>
          <span style={{ flex: 1 }} />
          <button type="button" style={{ ...IBTN, ...lockedStyle(locked) }} disabled={locked} aria-disabled={locked}
            aria-expanded={relation !== null}
            onClick={() => {
              if (locked) return
              setRelation(relation
                ? null
                : { b: others[0] ?? person, after: RELATION_OPTIONS[0], other: '', reason: '', fromChapter: String(address.latestWritten + 1) })
            }}>
            <Icon name="add" size={13} />Ghi thay đổi
          </button>
        </div>
        {relation && (
          <div role="group" aria-label="Ghi thay đổi quan hệ" style={{
            display: 'grid', gridTemplateColumns: '64px 150px minmax(0, 1fr)', gap: 12, alignItems: 'start',
            padding: 10, margin: '0 -10px 6px', borderRadius: 12,
            background: 'var(--kit-acc-5)', border: '1px solid var(--kit-acc-25)',
          }}>
            <span style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 12, color: 'var(--color-fg-3)' }}>ch.
              <input className="num" style={{ ...FIN, width: 38, padding: '0 6px' }} value={relation.fromChapter} aria-label="Từ chương"
                onChange={(event) => setRelation({ ...relation, fromChapter: event.target.value.replace(/\D+/g, '').slice(0, 3) })} />
            </span>
            <KitSelect aria-label="Với ai" value={relation.b} height={30}
              options={others.map((name) => ({ value: name, label: name }))}
              onChange={(name) => setRelation({ ...relation, b: name })} />
            <span style={{ display: 'flex', flexDirection: 'column', gap: 8, minWidth: 0 }}>
              <span style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
                {beforeChip?.after && (
                  <>
                    <Chip tone={CHIP_NEUTRAL} title="Quan hệ đang áp dụng ở chương liền trước">{beforeChip.after}</Chip>
                    <Icon name="arrow-right-2" size={12} style={{ color: 'var(--kit-fg-arrow)' }} />
                  </>
                )}
                <span style={{ width: 170, flex: '0 0 auto' }}>
                  <KitSelect aria-label="Quan hệ mới" value={relation.after} height={30}
                    options={[...RELATION_OPTIONS, RELATION_OTHER].map((kind) => ({ value: kind, label: kind }))}
                    onChange={(kind) => setRelation({ ...relation, after: kind })} />
                </span>
                {relation.after === RELATION_OTHER && (
                  <input style={{ ...FIN, width: 170 }} value={relation.other} aria-label="Quan hệ mới tự ghi" placeholder="Quan hệ mới"
                    onChange={(event) => setRelation({ ...relation, other: event.target.value })} />
                )}
              </span>
              <input style={FIN} value={relation.reason} aria-label="Lý do" placeholder="Chuyện gì xảy ra ở chương này (bắt buộc)"
                onChange={(event) => setRelation({ ...relation, reason: event.target.value })} />
              <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                <span style={{ flex: 1, fontSize: 12, color: 'var(--kit-fg-icon)' }}>Đổi quan hệ có thể kéo theo đổi xưng hô: thêm dòng ở bảng trên nếu cần.</span>
                <button type="button" style={IBTN} onClick={() => setRelation(null)}>Huỷ</button>
                <button type="button" style={{ ...SAVE, ...lockedStyle(locked) }} disabled={locked || saving || Boolean(relProblem)}
                  onClick={() => void saveRelation()}>Lưu</button>
              </span>
            </span>
          </div>
        )}
        <ol style={{ listStyle: 'none', margin: 0, padding: 0 }}>
          {relations.map((item: RelationChange & { other: string }, index) => (
            <li key={`${item.chapter}-${item.a}-${item.b}-${index}`} style={{ ...REL_ROW, borderTop: index ? '1px solid var(--kit-wash-5)' : 'none' }}>
              <span className="num" style={{ color: 'var(--kit-fg-icon)' }}>Chương {item.chapter}</span>
              <span style={{ fontWeight: 500 }}>{item.other}</span>
              <span style={{ display: 'flex', flexDirection: 'column', gap: 3, minWidth: 0 }}>
                <span style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
                  {item.before && (
                    <>
                      <Chip tone={CHIP_NEUTRAL}>{item.before}</Chip>
                      <Icon name="arrow-right-2" size={12} style={{ color: 'var(--kit-fg-arrow)' }} />
                    </>
                  )}
                  <Chip tone={REL_CHIP[relationTone(item.type)]}>{item.after}</Chip>
                </span>
                {/* Lý do rỗng (engine ghi): không hiện dòng lý do, không ghi “—” (Câu 20). */}
                {item.reason && (
                  <span style={{ fontSize: 12.5, color: 'var(--kit-fg-icon)', lineHeight: 1.5 }}>{item.reason}</span>
                )}
              </span>
            </li>
          ))}
        </ol>
        {relations.length === 0 && (
          <p style={{ margin: 0, padding: '8px 0', fontSize: 12.5, color: 'var(--color-fg-4)' }}>Chưa có thay đổi quan hệ nào.</p>
        )}
      </section>
    </>
  )
}
