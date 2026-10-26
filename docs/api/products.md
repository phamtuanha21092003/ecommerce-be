# HTTP API: Store và Product

Base URL local: `http://localhost:3000`.

Response thành công:

```json
{ "success": true, "data": {}, "message": "success" }
```

Response lỗi:

```json
{ "error": "conflict", "message": "sku already exists in this store" }
```

| HTTP | `error` | Khi nào |
| --- | --- | --- |
| 400 | `bad_request` | Body, query, hoặc cursor không hợp lệ |
| 404 | `not_found` | Store hoặc product không tồn tại trong store đó |
| 409 | `conflict` | SKU đã tồn tại trong store |
| 412 | `precondition_failed` | `If-Match` không còn đúng version |
| 428 | `precondition_required` | PATCH thiếu `If-Match` |
| 500 | `internal_server_error` | Lỗi không phân loại được |

Product của store khác cũng là 404. API không nói product đó có tồn tại ở store nào.

## Store

`POST /api/v1/stores`

```json
{ "name": "Store A" }
```

`201`, `data` gồm `id`, `name`, `createdAt`, `updatedAt`.

`GET /api/v1/stores/{storeID}` trả `200` với cùng shape.

## Tạo product

`POST /api/v1/stores/{storeID}/products`

```json
{
  "sku": "tshirt-001",
  "name": "T-Shirt",
  "description": "Cotton",
  "price": "19.99",
  "currency": "usd"
}
```

`price` có thể là string hoặc number. `description` có thể bỏ qua. SKU và currency được chuẩn hóa thành `TSHIRT-001` và `USD`.

`201` và header `ETag: "1"`.

```json
{
  "success": true,
  "data": {
    "id": "0193f0c0-7c2a-7d3e-8b10-111111111111",
    "storeId": "0193f0c0-7c2a-7d3e-8b10-000000000001",
    "sku": "TSHIRT-001",
    "name": "T-Shirt",
    "description": "Cotton",
    "price": "19.9900",
    "currency": "USD",
    "status": "active",
    "archivedAt": null,
    "version": 1,
    "createdAt": "2026-10-26T09:00:00.123456Z",
    "updatedAt": "2026-10-26T09:00:00.123456Z"
  },
  "message": "success"
}
```

Client không được gửi `id`, `createdAt`, `updatedAt`, `version`.

## Danh sách

`GET /api/v1/stores/{storeID}/products?limit=20&cursor={cursor}&status=active&q=shirt`

| Query | Ý nghĩa |
| --- | --- |
| `limit` | 1–100, mặc định 20 |
| `cursor` | Opaque. Bỏ trống cho trang đầu. Sai format thì 400 |
| `status` | `active` (mặc định) hoặc `archived` |
| `q` | SKU theo prefix, hoặc tên theo substring, không phân biệt hoa thường |

`data` luôn là object này, kể cả khi rỗng:

```json
{
  "items": [],
  "limit": 20,
  "hasNext": false,
  "nextCursor": null
}
```

Trang sau gửi nguyên `nextCursor`. Thứ tự ổn định là `created_at DESC, id DESC`. `nextCursor` lấy từ phần tử cuối của trang hiện tại.

## Chi tiết, sửa, archive

`GET /api/v1/stores/{storeID}/products/{productID}` trả một product và `ETag`.

`PATCH` cần `If-Match` bằng ETag đó.

| Body | Kết quả |
| --- | --- |
| Không gửi `name` | Giữ nguyên tên |
| `"name": "New T-Shirt"` | Đổi tên, `version + 1` |
| `"description": ""` hoặc `null` | Xóa mô tả |
| `"price": null` | 400 |
| Không gửi field nào | 400 |
| Không có `If-Match` | 428 |
| `If-Match` cũ | 412, không ghi đè |

`POST .../archive` và `POST .../restore` không cần body. Gọi lần hai khi đã đúng status thì `200`, version giữ nguyên. Product archived không xuất hiện ở danh sách mặc định. SKU của nó vẫn bị chiếm.

## Health

`GET /health/live` trả `{"status":"ok"}` khi process đang chạy.

`GET /health/ready` ping database. Mất kết nối thì 503.
