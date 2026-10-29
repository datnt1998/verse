import os, sys
sys.path.insert(0, os.path.dirname(__file__))
from mark import mark_group, W, H

OUT = sys.argv[1] if len(sys.argv) > 1 else os.path.join(os.path.dirname(os.path.abspath(__file__)), '..')
os.makedirs(OUT + '/svg', exist_ok=True)


def svg(w, h, inner, title='Noverse'):
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {w} {h}" width="{w}" height="{h}" role="img" aria-label="{title}">'
            f'<title>{title}</title>{inner}</svg>\n')


def save(name, s):
    open(f'{OUT}/svg/{name}', 'w').write(s)


# 1) Biểu tượng rời, nền trong suốt. Chừa lề cho bóng đổ.
P = 24
for kind, name in [('color', 'noverse-mark.svg'), ('flat', 'noverse-mark-flat.svg')]:
    d, g = mark_group('m', P, P, H, kind)
    save(name, svg(W + 2 * P, H + 2 * P + 10, f'<defs>{d}</defs>{g}'))
# Bản có quầng sáng, chỉ dùng trên nền tối (màn đăng nhập, ảnh bìa)
G = 70
d, g = mark_group('m', G, G, H, 'glow')
save('noverse-mark-glow.svg', svg(W + 2 * G, H + 2 * G, f'<defs>{d}</defs>{g}'))
for col, name in [('#0B0B0D', 'noverse-mark-mono-black.svg'), ('#FFFFFF', 'noverse-mark-mono-white.svg')]:
    d, g = mark_group('m', 0, 0, H, 'mono', col)
    save(name, svg(W, H, f'<defs>{d}</defs>{g}'))

# 2) App icon 1024. Bo góc 22.5% (gần squircle iOS). Mark rộng 70% ô.
S = 1024
mw = S * 0.72
mh = mw * H / W
mx, my = (S - mw) / 2, (S - mh) / 2 - 8


def appicon(bg, rim, pre, rounded=True, kind='color'):
    d, g = mark_group(pre, mx, my, mh, 'glow' if bg == 'dark' else kind)
    rr = S * 0.225 if rounded else 0
    bgdef = ''
    if bg == 'dark':
        bgdef = (f'<linearGradient id="{pre}bg" x1="0" y1="0" x2="0" y2="1">'
                 '<stop offset="0" stop-color="#1D1A2E"/><stop offset="1" stop-color="#0B0B0D"/></linearGradient>'
                 f'<radialGradient id="{pre}glow" cx=".5" cy=".46" r=".5">'
                 '<stop offset="0" stop-color="#9A63E3" stop-opacity=".30"/><stop offset="1" stop-color="#9A63E3" stop-opacity="0"/></radialGradient>')
        fill = f'url(#{pre}bg)'
    else:
        bgdef = (f'<linearGradient id="{pre}bg" x1="0" y1="0" x2="0" y2="1">'
                 '<stop offset="0" stop-color="#FFFFFF"/><stop offset="1" stop-color="#F1F0F5"/></linearGradient>')
        fill = f'url(#{pre}bg)'
    shape = (f'<rect width="{S}" height="{S}" rx="{rr}" fill="{fill}"/>' if rounded
             else f'<rect width="{S}" height="{S}" fill="{fill}"/>')
    glow = f'<rect width="{S}" height="{S}" rx="{rr}" fill="url(#{pre}glow)"/>' if bg == 'dark' else ''
    edge = (f'<rect x="1.5" y="1.5" width="{S-3}" height="{S-3}" rx="{rr-1.5}" fill="none" stroke="{rim}" stroke-width="3"/>'
            if rounded else '')
    return svg(S, S, f'<defs>{bgdef}{d}</defs>{shape}{glow}{g}{edge}')


save('noverse-appicon-dark.svg', appicon('dark', 'rgba(255,255,255,.08)', 'a'))
save('noverse-appicon-light.svg', appicon('light', 'rgba(11,11,13,.06)', 'a'))
# Ô vuông không bo: để iOS / Android tự cắt (apple-touch-icon, maskable)
save('noverse-appicon-dark-square.svg', appicon('dark', '', 'a', rounded=False))

# 3) Bản tròn ("Icon only" trong bộ thương hiệu): avatar, ảnh đại diện mạng xã hội
def round_icon():
    d, g = mark_group('r', mx, my, mh, 'glow')
    bg = ('<radialGradient id="rbg" cx=".5" cy=".42" r=".6"><stop offset="0" stop-color="#25203A"/>'
          '<stop offset="1" stop-color="#0B0B0D"/></radialGradient>')
    return svg(S, S, f'<defs>{bg}{d}</defs><circle cx="{S/2}" cy="{S/2}" r="{S/2}" fill="url(#rbg)"/>{g}'
               f'<circle cx="{S/2}" cy="{S/2}" r="{S/2-1.5}" fill="none" stroke="rgba(255,255,255,.08)" stroke-width="3"/>')


save('noverse-avatar.svg', round_icon())

# 4) Favicon: bản phẳng, mark chiếm gần hết ô, nền trong suốt
fw = 64
fh = fw * H / W
d, g = mark_group('f', 0, (64 - fh) / 2, fh, 'flat')
save('favicon.svg', svg(64, 64, f'<defs>{d}</defs>{g}'))
print('icons ok')

# 5) Maskable (Android): mark nhỏ hơn để nằm gọn trong vùng an toàn 80%
mw2 = S * 0.60; mh2 = mw2 * H / W
d, g = mark_group('k', (S - mw2) / 2, (S - mh2) / 2 - 6, mh2, 'glow')
save('noverse-appicon-maskable.svg', svg(S, S, f'<defs><linearGradient id="kbg" x1="0" y1="0" x2="0" y2="1">'
     '<stop offset="0" stop-color="#1D1A2E"/><stop offset="1" stop-color="#0B0B0D"/></linearGradient>'
     f'{d}</defs><rect width="{S}" height="{S}" fill="url(#kbg)"/>{g}'))
