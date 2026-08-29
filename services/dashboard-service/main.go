package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/httpx"
	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/logx"
)

const (
	defaultSalesServiceURL     = "http://sales-service:8080"
	defaultAuditServiceURL     = "http://audit-service:8080"
	defaultInventoryServiceURL = "http://inventory-service:8080"
	serviceName                = "dashboard-service"
	listenAddr                 = ":8080"
	requestTimeout             = 3 * time.Second

	summaryPath = "/internal/summary"

	errSalesUnavailable     = "sales service unavailable"
	errAuditUnavailable     = "audit service unavailable"
	errInventoryUnavailable = "inventory service unavailable"
	errMethodNotAllowed     = "method not allowed"
)

type dashboardClient struct {
	httpClient *http.Client
	sales      string
	audit      string
	inventory  string
}

type salesSummary struct {
	SalesToday  float64 `json:"sales_today"`
	OrdersToday int     `json:"orders_today"`
}

type auditSummary struct {
	AuditScore     float64 `json:"audit_score"`
	OpenIssues     int     `json:"open_issues"`
	OverdueIssues  int     `json:"overdue_issues"`
	CriticalIssues int     `json:"critical_issues"`
}

type inventorySummary struct {
	InventoryAlerts int `json:"inventory_alerts"`
	TotalItems      int `json:"total_items"`
}

type dashboardResponse struct {
	SalesToday      float64 `json:"sales_today"`
	OrdersToday     int     `json:"orders_today"`
	AuditScore      float64 `json:"audit_score"`
	OpenIssues      int     `json:"open_issues"`
	InventoryAlerts int     `json:"inventory_alerts"`
	TotalItems      int     `json:"total_items"`
}

func main() {
	closeLog, logErr := logx.Configure(serviceName)
	if logErr != nil {
		log.Printf("event=log_config_error error=%q", logErr)
	} else {
		defer closeLog()
	}

	c := &dashboardClient{
		httpClient: &http.Client{Timeout: requestTimeout},
		sales:      httpx.Env("SALES_SERVICE_URL", defaultSalesServiceURL),
		audit:      httpx.Env("AUDIT_SERVICE_URL", defaultAuditServiceURL),
		inventory:  httpx.Env("INVENTORY_SERVICE_URL", defaultInventoryServiceURL),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/livez", httpx.LivenessHandler(serviceName))
	mux.HandleFunc("/readyz", httpx.ReadinessHandler(serviceName, 0))
	mux.HandleFunc("/api/dashboard", c.handleDashboard)

	log.Println(serviceName + " listening on " + listenAddr)
	server := httpx.NewServer(listenAddr, httpx.RequestLogger(serviceName, mux))
	if err := httpx.RunServer(server, serviceName, 0); err != nil {
		log.Fatal(err)
	}
}

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": serviceName,
	})
}

// writeError mengirimkan JSON error yang konsisten.
func writeError(w http.ResponseWriter, status int, msg string) {
	httpx.JSON(w, status, map[string]string{"error": msg})
}

// fetchJSON mengambil data JSON dari URL upstream dan memasukkannya ke dest.
func (c *dashboardClient) fetchJSON(ctx context.Context, url string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if requestID := httpx.RequestID(ctx); requestID != "" {
		req.Header.Set("X-Request-ID", requestID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

// ============================================================
// DASHBOARD
// ============================================================

func (c *dashboardClient) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	sales, err := c.fetchSalesSummary(r.Context())
	if err != nil {
		log.Printf("sales service error: %v", err)
		writeError(w, http.StatusBadGateway, errSalesUnavailable)
		return
	}

	audit, err := c.fetchAuditSummary(r.Context())
	if err != nil {
		log.Printf("audit service error: %v", err)
		writeError(w, http.StatusBadGateway, errAuditUnavailable)
		return
	}

	inventory, err := c.fetchInventorySummary(r.Context())
	if err != nil {
		log.Printf("inventory service error: %v", err)
		writeError(w, http.StatusBadGateway, errInventoryUnavailable)
		return
	}

	httpx.JSON(w, http.StatusOK, buildDashboardResponse(sales, audit, inventory))
}

func (c *dashboardClient) fetchSalesSummary(ctx context.Context) (salesSummary, error) {
	var s salesSummary
	err := c.fetchJSON(ctx, c.sales+summaryPath, &s)
	return s, err
}

func (c *dashboardClient) fetchAuditSummary(ctx context.Context) (auditSummary, error) {
	var a auditSummary
	err := c.fetchJSON(ctx, c.audit+summaryPath, &a)
	return a, err
}

func (c *dashboardClient) fetchInventorySummary(ctx context.Context) (inventorySummary, error) {
	var i inventorySummary
	err := c.fetchJSON(ctx, c.inventory+summaryPath, &i)
	return i, err
}

func buildDashboardResponse(s salesSummary, a auditSummary, i inventorySummary) dashboardResponse {
	return dashboardResponse{
		SalesToday:      s.SalesToday,
		OrdersToday:     s.OrdersToday,
		AuditScore:      a.AuditScore,
		OpenIssues:      a.OpenIssues,
		InventoryAlerts: i.InventoryAlerts,
		TotalItems:      i.TotalItems,
	}
}
