# Kiểm tra luật hình trên mọi artboard (đợt 26). Chạy từ docs/design/v1: python3 artboards/audit_rules.py  → in vi phạm, exit 1 nếu có.
# Luật: (1) avatar (ô chữ cái / ô ảnh đại diện kênh, nền tảng, nhà cung cấp, tài khoản) luôn tròn;
#       (2) không vạch màu một bên: border-left/right ≥ 2px, hoặc 1px có màu; box-shadow inset một cạnh;
#           ::before/::after dải dọc sát mép. Vạch ngăn cột 1px trung tính (rgba(255,255,255,…)) được phép.
import re, glob, sys, os
R = os.path.dirname(os.path.abspath(__file__)) + '/'
SKIP = {'Today.dc.html', 'CompVideo.dc.html', 'Agents.dc.html', 'DirB.dc.html', 'DirC.dc.html', 'Logo.dc.html'}  # board cũ không nằm trong canvas
ALLOW = [r'\.io\{']  # khung vào/ra trên timeline Editor: viền hai bên của một vùng chọn, không phải vạch nhấn
bad = []
for f in sorted(glob.glob(R + '*.dc.html')):
    n = f.split('/')[-1]
    if n in SKIP: continue
    s = open(f).read()
    for m in re.finditer(r'border-(left|right|inline-start|inline-end)\s*:\s*([^;"}]+)', s):
        v = m.group(2).strip(); ctx = s[max(0, m.start() - 200):m.start()]
        if any(re.search(a, ctx) for a in ALLOW): continue
        w = re.match(r'([\d.]+)px', v)
        neutral = re.search(r'rgba\(255, ?255, ?255, ?[.\d]+\)|transparent|\{\{[\w.]*line', v)
        if (w and float(w.group(1)) >= 2) or not neutral:
            bad.append((n, 'vạch một bên', s[m.start():m.end()][:70]))
    for m in re.finditer(r'inset\s+-?[1-9][\d.]*px\s+0\s+0\b(?!\s+[1-9])', s):
        bad.append((n, 'inset một cạnh', s[m.start() - 20:m.end() + 20].replace('\n', ' ')))
    body = s.split('<script type="text/x-dc"')[0]
    for m in re.finditer(r'\{\{[\w.]*\.(ini|abbr)\}\}', body):
        a = max(body.rfind('<span', 0, m.start()), body.rfind('<div', 0, m.start()))
        tag = body[a:body.find('>', a) + 1]
        if 'class="ic ' in tag: a = body.rfind('<span', 0, a - 1); tag = body[a:body.find('>', a) + 1]
        cls = (re.search(r'class="([^"]*)"', tag) or [None, ''])[1].split()
        r = re.search(r'border-radius:\s*([^;"]+)', tag)
        if not r and set(cls) & {'av', 'cav', 'vv-avatar'}: continue
        rv = r.group(1).strip() if r else '(không có)'
        if rv not in ('50%', '{{r.rad}}', '{{dlg.rad}}', '999px'):
            bad.append((n, 'avatar không tròn', rv + ' · ' + tag[:90]))
for b in bad: print(' | '.join(b))
print(len(bad), 'vi phạm')
sys.exit(1 if bad else 0)
