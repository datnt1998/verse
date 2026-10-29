import os, sys
sys.path.insert(0, os.path.dirname(__file__))
from mark import mark_group, W as MW, H as MH
from textpath import text_path

OUT = sys.argv[1] if len(sys.argv) > 1 else os.path.join(os.path.dirname(os.path.abspath(__file__)), '..')
FONT = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', 'font', 'PlusJakartaSans[wght].ttf')
os.makedirs(OUT + '/svg', exist_ok=True)

THEMES = {
    'on-dark':    dict(text='#F5F4FA', tag='#B9B6CC', mark='color'),
    'on-light':   dict(text='#0B0B0D', tag='#6C6A7A', mark='color'),
    'mono-black': dict(text='#0B0B0D', tag='#0B0B0D', mark='mono', mc='#0B0B0D'),
    'mono-white': dict(text='#FFFFFF', tag='#FFFFFF', mark='mono', mc='#FFFFFF'),
}
F = 200            # cỡ chữ Noverse (đơn vị nội bộ)
TAG = 'WRITE WORLDS WITH AI'
WM_TRACK = -0.012  # khoảng chữ của Noverse (em)


def wordmark(wght, x, base, color):
    d, w, b, m = text_path(FONT, wght, 'Noverse', F, 0, 0, WM_TRACK)
    dx = x - b[0]   # canh theo mép mực, không theo lề glyph
    d, w, b, m = text_path(FONT, wght, 'Noverse', F, dx, base, WM_TRACK)
    return f'<path fill="{color}" d="{d}"/>', b, m


def tagline(x, base, target_w, color, center=None):
    size = F * 0.165
    # tìm tracking để dòng khẩu hiệu dài đúng bằng chữ Noverse
    lo, hi = 0.0, 1.0
    for _ in range(30):
        mid = (lo + hi) / 2
        _, _, b, _ = text_path(FONT, 600, TAG, size, 0, 0, mid)
        (lo, hi) = (mid, hi) if (b[2] - b[0]) < target_w else (lo, mid)
    tr = lo
    _, _, b, _ = text_path(FONT, 600, TAG, size, 0, 0, tr)
    ox = (center - (b[2] - b[0]) / 2 if center is not None else x) - b[0]
    d, _, b2, _ = text_path(FONT, 600, TAG, size, ox, base, tr)
    return f'<path fill="{color}" d="{d}"/>', b2, tr


def svg(w, h, inner, title):
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {w:.1f} {h:.1f}" width="{w:.0f}" height="{h:.0f}" role="img" aria-label="{title}">'
            f'<title>{title}</title>{inner}</svg>\n')


def mk(theme, pre, x, y, h):
    t = THEMES[theme]
    return mark_group(pre, x, y, h, t['mark'], t.get('mc', '#000'))


def horizontal(theme, wght=700, tag=False):
    t = THEMES[theme]
    # dựng chữ ở gốc toạ độ để đo
    _, b0, m = wordmark(wght, 0, 0, t['text'])
    cap = m['cap']; ww = b0[2] - b0[0]
    if tag:
        tag_base = F * 0.30 + F * 0.165 * 0.72
        top, bot = -cap, tag_base
        mh = (bot - top) * 1.18
    else:
        top, bot = -cap, 0
        mh = cap * 1.9
    mw = mh * MW / MH
    pad = mh * 0.06
    gap = mh * 0.30
    cy = (top + bot) / 2
    mx, my = pad, pad
    tx = pad + mw + gap
    base = my + mh / 2 - cy
    parts = []
    d, g = mk(theme, 'L', mx, my, mh)
    wm, b, _ = wordmark(wght, tx, base, t['text'])
    parts += [g, wm]
    if tag:
        tp, _, _ = tagline(tx, base + tag_base, ww, t['tag'])
        parts.append(tp)
    W = tx + ww + pad; H = mh + 2 * pad
    return svg(W, H, f'<defs>{d}</defs>' + ''.join(parts), 'Noverse' + (' — Write worlds with AI' if tag else ''))


def stacked(theme, wght=700, tag=False):
    t = THEMES[theme]
    _, b0, m = wordmark(wght, 0, 0, t['text'])
    cap = m['cap']; ww = b0[2] - b0[0]
    mw = ww * 0.92; mh = mw * MH / MW
    pad = mh * 0.08
    W = max(ww, mw) + 2 * pad
    cx = W / 2
    d, g = mk(theme, 'S', cx - mw / 2, pad, mh)
    base = pad + mh + F * 0.20 + cap
    wm, b, _ = wordmark(wght, cx - ww / 2, base, t['text'])
    parts = [g, wm]
    bottom = base
    if tag:
        tb = base + F * 0.30 + F * 0.165 * 0.72
        tp, _, _ = tagline(0, tb, ww, t['tag'], center=cx)
        parts.append(tp); bottom = tb
    H = bottom + pad + F * 0.05
    return svg(W, H, f'<defs>{d}</defs>' + ''.join(parts), 'Noverse' + (' — Write worlds with AI' if tag else ''))


def wordmark_only(theme, wght):
    t = THEMES[theme]
    _, b0, m = wordmark(wght, 0, 0, t['text'])
    pad = F * 0.06
    wm, b, _ = wordmark(wght, pad, pad - b0[1], t['text'])
    return svg(b0[2] - b0[0] + 2 * pad, b0[3] - b0[1] + 2 * pad, wm, 'Noverse')


def save(name, s):
    open(f'{OUT}/svg/{name}', 'w').write(s)


for th in THEMES:
    save(f'noverse-logo-h-{th}.svg', horizontal(th))
    save(f'noverse-logo-h-tagline-{th}.svg', horizontal(th, tag=True))
    save(f'noverse-logo-v-{th}.svg', stacked(th))
    save(f'noverse-logo-v-tagline-{th}.svg', stacked(th, tag=True))
    save(f'noverse-wordmark-{th}.svg', wordmark_only(th, 700))
    # Bản đậm hơn (ExtraBold 800) cho cỡ nhỏ: thanh trên cùng, chân email
    save(f'noverse-logo-h-extrabold-{th}.svg', horizontal(th, wght=800))
    save(f'noverse-wordmark-extrabold-{th}.svg', wordmark_only(th, 800))
print('logos ok')

# Ảnh chia sẻ (Open Graph) 1200 x 630
import re
v = stacked('on-dark', tag=True)
vw, vh = map(float, re.search(r'viewBox="0 0 ([\d.]+) ([\d.]+)"', v).groups())
inner = re.search(r'<svg[^>]*>(.*)</svg>', v, re.S).group(1).replace('<title>Noverse — Write worlds with AI</title>', '')
sc = 430 / vh
og = (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1200 630" width="1200" height="630">'
      '<defs><radialGradient id="ogg" cx=".5" cy=".38" r=".55"><stop offset="0" stop-color="#9A63E3" stop-opacity=".28"/>'
      '<stop offset="1" stop-color="#9A63E3" stop-opacity="0"/></radialGradient></defs>'
      '<rect width="1200" height="630" fill="#0B0B0D"/><rect width="1200" height="630" fill="url(#ogg)"/>'
      f'<g transform="translate({600 - vw * sc / 2:.1f} {315 - vh * sc / 2:.1f}) scale({sc:.4f})">{inner}</g></svg>\n')
save('noverse-og-image.svg', og)
