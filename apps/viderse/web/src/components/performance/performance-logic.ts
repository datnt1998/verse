/** Link YouTube hợp lệ: youtube.com/watch?v=…, /shorts/…, /live/…, youtu.be/…; rỗng = không có link. */
export function isYoutubeLink(raw: string): boolean {
  const value = raw.trim()
  if (!value) return true
  let url: URL
  try { url = new URL(/^[a-z]+:\/\//i.test(value) ? value : `https://${value}`) } catch { return false }
  const host = url.hostname.replace(/^(www|m|music)\./, '')
  if (host === 'youtu.be') return url.pathname.length > 1
  if (host !== 'youtube.com') return false
  if (url.pathname === '/watch') return !!url.searchParams.get('v')
  return /^\/(shorts|live|embed)\/[^/]+/.test(url.pathname)
}
