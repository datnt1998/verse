# V36 · Oracle bộ lọc/tìm model

Test-only: `tests/viderse-workflow/settings-state-audit.pw.ts`. Lượt52 sau sửa **14pass/0fail/2skip có tên**; filter/search/manual-cancel desktop normal/reduced đều qua. Hai skip là recheck mobile thuộc gate PR6, không liên quan cache. Không đổi production cache, timeout hoặc dữ liệu provider.

## Nguyên nhân lượt47

Hai ca desktop chờ một GET khi bấm về Tất cả. `ModelCatalog` dùng key `(provider,q,filter,cursor)`; `useResource` giữ dữ liệu mới trong2giây. Quay về key `all` hoặc xoá tìm kiếm có thể dùng cache đúng chức năng, không gửi request mới. `api/ai.ts` cũng bỏ query `filter` khi là all; không được đóng đinh cách serialize này thành hợp đồng UI.

Bằng chứng source: `screens/settings/connect/ModelCatalog.tsx`, `data/useResource.ts`, shared `data/cache.ts`, `api/ai.ts`. Lượt47 đã đổi radio/list nhưng oracle `waitApi` vẫn chờ30giây. Đây là lỗi harness, không phải cớ tắt cache hoặc nới timeout app.

## Sửa hiện hành

- Không có helper polling quiet-window/listener mới. Parent bỏ `observeApi`1,2giây do delegate đề xuất: chỉ cần oracle dữ liệu và observable UI.
- GET read-only qua `page.request` lấy tập model thật cho Gợi ý/tìm kiếm, cùng page-size200 và cursor đầu như component. Đây là oracle độc lập, không yêu cầu UI lặp GET.
- Native click Gợi ý/Đã chọn/Tất cả rồi poll đúng radio và danh sách ID hiển thị theo dữ liệu thật. Tất cả phải trở về tập ban đầu; Đã chọn chỉ còn tập selected và checkbox đúng.
- Tìm tên model thật phải thu hẹp tập khi dataset có đủ mục, giữ Tất cả, khớp tập server trả và số tổng. Xoá ô tìm phải khôi phục tập ban đầu. Không ép network request trên cache hit.
- Page đầu luôn ghim những ID selected còn trong draft, kể cả ngoài bộ lọc/search. Oracle hợp `server.models ∪ selected` đúng hành vi đã có, không coi chúng là kết quả tìm kiếm giả hoặc tự đổi sản phẩm.
- Manual panel vẫn nhập rồi Huỷ, không gửi `/models/check`; ledger cuối vẫn yêu cầu không mutation.

Không mock danh mục/đăng nhập, không cache-busting, không tăng timeout, không đổi schema. Hai lần mở catalog đầu tiên vẫn chờ response của mount mới trong browser context mới; nếu sau này app preload, cần chuyển chính hai oracle đó sang dữ liệu+UI thay vì ép GET.
