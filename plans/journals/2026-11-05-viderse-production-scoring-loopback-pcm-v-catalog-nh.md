---
title: "Viderse production scoring: loopback PCM và catalog ảnh"
date: 2026-11-05
summary: "Gỡ kẹt score production100/100, export thật; giữ gate UI riêng"
---

# Viderse production scoring: loopback PCM và catalog ảnh

## Sự cố
Video production kẹt score: loopback Chrome bị hosted origin guard chặn. Workaround public origin claim được nhưng PCM dài56phút nghẽn proxy, timeout10phút. Benchmark9.2MB: public36.3s, loopback53ms. Rollout từng cài file mới nhưng launchd còn inode cũ.

## Sửa và kiểm
Scoped direct-loopback render/measure với authority cố định, raw peer, không proxy metadata; giữ token/nonce/fence/binding. Đo thật1.298GB khoảng20s. Score75 kế tiếp do render.Inputs thiếu catalog ảnh trung gian, dù ảnh đã duyệt còn trong DB/blobstore. Nạp catalog đúngjob, output hiện hành ưu tiên và ambiguity lỗi; giữ hash. Score cuối100/100, không override, không gọi lại AI. Export thật đã claim, heartbeat và render4375/101421frames; chưa nhận MP4 hoàn tất.

## Vận hành
Production source401d1b45, UI cũ giữ nguyên; cài rồi kickstart-k và kiểm mapped vnode. PR9/10 backend riêng; PR7/Safari thiết bị thật vẫn giữ. Affected production/integration suites và scoped/catalog race qua. Test15ms lỗi khi chạy song song được giữ lịch sử; isolated/final serial qua, không nới test. Không tăng timeout hoặc bỏ quality gate che nguyên nhân.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
