package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
)

func TestHealthzHandler(t *testing.T) {
	w := httptest.NewRecorder()
	healthzHandler(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), serviceName) {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestValidateItem(t *testing.T) {
	valid := item{SKU: "SKU-1", Name: "Rice", Unit: "kg", Stock: 10, ReorderLevel: 2}
	if got := validateItem(valid); got != "" {
		t.Fatalf("valid item rejected: %q", got)
	}
	for _, x := range []item{
		{Name: "Rice", Unit: "kg"},
		{SKU: "SKU", Unit: "kg"},
		{SKU: "SKU", Name: "Rice"},
		{SKU: "SKU", Name: "Rice", Unit: "kg", Stock: -1},
		{SKU: "SKU", Name: "Rice", Unit: "kg", ReorderLevel: -1},
	} {
		if got := validateItem(x); got != errInventoryInvalid {
			t.Fatalf("invalid item %+v result=%q", x, got)
		}
	}
}

func TestInventoryHandlersRejectInvalidRequestsBeforeDatabaseAccess(t *testing.T) {
	a := &app{}

	w := httptest.NewRecorder()
	a.inventory(w, httptest.NewRequest(http.MethodDelete, "/api/inventory", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("inventory delete status=%d", w.Code)
	}

	w = httptest.NewRecorder()
	a.createItem(w, httptest.NewRequest(http.MethodPost, "/api/inventory", strings.NewReader(`{"sku":"","name":"","unit":""}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid item status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.adjust(w, httptest.NewRequest(http.MethodGet, "/api/inventory/adjust", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("adjust get status=%d", w.Code)
	}

	w = httptest.NewRecorder()
	a.adjust(w, httptest.NewRequest(http.MethodPost, "/api/inventory/adjust", strings.NewReader(`{"item_id":0,"delta":0,"reason":""}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid adjust status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.movements(w, httptest.NewRequest(http.MethodPost, "/api/inventory/movements", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("movements post status=%d", w.Code)
	}

	w = httptest.NewRecorder()
	a.suppliers(w, httptest.NewRequest(http.MethodDelete, "/api/suppliers", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("suppliers delete status=%d", w.Code)
	}

	w = httptest.NewRecorder()
	a.createSupplier(w, httptest.NewRequest(http.MethodPost, "/api/suppliers", strings.NewReader(`{"name":"   "}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid supplier status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestInventoryDatabasePaths(t *testing.T) {
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	db, script := openTestDB(t,
		execStep("CREATE TABLE IF NOT EXISTS suppliers", 0, 0),
		execStep("CREATE TABLE IF NOT EXISTS inventory_items", 0, 0),
		execStep("CREATE TABLE IF NOT EXISTS stock_movements", 0, 0),
		queryStep("FROM inventory_items ORDER BY name", []string{"id", "sku", "name", "unit", "stock", "reorder_level", "supplier_id", "updated_at"},
			row(int64(1), "SKU-1", "Rice", "kg", float64(10), float64(2), int64(3), now)),
		execStep("INSERT INTO inventory_items", 2, 1),
		execStep("INSERT INTO stock_movements", 3, 1),
		queryStep("SELECT stock FROM inventory_items", []string{"stock"}, row(float64(10))),
		execStep("UPDATE inventory_items SET stock", 0, 1),
		execStep("INSERT INTO stock_movements", 4, 1),
		queryStep("FROM stock_movements", []string{"id", "item_id", "item_name", "delta", "reason", "created_at"},
			row(int64(4), int64(2), "Sugar", float64(5), "restock", now)),
		queryStep("FROM suppliers ORDER BY name", []string{"id", "name", "contact", "phone"},
			row(int64(3), "Supplier A", "Alice", "0800")),
		execStep("INSERT INTO suppliers", 4, 1),
		queryStep("stock <= reorder_level", []string{"count"}, row(int64(2))),
		queryStep("SELECT COUNT(*) FROM inventory_items", []string{"count"}, row(int64(7))),
		queryStep("SUM(stock)", []string{"sum"}, row(float64(42.5))),
	)
	a := &app{db: db}

	if err := a.migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	w := httptest.NewRecorder()
	a.inventory(w, httptest.NewRequest(http.MethodGet, "/api/inventory", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"sku":"SKU-1"`) {
		t.Fatalf("inventory status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.inventory(w, httptest.NewRequest(http.MethodPost, "/api/inventory", strings.NewReader(`{"sku":" SKU-2 ","name":" Sugar ","unit":" kg ","stock":5,"reorder_level":1,"supplier_id":3}`)))
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"id":2`) {
		t.Fatalf("create item status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.adjust(w, httptest.NewRequest(http.MethodPost, "/api/inventory/adjust", strings.NewReader(`{"item_id":2,"delta":5,"reason":" restock "}`)))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"stock":15`) || !strings.Contains(w.Body.String(), `"reason":"restock"`) {
		t.Fatalf("adjust status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.movements(w, httptest.NewRequest(http.MethodGet, "/api/inventory/movements", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"item_name":"Sugar"`) {
		t.Fatalf("movements status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.suppliers(w, httptest.NewRequest(http.MethodGet, "/api/suppliers", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"Supplier A"`) {
		t.Fatalf("suppliers status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.suppliers(w, httptest.NewRequest(http.MethodPost, "/api/suppliers", strings.NewReader(`{"name":" Supplier B ","contact":" Bob ","phone":" 0812 "}`)))
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"id":4`) {
		t.Fatalf("create supplier status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.summary(w, httptest.NewRequest(http.MethodGet, "/internal/summary", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"inventory_alerts":2`) || !strings.Contains(w.Body.String(), `"total_items":7`) {
		t.Fatalf("summary status=%d body=%q", w.Code, w.Body.String())
	}
	script.assertDone(t)
}

func TestInventoryAdjustmentDomainErrors(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		db, script := openTestDB(t, queryStep("SELECT stock FROM inventory_items", []string{"stock"}))
		a := &app{db: db}
		w := httptest.NewRecorder()
		a.adjust(w, httptest.NewRequest(http.MethodPost, "/api/inventory/adjust", strings.NewReader(`{"item_id":999,"delta":1,"reason":"count"}`)))
		if w.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
		script.assertDone(t)
	})

	t.Run("negative stock", func(t *testing.T) {
		db, script := openTestDB(t, queryStep("SELECT stock FROM inventory_items", []string{"stock"}, row(float64(2))))
		a := &app{db: db}
		w := httptest.NewRecorder()
		a.adjust(w, httptest.NewRequest(http.MethodPost, "/api/inventory/adjust", strings.NewReader(`{"item_id":1,"delta":-3,"reason":"waste"}`)))
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), errStockNegative) {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
		script.assertDone(t)
	})
}

func TestInventoryDatabaseErrorsAreHandled(t *testing.T) {
	boom := errors.New("db unavailable")
	db, script := openTestDB(t,
		queryErrorStep("FROM inventory_items ORDER BY name", boom),
		queryErrorStep("FROM stock_movements", boom),
		queryErrorStep("FROM suppliers ORDER BY name", boom),
		queryErrorStep("stock <= reorder_level", boom),
	)
	a := &app{db: db}

	w := httptest.NewRecorder()
	a.getItems(w, httptest.NewRequest(http.MethodGet, "/api/inventory", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("items error status=%d", w.Code)
	}

	w = httptest.NewRecorder()
	a.movements(w, httptest.NewRequest(http.MethodGet, "/api/inventory/movements", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("movements error status=%d", w.Code)
	}

	w = httptest.NewRecorder()
	a.getSuppliers(w, httptest.NewRequest(http.MethodGet, "/api/suppliers", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("suppliers error status=%d", w.Code)
	}

	w = httptest.NewRecorder()
	a.summary(w, httptest.NewRequest(http.MethodGet, "/internal/summary", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("summary error status=%d", w.Code)
	}
	script.assertDone(t)
}

func TestInsertItemWithMovementAdditionalBranches(t *testing.T) {
	t.Run("zero stock skips initial movement", func(t *testing.T) {
		db, script := openTestDB(t, execStep("INSERT INTO inventory_items", 12, 1))
		a := &app{db: db}
		id, err := a.insertItemWithMovement(context.Background(), item{SKU: "SKU-12", Name: "Empty", Unit: "pcs", Stock: 0})
		if err != nil || id != 12 {
			t.Fatalf("id=%d err=%v", id, err)
		}
		script.assertDone(t)
	})

	t.Run("item insert error is returned", func(t *testing.T) {
		boom := errors.New("insert failed")
		db, script := openTestDB(t, execErrorStep("INSERT INTO inventory_items", boom))
		a := &app{db: db}
		if _, err := a.insertItemWithMovement(context.Background(), item{SKU: "SKU-13", Name: "Item", Unit: "pcs", Stock: 1}); !errors.Is(err, boom) {
			t.Fatalf("expected insert error, got %v", err)
		}
		script.assertDone(t)
	})

	t.Run("movement insert error is returned", func(t *testing.T) {
		boom := errors.New("movement failed")
		db, script := openTestDB(t,
			execStep("INSERT INTO inventory_items", 14, 1),
			execErrorStep("INSERT INTO stock_movements", boom),
		)
		a := &app{db: db}
		if _, err := a.insertItemWithMovement(context.Background(), item{SKU: "SKU-14", Name: "Item", Unit: "pcs", Stock: 2}); !errors.Is(err, boom) {
			t.Fatalf("expected movement error, got %v", err)
		}
		script.assertDone(t)
	})
}

func TestInventorySummaryRemainingBranches(t *testing.T) {
	t.Run("rejects unsupported method", func(t *testing.T) {
		w := httptest.NewRecorder()
		(&app{}).summary(w, httptest.NewRequest(http.MethodPost, "/internal/summary", nil))
		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
	})

	t.Run("total count error", func(t *testing.T) {
		boom := errors.New("total failed")
		db, script := openTestDB(t,
			queryStep("stock <= reorder_level", []string{"count"}, row(int64(2))),
			queryErrorStep("SELECT COUNT(*) FROM inventory_items", boom),
		)
		a := &app{db: db}
		w := httptest.NewRecorder()
		a.summary(w, httptest.NewRequest(http.MethodGet, "/internal/summary", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
		script.assertDone(t)
	})

	t.Run("stock sum error", func(t *testing.T) {
		boom := errors.New("stock sum failed")
		db, script := openTestDB(t,
			queryStep("stock <= reorder_level", []string{"count"}, row(int64(2))),
			queryStep("SELECT COUNT(*) FROM inventory_items", []string{"count"}, row(int64(7))),
			queryErrorStep("SUM(stock)", boom),
		)
		a := &app{db: db}
		w := httptest.NewRecorder()
		a.summary(w, httptest.NewRequest(http.MethodGet, "/internal/summary", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
		script.assertDone(t)
	})
}

func TestInventoryMutationDatabaseErrors(t *testing.T) {
	t.Run("adjust query error", func(t *testing.T) {
		boom := errors.New("stock lookup failed")
		db, script := openTestDB(t, queryErrorStep("SELECT stock FROM inventory_items", boom))
		a := &app{db: db}
		w := httptest.NewRecorder()
		a.adjust(w, httptest.NewRequest(http.MethodPost, "/api/inventory/adjust", strings.NewReader(`{"item_id":1,"delta":1,"reason":"count"}`)))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
		script.assertDone(t)
	})

	t.Run("adjust stock update error", func(t *testing.T) {
		boom := errors.New("stock update failed")
		db, script := openTestDB(t,
			queryStep("SELECT stock FROM inventory_items", []string{"stock"}, row(float64(2))),
			execErrorStep("UPDATE inventory_items SET stock", boom),
		)
		a := &app{db: db}
		w := httptest.NewRecorder()
		a.adjust(w, httptest.NewRequest(http.MethodPost, "/api/inventory/adjust", strings.NewReader(`{"item_id":1,"delta":1,"reason":"count"}`)))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
		script.assertDone(t)
	})

	t.Run("adjust movement insert error", func(t *testing.T) {
		boom := errors.New("movement insert failed")
		db, script := openTestDB(t,
			queryStep("SELECT stock FROM inventory_items", []string{"stock"}, row(float64(2))),
			execStep("UPDATE inventory_items SET stock", 0, 1),
			execErrorStep("INSERT INTO stock_movements", boom),
		)
		a := &app{db: db}
		w := httptest.NewRecorder()
		a.adjust(w, httptest.NewRequest(http.MethodPost, "/api/inventory/adjust", strings.NewReader(`{"item_id":1,"delta":1,"reason":" count "}`)))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
		script.assertDone(t)
	})

	t.Run("create supplier database error", func(t *testing.T) {
		boom := errors.New("supplier insert failed")
		db, script := openTestDB(t, execErrorStep("INSERT INTO suppliers", boom))
		a := &app{db: db}
		w := httptest.NewRecorder()
		a.createSupplier(w, httptest.NewRequest(http.MethodPost, "/api/suppliers", strings.NewReader(`{"name":"Supplier C","contact":"Carol","phone":"0813"}`)))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
		script.assertDone(t)
	})
}

func TestCreateItemErrorResponses(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		(&app{}).createItem(w, httptest.NewRequest(http.MethodPost, "/api/inventory", strings.NewReader(`{"sku":`)))
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), errInvalidJSON) {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
	})

	t.Run("duplicate SKU", func(t *testing.T) {
		db, script := openTestDB(t, execErrorStep("INSERT INTO inventory_items", &mysql.MySQLError{Number: 1062, Message: "duplicate"}))
		a := &app{db: db}
		w := httptest.NewRecorder()
		a.createItem(w, httptest.NewRequest(http.MethodPost, "/api/inventory", strings.NewReader(`{"sku":"SKU-DUP","name":"Rice","unit":"kg","stock":1}`)))
		if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), errSKUExists) {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
		script.assertDone(t)
	})

	t.Run("database failure", func(t *testing.T) {
		boom := errors.New("insert unavailable")
		db, script := openTestDB(t, execErrorStep("INSERT INTO inventory_items", boom))
		a := &app{db: db}
		w := httptest.NewRecorder()
		a.createItem(w, httptest.NewRequest(http.MethodPost, "/api/inventory", strings.NewReader(`{"sku":"SKU-ERR","name":"Rice","unit":"kg","stock":1}`)))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
		script.assertDone(t)
	})
}
