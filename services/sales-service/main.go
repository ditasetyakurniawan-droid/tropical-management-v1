package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/configx"
	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/dbx"
	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/httpx"
	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/logx"
)

const (
	defaultDSN  = "tropical:tropical@tcp(mysql:3306)/tropical_sales?parseTime=true&charset=utf8mb4"
	serviceName = "sales-service"
	listenAddr  = ":8080"

	defaultChannel        = "dine-in"
	dateFormat            = "2006-01-02"
	defaultSalesListLimit = 100

	// Error messages
	errInvalidJSON      = "invalid json"
	errMethodNotAllowed = "method not allowed"
	errInvalidSalesData = "invalid sales data"

	// SQL queries
	createSalesTable = `CREATE TABLE IF NOT EXISTS sales_entries(
		id BIGINT PRIMARY KEY AUTO_INCREMENT,
		business_date DATE NOT NULL,
		orders INT NOT NULL,
		revenue DECIMAL(14,2) NOT NULL,
		channel VARCHAR(60) NOT NULL DEFAULT 'dine-in',
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_business_date(business_date)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

	selectSalesQuery = `SELECT id,DATE_FORMAT(business_date,'%Y-%m-%d'),orders,revenue,channel,created_at
		FROM sales_entries ORDER BY business_date DESC,id DESC LIMIT ?`

	insertSaleQuery = `INSERT INTO sales_entries(business_date,orders,revenue,channel)
		VALUES(?,?,?,?)`

	summaryQuery = `SELECT COALESCE(SUM(revenue),0),COALESCE(SUM(orders),0)
		FROM sales_entries WHERE business_date = CURDATE()`
)

type app struct {
	db           *sql.DB
	queryTimeout time.Duration
}

type sale struct {
	ID           int64     `json:"id"`
	BusinessDate string    `json:"business_date"`
	Orders       int       `json:"orders"`
	Revenue      float64   `json:"revenue"`
	Channel      string    `json:"channel"`
	CreatedAt    time.Time `json:"created_at"`
}

func main() {
	defer logx.ConfigureBestEffort(serviceName)()

	dbConfig := dbx.RuntimeConfig()
	db, err := dbx.OpenWithConfig(configx.Sensitive("SALES_DB_DSN", defaultDSN), dbConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	a := &app{db: db, queryTimeout: dbConfig.QueryTimeout}

	mux := http.NewServeMux()
	httpx.RegisterHealthRoutes(mux, serviceName, httpx.DBReadinessCheck("mysql", db))
	mux.HandleFunc("/api/sales", a.sales)
	mux.HandleFunc("/internal/summary", a.summary)

	log.Println(serviceName + " listening on " + listenAddr)
	server := httpx.NewServer(listenAddr, httpx.RequestLogger(serviceName, mux))
	if err := httpx.RunServer(server, serviceName, 0); err != nil {
		log.Fatal(err)
	}
}

func (a *app) migrate() error {
	ctx, cancel := dbx.WithQueryTimeout(context.Background(), a.queryTimeout)
	defer cancel()
	_, err := a.db.ExecContext(ctx, createSalesTable)
	return err
}

// ============================================================
// SALES
// ============================================================

func (a *app) sales(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.getSales(w, r)
	case http.MethodPost:
		a.createSale(w, r)
	default:
		httpx.WriteError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}

func (a *app) getSales(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := dbx.WithQueryTimeout(r.Context(), a.queryTimeout)
	defer cancel()
	rows, err := a.db.QueryContext(ctx, selectSalesQuery, defaultSalesListLimit)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	defer rows.Close()

	out, err := scanSales(rows)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func scanSales(rows *sql.Rows) ([]sale, error) {
	out := []sale{}
	for rows.Next() {
		var x sale
		if err := rows.Scan(&x.ID, &x.BusinessDate, &x.Orders, &x.Revenue, &x.Channel, &x.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (a *app) createSale(w http.ResponseWriter, r *http.Request) {
	var x sale
	if err := httpx.DecodeJSON(r, &x); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, errInvalidJSON)
		return
	}

	if !normalizeAndValidateSale(&x, time.Now()) {
		httpx.WriteError(w, http.StatusBadRequest, errInvalidSalesData)
		return
	}

	ctx, cancel := dbx.WithQueryTimeout(r.Context(), a.queryTimeout)
	defer cancel()
	res, err := a.db.ExecContext(ctx, insertSaleQuery, x.BusinessDate, x.Orders, x.Revenue, x.Channel)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, errInvalidSalesData)
		return
	}

	id, err := res.LastInsertId()
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	x.ID = id
	x.CreatedAt = time.Now()
	httpx.JSON(w, http.StatusCreated, x)
}

func normalizeAndValidateSale(x *sale, now time.Time) bool {
	x.BusinessDate = strings.TrimSpace(x.BusinessDate)
	if x.BusinessDate == "" {
		x.BusinessDate = now.Format(dateFormat)
	} else if _, err := time.Parse(dateFormat, x.BusinessDate); err != nil {
		return false
	}

	x.Channel = strings.TrimSpace(x.Channel)
	if x.Channel == "" {
		x.Channel = defaultChannel
	}
	return x.Orders >= 0 && x.Revenue >= 0
}

// ============================================================
// SUMMARY
// ============================================================

func (a *app) summary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.WriteError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	revenue, orders, err := a.getTodaySummary(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"sales_today":  revenue,
		"orders_today": orders,
	})
}

func (a *app) getTodaySummary(parent context.Context) (float64, int, error) {
	ctx, cancel := dbx.WithQueryTimeout(parent, a.queryTimeout)
	defer cancel()
	var revenue float64
	var orders int
	err := a.db.QueryRowContext(ctx, summaryQuery).Scan(&revenue, &orders)
	return revenue, orders, err
}
