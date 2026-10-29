"""Noverse mark: 4 quả cầu. Toạ độ gốc trong hộp 562 x 410.
Thứ tự vẽ (dưới → trên): xanh dương (Kiến trúc), tím (Người viết), cam (Biên tập), xanh ngọc (Trọng tài).
Cùng họ màu với orb trong app (.o-architect, .o-writer, .o-editor, .o-arbiter), chỉnh theo bộ thương hiệu."""

W, H = 562, 410
SPHERES = [  # id, cx, cy, r, stops (offset, colour)
    ('b', 115, 206, 114, [(0, '#DCE8FF'), (.22, '#A9C2FA'), (.58, '#7896F2'), (1, '#5360DC')]),
    ('p', 281, 134, 134, [(0, '#F1E2FF'), (.22, '#CDA4F7'), (.58, '#A472EB'), (1, '#7C46D4')]),
    ('o', 447, 216, 114, [(0, '#FFEACF'), (.22, '#FBC48A'), (.58, '#F29A5A'), (1, '#D9663A')]),
    ('g', 281, 308, 102, [(0, '#DDFBEE'), (.22, '#A0EDCF'), (.58, '#5FCFB0'), (1, '#36A29C')]),
]
# Màu phẳng cho bản nhỏ (favicon) và bản một màu
FLAT = {'b': '#7C93F3', 'p': '#B27FEE', 'o': '#F0A25E', 'g': '#6FD0B6'}


def defs(pre, shadow=True, flat=False):
    d = []
    for sid, cx, cy, r, stops in SPHERES:
        if flat:
            st = [(0, stops[1][1]), (.55, stops[2][1]), (1, stops[3][1])]
        else:
            st = stops
        d.append(f'<radialGradient id="{pre}{sid}" cx=".34" cy=".28" fx=".34" fy=".28" r=".95">'
                 + ''.join(f'<stop offset="{o}" stop-color="{c}"/>' for o, c in st) + '</radialGradient>')
    if not flat:
        # vành tối dưới-phải + ánh sáng trên-trái (giống box-shadow inset của .orb)
        d.append(f'<radialGradient id="{pre}rim" cx=".42" cy=".38" r=".66">'
                 '<stop offset=".62" stop-color="#140A2E" stop-opacity="0"/>'
                 '<stop offset="1" stop-color="#140A2E" stop-opacity=".30"/></radialGradient>')
        d.append(f'<radialGradient id="{pre}hl" cx=".34" cy=".27" r=".24">'
                 '<stop offset="0" stop-color="#fff" stop-opacity=".35"/>'
                 '<stop offset="1" stop-color="#fff" stop-opacity="0"/></radialGradient>')
    d.append(f'<filter id="{pre}gl" x="-40%" y="-40%" width="180%" height="180%"><feGaussianBlur stdDeviation="28"/></filter>')
    if shadow:
        d.append(f'<filter id="{pre}sh" x="-30%" y="-30%" width="160%" height="170%" color-interpolation-filters="sRGB">'
                 '<feDropShadow dx="0" dy="10" stdDeviation="12" flood-color="#1A0B3D" flood-opacity=".32"/></filter>')
    return ''.join(d)


def body(pre, shadow=True, flat=False, glow=False):
    out = []
    if glow:
        out.append(f'<g filter="url(#{pre}gl)" opacity=".55">' + ''.join(
            f'<circle cx="{cx}" cy="{cy}" r="{r}" fill="{FLAT[sid]}"/>' for sid, cx, cy, r, _ in SPHERES) + '</g>')
    for i, (sid, cx, cy, r, _) in enumerate(SPHERES):
        f = f' filter="url(#{pre}sh)"' if (shadow and i > 0) else ''
        g = [f'<circle cx="{cx}" cy="{cy}" r="{r}" fill="url(#{pre}{sid})"/>']
        if not flat:
            g.append(f'<circle cx="{cx}" cy="{cy}" r="{r}" fill="url(#{pre}rim)"/>')
            g.append(f'<circle cx="{cx}" cy="{cy}" r="{r}" fill="url(#{pre}hl)"/>')
        out.append(f'<g{f}>' + ''.join(g) + '</g>')
    return ''.join(out)


def mono(pre, color, gap=11):
    """Một màu: mỗi quả cầu bị khoét bởi các quả nằm trên nó (khe hở trong suốt, nền nào cũng đúng)."""
    d, b = [], []
    for i, (sid, cx, cy, r, _) in enumerate(SPHERES):
        above = SPHERES[i + 1:]
        if above:
            d.append(f'<mask id="{pre}m{sid}" maskUnits="userSpaceOnUse" x="-20" y="-20" width="{W+40}" height="{H+40}">'
                     f'<rect x="-20" y="-20" width="{W+40}" height="{H+40}" fill="#fff"/>'
                     + ''.join(f'<circle cx="{a[1]}" cy="{a[2]}" r="{a[3]+gap}" fill="#000"/>' for a in above)
                     + '</mask>')
            b.append(f'<circle cx="{cx}" cy="{cy}" r="{r}" fill="{color}" mask="url(#{pre}m{sid})"/>')
        else:
            b.append(f'<circle cx="{cx}" cy="{cy}" r="{r}" fill="{color}"/>')
    return ''.join(d), ''.join(b)


def mark_group(pre, x, y, h, kind='color', color='#0B0B0D'):
    """Trả (defs, group) đặt mark cao h tại (x, y)."""
    s = h / H
    if kind == 'mono':
        dd, bb = mono(pre, color)
    else:
        flat = kind == 'flat'
        dd, bb = defs(pre, shadow=not flat, flat=flat), body(pre, shadow=not flat, flat=flat, glow=kind == 'glow')
    return dd, f'<g transform="translate({x:.2f} {y:.2f}) scale({s:.5f})">{bb}</g>'
