package main

import (
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
