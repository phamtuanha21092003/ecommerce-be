# Commerce Platform API

Backend Go cho catalog nhiều cửa hàng. Phase này hoàn thiện Store và Product: constraint ở PostgreSQL, keyset pagination, và optimistic locking. Inventory, Order, RabbitMQ chưa nằm trong phạm vi.

## Layer

Request đi một chiều:

`controller` nhận HTTP và DTO, `service` giữ quy tắc nghiệp vụ, `repository` giữ SQL. Repository trả về struct trong `internal/mapper`, không trả `internal/model`. Model chỉ là dữ liệu ghi. Response đi ra ngoài qua `internal/dto`.

Danh sách product luôn dùng một envelope cố định, `base.CursorPagingResponseDto`: `items`, `limit`, `hasNext`, `nextCursor`.

Upload file đi qua multipart trên R2. API chỉ trả `uploadId` nội bộ. Session nằm ở `storage.upload_sessions`; `s3_upload_id` không ra khỏi server.

## Chạy local

PostgreSQL 18 là bắt buộc vì schema dùng `uuidv7()`.

Máy này đã có Postgres 18 ở cổng 5432, nên `environment/.env.api` trỏ tới database `commerce` và role `ecommerce_app`. File đó không được commit.

Nếu dựng database mới bằng Compose:

```bash
docker compose up -d
cp environment/.env.api.example environment/.env.api
```

Example dùng `postgres:postgres` và database `commerce`, đúng với `docker-compose.yml`.

Migration:

```bash
export POSTGRESQL_URI='postgresql://ecommerce_app:ecommerce_app@localhost:5432/commerce?sslmode=disable'
migrate -path migrations -database "$POSTGRESQL_URI" up
```

`golang-migrate` chấp nhận scheme `postgresql://` với driver pgx. Không chạy migration lúc process API khởi động.

API:

```bash
go run ./cmd/api
```

Air đọc `.air.toml` và build `./cmd/api`. Health check: `GET /health/live` và `GET /health/ready`.

## Test

```bash
go test ./...
TEST_POSTGRESQL_URI="$POSTGRESQL_URI" go test ./internal/domain/api/ ./internal/base/
```

Không có `TEST_POSTGRESQL_URI` thì integration test được skip. Bộ test đó chạy trên PostgreSQL thật: SKU trùng, giá âm, 20 request tạo cùng SKU, lost update, cursor hỏng, và keyset khi nhiều dòng cùng `created_at`.

## Tài liệu

- [Yêu cầu Product v1](docs/requirements/product.md)
- [ADR các quyết định catalog](docs/adr/0001-product-catalog-decisions.md)
- [HTTP API](docs/api/products.md)
- [Upload API](docs/api/uploads.md)
