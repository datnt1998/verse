// @vitest-environment happy-dom
// Router Viderse: mọi route + tham số query có kiểu; hash lạ → Bàn dựng.
import { describe, expect, it } from 'vitest'
import { parseHash, routePath } from './router'

describe('parseHash', () => {
  it('#/ và hash trống → Bàn dựng', () => {
    expect(parseHash('#/').route).toEqual({ name: 'workbench', channel: undefined, video: undefined })
    expect(parseHash('').route.name).toBe('workbench')
  })
  it('#/?channel=&video= — tham số thường của Bàn dựng', () => {
    expect(parseHash('#/?channel=kenh-thuc-te&video=v123').route)
      .toEqual({ name: 'workbench', channel: 'kenh-thuc-te', video: 'v123' })
  })
  it('#/review', () => {
    expect(parseHash('#/review').route).toEqual({ name: 'review', channel: undefined })
    expect(parseHash('#/review?channel=a').route).toEqual({ name: 'review', channel: 'a' })
  })
  it('#/queue', () => {
    expect(parseHash('#/queue').route).toEqual({ name: 'queue' })
  })
  it('#/channels/<id> với tab', () => {
    expect(parseHash('#/channels/kenh-thuc-te').route)
      .toEqual({ name: 'channel', channelId: 'kenh-thuc-te', tab: undefined })
    expect(parseHash('#/channels/kenh-thuc-te?tab=templates').route)
      .toEqual({ name: 'channel', channelId: 'kenh-thuc-te', tab: 'templates' })
  })
  it('tab không hợp lệ rơi về mặc định (undefined)', () => {
    expect(parseHash('#/channels/a?tab=nonsense').route)
      .toEqual({ name: 'channel', channelId: 'a', tab: undefined })
  })
  it('#/channels/new?step=', () => {
    expect(parseHash('#/channels/new').route).toEqual({ name: 'channel-new', step: undefined })
    expect(parseHash('#/channels/new?step=done').route).toEqual({ name: 'channel-new', step: 'done' })
  })
  it('#/explore?rival=&video=', () => {
    expect(parseHash('#/explore').route).toEqual({ name: 'explore', rival: undefined, video: undefined })
    expect(parseHash('#/explore?rival=%40motphim&video=abc').route)
      .toEqual({ name: 'explore', rival: '@motphim', video: 'abc' })
  })
  it('query giải mã đúng một lần: %, escape hỏng, %25, Unicode, encode đôi', () => {
    // `%25` → dấu %; `decodeURIComponent` lần hai trên giá trị này từng ném URIError.
    expect(parseHash('#/?channel=%25').route).toEqual({ name: 'workbench', channel: '%', video: undefined })
    // Escape không đúng khuôn — URLSearchParams giữ nguyên byte, không được quăng.
    expect(() => parseHash('#/?channel=%')).not.toThrow()
    expect(parseHash('#/?channel=%').route).toEqual({ name: 'workbench', channel: '%', video: undefined })
    expect(() => parseHash('#/?channel=%zz').route).not.toThrow()
    expect(parseHash('#/?channel=%zz').route).toEqual({ name: 'workbench', channel: '%zz', video: undefined })
    // Encode đôi: `%2525` → literal `%25`, không được chạy một vòng giải mã thứ hai.
    expect(parseHash('#/?channel=%2525').route).toEqual({ name: 'workbench', channel: '%25', video: undefined })
    // Unicode giải một lần.
    expect(parseHash('#/?channel=%C4%90%E1%BA%A1o').route).toEqual({ name: 'workbench', channel: 'Đạo', video: undefined })
    // Đường dẫn tương ứng: routePath encode phần trăm thành %25.
    expect(routePath({ name: 'workbench', channel: '%' })).toBe('/?channel=%25')
  })
  it('#/new', () => {
    expect(parseHash('#/new').route).toEqual({ name: 'new' })
  })
  it('#/library?tab=&prompt=&nguon=&kenh=', () => {
    expect(parseHash('#/library').route).toEqual({ name: 'library', tab: undefined, prompt: undefined, nguon: undefined, kenh: undefined })
    expect(parseHash('#/library?tab=prompts').route).toEqual({ name: 'library', tab: 'prompts', prompt: undefined, nguon: undefined, kenh: undefined })
    expect(parseHash('#/library?tab=prompts&prompt=new').route).toEqual({ name: 'library', tab: 'prompts', prompt: 'new', nguon: undefined, kenh: undefined })
    expect(parseHash('#/library?tab=backdrops&nguon=videos').route).toEqual({ name: 'library', tab: 'backdrops', prompt: undefined, nguon: 'videos', kenh: undefined })
    expect(parseHash('#/library?tab=backdrops&kenh=twitch%3Aslowroads').route).toEqual({ name: 'library', tab: 'backdrops', prompt: undefined, nguon: undefined, kenh: 'twitch:slowroads' })
    expect(routePath({ name: 'library', tab: 'backdrops', nguon: 'channels', kenh: 'twitch:slowroads' })).toBe('/library?tab=backdrops&nguon=channels&kenh=twitch%3Aslowroads')
  })
  it('#/templates/<id>?section=', () => {
    expect(parseHash('#/templates/t1').route).toEqual({ name: 'template', templateId: 't1', section: undefined })
    expect(parseHash('#/templates/t1?section=cover').route).toEqual({ name: 'template', templateId: 't1', section: 'cover' })
  })
  it('#/editor/<projectId>', () => {
    expect(parseHash('#/editor/proj-9').route).toEqual({ name: 'editor', projectId: 'proj-9' })
  })
  it('#/settings?section=', () => {
    expect(parseHash('#/settings').route).toEqual({ name: 'settings', section: undefined })
    expect(parseHash('#/settings?section=connect').route).toEqual({ name: 'settings', section: 'connect' })
  })
  it('#/account', () => {
    expect(parseHash('#/account').route).toEqual({ name: 'account' })
  })
  it('#/dev/kit chỉ trong dev (vitest luôn DEV)', () => {
    expect(parseHash('#/dev/kit').route).toEqual({ name: 'devkit', board: undefined, section: undefined })
    expect(parseHash('#/dev/kit?board=mkit&section=sheets').route).toEqual({ name: 'devkit', board: 'mkit', section: 'sheets' })
  })
  it('hash lạ và tham số query lạ → Bàn dựng', () => {
    expect(parseHash('#/khong-co-gi/nay').route.name).toBe('workbench')
    expect(parseHash('#/settings?section=nonsense').route).toEqual({ name: 'settings', section: undefined })
  })
})

describe('routePath', () => {
  it('viết lại đường dẫn chuẩn kèm tham số', () => {
    expect(routePath({ name: 'workbench', channel: 'a', video: 'v' })).toBe('/?channel=a&video=v')
    expect(routePath({ name: 'channel', channelId: 'k x', tab: 'sources' })).toBe('/channels/k%20x?tab=sources')
    expect(routePath({ name: 'template', templateId: 't1', section: 'cover' })).toBe('/templates/t1?section=cover')
    expect(routePath({ name: 'devkit', board: 'mkit', section: 'bars' })).toBe('/dev/kit?board=mkit&section=bars')
    expect(routePath({ name: 'workbench' })).toBe('/')
  })
  it('parse ↔ path khép vòng', () => {
    for (const hash of ['#/', '#/review', '#/queue', '#/channels/a?tab=templates', '#/explore?video=v', '#/editor/p1']) {
      const { route } = parseHash(hash)
      expect(parseHash(`#${routePath(route)}`).route).toEqual(route)
    }
  })
})
