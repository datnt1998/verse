// Bảng vai trò màu ↔ literal Noverse — nguồn cho lớp "semantic roles" của defineBrand.
// Một vai trò có thể sở hữu nhiều dạng literal cùng một màu (#HEX và bộ ba "r,g,b"
// trong rgba()/biến aura). gen-tokens.mjs nhúng bảng này vào src/tokenDefaults.ts.

/** role → [literal,...]. Literal dạng '#RRGGBB' hoặc 'r,g,b' (dùng trong rgba() và giá trị aura). */
export const ROLE_LITERALS = {
  // Accent (tím Noverse)
  accent: ['#C9A6F5', '201,166,245'],
  accentSoft: ['#D9CCF7'],
  accentPale: ['#DCC8F8'],
  accentPale2: ['#F3E8FF'],
  accentPale3: ['#E2D0FF'],
  accentPale4: ['#E6D6FA'],
  accentBar: ['#E4D6FA'],
  accentDeep: ['#9A63E3', '154,99,227'],
  accentPink: ['#C9457F'],
  indigo: ['#5A57E3'],
  glow: ['#9C7CF0'],
  // Trạng thái ok/xanh
  ok: ['#A6EDC1', '166,237,193'],
  okText: ['#BDF2D2'],
  okText2: ['#CDEFD9'],
  okText3: ['#C9F5DA'],
  okMint: ['#BDF0CF'],
  // Cảnh báo/vàng
  warn: ['#F6CB8C', '246,203,140'],
  warnText: ['#F3D9AE'],
  warnBody: ['#E9D9BD'],
  warnMark: ['#FBE3BD'],
  warnCream: ['#F3E2C4'],
  warnPickBg: ['#2A2114'],
  warnBadgeBg: ['#3A2E1A'],
  warnDark: ['#1A1408'],
  // Lỗi/đỏ
  err: ['#FF9AA2', '255,154,162'],
  errSoft: ['#FFB3BA'],
  errText: ['#FFC9CE'],
  errText2: ['#FFC2C8'],
  errBright: ['#FF9FA8', '255,159,168'],
  errStrong: ['#FF7A84'],
  errHot: ['#FF5F6E', '255,95,110'],
  danger: ['#E5484D', '229,72,77'],
  dangerHi: ['#EC5D62'],
  // Thông tin/xanh dương
  info: ['#B9D0F7', '185,208,247'],
  infoSoft: ['#C8D8F8'],
  infoBadge: ['#C8D3F8'],
  infoDeep: ['#9DB0F7', '157,176,247'],
  infoStrong: ['#7C93F3', '124,147,243'],
  // Người dùng/lục lam
  user: ['#5DC1B0', '93,193,176'],
  userFg: ['#9FE3D6'],
  userSoft: ['#8FDCCB', '143,220,203'],
  // Hồng (chip "mục luật của bộ")
  pink: ['#FFB3CB', '255,179,203'],
  pinkFg: ['#FFC4D6'],
  // Lẻ
  admin: ['#B89A6E', '184,154,110'],
  adminFg: ['#E3C99E'],
  lockNote: ['#C9A9AD'],
  roundStar: ['#E8C36A'],
  dotBack: ['#9696FF'],
  dotFront: ['#F0DEFF'],
  // Nắp gradient orb/thanh nav — chỉ xuất hiện bên trong giá trị phức
  orbLight1: ['#EEF4FF'],
  orbLight2: ['#FFF1DC'],
  orbLight3: ['#E9FBEF'],
  editDeep: ['#EC9548', '236,149,72'],
  editFlame: ['#E25E36'],
  arbBlue: ['#3E73D6'],
  navTile: ['#17141F'],
  navA: ['#2C2140'],
  navB: ['#1A1726'],
  leadBg: ['#2A2336'],
  antaBg: ['#302225'],
  allyBg: ['#1F2C27'],
  edgeWork: ['#8C9BC0'],
}

/** Token KHÔNG đổi theo brand: biểu tượng Noverse, avatar nhân vật 6 cặp, ô báo icon thiếu. */
export const FIXED_TOKENS = new Set([
  '--brand-architect-1', '--brand-architect-2', '--brand-architect-3', '--brand-architect-4',
  '--brand-writer-1', '--brand-writer-2', '--brand-writer-3', '--brand-writer-4',
  '--brand-editor-1', '--brand-editor-2', '--brand-editor-3', '--brand-editor-4',
  '--brand-arbiter-1', '--brand-arbiter-2', '--brand-arbiter-3', '--brand-arbiter-4',
  '--brand-rim', '--brand-shadow',
  '--brain-av-1a', '--brain-av-1b', '--brain-av-2a', '--brain-av-2b', '--brain-av-3a', '--brain-av-3b',
  '--brain-av-4a', '--brain-av-4b', '--brain-av-5a', '--brain-av-5b', '--brain-av-6a', '--brain-av-6b',
  '--kit-dev-miss', '--kit-dev-miss-bg',
])
