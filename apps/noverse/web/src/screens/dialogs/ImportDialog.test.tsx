import { renderToStaticMarkup } from 'react-dom/server'
import type { ReactNode } from 'react'
import { expect, it, vi } from 'vitest'

vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()),
  Dialog: ({ children, footer }: { children: ReactNode; footer?: ReactNode }) => <div>{children}<footer>{footer}</footer></div>,
}))
vi.mock('../../data', () => ({
  keys: { story: (...parts: unknown[]) => ['story', ...parts], jobs: () => ['jobs'], project: (...parts: unknown[]) => ['project', ...parts] },
  subscribeEvents: () => () => {},
  useConnection: () => ({ writable: true }),
  useAction: () => ({ busy: false, pending: false, success: false, error: null, run: vi.fn() }),
  useResource: () => ({ data: { job: null }, status: 'ready', error: null, refreshing: false, updatedAt: null, refresh: vi.fn() }),
}))
import { ImportDialog } from './ImportDialog'

it('E2EWORK-11: nút “Bắt đầu nhập” bị khoá nói rõ lý do khi chưa có tệp nguồn', () => {
  const html = renderToStaticMarkup(<ImportDialog project="p" story="s" onClose={() => {}} />)
  const button = html.match(/<button[^>]*>(?:(?!<\/button>)[\s\S])*Bắt đầu nhập[\s\S]*?<\/button>/)?.[0] ?? ''
  expect(button).toContain('disabled=""')
  expect(button).toContain('aria-describedby="imp-start-reason"')
  expect(html).toMatch(/id="imp-start-reason"[^>]*>Nhập đường dẫn tệp nguồn để bắt đầu nhập\.</)
})
