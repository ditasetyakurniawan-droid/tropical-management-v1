package main

import (
	"database/sql"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/dbx"
	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/httpx"
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
	db *sql.DB
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
	db, err := dbx.Open(httpx.Env("SALES_DB_DSN", defaultDSN))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	a := &app{db: db}
	if err := a.migrate(); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/api/sales", a.sales)
	mux.HandleFunc("/internal/summary", a.summary)

	log.Println(serviceName + " listening on " + listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, mux))
}

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": serviceName,
	})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	httpx.JSON(w, status, map[string]string{"error": msg})
}

func (a *app) migrate() error {
	_, err := a.db.Exec(createSalesTable)
	return err
}

// ============================================================
// SALES
// ============================================================

func (a *app) sales(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.getSales(w)
	case http.MethodPost:
		a.createSale(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}

func (a *app) getSales(w http.ResponseWriter) {
	rows, err := a.db.Query(selectSalesQuery, defaultSalesListLimit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	out, err := scanSales(rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
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
		writeError(w, http.StatusBadRequest, errInvalidJSON)
		return
	}

	x.BusinessDate = strings.TrimSpace(x.BusinessDate)
	if x.BusinessDate == "" {
		x.BusinessDate = time.Now().Format(dateFormat)
	}

	x.Channel = strings.TrimSpace(x.Channel)
	if x.Channel == "" {
		x.Channel = defaultChannel
	}

	res, err := a.db.Exec(insertSaleQuery, x.BusinessDate, x.Orders, x.Revenue, x.Channel)
	if err != nil {
		writeError(w, http.StatusBadRequest, errInvalidSalesData)
		return
	}

	id, err := res.LastInsertId()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	x.ID = id
	x.CreatedAt = time.Now()
	httpx.JSON(w, http.StatusCreated, x)
}

// ============================================================
// SUMMARY
// ============================================================

func (a *app) summary(w http.ResponseWriter, _ *http.Request) {
	revenue, orders, err := a.getTodaySummary()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"sales_today":  revenue,
		"orders_today": orders,
	})
}

func (a *app) getTodaySummary() (float64, int, error) {
	var revenue float64
	var orders int
	err := a.db.QueryRow(summaryQuery).Scan(&revenue, &orders)
	return revenue, orders, err
}
