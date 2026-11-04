# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: settings-state-audit.pw.ts >> settings frame: genuine section values, desktop/mobile navigation and read-only agents/accounts
- Location: tests/viderse-workflow/settings-state-audit.pw.ts:199:5

# Error details

```
Error: iconX must form one sidebar column

expect(received).toBeLessThanOrEqual(expected)

Expected: <= 2
Received:    37.53125
```

# Page snapshot

```yaml
- generic [ref=e2]:
  - generic [ref=e3]:
    - banner [ref=e4]:
      - generic [ref=e5]:
        - link "Viderse" [ref=e6] [cursor=pointer]:
          - /url: "#/"
        - generic [ref=e15]: Cài đặt · máy này
      - navigation "Điều hướng chính" [ref=e16]:
        - link "Bàn dựng" [ref=e17] [cursor=pointer]:
          - /url: "#/"
        - link "Duyệt 2" [ref=e19] [cursor=pointer]:
          - /url: "#/review"
          - text: Duyệt
          - generic "2" [ref=e21]
        - link "Kênh" [ref=e22] [cursor=pointer]:
          - /url: "#/channels/c76cc725-0e9d-43fd-98d1-4d55412510e8"
        - link "Khám phá" [ref=e24] [cursor=pointer]:
          - /url: "#/explore"
        - link "Video mới" [ref=e26] [cursor=pointer]:
          - /url: "#/new"
      - generic [ref=e28]:
        - 'link "Việc nền: Máy rảnh" [ref=e30] [cursor=pointer]':
          - /url: "#/queue"
          - generic [ref=e32]: Máy rảnh
          - img "có thông báo cần bạn" [ref=e33]
        - link "Cài đặt · 3 việc cần xem" [ref=e34] [cursor=pointer]:
          - /url: "#/settings?section=connect"
        - button "Tài khoản của bạn" [ref=e38] [cursor=pointer]: W
    - generic [ref=e40]:
      - complementary "Cài đặt" [ref=e41]:
        - generic [ref=e42]: Máy này
        - button "Công cụ trên máy Cài đặt · 1 việc cần xem" [ref=e43] [cursor=pointer]:
          - generic [ref=e45]:
            - generic [ref=e47]: Công cụ trên máy
            - generic "Cài đặt · 1 việc cần xem" [ref=e48]
        - button "Kết nối AI Cài đặt · 1 việc cần xem" [ref=e49] [cursor=pointer]:
          - generic [ref=e51]:
            - generic [ref=e53]: Kết nối AI
            - generic "Cài đặt · 1 việc cần xem" [ref=e54]
        - button "Trợ lý AI" [ref=e55] [cursor=pointer]
        - button "Tài khoản nền tảng" [ref=e60] [cursor=pointer]
        - button "Thông báo" [ref=e65] [cursor=pointer]
        - generic [ref=e70]: Dữ liệu
        - button "Lưu trữ 2,2 GB" [ref=e71] [cursor=pointer]:
          - generic [ref=e73]:
            - generic [ref=e75]: Lưu trữ
            - generic [ref=e76]: 2,2 GB
        - button "Báo lỗi 26" [ref=e77] [cursor=pointer]:
          - generic [ref=e79]:
            - generic [ref=e81]: Báo lỗi
            - generic [ref=e82]: "26"
        - generic [ref=e83]: Ở trang khác
        - link "Luồng chạy · hàng chờ" [ref=e84] [cursor=pointer]:
          - /url: "#/queue"
        - generic [ref=e85]:
          - generic [ref=e86]: Mọi cài đặt ở đây áp dụng cho cả máy, mọi kênh dùng chung.
          - generic [ref=e87]: Viderse 0.0.0
      - main [ref=e88]:
        - generic [ref=e89]:
          - generic [ref=e90]:
            - heading "Công cụ trên máy" [level=1] [ref=e91]
            - paragraph [ref=e92]: Viderse tự tải, cài và cập nhật các công cụ này. Mọi việc xử lý video đều chạy trên máy của bạn.
          - status [ref=e93]:
            - generic [ref=e95]:
              - generic [ref=e96]: Máy sẵn sàng · có 1 bản cập nhật
              - generic [ref=e97]: Kiểm tra lúc 11:46. Nên cập nhật nếu tải video bị lỗi.
            - button "Kiểm tra lại" [ref=e98] [cursor=pointer]
          - list "Công cụ trên máy" [ref=e102]:
            - listitem [ref=e103]:
              - generic [ref=e106]:
                - generic [ref=e107]: Tải video và phụ đề
                - generic [ref=e109]: Có bản 2026.10.26 · bạn đang dùng 2026.08.19. YouTube hay đổi cách tải, nên cập nhật.
              - generic [ref=e110]:
                - generic [ref=e111]: Có bản mới
                - button "Cập nhật" [ref=e112] [cursor=pointer]
            - listitem [ref=e115]:
              - generic [ref=e118]:
                - generic [ref=e119]:
                  - generic [ref=e120]: Xử lý hình và tiếng
                  - generic [ref=e121]: 91,1 MB
                - generic [ref=e122]: Cắt cảnh, xử lý âm thanh và dựng video · bản 6.0
              - generic [ref=e123]: Đã cài
            - listitem [ref=e125]:
              - generic [ref=e128]:
                - generic [ref=e129]:
                  - generic [ref=e130]: Tách tiếng và tạo phụ đề
                  - generic [ref=e131]: 1,2 GB
                - generic [ref=e132]: Chép lời thoại thành chữ ngay trên máy · mô hình giọng nói đã tải
              - generic [ref=e133]: Đã cài
            - listitem [ref=e135]:
              - generic [ref=e138]:
                - generic [ref=e139]:
                  - generic [ref=e140]: Dựng và kiểm tra video
                  - generic [ref=e141]: 378,8 MB
                - generic [ref=e142]: Dùng Chrome for Testing chạy ngầm để xuất video và đo âm thanh · bản 155.0.8059.79
              - generic [ref=e143]: Đã cài
            - listitem [ref=e145]:
              - generic [ref=e148]:
                - generic [ref=e149]:
                  - generic [ref=e150]: Công cụ vẽ ảnh
                  - generic [ref=e151]: Có sẵn
                - generic [ref=e152]: Vẽ ảnh cho từng cảnh · chạy tách riêng, không đụng dữ liệu khác · bản 0.162.1
              - generic [ref=e153]: Đã cài
            - listitem [ref=e155]:
              - generic [ref=e158]:
                - generic [ref=e159]: Đăng nhập ChatGPT
                - generic [ref=e161]: Ảnh được vẽ bằng tài khoản ChatGPT của bạn, nên bước này cần bạn tự đăng nhập.
              - generic [ref=e162]: Cần bạn
            - group [ref=e164]:
              - generic "Nâng cao · thư mục dữ liệu và phiên bản" [ref=e165] [cursor=pointer]
      - complementary "Cài đặt" [ref=e168]:
        - generic [ref=e169]:
          - heading "Khi nào cần làm gì" [level=2] [ref=e170]
          - paragraph [ref=e171]: Tải video lỗi hàng loạt thường do YouTube vừa đổi cách tải. Bấm Cập nhật ở Tải video và phụ đề là đủ.
          - paragraph [ref=e172]: Thiếu công cụ khi đang làm video, app chỉ cách sửa rồi quay về đúng video đó.
        - generic [ref=e173]:
          - generic [ref=e174]: Mẹo
          - generic [ref=e175]:
            - generic [aria-hidden] [ref=e176]: •
            - generic [ref=e177]: App tự kiểm tra mỗi lần mở.
          - generic [ref=e178]:
            - generic [aria-hidden] [ref=e179]: •
            - generic [ref=e180]: Chuyển sang máy khác thì không cần chép công cụ, app tự cài lại.
  - region "Thông báo"
```

# Test source

```ts
  180 | test.afterEach(async ({ page }, testInfo) => {
  181 |   // Truthful evidence provenance only: no cookies, credentials or request
  182 |   // headers. Missing env values stay null instead of being inferred.
  183 |   await testInfo.attach('settings-evidence-metadata.json', {
  184 |     body: JSON.stringify({
  185 |       project: testInfo.project.name,
  186 |       viewport: page.viewportSize(),
  187 |       baseURL: testInfo.project.use.baseURL ?? process.env.WORKFLOW_APP_URL ?? null,
  188 |       sourceRevision: process.env.WORKFLOW_SOURCE_REVISION ?? null,
  189 |     }, null, 2),
  190 |     contentType: 'application/json',
  191 |   })
  192 |   await testInfo.attach('settings-controls.json', { body: JSON.stringify(observations.get(page) ?? [], null, 2), contentType: 'application/json' })
  193 |   await testInfo.attach('settings-mutations.json', { body: JSON.stringify(mutations.get(page) ?? [], null, 2), contentType: 'application/json' })
  194 |   const faults = runtimeFaults.get(page) ?? []
  195 |   await testInfo.attach('runtime-errors.json', { body: JSON.stringify(faults), contentType: 'application/json' })
  196 |   expect(faults).toEqual([])
  197 | })
  198 | 
  199 | test('settings frame: genuine section values, desktop/mobile navigation and read-only agents/accounts', async ({ page }, testInfo) => {
  200 |   const mobile = isMobile(testInfo)
  201 |   const [providers, platforms, agents, tools] = await Promise.all([
  202 |     getJson<Provider[]>(page, '/api/ai/providers'),
  203 |     getJson<{ id: string; label: string; state: string; login: { saved: boolean; refreshError?: unknown }; usage: unknown }[]>(page, '/api/platforms'),
  204 |     getJson<{ artist: { draw: { provider: string; model: string } } }>(page, '/api/ai/agents'),
  205 |     getJson<ToolsStatus>(page, '/api/tools'),
  206 |   ])
  207 |   const expectSectionContent = async (section: string) => {
  208 |     if (section === 'connect') await expect(page.locator('.conn-card')).toHaveCount(providers.length)
  209 |     if (section === 'tools') await expect(page.locator('.vv-tools__row')).toHaveCount(tools.tools.length)
  210 |     if (section === 'agents') await expect(page.getByText('Đạo diễn', { exact: true }).first()).toBeVisible()
  211 |     if (section === 'accounts') for (const platform of platforms) await expect(page.getByRole('region', { name: platform.label })).toBeVisible()
  212 |     if (section === 'storage') await expect(page.locator('.vv-sto__card').first()).toBeVisible()
  213 |     if (section === 'logs') await expect(page.locator('.vv-logs__export')).toBeVisible()
  214 |   }
  215 | 
  216 |   await boot(page, '/#/settings?section=tools')
  217 |   await expectTitle(page, mobile, 'Công cụ trên máy')
  218 |   await expectSectionContent('tools')
  219 |   await capture(page, testInfo, 'settings-frame-tools-entry')
  220 |   record(page, testInfo, 'frame.tools', 'Native tools section opens with its desktop/mobile title', (await page.locator(mobile ? '.vv-mset__title' : '.vv-set__title').textContent()) ?? '')
  221 | 
  222 |   if (mobile) {
  223 |     await page.getByRole('button', { name: 'Quay lại' }).click()
  224 |     await expect(page).toHaveURL(/#\/settings$/)
  225 |     const list = page.locator('.vv-mset__list')
  226 |     await expect(list).toBeVisible()
  227 |     // Values in the mobile list come from the same live resources, not static copy.
  228 |     const connectRow = list.locator('a[href="#/settings?section=connect"]')
  229 |     await expect(connectRow).toContainText(`${providers.length} nhà cung cấp`)
  230 |     const accountsRow = list.locator('a[href="#/settings?section=accounts"]')
  231 |     await expect(accountsRow).toContainText(`${platforms.length} nền tảng`)
  232 |     const artistProvider = providers.find((provider) => provider.id === agents.artist.draw.provider)
  233 |     const agentsRow = list.locator('a[href="#/settings?section=agents"]')
  234 |     if (agents.artist.draw.provider) await expect(agentsRow).toContainText(`Vẽ bằng ${artistProvider?.label ?? agents.artist.draw.provider}`)
  235 |     else await expect(agentsRow).toContainText('Chưa chọn nguồn vẽ')
  236 |     const updateCount = tools.tools.filter((tool) => tool.state === 'update').length
  237 |     const installingCount = tools.tools.filter((tool) => tool.state === 'installing').length
  238 |     const toolsRow = list.locator('a[href="#/settings?section=tools"]')
  239 |     if (installingCount > 0) await expect(toolsRow).toContainText(`Đang cài ${installingCount} công cụ`)
  240 |     else if (updateCount > 0) await expect(toolsRow).toContainText(`${updateCount} bản cập nhật`)
  241 |     else await expect(toolsRow).toContainText('Mọi công cụ đã cài')
  242 |     // MSettings has no logs child screen row: "Báo lỗi" is the native export
  243 |     // action (a download), so it is asserted but never activated.
  244 |     await expect(list.getByText('Xuất log', { exact: true })).toBeVisible()
  245 |     await capture(page, testInfo, 'settings-frame-mobile-list')
  246 |     record(page, testInfo, 'frame.mobile-list', 'List values match live providers/platforms/agents/tools resources', `${providers.length} providers, ${platforms.length} platforms`)
  247 | 
  248 |     for (const section of ['connect', 'agents', 'accounts', 'tools', 'storage'] as const) {
  249 |       if (section !== 'connect') {
  250 |         await page.getByRole('button', { name: 'Quay lại' }).click()
  251 |         await expect(page).toHaveURL(/#\/settings$/)
  252 |       }
  253 |       const row = page.locator(`a[href="#/settings?section=${section}"]`)
  254 |       await row.click()
  255 |       await expect(page).toHaveURL(new RegExp(`section=${section}`))
  256 |       await expectTitle(page, true, SECTION_TITLE[section])
  257 |       await expectSectionContent(section)
  258 |       await capture(page, testInfo, `settings-frame-${section}`)
  259 |       record(page, testInfo, `frame.${section}`, `Mobile list row opens ${SECTION_TITLE[section]}`, (await page.locator('.vv-mset__title').textContent()) ?? '')
  260 |     }
  261 |     // The router still accepts the logs child screen; MSettings simply has no
  262 |     // row for it. Exercise the deep link without pretending a missing row exists.
  263 |     await boot(page, '/#/settings?section=logs')
  264 |     await expectTitle(page, true, 'Báo lỗi')
  265 |     await expect(page.locator('.vv-logs__export')).toBeVisible()
  266 |     await capture(page, testInfo, 'settings-frame-logs-deeplink')
  267 |     record(page, testInfo, 'frame.logs', 'Mobile logs deep link renders; list row is the native export action, not a section row', 'logs screen + export card visible')
  268 |   } else {
  269 |     const nav = page.locator('aside.vv-set__side')
  270 |     await expect(nav).toBeVisible()
  271 |     const navigation = await nav.locator('button').evaluateAll(buttons => buttons.map(button => {
  272 |       const row = button.getBoundingClientRect()
  273 |       const icon = button.querySelector('.vv-ic')!.getBoundingClientRect()
  274 |       const label = button.querySelector('.vv-set__nav-label')!.getBoundingClientRect()
  275 |       const count = button.querySelector('.vv-set__nav-num')!.getBoundingClientRect()
  276 |       return { name: button.textContent?.trim(), height: row.height, iconX: icon.x, labelX: label.x, countRight: count.right }
  277 |     }))
  278 |     for (const edge of ['iconX', 'labelX', 'countRight'] as const) {
  279 |       const positions = navigation.map(row => row[edge])
> 280 |       expect(Math.max(...positions) - Math.min(...positions), `${edge} must form one sidebar column`).toBeLessThanOrEqual(2)
      |                                                                                                       ^ Error: iconX must form one sidebar column
  281 |     }
  282 |     for (const row of navigation) expect(Math.abs(row.height - 34), `${row.name} navigation row height`).toBeLessThanOrEqual(1)
  283 |     const queueLink = nav.getByRole('link', { name: 'Luồng chạy · hàng chờ', exact: true })
  284 |     await expect(queueLink).toHaveAttribute('href', '#/queue')
  285 |     const queueGeometry = await queueLink.evaluate(link => {
  286 |       const box = link.getBoundingClientRect()
  287 |       const icon = link.firstElementChild!.getBoundingClientRect()
  288 |       const label = link.children[1]?.getBoundingClientRect()
  289 |       return { height: box.height, display: getComputedStyle(link).display, iconX: icon.x, labelX: label?.x }
  290 |     })
  291 |     expect(queueGeometry.display).toBe('flex')
  292 |     expect(Math.abs(queueGeometry.height - 34)).toBeLessThanOrEqual(1)
  293 |     expect(Math.abs(queueGeometry.iconX - navigation[0].iconX)).toBeLessThanOrEqual(2)
  294 |     expect(Math.abs(queueGeometry.labelX! - navigation[0].labelX)).toBeLessThanOrEqual(2)
  295 |     await testInfo.attach('settings-sidebar-geometry.json', { body: JSON.stringify({ navigation, queue: queueGeometry }), contentType: 'application/json' })
  296 |     for (const section of ['connect', 'agents', 'accounts', 'tools', 'storage', 'logs'] as const) {
  297 |       const label = SECTION_TITLE[section]
  298 |       await nav.getByRole('button', { name: new RegExp(`^${label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}`) }).click()
  299 |       await expect(page).toHaveURL(new RegExp(`section=${section}`))
  300 |       await expectTitle(page, false, label)
  301 |       await expectSectionContent(section)
  302 |       await capture(page, testInfo, `settings-frame-${section}`)
  303 |       record(page, testInfo, `frame.${section}`, `Desktop nav opens ${label}`, (await page.locator('.vv-set__title').textContent()) ?? '')
  304 |     }
  305 |     await queueLink.click()
  306 |     await expect(page).toHaveURL(/#\/queue$/)
  307 |     await page.goBack()
  308 |     await expect(page).toHaveURL(/section=logs/)
  309 |   }
  310 | 
  311 |   // Agents is read-only for this smoke: assert the live director/artist labels
  312 |   // and selected model ids are rendered without opening a picker (pick = PUT).
  313 |   if (mobile) {
  314 |     await page.getByRole('button', { name: 'Quay lại' }).click()
  315 |     await page.locator('a[href="#/settings?section=agents"]').click()
  316 |     await expectTitle(page, true, 'Trợ lý AI')
  317 |   } else {
  318 |     await page.locator('aside.vv-set__side').getByRole('button', { name: /^Trợ lý AI/ }).click()
  319 |     await expectTitle(page, false, 'Trợ lý AI')
  320 |   }
  321 |   await expect(page.getByText('Đạo diễn', { exact: true }).first()).toBeVisible()
  322 |   await expect(page.getByText(/^Hoạ sĩ/).first()).toBeVisible()
  323 |   await expect(page.getByText('Trinh sát', { exact: true }).first()).toBeVisible()
  324 |   await expect(page.getByText('Kiểm định', { exact: true }).first()).toBeVisible()
  325 |   const drawProvider = providers.find((provider) => provider.id === agents.artist.draw.provider)
  326 |   if (drawProvider?.quota) {
  327 |     const quota = drawProvider.quota
  328 |     const group = page.getByRole('radiogroup', { name: mobile ? 'Hoạ sĩ vẽ ảnh bằng' : 'Vẽ ảnh bằng' })
  329 |     await expect(group).toContainText(`${quota.used}/${quota.limit}`)
  330 |     await expect(group).toContainText(quota.period === 'week' ? 'tuần này' : 'hôm nay')
  331 |     if (!mobile) {
  332 |       const bar = group.locator('.agt-drawcard__bar').first()
  333 |       const barBox = await bar.boundingBox()
  334 |       const fillBox = await bar.locator('span').first().boundingBox()
  335 |       expect(fillBox!.width).toBeLessThanOrEqual(barBox!.width + 1)
  336 |     }
  337 |     record(page, testInfo, 'agents.quota', 'Draw-source quota mirrors provider.quota without inventing a plan', `${quota.used}/${quota.limit} ${quota.period}`)
  338 |   } else {
  339 |     testInfo.annotations.push({ type: 'state-gap', description: 'agents.quota: draw provider has no provider.quota in genuine state; agents quota row is not renderable without a seeded/logged-in subscription' })
  340 |   }
  341 |   await capture(page, testInfo, 'settings-agents-readonly')
  342 |   record(page, testInfo, 'agents.readonly', 'Agent roles render and navigation returns to the selected section without changing configuration', 'director/artist/scout/qa visible')
  343 | 
  344 |   // Accounts is a read-only render of /api/platforms. Login buttons are present
  345 |   // but deliberately not activated (QR start is a server POST).
  346 |   if (mobile) {
  347 |     await page.getByRole('button', { name: 'Quay lại' }).click()
  348 |     await page.locator('a[href="#/settings?section=accounts"]').click()
  349 |     await expectTitle(page, true, 'Tài khoản nền tảng')
  350 |   } else {
  351 |     await page.locator('aside.vv-set__side').getByRole('button', { name: /^Tài khoản nền tảng/ }).click()
  352 |     await expectTitle(page, false, 'Tài khoản nền tảng')
  353 |   }
  354 |   for (const platform of platforms) {
  355 |     const card = page.getByRole('region', { name: platform.label })
  356 |     await expect(card).toBeVisible()
  357 |     const expectedState = platform.state === 'paused'
  358 |       ? 'Tạm chặn'
  359 |       : platform.login.saved && platform.login.refreshError
  360 |         ? mobile ? 'Hết hạn' : 'Cần đăng nhập lại'
  361 |         : 'Sẵn sàng'
  362 |     await expect(card).toContainText(expectedState)
  363 |   }
  364 |   await capture(page, testInfo, 'settings-frame-accounts-readonly')
  365 |   record(page, testInfo, 'accounts.readonly', 'Platform cards render genuine status and login controls without activation', `${platforms.length} platform cards, states asserted`)
  366 |   assertNoMutations(page)
  367 | })
  368 | 
  369 | test('connect: truthful provider cards, all safe add radios, scrim/Esc and conditional model catalog', async ({ page }, testInfo) => {
  370 |   const mobile = isMobile(testInfo)
  371 |   const providers = await getJson<Provider[]>(page, '/api/ai/providers')
  372 |   const gaps: Gap[] = []
  373 |   await boot(page, '/#/settings?section=connect')
  374 |   await expectTitle(page, mobile, 'Kết nối AI')
  375 | 
  376 |   if (providers.length === 0) {
  377 |     await expect(page.getByText('Chưa có nhà cung cấp nào', { exact: false })).toBeVisible()
  378 |   } else {
  379 |     await expect(page.locator('.conn-card')).toHaveCount(providers.length)
  380 |   }
```