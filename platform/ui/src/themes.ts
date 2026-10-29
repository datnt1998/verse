// Slot theme có cấu trúc: `dark` là theme hiện tại — tất cả biến trong tokens.stylex.ts.
// Theme khác đăng ký bằng object { --tên-biến: giá trị } cùng tập khoá; applyTheme() gắn
// data-theme lên :root rồi CSS layer [data-theme="<tên>"] ghi đè (slot mở, chưa khai theme nào khác).
export type ThemeName = 'dark'

export type ThemeVars = Record<`--${string}`, string>

/** Registry: theme mới đi vào đây; giá trị phải đủ khoá của theme dark. */
export const themes: Record<ThemeName, { label: string; vars?: ThemeVars }> = {
  dark: { label: 'Dark' },
}

export const themeNames = Object.keys(themes) as ThemeName[]

/** Gắn theme hiện hành lên <html data-theme>; dark là mặc định (không thuộc tính). */
export function applyTheme(name: ThemeName): void {
  if (name === 'dark') document.documentElement.removeAttribute('data-theme')
  else document.documentElement.setAttribute('data-theme', name)
}
