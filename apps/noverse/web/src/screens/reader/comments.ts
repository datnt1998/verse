import { useState } from 'react'
import { addParagraphComment, deleteParagraphComment, paragraphComments, patchParagraphComment } from '../../api/client'
import { PARAGRAPH_COMMENT_CHIPS, type ParagraphComment, type ParagraphCommentInput, type ParagraphCommentPatch, type ParagraphCommentStatus, type User } from '../../api/types'
import { cache, keys, useAction, useResource, type Key } from '../../data'
import { useSession } from '../../api/session'
import { toast } from '@noverse/ui'
import { parseChapterText, parseInline, type ChapterBlock } from '../../lib/markdown'
export { PARAGRAPH_COMMENT_CHIPS }
export type ParagraphSelection = { paragraph: number | null; quote: string }
const normalize = (text: string) => text.replace(/\s+/g, ' ').trim()
/** Một bộ tách đoạn dùng chung cho hai nền tảng (Gói AI): chỉ khối `paragraph`,
 *  phẳng theo đúng chữ đang hiện — không đếm `p` render (dấu ngắt cảnh/nhãn diff). */
export function chapterParagraphs(blocks: readonly ChapterBlock[]): string[] {
  return blocks.filter((block): block is ChapterBlock & { kind: 'paragraph' } => block.kind === 'paragraph')
    .map((block) => parseInline(block.text).map((token) => token.text).join(''))
}

/** Trạng thái vòng đời; bản cũ thiếu `status` suy ra từ `appliedAt` (đã gửi). */
export function commentStatus(comment: ParagraphComment): ParagraphCommentStatus {
  return comment.status ?? (comment.appliedAt ? 'sent' : 'pending')
}

/**
 * Ai được sửa/xoá một góp ý — cùng ranh giới với máy chủ: người tạo góp ý, chủ dự
 * án hoặc quản trị viên (`manages` = `canManageProject`, admin cũng trả true).
 * Cộng tác viên chỉ đổi được góp ý của chính mình; thẻ của người khác là chỉ đọc.
 */
export function canChangeComment(author: string, user: User | null, manages: boolean): boolean {
  if (!user) return false
  return manages || (!!author && author === user.username)
}

/** Đoạn đã giải mà badge/gạch chân gắn vào: ưu tiên `anchor.paragraph` (máy chủ giải
 *  sau mỗi lần ghi); thiếu anchor thì về `paragraph` (góp ý mới/ngoại tuyến). */
export function anchorParagraph(comment: ParagraphComment): number | null {
  const status = commentStatus(comment)
  if (status !== 'pending' && status !== 'sent' && status !== 'fixed') return null
  return comment.anchor?.paragraph ?? comment.paragraph
}

/** Ne devine jamais parmi deux passages identiques: l'index explicite conserve l'ancrage. */
export function paragraphAnchor(paragraphs: readonly string[], quote: string, index?: number): ParagraphSelection {
  const selected = normalize(quote)
  if (!selected) return { paragraph: null, quote: '' }
  if (index !== undefined && paragraphs[index] !== undefined && normalize(paragraphs[index]).includes(selected)) return { paragraph: index, quote: selected }
  const matches = paragraphs.flatMap((text, paragraph) => normalize(text).includes(selected) ? [paragraph] : [])
  return { paragraph: matches.length === 1 ? matches[0] : null, quote: selected }
}
export function selectionInReader(root: HTMLElement, selection: Selection | null): ParagraphSelection {
  if (!selection || selection.isCollapsed || !selection.anchorNode || !selection.focusNode || !root.contains(selection.anchorNode) || !root.contains(selection.focusNode)) return { paragraph: null, quote: '' }
  // Chỉ đếm đoạn văn thật (`data-prose`), khớp `chapterParagraphs` — `p` của dấu
  // ngắt cảnh/diff/attach không được lệch chỉ số. Giá trị data-prose là chỉ số
  // canonical: một đoạn bị diff tách đôi có hai p cùng chỉ số.
  const nodes = [...root.querySelectorAll<HTMLElement>('p[data-prose]')]
  const texts: string[] = []
  for (const node of nodes) {
    const raw = node.dataset.prose
    const index = raw === undefined || raw === '' ? texts.length : Number(raw)
    texts[index] = (texts[index] ?? '') + (node.textContent ?? '')
  }
  const anchor = selection.anchorNode.nodeType === Node.ELEMENT_NODE ? selection.anchorNode as Element : selection.anchorNode.parentElement
  const focus = selection.focusNode.nodeType === Node.ELEMENT_NODE ? selection.focusNode as Element : selection.focusNode.parentElement
  const paragraph = anchor?.closest('p[data-prose]') as HTMLElement | null
  if (!paragraph || paragraph !== focus?.closest('p[data-prose]')) return { paragraph: null, quote: normalize(selection.toString()) }
  const raw = paragraph.dataset.prose
  const index = raw === undefined || raw === '' ? nodes.indexOf(paragraph) : Number(raw)
  return paragraphAnchor(texts, selection.toString(), index)
}

/** Vùng của `quote` trong đoạn (khớp nguyên văn sau khi gộp khoảng trắng); -1 = không thấy. */
export function quoteRange(text: string, quote: string): { at: number; len: number } | null {
  const needle = normalize(quote)
  if (!needle) return null
  const map: number[] = []
  const flat: string[] = []
  let lastSpace = false
  for (let i = 0; i < text.length; i++) {
    const space = /\s/.test(text[i])
    if (space && lastSpace) continue
    const start = flat.length === 0
    if (start && space) continue
    map.push(i)
    flat.push(space ? ' ' : text[i])
    lastSpace = space
  }
  const normalized = flat.join('').replace(/ +$/g, '')
  const found = normalized.indexOf(needle)
  if (found < 0) return null
  const end = normalized.indexOf(' ', found + needle.length)
  const upto = end < 0 ? normalized.length : end
  return { at: map[found], len: map[upto - 1] + 1 - map[found] }
}

export function optimisticCommentAdd(key: Key, input: ParagraphCommentInput, author: string) {
  const previous = cache.get<{ comments: ParagraphComment[] }>(key).data
  const comment: ParagraphComment = { ...input, id: `pending-${Date.now()}`, author, createdAt: new Date().toISOString(), status: 'pending' }
  cache.setData(key, { comments: [...(previous?.comments ?? []), comment] })
  return () => cache.restore(key, previous)
}

/**
 * Defer the irreversible request for the 5 s undo window, so undo keeps the original
 * author, ID and timestamp. Resolves the commit result, or `undefined` when undone.
 */
export function deleteCommentWithUndo<T>(commit: () => Promise<T>): Promise<T | undefined> {
  return new Promise((resolve, reject) => {
    let waiting = true
    const timer = setTimeout(() => {
      waiting = false
      void commit().then(resolve, reject)
    }, 5000)
    toast('Đã xoá góp ý', 'ok', { label: 'Hoàn tác', onClick: () => {
      if (!waiting) return
      waiting = false
      clearTimeout(timer)
      toast.ok('Đã hoàn tác')
      resolve(undefined)
    } }, { undo: true, duration: 5000 })
  })
}

export function useParagraphComments(project: string, story: string, chapter: number | null) {
  const { user } = useSession()
  const key = keys.story(project, story, 'comments', chapter)
  const resource = useResource(key, () => paragraphComments(project, story, chapter!), { enabled: chapter !== null })
  const [hidden, setHidden] = useState<string[]>([])
  const add = useAction(async (input: ParagraphCommentInput) => {
    const comment = await addParagraphComment(project, story, chapter!, input)
    return comment
  }, {
    success: 'Đã lưu góp ý', invalidate: [key],
    optimistic: (input) => optimisticCommentAdd(key, input, user?.username ?? ''),
    undo: async (comment) => { await deleteParagraphComment(project, story, chapter!, comment.id) },
  })
  // Server failures surface as the action's error toast ("Thử lại"); the comment reappears (rollback).
  const commitDelete = useAction((id: string) => deleteParagraphComment(project, story, chapter!, id), { invalidate: [key] })
  const patch = useAction((id: string, body: ParagraphCommentPatch) => patchParagraphComment(project, story, chapter!, id, body), { invalidate: [key], error: 'Không cập nhật được góp ý' })
  /** Hide now, DELETE after the undo window; other comments stay deletable meanwhile. */
  const remove = async (id: string) => {
    setHidden((ids) => [...ids, id])
    try {
      const deleted = await deleteCommentWithUndo(() => commitDelete.run(id))
      if (deleted) cache.setData<{ comments: ParagraphComment[] }>(key, (current) => ({ comments: (current?.comments ?? []).filter((comment) => comment.id !== id) }))
    } finally {
      setHidden((ids) => ids.filter((value) => value !== id))
    }
  }
  const data = resource.data ? { comments: resource.data.comments.filter((comment) => !hidden.includes(comment.id)) } : undefined
  return { ...resource, data, add, remove, patch }
}

/** Kết quả của `useParagraphComments` — hợp đồng dữ liệu + tác vụ cho ngăn/sheet góp ý. */
export type ParagraphComments = ReturnType<typeof useParagraphComments>
