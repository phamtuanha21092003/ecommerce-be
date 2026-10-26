# Product module v1

Catalog cho nhiều store. Mỗi store sở hữu sản phẩm của mình. Chưa có inventory, order, auth, hay message queue.

## Dữ liệu

| Field | Quy tắc |
| --- | --- |
| id | `uuidv7()` do PostgreSQL sinh |
| store_id | Store sở hữu product |
| sku | Duy nhất trong store với các dòng `deleted_at IS NULL`, đã uppercase và trim |
| name | Bắt buộc, 1–255 ký tự sau khi trim |
| description | Có thể rỗng, tối đa 5000 ký tự |
| price | `NUMERIC(19,4)`, không âm, tối đa 4 chữ số thập phân |
| currency | 3 chữ cái ASCII, ví dụ `USD` |
| status | `active` hoặc `archived` |
| archived_at | NULL khi active, có giá trị khi archived |
| version | Tăng khi nội dung hoặc status thực sự đổi |
| created_at, updated_at | `updated_at` do trigger `platform.set_updated_at()` |
| deleted_at | Soft-delete. Chưa có API. Khi có giá trị, SKU được giải phóng |

## API

Base path: `/api/v1/stores/{storeID}/products`

| Method | Path | Việc cần làm |
| --- | --- | --- |
| POST | `/api/v1/stores` | Tạo store |
| GET | `/api/v1/stores/{storeID}` | Chi tiết store |
| POST | `/api/v1/stores/{storeID}/products` | Tạo product |
| GET | `/api/v1/stores/{storeID}/products` | Danh sách keyset |
| GET | `/api/v1/stores/{storeID}/products/{productID}` | Chi tiết, phải đúng store |
| PATCH | `/api/v1/stores/{storeID}/products/{productID}` | Sửa field được gửi, cần `If-Match` |
| POST | `.../{productID}/archive` | Archive, không xóa dòng |
| POST | `.../{productID}/restore` | Đưa về active |

Danh sách nhận `limit`, `cursor`, `status`, `q`. Mặc định `status=active`, `limit=20`, tối đa 100. Thứ tự `created_at DESC, id DESC`.

## Quy tắc đã chốt

- R1: chỉ `INSERT`. Unique index từ chối SKU trùng dưới concurrency. API trả 409.
- R2: product archived vẫn giữ SKU. SKU chỉ trống sau khi `deleted_at` được set.
- R3: PATCH phân biệt field không gửi, `null`, và giá trị. `description` null hoặc `""` xóa mô tả. `name`, `sku`, `price`, `currency` không được null.
- R4: cột `version` cộng `ETag` / `If-Match`. Thiếu header là 428, version cũ là 412.
- R5: mọi câu đọc và ghi product đều có `store_id` trong `WHERE`. Product của store khác trả 404.

## Việc test đã phủ

`TestCatalogAPI` trong `internal/domain/api/catalog_integration_test.go`:

- Cùng SKU trong một store bị từ chối, hai store khác nhau thì được
- Giá âm bị service và CHECK của database từ chối
- 20 goroutine tạo một SKU, đúng một request thành công
- Hai PATCH cùng version, một 200 và một 412, chỉ một field được ghi
- Store khác không đọc được product
- Cursor hỏng trả 400
- Đi hết các trang không trùng, không sót, kể cả khi `created_at` giống nhau
- Archive hai lần không tăng version và không nhả SKU
- Set `deleted_at` thì SKU tạo lại được

## Chưa làm trong v1

Soft-delete API, auth, phân quyền theo store, index `pg_trgm` cho `q`, product variants, outbox.
