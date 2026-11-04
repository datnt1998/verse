import { test, expect } from '@playwright/test'
import { readFileSync, realpathSync } from 'node:fs'
import { resolve, sep } from 'node:path'

const cookies = process.env.WORKFLOW_QA_COOKIE_FILE
const fixturePath = process.env.WORKFLOW_QA_FIXTURE
if (!cookies || !fixturePath || realpathSync(cookies).startsWith(resolve(__dirname, '../..') + sep)) throw new Error('Private QA session required')
const fixture = JSON.parse(readFileSync(fixturePath, 'utf8')) as { username: string; jobs: { id: string; channelId: string; sourceTitle: string }[] }
if (fixture.username !== 'workflow.qa') throw new Error('Only isolated workflow.qa may delete fixture rows')

test('confirmed deletion removes the actual job and its channel row without POST delete', async ({ page, context }, info) => {
  await context.addCookies(JSON.parse(readFileSync(cookies, 'utf8')))
  const me = await page.request.get('/api/me')
  expect((await me.json()).user.username).toBe('workflow.qa')
  const job = fixture.jobs[info.project.name.endsWith('reduced') ? 1 : 2]
  const before = await page.request.get(`/api/clone/${job.id}`)
  expect(before.status()).toBe(200)
  const paused = (await before.json()).stageGroup === 'paused'
  if (paused) {
    expect((await page.request.delete(`/api/clone/${job.id}`)).status()).toBe(409)
    expect((await page.request.get(`/api/clone/${job.id}`)).status()).toBe(200)
  }
  const writes: { method: string; path: string; body: string | null }[] = []
  page.on('request', request => {
    if (request.method() !== 'GET' && new URL(request.url()).pathname.includes(`/api/clone/${job.id}`)) writes.push({ method: request.method(), path: new URL(request.url()).pathname, body: request.postData() })
  })
  await page.goto(`/#/channels/${job.channelId}?tab=videos`, { waitUntil: 'domcontentloaded' })
  const trigger = page.getByRole('button', { name: `Thêm thao tác cho ${job.sourceTitle}`, exact: true })
  await trigger.click()
  if (paused) {
    await page.getByRole('menuitem', { name: 'Dừng hẳn', exact: true }).click()
    const stop = page.getByRole('alertdialog')
    await expect(stop).toBeVisible()
    const stopped = page.waitForResponse(response => response.request().method() === 'POST' && new URL(response.url()).pathname === `/api/clone/${job.id}`)
    await stop.getByRole('button', { name: 'Dừng hẳn', exact: true }).click()
    expect((await stopped).ok()).toBe(true)
    await expect(stop).toHaveCount(0)
    await trigger.click()
  }
  await page.getByRole('menuitem', { name: 'Xoá video', exact: true }).click()
  const confirmation = page.getByRole('alertdialog')
  await expect(confirmation).toBeVisible()
  await expect(confirmation).toContainText(job.sourceTitle)
  expect(await page.locator('[role="dialog"], [role="alertdialog"]').count()).toBe(1)
  await page.screenshot({ path: info.outputPath('delete-consequences.png') })
  const deleted = page.waitForResponse(response => response.request().method() === 'DELETE' && new URL(response.url()).pathname === `/api/clone/${job.id}`)
  await confirmation.getByRole('button', { name: 'Xoá video', exact: true }).click()
  const response = await deleted
  expect(response.ok()).toBe(true)
  await expect(confirmation).toHaveCount(0)
  await expect(trigger).toHaveCount(0)
  expect((await page.request.get(`/api/clone/${job.id}`)).status()).toBe(404)
  expect(writes.filter(write => write.method === 'POST' && write.body?.includes('delete'))).toEqual([])
  expect(writes.filter(write => write.method === 'DELETE')).toHaveLength(1)
  await page.screenshot({ path: info.outputPath('deleted-readback.png') })
  await info.attach('delete-readback.json', { body: JSON.stringify({ jobId: job.id, response: response.status(), after: 404, writes, viewport: page.viewportSize() }, null, 2), contentType: 'application/json' })
})
