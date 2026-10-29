#!/usr/bin/env bash
# Sinh lại toàn bộ SVG của bộ nhận diện vào ../svg.
# Cần: python3, fonttools, uharfbuzz (pip install fonttools uharfbuzz).
# Muốn đổi font chữ Noverse (vd. Satoshi): đặt tệp vào ../font rồi sửa biến FONT trong gen_logo.py.
# PNG: node render.js '<jobs json>' (cần playwright + chromium), xem 23-brand.md.
set -e
cd "$(dirname "$0")"
python3 gen_icons.py
python3 gen_logo.py
