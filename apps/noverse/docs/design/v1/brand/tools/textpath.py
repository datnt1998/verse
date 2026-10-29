"""Chuyển chữ thành path SVG (shaping bằng HarfBuzz, có kerning), font biến thiên → cố định theo wght."""
import uharfbuzz as hb
from fontTools.ttLib import TTFont
from fontTools.varLib import instancer
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.pens.boundsPen import BoundsPen
import io

_cache = {}


def _load(path, wght):
    k = (path, wght)
    if k not in _cache:
        vf = TTFont(path)
        st = instancer.instantiateVariableFont(vf, {'wght': wght})
        buf = io.BytesIO(); st.save(buf); data = buf.getvalue()
        face = hb.Face(data); font = hb.Font(face)
        _cache[k] = (TTFont(io.BytesIO(data)), font)
    return _cache[k]


def text_path(path, wght, text, size, x=0, y=0, tracking=0.0):
    """Trả (d, width, upm-scaled metrics). y là baseline. tracking tính theo em."""
    tt, font = _load(path, wght)
    upm = tt['head'].unitsPerEm
    buf = hb.Buffer(); buf.add_str(text); buf.guess_segment_properties()
    hb.shape(font, buf, {'kern': True, 'liga': False})
    gs = tt.getGlyphSet(); order = tt.getGlyphOrder()
    s = size / upm
    pen = SVGPathPen(gs, ntos=lambda v: f'{v:.2f}'.rstrip('0').rstrip('.'))
    bp = BoundsPen(gs)
    cx = 0
    n = len(buf.glyph_infos)
    for i, (info, pos) in enumerate(zip(buf.glyph_infos, buf.glyph_positions)):
        name = order[info.codepoint]
        t = (s, 0, 0, -s, x + (cx + pos.x_offset) * s, y - pos.y_offset * s)
        gs[name].draw(TransformPen(pen, t))
        gs[name].draw(TransformPen(bp, t))
        cx += pos.x_advance + (tracking * upm if i < n - 1 else 0)
    return pen.getCommands(), cx * s, bp.bounds, {'cap': tt['OS/2'].sCapHeight * s, 'x': tt['OS/2'].sxHeight * s}


def width(path, wght, text, size, tracking=0.0):
    return text_path(path, wght, text, size, tracking=tracking)[1]
