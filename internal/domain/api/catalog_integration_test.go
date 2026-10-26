package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/phamtuanha21092003/ecommerce-be/internal/testutil"
	"github.com/phamtuanha21092003/ecommerce-be/platform/database"
)

func TestCatalogAPI(t *testing.T) {
	server, db := newCatalogServer(t)

	storeA := createStore(t, server, "Store A")
	storeB := createStore(t, server, "Store B")

	created := createProduct(t, server, storeA, map[string]any{
		"sku":         "tshirt-001",
		"name":        " T-Shirt ",
		"description": " Cotton ",
		"price":       "19.99",
		"currency":    "usd",
	})
	if created.SKU != "TSHIRT-001" || created.Name != "T-Shirt" || created.Currency != "USD" {
		t.Fatalf("normalized product = %+v", created)
	}
	if created.Description == nil || *created.Description != "Cotton" {
		t.Fatalf("description = %v", created.Description)
	}
	if created.Version != 1 || created.Status != "active" {
		t.Fatalf("created state = %+v", created)
	}
	if !created.Price.Equal(decimal.RequireFromString("19.99")) {
		t.Fatalf("price = %s", created.Price)
	}

	sameSKU := postProduct(t, server, storeA, map[string]any{
		"sku": "TSHIRT-001", "name": "Other", "price": 10, "currency": "USD",
	})
	if sameSKU.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate sku status = %d body = %s", sameSKU.StatusCode, sameSKU.Body)
	}

	otherStore := createProduct(t, server, storeB, map[string]any{
		"sku": "TSHIRT-001", "name": "Other store", "price": 10, "currency": "USD",
	})
	if otherStore.StoreID != storeB {
		t.Fatalf("store = %s", otherStore.StoreID)
	}

	negative := postProduct(t, server, storeA, map[string]any{
		"sku": "NEG-1", "name": "Bad", "price": -1, "currency": "USD",
	})
	if negative.StatusCode != http.StatusBadRequest {
		t.Fatalf("negative price status = %d", negative.StatusCode)
	}

	_, err := db.Exec(`
		INSERT INTO catalog.products (store_id, sku, name, price, currency)
		VALUES ($1, 'NEG-DB', 'Bad', -1, 'USD')
	`, storeA)
	if err == nil || !database.IsCheckViolation(err) {
		t.Fatalf("database accepted a negative price: %v", err)
	}

	missingStore := postProduct(t, server, uuid.New(), map[string]any{
		"sku": "ORPHAN-1", "name": "Orphan", "price": 1, "currency": "USD",
	})
	if missingStore.StatusCode != http.StatusNotFound {
		t.Fatalf("missing store status = %d body = %s", missingStore.StatusCode, missingStore.Body)
	}

	foreign := getJSON[productBody](t, server, fmt.Sprintf("/api/v1/stores/%s/products/%s", storeB, created.ID), nil)
	if foreign.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-store get status = %d", foreign.StatusCode)
	}

	got := getJSON[productBody](t, server, fmt.Sprintf("/api/v1/stores/%s/products/%s", storeA, created.ID), nil)
	if got.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d", got.StatusCode)
	}
	if got.Header.Get("ETag") != `"1"` {
		t.Fatalf("etag = %q", got.Header.Get("ETag"))
	}

	noMatch := patchProduct(t, server, storeA, created.ID, "", map[string]any{"name": "Nope"})
	if noMatch.StatusCode != http.StatusPreconditionRequired {
		t.Fatalf("missing If-Match status = %d", noMatch.StatusCode)
	}

	renamed := patchProduct(t, server, storeA, created.ID, `"1"`, map[string]any{"name": "New T-Shirt"})
	if renamed.StatusCode != http.StatusOK || renamed.Data.Name != "New T-Shirt" || renamed.Data.Version != 2 {
		t.Fatalf("rename = %d %+v", renamed.StatusCode, renamed.Data)
	}
	if renamed.Data.Description == nil || *renamed.Data.Description != "Cotton" {
		t.Fatalf("omitted description changed: %+v", renamed.Data.Description)
	}

	cleared := patchProduct(t, server, storeA, created.ID, renamed.Header.Get("ETag"), map[string]any{"description": nil})
	if cleared.StatusCode != http.StatusOK || cleared.Data.Description != nil || cleared.Data.Version != 3 {
		t.Fatalf("clear description = %d %+v", cleared.StatusCode, cleared.Data)
	}

	nullPrice := patchProduct(t, server, storeA, created.ID, cleared.Header.Get("ETag"), map[string]any{"price": nil})
	if nullPrice.StatusCode != http.StatusBadRequest {
		t.Fatalf("null price status = %d body = %s", nullPrice.StatusCode, nullPrice.Body)
	}

	stale := patchProduct(t, server, storeA, created.ID, `"1"`, map[string]any{"price": "5"})
	if stale.StatusCode != http.StatusPreconditionFailed {
		t.Fatalf("stale update status = %d", stale.StatusCode)
	}

	runLostUpdate(t, server, storeA, created.ID, cleared.Data.Version)

	archived := postEmpty(t, server, fmt.Sprintf("/api/v1/stores/%s/products/%s/archive", storeA, created.ID))
	if archived.StatusCode != http.StatusOK || archived.Data.Status != "archived" || archived.Data.ArchivedAt == nil {
		t.Fatalf("archive = %d %+v", archived.StatusCode, archived.Data)
	}
	again := postEmpty(t, server, fmt.Sprintf("/api/v1/stores/%s/products/%s/archive", storeA, created.ID))
	if again.StatusCode != http.StatusOK || again.Data.Version != archived.Data.Version {
		t.Fatalf("second archive changed version: %d -> %d", archived.Data.Version, again.Data.Version)
	}

	reused := postProduct(t, server, storeA, map[string]any{
		"sku": "TSHIRT-001", "name": "Reuse", "price": 1, "currency": "USD",
	})
	if reused.StatusCode != http.StatusConflict {
		t.Fatalf("archived sku was reused, status = %d", reused.StatusCode)
	}

	defaultList := getJSON[pageBody](t, server, fmt.Sprintf("/api/v1/stores/%s/products", storeA), nil)
	if defaultList.StatusCode != http.StatusOK {
		t.Fatal(defaultList.Body)
	}
	for _, item := range defaultList.Data.Items {
		if item.ID == created.ID {
			t.Fatal("archived product appeared in the default list")
		}
	}

	archivedList := getJSON[pageBody](t, server, fmt.Sprintf("/api/v1/stores/%s/products?status=archived", storeA), nil)
	if len(archivedList.Data.Items) != 1 || archivedList.Data.Items[0].ID != created.ID {
		t.Fatalf("archived list = %+v", archivedList.Data.Items)
	}

	restored := postEmpty(t, server, fmt.Sprintf("/api/v1/stores/%s/products/%s/restore", storeA, created.ID))
	if restored.StatusCode != http.StatusOK || restored.Data.Status != "active" || restored.Data.ArchivedAt != nil {
		t.Fatalf("restore = %d %+v", restored.StatusCode, restored.Data)
	}
	if restored.Data.Version != archived.Data.Version+1 {
		t.Fatalf("restore version = %d, archive version = %d", restored.Data.Version, archived.Data.Version)
	}

	badCursor := getJSON[pageBody](t, server, fmt.Sprintf("/api/v1/stores/%s/products?cursor=!!!", storeA), nil)
	if badCursor.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad cursor status = %d", badCursor.StatusCode)
	}

	createProduct(t, server, storeA, map[string]any{
		"sku": "COTTON-1", "name": "100% Cotton", "price": 8, "currency": "USD",
	})
	createProduct(t, server, storeA, map[string]any{
		"sku": "PLAIN-1", "name": "Plain", "price": 8, "currency": "USD",
	})
	found := getJSON[pageBody](t, server, fmt.Sprintf("/api/v1/stores/%s/products?q=%s", storeA, "100%25"), nil)
	if len(found.Data.Items) != 1 || found.Data.Items[0].SKU != "COTTON-1" {
		t.Fatalf("search items = %+v", found.Data.Items)
	}

	assertKeysetPages(t, server, db, storeA)
	assertSingleSKUWinner(t, server, storeA)

	if _, err := db.Exec(`UPDATE catalog.products SET deleted_at = now() WHERE store_id = $1 AND sku = 'TSHIRT-001'`, storeA); err != nil {
		t.Fatal(err)
	}
	released := postProduct(t, server, storeA, map[string]any{
		"sku": "TSHIRT-001", "name": "Released", "price": 1, "currency": "USD",
	})
	if released.StatusCode != http.StatusCreated {
		t.Fatalf("soft-deleted sku was not released, status = %d body = %s", released.StatusCode, released.Body)
	}
}

func assertKeysetPages(t *testing.T, server *httptest.Server, db *database.SqlxDatabase, storeID uuid.UUID) {
	t.Helper()

	const pageSize = 2
	const total = 5
	ids := make([]uuid.UUID, 0, total)
	for i := 0; i < total; i++ {
		item := createProduct(t, server, storeID, map[string]any{
			"sku": fmt.Sprintf("PAGE-%02d", i), "name": "Page", "price": 1, "currency": "USD",
		})
		ids = append(ids, item.ID)
	}

	if _, err := db.Exec(`UPDATE catalog.products SET created_at = $1 WHERE sku LIKE 'PAGE-%'`, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}

	seen := map[uuid.UUID]struct{}{}
	cursor := ""
	for page := 0; page < 10; page++ {
		url := fmt.Sprintf("/api/v1/stores/%s/products?limit=%d", storeID, pageSize)
		if cursor != "" {
			url += "&cursor=" + cursor
		}
		res := getJSON[pageBody](t, server, url, nil)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("page status = %d body = %s", res.StatusCode, res.Body)
		}
		if len(res.Data.Items) == 0 {
			t.Fatal("empty page before the catalog was exhausted")
		}
		for _, item := range res.Data.Items {
			if _, ok := seen[item.ID]; ok {
				t.Fatalf("duplicate id %s", item.ID)
			}
			seen[item.ID] = struct{}{}
		}
		if !res.Data.HasNext {
			if res.Data.NextCursor != nil {
				t.Fatal("nextCursor set on the last page")
			}
			break
		}
		if res.Data.NextCursor == nil || *res.Data.NextCursor == cursor {
			t.Fatal("cursor did not advance")
		}
		cursor = *res.Data.NextCursor
	}

	for _, id := range ids {
		if _, ok := seen[id]; !ok {
			t.Fatalf("missing paged product %s", id)
		}
	}
}

func assertSingleSKUWinner(t *testing.T, server *httptest.Server, storeID uuid.UUID) {
	t.Helper()

	const n = 20
	var (
		okCount       atomic.Int32
		conflictCount atomic.Int32
		wg            sync.WaitGroup
		start         = make(chan struct{})
	)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			res := postProduct(t, server, storeID, map[string]any{
				"sku": "RACE-1", "name": "Race", "price": 3, "currency": "USD",
			})
			switch res.StatusCode {
			case http.StatusCreated:
				okCount.Add(1)
			case http.StatusConflict:
				conflictCount.Add(1)
			default:
				t.Errorf("unexpected race status %d body %s", res.StatusCode, res.Body)
			}
		}()
	}
	close(start)
	wg.Wait()

	if okCount.Load() != 1 || conflictCount.Load() != int32(n-1) {
		t.Fatalf("created = %d conflict = %d", okCount.Load(), conflictCount.Load())
	}
}

func runLostUpdate(t *testing.T, server *httptest.Server, storeID, productID uuid.UUID, version int) {
	t.Helper()

	etag := strconv.Quote(strconv.Itoa(version))
	bodies := []map[string]any{{"name": "From A"}, {"price": "42"}}
	results := make([]httpResult[productBody], len(bodies))
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i, body := range bodies {
		wg.Add(1)
		go func(i int, body map[string]any) {
			defer wg.Done()
			<-start
			results[i] = patchProduct(t, server, storeID, productID, etag, body)
		}(i, body)
	}
	close(start)
	wg.Wait()

	success := 0
	conflict := 0
	for _, res := range results {
		switch res.StatusCode {
		case http.StatusOK:
			success++
		case http.StatusPreconditionFailed:
			conflict++
		default:
			t.Fatalf("lost update status = %d body = %s", res.StatusCode, res.Body)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("lost update success = %d conflict = %d", success, conflict)
	}

	current := getJSON[productBody](t, server, fmt.Sprintf("/api/v1/stores/%s/products/%s", storeID, productID), nil)
	nameChanged := current.Data.Name == "From A"
	priceChanged := current.Data.Price.Equal(decimal.RequireFromString("42"))
	if nameChanged == priceChanged {
		t.Fatalf("both or neither update landed: %+v", current.Data)
	}
}

func newCatalogServer(t *testing.T) (*httptest.Server, *database.SqlxDatabase) {
	t.Helper()
	db := testutil.Open(t)
	testutil.Reset(t, db)

	log := zap.NewNop()
	repos := newRepository(db, log)
	services := newService(repos, log, nil, "")
	controllers := newController(log, services)
	server := httptest.NewServer(newRouter(log, db, controllers))
	t.Cleanup(server.Close)
	return server, db
}

type productBody struct {
	ID          uuid.UUID       `json:"id"`
	StoreID     uuid.UUID       `json:"storeId"`
	SKU         string          `json:"sku"`
	Name        string          `json:"name"`
	Description *string         `json:"description"`
	Price       decimal.Decimal `json:"price"`
	Currency    string          `json:"currency"`
	Status      string          `json:"status"`
	ArchivedAt  *time.Time      `json:"archivedAt"`
	Version     int             `json:"version"`
}

type pageBody struct {
	Items      []productBody `json:"items"`
	Limit      int           `json:"limit"`
	HasNext    bool          `json:"hasNext"`
	NextCursor *string       `json:"nextCursor"`
}

type storeBody struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type envelope[T any] struct {
	Success bool   `json:"success"`
	Data    T      `json:"data"`
	Message string `json:"message"`
}

type httpResult[T any] struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	Data       T
}

func createStore(t *testing.T, server *httptest.Server, name string) uuid.UUID {
	t.Helper()
	res := doJSON[storeBody](t, server, http.MethodPost, "/api/v1/stores", map[string]any{"name": name}, nil)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create store status = %d body = %s", res.StatusCode, res.Body)
	}
	return res.Data.ID
}

func createProduct(t *testing.T, server *httptest.Server, storeID uuid.UUID, body map[string]any) productBody {
	t.Helper()
	res := postProduct(t, server, storeID, body)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create product status = %d body = %s", res.StatusCode, res.Body)
	}
	return res.Data
}

func postProduct(t *testing.T, server *httptest.Server, storeID uuid.UUID, body map[string]any) httpResult[productBody] {
	t.Helper()
	return doJSON[productBody](t, server, http.MethodPost, fmt.Sprintf("/api/v1/stores/%s/products", storeID), body, nil)
}

func patchProduct(t *testing.T, server *httptest.Server, storeID, productID uuid.UUID, ifMatch string, body map[string]any) httpResult[productBody] {
	t.Helper()
	headers := map[string]string{}
	if ifMatch != "" {
		headers["If-Match"] = ifMatch
	}
	return doJSON[productBody](t, server, http.MethodPatch, fmt.Sprintf("/api/v1/stores/%s/products/%s", storeID, productID), body, headers)
}

func postEmpty(t *testing.T, server *httptest.Server, path string) httpResult[productBody] {
	t.Helper()
	return doJSON[productBody](t, server, http.MethodPost, path, nil, nil)
}

func getJSON[T any](t *testing.T, server *httptest.Server, path string, headers map[string]string) httpResult[T] {
	t.Helper()
	return doJSON[T](t, server, http.MethodGet, path, nil, headers)
}

func doJSON[T any](t *testing.T, server *httptest.Server, method, path string, body any, headers map[string]string) httpResult[T] {
	t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, server.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	var result httpResult[T]
	result.StatusCode = res.StatusCode
	result.Header = res.Header
	result.Body = raw
	if res.StatusCode < 300 && len(raw) > 0 {
		var payload envelope[T]
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatalf("decode %s: %v body = %s", path, err, raw)
		}
		result.Data = payload.Data
	}
	return result
}
