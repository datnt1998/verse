// Consumer smoke: đóng gói @noverse/ui → app rỗng trong /tmp cài tarball → render
// Button, Sheet (BottomSheet), Icon + kiểm tra tokens.css/style.css tồn tại.
// Chạy: node scripts/smoke-consumer.mjs   (từ packages/ui)
import { execFileSync, execSync } from 'node:child_process'
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

const PKG = fileURLToPath(new URL('..', import.meta.url))
const NM = join(PKG, '..', '..', 'node_modules') // web/node_modules — nguồn dependency offline

const log = (msg) => console.log(`[smoke] ${msg}`)
const fail = (msg) => { console.error(`[smoke] FAIL ${msg}`); process.exit(1) }

// 1. Pack tarball
execSync('npm run build --silent', { cwd: PKG, stdio: 'inherit' })
const packOut = execSync('npm pack --silent', { cwd: PKG, encoding: 'utf8' }).trim().split('\n').pop()
const tarball = join(PKG, packOut)
log(`tarball ${packOut}`)
for (const f of ['dist/index.js', 'dist/index.d.ts', 'dist/tokens.stylex.js', 'dist/tokens.stylex.d.ts', 'dist/style.css', 'tokens.css', 'tokens.json', 'src/kit.css', 'src/Toast.css', 'src/icons']) {
  const listing = execSync(`tar -tzf ${tarball}`, { encoding: 'utf8' })
  if (!listing.split('\n').some((l) => l.replace(/^package\//, '') === f || l.replace(/^package\//, '').startsWith(`${f}/`))) {
    fail(`thiếu ${f} trong tarball`)
  }
}

// 2. App consumer trong /tmp — dependency nội bộ qua file:// (không cần mạng)
const app = mkdtempSync(join(tmpdir(), 'noverse-ui-smoke-'))
log(`consumer ${app}`)
writeFileSync(join(app, 'package.json'), JSON.stringify({
  name: 'smoke-consumer', type: 'module', private: true,
  dependencies: {
    '@noverse/ui': `file:${tarball}`,
    'react': `file:${join(NM, 'react')}`,
    'react-dom': `file:${join(NM, 'react-dom')}`,
    '@stylexjs/stylex': `file:${join(NM, '@stylexjs', 'stylex')}`,
    'qrcode-generator': `file:${join(NM, 'qrcode-generator')}`,
    'happy-dom': `file:${join(NM, 'happy-dom')}`,
  },
}, null, 2))
execFileSync('npm', ['install', '--no-audit', '--no-fund', '--silent'], { cwd: app, stdio: 'inherit' })

// 3. Main render client-side trong happy-dom (Sheet/Dialog/ConfirmDialog cần DOM + portal)
execFileSync('cp', [join(PKG, 'scripts', 'smoke-main.mjs'), join(app, 'main.mjs')])
execFileSync('node', [join(app, 'main.mjs')], { cwd: app, stdio: 'inherit' })

// 4. Theme dùng được: tokens.css khai báo biến, style.css (stylex biên dịch) có nội dung
const uiDist = join(app, 'node_modules', '@noverse/ui')
const tokensCss = readFileSync(join(uiDist, 'tokens.css'), 'utf8')
if (!tokensCss.includes('--color-accent')) fail('tokens.css thiếu --color-accent')
const styleCss = readFileSync(join(uiDist, 'dist', 'style.css'), 'utf8')
if (!styleCss.includes('--color-bg')) fail('dist/style.css thiếu biến StyleX')
log('PASS consumer smoke')
rmSync(app, { recursive: true, force: true })
