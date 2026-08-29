package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/httpx"
)

func TestHealthzHandler(t *testing.T) {
	w := httptest.NewRecorder()
	httpx.HealthHandler(serviceName)(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), serviceName) {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestBuildDashboardResponse(t *testing.T) {
	got := buildDashboardResponse(
		salesSummary{SalesToday: 100, OrdersToday: 3},
		auditSummary{AuditScore: 91.5, OpenIssues: 2, OverdueIssues: 1, CriticalIssues: 1},
		inventorySummary{InventoryAlerts: 4, TotalItems: 20},
	)
	if got.SalesToday != 100 || got.OrdersToday != 3 || got.AuditScore != 91.5 || got.OpenIssues != 2 || got.InventoryAlerts != 4 || got.TotalItems != 20 {
		t.Fatalf("unexpected dashboard response: %+v", got)
	}
}

func TestFetchJSON(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"sales_today":123.5,"orders_today":7}`))
		}))
		defer server.Close()
		c := &dashboardClient{httpClient: server.Client()}
		var out salesSummary
		if err := c.fetchJSON(context.Background(), server.URL, &out); err != nil {
			t.Fatal(err)
		}
		if out.SalesToday != 123.5 || out.OrdersToday != 7 {
			t.Fatalf("out=%+v", out)
		}
	})

	t.Run("unexpected status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
		defer server.Close()
		c := &dashboardClient{httpClient: server.Client()}
		if err := c.fetchJSON(context.Background(), server.URL, &salesSummary{}); err == nil || !strings.Contains(err.Error(), "unexpected status") {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`not-json`)) }))
		defer server.Close()
		c := &dashboardClient{httpClient: server.Client()}
		if err := c.fetchJSON(context.Background(), server.URL, &salesSummary{}); err == nil || !strings.Contains(err.Error(), "decode response") {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("network error", func(t *testing.T) {
		c := &dashboardClient{httpClient: &http.Client{Timeout: 10 * time.Millisecond}}
		if err := c.fetchJSON(context.Background(), "http://127.0.0.1:1", &salesSummary{}); err == nil || !strings.Contains(err.Error(), "request failed") {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestFetchJSONPropagatesRequestID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Request-ID"); got != "trace-dashboard-123" {
			t.Fatalf("X-Request-ID=%q", got)
		}
		_, _ = w.Write([]byte(`{"sales_today":1}`))
	}))
	defer server.Close()

	request := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	request.Header.Set("X-Request-ID", "trace-dashboard-123")
	var callErr error
	handler := httpx.RequestLogger("dashboard-test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := &dashboardClient{httpClient: server.Client()}
		callErr = c.fetchJSON(r.Context(), server.URL, &salesSummary{})
		w.WriteHeader(http.StatusNoContent)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if callErr != nil {
		t.Fatal(callErr)
	}
}

func TestHandleDashboard(t *testing.T) {
	makeServer := func(body string, status int) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}))
	}

	sales := makeServer(`{"sales_today":100,"orders_today":2}`, http.StatusOK)
	defer sales.Close()
	audit := makeServer(`{"audit_score":90,"open_issues":3,"overdue_issues":1,"critical_issues":1}`, http.StatusOK)
	defer audit.Close()
	inventory := makeServer(`{"inventory_alerts":4,"total_items":10}`, http.StatusOK)
	defer inventory.Close()

	c := &dashboardClient{httpClient: &http.Client{Timeout: time.Second}, sales: sales.URL, audit: audit.URL, inventory: inventory.URL}

	w := httptest.NewRecorder()
	c.handleDashboard(w, httptest.NewRequest(http.MethodGet, "/api/dashboard", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
	for _, want := range []string{`"sales_today":100`, `"audit_score":90`, `"inventory_alerts":4`} {
		if !strings.Contains(w.Body.String(), want) {
			t.Fatalf("body=%q missing %q", w.Body.String(), want)
		}
	}

	w = httptest.NewRecorder()
	c.handleDashboard(w, httptest.NewRequest(http.MethodPost, "/api/dashboard", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status=%d", w.Code)
	}
}

func TestHandleDashboardUpstreamFailures(t *testing.T) {
	okSales := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"sales_today":1}`)) }))
	defer okSales.Close()
	okAudit := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"audit_score":1}`)) }))
	defer okAudit.Close()
	okInventory := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"inventory_alerts":1}`)) }))
	defer okInventory.Close()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer bad.Close()

	for name, c := range map[string]dashboardClient{
		"sales":     {httpClient: http.DefaultClient, sales: bad.URL, audit: okAudit.URL, inventory: okInventory.URL},
		"audit":     {httpClient: http.DefaultClient, sales: okSales.URL, audit: bad.URL, inventory: okInventory.URL},
		"inventory": {httpClient: http.DefaultClient, sales: okSales.URL, audit: okAudit.URL, inventory: bad.URL},
	} {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c.handleDashboard(w, httptest.NewRequest(http.MethodGet, "/api/dashboard", nil))
			if w.Code != http.StatusBadGateway {
				t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
			}
		})
	}
}
