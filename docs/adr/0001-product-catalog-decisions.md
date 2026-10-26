# ADR 0001: Catalog product v1

Ngày: 2026-10-26. Trạng thái: đã áp dụng.

## Bối cảnh

Module đầu tiên của commerce platform là Store và Product. Schema `platform.stores` và `catalog.products` đã có từ migration `000001`: UUID v7, `NUMERIC(19,4)`, partial unique `(store_id, sku) WHERE deleted_at IS NULL`. Các quyết định dưới đây đóng những chỗ yêu cầu v1 còn để mở, mà không đổi index unique đã chọn.

## SKU sau khi archive

Quyết định: archive không giải phóng SKU. Partial unique index hiện tại đã nói điều đó, vì archive không set `deleted_at`.

Hệ quả: tạo lại cùng SKU sau archive trả 409. Restore không bao giờ đụng unique. Khi sau này có soft-delete, set `deleted_at` sẽ nhả SKU mà không cần đổi index. Test integration chứng minh cả hai phía.

Phương án bỏ: unique chỉ trên product active. Cách đó cần partial index `WHERE deleted_at IS NULL AND status = 'active'`, và restore có thể thất bại nếu SKU đã bị dùng lại.

## Chuẩn hóa SKU và currency

Quyết định: service làm `upper(trim(...))` trước khi ghi. Database chặn lần nữa bằng CHECK `sku = upper(btrim(sku))` và `currency ~ '^[A-Z]{3}$'`. SKU khớp `^[A-Z0-9][A-Z0-9_-]{0,63}$`.

Hệ quả: `tshirt-001` và `TSHIRT-001` là một SKU. Không phân biệt hoa thường. Currency lưu `USD`, không phải `usd`.

## Tiền

Quyết định: `shopspring/decimal` ở Go, `NUMERIC(19,4)` ở PostgreSQL. JSON trả price dạng string. Input nhận string hoặc number. Quá 4 chữ số thập phân thì 400, không để PostgreSQL làm tròn.

Hệ quả: không còn `float64` trên giá. Client phải đọc price như decimal string, ví dụ `"19.9900"`.

## PATCH và version

Quyết định: `base.Optional[T]` cho body PATCH. `version INTEGER` tăng trong cùng câu `UPDATE ... WHERE version = $expected`. GET, POST, PATCH, archive và restore của một product trả `ETag: "<version>"`. PATCH bắt buộc `If-Match`. Archive hoặc restore lặp lại không chạy UPDATE nên không tăng version.

Hệ quả: hai admin sửa cùng một bản đọc thì một người nhận 412. Mutex trong process không đủ, vì nhiều instance dùng chung một database. Client có thể gửi `If-Match: "3"` hoặc `If-Match: 3`.

## Danh sách

Quyết định: keyset `(created_at, id) < (cursor_time, cursor_id)` với `ORDER BY created_at DESC, id DESC`. Cursor là base64url của `{"createdAt":"<RFC3339Nano UTC>","id":"<uuid>"}`. Cursor hỏng trả 400. `q` tìm `sku ILIKE prefix` hoặc `name ILIKE %q%`, và `%`, `_`, `\` trong input được escape.

Hệ quả: trang không bị trùng khi nhiều dòng cùng `created_at`. Offset không được dùng. `q` chưa có index trigram; đo `EXPLAIN` trước khi thêm index ở phase sau.

## Ranh giới layer

Quyết định: repository trả mapper. Service map sang DTO. Model không đi ra HTTP.

Hệ quả: câu query đổi cột thì sửa mapper và DTO, không buộc JSON public bám shape của bảng.
