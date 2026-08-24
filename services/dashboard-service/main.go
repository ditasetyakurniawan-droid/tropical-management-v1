package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/httpx"
)

const (
	defaultSalesServiceURL     = "http://sales-service:8080"
	defaultAuditServiceURL     = "http://audit-service:8080"
	defaultInventoryServiceURL = "http://inventory-service:8080"
	serviceName                = "dashboard-service"
	listenAddr                 = ":8080"
	requestTimeout             = 3 * time.Second

	errSalesUnavailable     = "sales service unavailable"
	errAuditUnavailable     = "audit service unavailable"
	errInventoryUnavailable = "inventory service unavailable"
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
	c := &dashboardClient{
		httpClient: &http.Client{Timeout: requestTimeout},
		sales:      httpx.Env("SALES_SERVICE_URL", defaultSalesServiceURL),
		audit:      httpx.Env("AUDIT_SERVICE_URL", defaultAuditServiceURL),
		inventory:  httpx.Env("INVENTORY_SERVICE_URL", defaultInventoryServiceURL),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"service": serviceName,
		})
	})
	mux.HandleFunc("/api/dashboard", c.dashboard)

	log.Println(serviceName + " listening on " + listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, mux))
}

// writeError mengirimkan JSON error yang konsisten.
func writeError(w http.ResponseWriter, status int, msg string) {
	httpx.JSON(w, status, map[string]string{"error": msg})
}

// fetchJSON mengambil data JSON dari URL upstream dan memasukkannya ke dest.
func (c *dashboardClient) fetchJSON(url string, dest any) error {
	resp, err := c.httpClient.Get(url)
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

func (c *dashboardClient) dashboard(w http.ResponseWriter, _ *http.Request) {
	var sales salesSummary
	var audit auditSummary
	var inventory inventorySummary

	if err := c.fetchJSON(c.sales+"/internal/summary", &sales); err != nil {
		log.Printf("sales service error: %v", err)
		writeError(w, http.StatusBadGateway, errSalesUnavailable)
		return
	}

	if err := c.fetchJSON(c.audit+"/internal/summary", &audit); err != nil {
		log.Printf("audit service error: %v", err)
		writeError(w, http.StatusBadGateway, errAuditUnavailable)
		return
	}

	if err := c.fetchJSON(c.inventory+"/internal/summary", &inventory); err != nil {
		log.Printf("inventory service error: %v", err)
		writeError(w, http.StatusBadGateway, errInventoryUnavailable)
		return
	}

	out := dashboardResponse{
		SalesToday:      sales.SalesToday,
		OrdersToday:     sales.OrdersToday,
		AuditScore:      audit.AuditScore,
		OpenIssues:      audit.OpenIssues,
		InventoryAlerts: inventory.InventoryAlerts,
		TotalItems:      inventory.TotalItems,
	}

	httpx.JSON(w, http.StatusOK, out)
}
