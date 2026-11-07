# Bàn dựng · asset dùng chung và nền · #95A,07/11/2026

## Cause và scope
API đã gửi nguyên templateSnapshot đã chốt nhưng readCloneJob chỉ lấy templateName;UI không có danh sách layer dùng chung. Inspector Nền suy từ existence của backdrop view thay vì có recipe/source nên0clip vẫn cảnh báo thiếu/lặp. Production recipe backdrop=null,randomPicksfalse,wantedCount0;template có8shared tracks(tuyết,Subscribe đầu/cuối,sóng giọng). Không có clip nền cần sửa hoặc thêm dữ liệu giả.

Người dùng chọn#95A: giữ2tab,hiện danh sách chỉ đọc trong Thông tin,dưới Template;phone dùng cùng component trong sheet. Không thay artboard,không thêm tab Tài nguyên hoặc khả năng sửa/xoá shared assets. Asset projection chỉ từ snapshot.project.timeline của video,không đọc template mới hoặc toàn bộ thư viện;dynamic slots bị loại,track rỗng/orphan không bịa thêm,layer tắt được ghi rõ. Snapshot legacy thiếu inventory không bị coi như template rỗng. Nền không cấu hình hiển thị đúng empty state và link Thư viện,không coverage/repair CTA giả.

## Verification
- Permanent regression trước vá:unconfigured template vẫn có progressbar/cảnh báo,1test fail. Sau vá5file/24tests passed(bao gồm mobile11,APIguard,slot filtering/unknown-vs-empty,track grouping và no-background state).
- Clean archive typecheck/build/size/native build đạt;entry142.6/150KiB,phone179.8/200KiB,CSS33.9/40KiB. Rollup annotation/static-dynamic import/lazychunk warnings vẫn được giữ.
- Read-only local transport dùng real production GET/existing session,không fixture/DOM injection,block mọi mutation. PC1440×900hiện8layer đúng tên,snow Video/Lặp;không câu Nền sẽ lặp hoặc overflow. Phone390×844cùng8layer,sheet1dialog,y118.172→844,height725.828,có scroll,nền không cấu hình đúng;tap scrim đóng,mở lại đạt.
- Inline review:không server API/schema/encoder changes;projected labels chỉ render React text,internal IDs chỉ làm key;không import Editor vào initial bundle. Other user dirty changes/index không đưa vào candidate.

## Deployment
Pending native artifact commit/merge/hash/process/public and actual PC/mobile smoke. Không gọi local smoke là production acceptance.

## Xuất chậm
[Phân tích riêng](export-performance.md):89m21s là elapsed thật,78m24s browser supervision và10m57s hậu xử lý. Chưa benchmark hoặc đổi chất lượng/cấu hình xuất;không hứa tốc độ tăng chưa đo.
