package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
