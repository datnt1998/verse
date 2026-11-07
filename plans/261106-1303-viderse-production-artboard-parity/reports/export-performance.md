# Xuất57phút mất89phút · phân tích07/11/2026

## Kết luận có bằng chứng
Lần xuất đã hoàn tất,không phải ETA:clone_steps export5361.163s(89m21s),stored durationMs4704437chỉ bao quanh browser supervision. Browser request4703.668s(78m24s)bao gồm preparation/render/encode/upload/publication;656.7s(10m57s)còn lại là backend compress/finalize. Score23.788s ngoài khoảng này;claim delay2.305s,không phải queue là nút thắt chính.

Artifact đúng revision của lần xuất:3474.967s,1920×1080,30fps,104249frames. File ban đầu2656758643B;sau nén444607339B. Project có đúng1snow video(screen,opacity.45),5ảnh cảnh,caption,audio,shape/text;asset snow300frame/10giây. Hash trùng trong snapshot.Overlays không tạo duplicate render;đã loại giả thuyết tuyết nhân đôi. Kiểm bằng metadata revision-bound và ffprobe read-only,không rerender full video.

## Critical path
- Frame loop tuần tự,có overlap giới hạn với encoder:`web/src/editor/engine/features/export/utils/pipelined-frame-loop.ts:68–106`.
- Layer tĩnh được redraw vào canvas full-size từng frame;pool chỉ tái dùng canvas,không cache pixel tĩnh:`frame-render-tasks.ts:222–237`.
- Snow screen kích hoạt GPU blend toàn stack;upload nhiều surface full-size,full-screen pass mỗi layer,wait GPU rồi draw về render canvas:`client-render-engine.ts:2323–2343,2573`;`frame-compositing.ts:139–191`.
- Loop video wrap reset iterator,không cache toàn bộ decoded loop;không phải seek keyframe mỗi frame thường:`canvas-video-extractor.ts:303–324`.
- WebCodecs queue/backpressure thực sự tồn tại;không chỉ encode1frame cho tới xong. Hardware encoder thực tế chưa đo;Chrome export không đặt disable-gpu.
- Caption coverage có accumulation mỗi frame nhưng pixel scan theo expiration/finalization;không phải104249CPUreadback. Sóng giọng dùng envelope cache,không FFT mỗi frame.
- Backend nén lại bằnglibx264 veryfast CRF28 maxrate900k,copiedaudio:`internal/render/compress.go:147–149`;thực hiện sauSupervisor nên DurationMSkhông bao gồm nó:`internal/render/export.go:27,44–80`.

## Đề xuất,không hứa tốc độ chưa đo
1. Profile12–30s của đúng project/1080p30/giữ mọi hiệu ứng,bao gồm wrap10s và biên cảnh. Táchrender/decode,GPU upload/composite/wait,VideoSample capture,encoder wait/flush,upload/server commit/compression;không log token/URL/cookie. Không xuất lại57phút chỉ để xác nhận baseline.
2. Nếu repeated raster/upload chiếm chính,tái dùng surface/texture tĩnh với invalidation đầy đủ cho motion,crop,mask,opacity,effect,captions và chuyển cảnh. Không bỏsyncGPUmù vì có hazard black-frame/lifetime.
3. Decoded snow cache phải budget RAM:300frame1080pRGBA≈2.49GB. Không cache vô hạn trên máy16GB.
4. Tắt compress chỉ bỏ được tối đa khoảng11phút,đổi ra filegần6lần lớn hơn;không giải quyết78phút browser. Đổi hardware encoder cần samplevisual/size,nó không mặc định tương đương chất lượng.
5. Snapshot máy lúc điều tra:M4/10core/16GiB,swap3.78GB,data volume95%full,còn≈11GiB,có ứng dụng khác dùng tài nguyên. [INFERENCE]có thể làm chậm;không có CPU/GPUtrace của lần xuất cũ để định lượng. Không tự killprocess/prunecache hoặc tăng concurrent exports.

## Thay đổi đã phát hành07/11/2026
Production đã chạy main `2b55c0241ee3c394f09dba89099cb132a1a02c9f` (PR#20): bỏ queue-completion fence thừa trước phép copy Canvas2D đồng bộ; không đổi codec,bitrate,FPS,resolution,audio hay hiệu ứng. Mẫu30s/300s,Chrome trực tiếp không CDP và chuyển cảnh thật đều khớp từng frame giải mã,PCM và caption coverage. [Phép đo và giới hạn](export-real-baseline-20261107.md).

Mẫu300s đo46.361→45.124s;đây là một lượt đo,mức chênh≈2.7% chưa chứng minh tăng tốc ổn định. Mẫu dùng output QA local,không tái hiện publication production hay nén backend. **Chưa tái hiện hoặc giải quyết được nguyên nhân lần89phút;không ngoại suy thời gian57phút.** Native/public-asset/process/auth/playback smoke qua;Safari và parity7nhóm không được suy ra từ phép đo này.
