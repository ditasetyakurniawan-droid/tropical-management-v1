package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealthzHandler(t *testing.T) {
	w := httptest.NewRecorder()
	healthzHandler(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), serviceName) {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestNormalizeAndValidateSale(t *testing.T) {
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)

	t.Run("defaults date and channel", func(t *testing.T) {
		x := sale{Orders: 10, Revenue: 250000}
		if !normalizeAndValidateSale(&x, now) {
			t.Fatal("valid sale rejected")
		}
		if x.BusinessDate != "2026-08-29" || x.Channel != defaultChannel {
			t.Fatalf("unexpected normalized sale: %+v", x)
		}
	})

	t.Run("trims explicit values", func(t *testing.T) {
		x := sale{BusinessDate: " 2026-08-28 ", Orders: 1, Revenue: 1, Channel: " takeaway "}
		if !normalizeAndValidateSale(&x, now) || x.BusinessDate != "2026-08-28" || x.Channel != "takeaway" {
			t.Fatalf("unexpected sale: %+v", x)
		}
	})

	for name, x := range map[string]sale{
		"invalid date":     {BusinessDate: "29-08-2026", Orders: 1, Revenue: 1},
		"negative orders":  {Orders: -1, Revenue: 1},
		"negative revenue": {Orders: 1, Revenue: -1},
	} {
		t.Run(name, func(t *testing.T) {
			if normalizeAndValidateSale(&x, now) {
				t.Fatalf("invalid sale accepted: %+v", x)
			}
		})
	}
}

func TestSalesHandlersRejectInvalidRequestsBeforeDatabaseAccess(t *testing.T) {
	a := &app{}

	w := httptest.NewRecorder()
	a.sales(w, httptest.NewRequest(http.MethodDelete, "/api/sales", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("delete status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.createSale(w, httptest.NewRequest(http.MethodPost, "/api/sales", strings.NewReader(`{"business_date":"bad","orders":1,"revenue":10}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid sale status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.createSale(w, httptest.NewRequest(http.MethodPost, "/api/sales", strings.NewReader(`{"orders":-1,"revenue":10}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("negative sale status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestMigrateAndSalesDatabasePaths(t *testing.T) {
	now := time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC)
	db, script := openTestDB(t,
		execStep("CREATE TABLE IF NOT EXISTS sales_entries", 0, 0),
		queryStep("FROM sales_entries ORDER BY", []string{"id", "business_date", "orders", "revenue", "channel", "created_at"},
			row(int64(2), "2026-08-29", int64(4), float64(125000), "dine-in", now),
			row(int64(1), "2026-08-28", int64(3), float64(90000), "takeaway", now.Add(-24*time.Hour))),
		execStep("INSERT INTO sales_entries", 3, 1),
		queryStep("FROM sales_entries WHERE business_date = CURDATE()", []string{"revenue", "orders"}, row(float64(215000), int64(7))),
	)
	a := &app{db: db}

	if err := a.migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	w := httptest.NewRecorder()
	a.sales(w, httptest.NewRequest(http.MethodGet, "/api/sales", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"business_date":"2026-08-29"`) {
		t.Fatalf("get sales status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.sales(w, httptest.NewRequest(http.MethodPost, "/api/sales", strings.NewReader(`{"business_date":"2026-08-29","orders":2,"revenue":50000,"channel":" takeaway "}`)))
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"id":3`) || !strings.Contains(w.Body.String(), `"channel":"takeaway"`) {
		t.Fatalf("create sale status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.summary(w, httptest.NewRequest(http.MethodGet, "/internal/summary", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"sales_today":215000`) || !strings.Contains(w.Body.String(), `"orders_today":7`) {
		t.Fatalf("summary status=%d body=%q", w.Code, w.Body.String())
	}
	script.assertDone(t)
}

func TestSalesDatabaseErrorsAreHandled(t *testing.T) {
	boom := errors.New("db unavailable")
	db, script := openTestDB(t,
		queryErrorStep("FROM sales_entries ORDER BY", boom),
		execErrorStep("INSERT INTO sales_entries", boom),
		queryErrorStep("FROM sales_entries WHERE business_date = CURDATE()", boom),
	)
	a := &app{db: db}

	w := httptest.NewRecorder()
	a.getSales(w, httptest.NewRequest(http.MethodGet, "/api/sales", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("get error status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.createSale(w, httptest.NewRequest(http.MethodPost, "/api/sales", strings.NewReader(`{"orders":1,"revenue":100}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("create db error status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.summary(w, httptest.NewRequest(http.MethodGet, "/internal/summary", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("summary error status=%d body=%q", w.Code, w.Body.String())
	}
	script.assertDone(t)
}

func TestSalesRemainingCoverageBranches(t *testing.T) {
	t.Run("summary rejects unsupported method", func(t *testing.T) {
		w := httptest.NewRecorder()
		(&app{}).summary(w, httptest.NewRequest(http.MethodPost, "/internal/summary", nil))
		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
	})

	t.Run("migration returns database error", func(t *testing.T) {
		boom := errors.New("migration failed")
		db, script := openTestDB(t, execErrorStep("CREATE TABLE IF NOT EXISTS sales_entries", boom))
		if err := (&app{db: db}).migrate(); !errors.Is(err, boom) {
			t.Fatalf("expected migration error, got %v", err)
		}
		script.assertDone(t)
	})

	t.Run("scan sales returns row scan error", func(t *testing.T) {
		db, script := openTestDB(t,
			queryStep("FROM sales_entries ORDER BY", []string{"id", "business_date", "orders", "revenue", "channel", "created_at"},
				row("not-an-id", "2026-08-29", int64(1), float64(10), "dine-in", time.Now())),
		)
		rows, err := db.Query(selectSalesQuery, defaultSalesListLimit)
		if err != nil {
			t.Fatalf("query: %v", err)
		}
		defer rows.Close()
		if _, err := scanSales(rows); err == nil {
			t.Fatal("expected scan error")
		}
		script.assertDone(t)
	})
}
