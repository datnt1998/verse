import { afterEach, describe, expect, it, vi } from 'vitest'
import { cache, createAction, keys } from '../../../data'
import { optimisticCommentAdd, paragraphAnchor, selectionInReader } from '../comments'
import type { ParagraphComment, ParagraphCommentInput } from '../../../api/types'

const paragraphs = ['Đèn trường quay bật lên.', 'Nhược Vy siết chặt tờ giấy.', 'Đèn trường quay bật lên.']

describe('M2 · ancrage et sélection des paragraphes', () => {
  afterEach(() => vi.unstubAllGlobals())
  it('normalise les espaces sans deviner les passages répétés et garde l’index choisi', () => {
    expect(paragraphAnchor(paragraphs, ' Đèn  trường quay ')).toEqual({ paragraph: null, quote: 'Đèn trường quay' })
    expect(paragraphAnchor(paragraphs, 'Đèn trường quay', 2)).toEqual({ paragraph: 2, quote: 'Đèn trường quay' })
    expect(paragraphAnchor(paragraphs, 'Nhược Vy\nsiết chặt', 0)).toEqual({ paragraph: 1, quote: 'Nhược Vy siết chặt' })
    expect(paragraphAnchor(paragraphs, '')).toEqual({ paragraph: null, quote: '' })
    expect(paragraphAnchor(paragraphs, 'đoạn đã bị thay')).toEqual({ paragraph: null, quote: 'đoạn đã bị thay' })
  })
  it('mappe le texte imbriqué au p rendu, ignore les titres et refuse les sélections hors lecteur ou traversantes', () => {
    vi.stubGlobal('Node', { ELEMENT_NODE: 1 })
    const p = paragraphs.map((text) => ({ textContent: text, dataset: {} as Record<string, string>, closest: (_selector: string): unknown => undefined }))
    for (const node of p) node.closest = (selector: string) => selector === 'p[data-prose]' ? node : undefined
    const span = { nodeType: 1, closest: () => p[1] }
    const text = { nodeType: 3, parentElement: span }
    const root = { contains: (node: unknown) => [text, span, ...p].includes(node as never), querySelectorAll: () => p } as unknown as HTMLElement
    const select = (anchor: unknown, focus: unknown, quote = 'Nhược Vy') => ({ isCollapsed: false, anchorNode: anchor, focusNode: focus, toString: () => quote }) as Selection
    expect(selectionInReader(root, select(text, text))).toEqual({ paragraph: 1, quote: 'Nhược Vy' })
    expect(selectionInReader(root, select(text, { nodeType: 1, closest: () => p[2] }))).toEqual({ paragraph: null, quote: '' })
    expect(selectionInReader(root, select(text, p[2], 'Nhược Vy siết chặt tờ giấy. Đèn trường quay'))).toEqual({ paragraph: null, quote: 'Nhược Vy siết chặt tờ giấy. Đèn trường quay' })
    expect(selectionInReader(root, null)).toEqual({ paragraph: null, quote: '' })
  })
})

describe('M2 · commentaire optimiste', () => {
  it('affiche immédiatement le commentaire et restaure exactement la liste si le réseau échoue', async () => {
    const key = keys.story('mobile-test', 'rollback', 'comments', 24)
    const previous: { comments: ParagraphComment[] } = { comments: [{ id: 'saved', paragraph: 0, quote: 'Déjà enregistré', chips: [], text: 'Conserver', author: 'dat', createdAt: '2026-10-25T00:00:00Z' }] }
    const input: ParagraphCommentInput = { paragraph: 1, quote: paragraphs[1], chips: ['Ngắn lại'], text: 'Gọn hơn' }
    cache.setData(key, previous)
    let reject!: (reason: Error) => void
    const result = new Promise<ParagraphComment>((_resolve, fail) => { reject = fail })
    const action = createAction((_input: ParagraphCommentInput) => result, { optimistic: (value: ParagraphCommentInput) => optimisticCommentAdd(key, value, 'dat') })
    const request = action.run(input)
    const pending = cache.get<{ comments: ParagraphComment[] }>(key).data!.comments
    expect(pending).toHaveLength(2)
    expect(pending[1]).toMatchObject({ ...input, author: 'dat' })
    expect(pending[1].id).toMatch(/^pending-/)
    expect(previous.comments).toHaveLength(1)
    reject(new Error('Mất kết nối'))
    expect(await request).toBeUndefined()
    expect(cache.get(key).data).toBe(previous)
    expect(action.get().error).toBeInstanceOf(Error)
    expect(action.get().busy).toBe(false)
    action.dispose()
  })
})
