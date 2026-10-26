# HTTP API: Multipart upload

Các route nằm dưới `/api/v1`, cùng phiên bản với catalog.

Client không nhận upload id của R2. Server tạo multipart upload, lưu session vào `storage.upload_sessions`, và trả UUID nội bộ.

Part nhỏ nhất là 5 MiB, trừ part cuối. File từ 5 MiB trở xuống dùng một part. File lớn hơn được chia mỗi part 8 MiB. Giới hạn file là 5 GiB. Session sống 24 giờ. Presigned URL sống 15 phút.

## Tạo upload

`POST /api/v1/uploads`

```json
{
  "fileName": "photo.jpg",
  "contentType": "image/jpeg",
  "fileSize": 10485760
}
```

`201`:

```json
{
  "success": true,
  "data": {
    "uploadId": "0193f0c0-7c2a-7d3e-8b10-111111111111",
    "objectKey": "uploads/2026/10/27/0193f0c0-7c2a-7d3e-8b10-111111111111/photo.jpg",
    "partSize": 8388608,
    "partCount": 2,
    "expiresAt": "2026-10-28T09:00:00Z"
  },
  "message": "success"
}
```

`fileName` được giữ để hiển thị. Object key chỉ dùng ký tự an toàn. `contentType` là media type, ví dụ `image/jpeg`.

## Cấp presigned URL

`POST /api/v1/uploads/{uploadId}/parts/presign`

```json
{ "partNumbers": [1, 2] }
```

Mỗi lần gọi nhận tối đa 100 part. `partNumber` phải nằm trong `1..partCount` và không lặp.

`200`:

```json
{
  "success": true,
  "data": {
    "parts": [
      {
        "partNumber": 1,
        "url": "https://example.r2.cloudflarestorage.com/...",
        "expiresAt": "2026-10-27T09:15:00Z"
      }
    ]
  },
  "message": "success"
}
```

Client `PUT` binary của part đó lên `url`. Header `ETag` trong response của R2 là giá trị gửi lúc complete. Không thêm header khác, vì URL không ký những header đó.

## Hoàn tất

`POST /api/v1/uploads/{uploadId}/complete`

```json
{
  "parts": [
    { "partNumber": 1, "etag": "\"abc123\"" },
    { "partNumber": 2, "etag": "def456" }
  ]
}
```

Server gọi `ListParts`, đối chiếu đủ part, ETag và kích thước. Mọi part trừ part cuối phải đúng `partSize`. Tổng kích thước phải đúng `fileSize`. Sau đó mới `CompleteMultipartUpload`.

`200`:

```json
{
  "success": true,
  "data": {
    "uploadId": "0193f0c0-7c2a-7d3e-8b10-111111111111",
    "objectKey": "uploads/2026/10/27/0193f0c0-7c2a-7d3e-8b10-111111111111/photo.jpg",
    "url": "https://static.example.com/uploads/2026/10/27/0193f0c0-7c2a-7d3e-8b10-111111111111/photo.jpg",
    "fileSize": 10485760,
    "contentType": "image/jpeg"
  },
  "message": "success"
}
```

`url` là `STATIC_URL` cộng object key. Gọi complete lần nữa sau khi đã xong vẫn trả kết quả này.

## Hủy

`DELETE /api/v1/uploads/{uploadId}`

Hủy multipart trên R2 và đánh dấu session `aborted`. Lần gọi sau vẫn `204`. Upload đã complete trả `409`.

## Lỗi

| HTTP | Khi nào |
| --- | --- |
| 400 | Tên file, content type, part hoặc ETag không hợp lệ |
| 404 | Không có session |
| 409 | Session đã hủy, đã hết hạn, hoặc đã complete khi đang presign hay hủy |
| 500 | R2 hoặc database lỗi |
