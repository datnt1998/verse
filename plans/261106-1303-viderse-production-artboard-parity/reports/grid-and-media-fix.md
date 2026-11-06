# Grid ảnh và lỗi stale giả · 07/11/2026

## Cause
- Desktop cover chỉ span2cột,không span2hàng như Main.dc.html1082; scene cùng hàng bị stretch theo cover. Trên production1568×900, Cảnh1/2 cao277.719px nhưng preview123.75px,còn khoảng trống129.375px sau caption.
- Build đọc path legacy `editor/media/<id>/metadata.json` thay vì authoritative Editor repository head. Production có đủ4media snapshot: GETmetadata200,contentHash khớp4pins,source blob vẫn được tham chiếu; head đang ở `editor/revisions/*.json`,không còn legacy blob. Failed build chứa stale do hiểu nhầm media đã xoá. Không phải cần đăng nhập lại hoặc thay template.

## Source repair
- Cover span2×2,preview flex-grow để lấp ô,các scene không còn chịu chiều cao cover. Giữ preferred16:9 cho trường hợp chỉ có cover; không thay artboard hoặc selection/viewer behavior.
- Tích hợp pending fix cause-aligned: build.Deps.MediaMetadata nối Editor.GetMediaMetadata; bỏ việc tự đọc legacy metadata. Vẫn acquire pinned bytes/hash và chặn media bị xoá/thay thật. Di chuyển cùng native render-smoke wiring và test harness; regression dùng Editor/Postgres import thật bao gồm valid/deleted/replaced. Không bypass stale guard,không đổi dữ liệu hoặc reset job.
- User dirty index và pending source khác trong LiveStudio không được đưa vào candidate; chỉ patch layout từ clean source được stage ở private index.

## Verification observed
- Focused LiveStudio13test pass,typecheck/build pass.
- Build package Postgres tests pass3.448s. Cmd tests lúc đầu fail3do archive thiếu SQL migration identity Noverse; bổ sung đúng tracked migration+role SQL rồi cmd pass27s. Native build candidate pass; không overlay.
- Read-only local transport8046 dùng actual production GET với phiên thật,block mọi mutation; không fixture hoặc sửa DOM. 1568scene cao148.344px,cover308.688px,captionbottomGap0 ở6tile. 1440scene130.344px,cover272.688px,gap0;không tràn ngang. Tile→viewer→next→close hoạt động. Phone390×844workbench→video→Ảnh6không tràn ngang;nguồn/layout phone không thay.

## Deployment / retry
Pending: commit+merge,deploy artifact/hash/process proof,actual failed-job build retry and preserve image/snapshot/provider receipts. Không gọi local smoke là production PASS.
