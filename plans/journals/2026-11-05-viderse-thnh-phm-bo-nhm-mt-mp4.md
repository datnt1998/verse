---
title: Viderse Thành phẩm báo nhầm mất MP4
date: 2026-11-05
summary: "Vá resolver editor export cho UI production PC/mobile, giữ gate V40 và xác minh file thật"
---

# Viderse Thành phẩm báo nhầm mất MP4

## Sự cố
Ảnh người dùng cho thấy Thành phẩm báo mất file dù export succeeded. MP4432.805.526B còn trong blobstore/cache, SHA87f94172...7384 khớp; ffprobe H264/AAC1920x1080,3380.7s. Production UI cũ chỉ tìm video/mp4 artifact; backend editor export công bố projectId/exportId qua output.data và không có artifact.

## Sửa
Backport finishedVideo resolver/caller riêng vào UI production cũ, cả PC/mobile, giữ removedAt/legacyartifact và toàn bộ layout. Không dựng lại hoặc gọi AI. Phát hiện thêm durationMs là elapsed render859.471s, không phải độ dài3380.7s; không dùng làm media duration, bỏ assertion ghim nhầm.

## Kiểm và giới hạn
Production-focused43test/typecheck/Vite/native build qua; integration36tests qua. BuiltUI PC/mobile phát đúng byte-identical MP4 thật và seek1800s qua, mobile direct-open đúngURL, không trànngang. Dữ liệu đọc là fixture; không nhận authenticated-production hay Safari physical proof. Chrome-profile vắng, relay timeout; không mintsession/mởauth. Unauth HEAD401 đúngguard. Production source9d440603, mappedPID47165/inode2830537/SHA khớp, readypublic200 vàpublicchunks khớpbuild. Cloudflare chèn script khiến rawpublicHTML hash động; kiểmassetrefs/chunks thaythế. V40 PR7 vàphysicalSafari gate vẫn giữ.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
