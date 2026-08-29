package main

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/dbx"
	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/httpx"
	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/logx"
)

const (
	defaultDSN  = "tropical:tropical@tcp(mysql:3306)/tropical_inventory?parseTime=true&charset=utf8mb4"
	serviceName = "inventory-service"
	listenAddr  = ":8080"

	// Error messages
	errInvalidJSON      = "invalid json"
	errMethodNotAllowed = "method not allowed"
	errInventoryInvalid = "sku/name/unit required and stock values must be non-negative"
	errSKUExists        = "sku already exists or invalid data"
	errAdjustInvalid    = "item_id, non-zero delta, and reason are required"
	errItemNotFound     = "item not found"
	errStockNegative    = "stock adjustment would make stock negative"
	errSupplierNameReq  = "supplier name is required"
	errInternal         = "internal server error"

	// Default values
	initialStockReason   = "Initial stock"
	defaultMovementLimit = 100

	// SQL queries
	createSuppliersTable = `CREATE TABLE IF NOT EXISTS suppliers(
		id BIGINT PRIMARY KEY AUTO_INCREMENT,
		name VARCHAR(160) NOT NULL,
		contact VARCHAR(160),
		phone VARCHAR(60),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

	createInventoryItemsTable = `CREATE TABLE IF NOT EXISTS inventory_items(
		id BIGINT PRIMARY KEY AUTO_INCREMENT,
		sku VARCHAR(80) NOT NULL UNIQUE,
		name VARCHAR(180) NOT NULL,
		unit VARCHAR(40) NOT NULL,
		stock DECIMAL(12,2) NOT NULL DEFAULT 0,
		reorder_level DECIMAL(12,2) NOT NULL DEFAULT 0,
		supplier_id BIGINT NULL,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		INDEX idx_stock(stock)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

	createStockMovementsTable = `CREATE TABLE IF NOT EXISTS stock_movements(
		id BIGINT PRIMARY KEY AUTO_INCREMENT,
		item_id BIGINT NOT NULL,
		delta DECIMAL(12,2) NOT NULL,
		reason VARCHAR(220) NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_movement_item(item_id),
		INDEX idx_movement_created(created_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

	selectItemsQuery = `SELECT id,sku,name,unit,stock,reorder_level,COALESCE(supplier_id,0),updated_at
		FROM inventory_items ORDER BY name`

	insertItemQuery = `INSERT INTO inventory_items(sku,name,unit,stock,reorder_level,supplier_id)
		VALUES(?,?,?,?,?,NULLIF(?,0))`

	insertMovementQuery = `INSERT INTO stock_movements(item_id,delta,reason)
		VALUES(?,?,?)`

	selectStockForUpdateQuery = `SELECT stock FROM inventory_items WHERE id=? FOR UPDATE`

	updateStockQuery = `UPDATE inventory_items SET stock=? WHERE id=?`

	selectMovementsQuery = `SELECT m.id,m.item_id,i.name,m.delta,m.reason,m.created_at
		FROM stock_movements m
		JOIN inventory_items i ON i.id=m.item_id
		ORDER BY m.id DESC LIMIT ?`

	selectSuppliersQuery = `SELECT id,name,COALESCE(contact,''),COALESCE(phone,'')
		FROM suppliers ORDER BY name`

	insertSupplierQuery = `INSERT INTO suppliers(name,contact,phone) VALUES(?,?,?)`

	summaryAlertsQuery = `SELECT COUNT(*) FROM inventory_items WHERE stock <= reorder_level`
	summaryTotalQuery  = `SELECT COUNT(*) FROM inventory_items`
	summaryStockQuery  = `SELECT COALESCE(SUM(stock),0) FROM inventory_items`
)

// Sentinel errors untuk perbandingan errors.Is.
var (
	errSKUExistsSentinel     = errors.New(errSKUExists)
	errItemNotFoundSentinel  = errors.New(errItemNotFound)
	errStockNegativeSentinel = errors.New(errStockNegative)
)

type app struct{ db *sql.DB }

type item struct {
	ID           int64     `json:"id"`
	SKU          string    `json:"sku"`
	Name         string    `json:"name"`
	Unit         string    `json:"unit"`
	Stock        float64   `json:"stock"`
	ReorderLevel float64   `json:"reorder_level"`
	SupplierID   int64     `json:"supplier_id"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type supplier struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Contact string `json:"contact"`
	Phone   string `json:"phone"`
}

type movement struct {
	ID        int64     `json:"id"`
	ItemID    int64     `json:"item_id"`
	ItemName  string    `json:"item_name"`
	Delta     float64   `json:"delta"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

func main() {
	closeLog, logErr := logx.Configure(serviceName)
	if logErr != nil {
		log.Printf("event=log_config_error error=%q", logErr)
	} else {
		defer closeLog()
	}

	db, err := dbx.Open(httpx.Env("INVENTORY_DB_DSN", defaultDSN))
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
	mux.HandleFunc("/livez", httpx.LivenessHandler(serviceName))
	mux.HandleFunc("/readyz", httpx.ReadinessHandler(serviceName, 0, httpx.DBReadinessCheck("mysql", db)))
	mux.HandleFunc("/api/inventory", a.inventory)
	mux.HandleFunc("/api/inventory/adjust", a.adjust)
	mux.HandleFunc("/api/inventory/movements", a.movements)
	mux.HandleFunc("/api/suppliers", a.suppliers)
	mux.HandleFunc("/internal/summary", a.summary)

	log.Println(serviceName + " listening on " + listenAddr)
	server := httpx.NewServer(listenAddr, httpx.RequestLogger(serviceName, mux))
	if err := httpx.RunServer(server, serviceName, 0); err != nil {
		log.Fatal(err)
	}
}

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok", "service": serviceName})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	httpx.JSON(w, status, map[string]string{"error": msg})
}

func (a *app) migrate() error {
	queries := []string{
		createSuppliersTable,
		createInventoryItemsTable,
		createStockMovementsTable,
	}
	for _, q := range queries {
		if _, err := a.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// ============================================================
// INVENTORY ITEMS
// ============================================================

func (a *app) inventory(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.getItems(w)
	case http.MethodPost:
		a.createItem(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}

func (a *app) getItems(w http.ResponseWriter) {
	rows, err := a.db.Query(selectItemsQuery)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	defer rows.Close()

	out, err := scanItems(rows)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func scanItems(rows *sql.Rows) ([]item, error) {
	out := []item{}
	for rows.Next() {
		var x item
		if err := rows.Scan(&x.ID, &x.SKU, &x.Name, &x.Unit, &x.Stock, &x.ReorderLevel, &x.SupplierID, &x.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (a *app) createItem(w http.ResponseWriter, r *http.Request) {
	var x item
	if err := httpx.DecodeJSON(r, &x); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidJSON)
		return
	}

	x.SKU = strings.TrimSpace(x.SKU)
	x.Name = strings.TrimSpace(x.Name)
	x.Unit = strings.TrimSpace(x.Unit)

	if errMsg := validateItem(x); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	id, err := a.insertItemWithMovement(x)
	if err != nil {
		if errors.Is(err, errSKUExistsSentinel) {
			writeError(w, http.StatusConflict, errSKUExists)
		} else {
			httpx.InternalError(w, err)
		}
		return
	}

	x.ID = id
	x.UpdatedAt = time.Now()
	httpx.JSON(w, http.StatusCreated, x)
}

func validateItem(x item) string {
	if x.SKU == "" || x.Name == "" || x.Unit == "" {
		return errInventoryInvalid
	}
	if x.Stock < 0 || x.ReorderLevel < 0 {
		return errInventoryInvalid
	}
	return ""
}

func (a *app) insertItemWithMovement(x item) (int64, error) {
	tx, err := a.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(insertItemQuery, x.SKU, x.Name, x.Unit, x.Stock, x.ReorderLevel, x.SupplierID)
	if err != nil {
		if dbx.IsDuplicateKey(err) {
			return 0, errSKUExistsSentinel
		}
		return 0, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	if x.Stock > 0 {
		if _, err := tx.Exec(insertMovementQuery, id, x.Stock, initialStockReason); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// ============================================================
// ADJUST STOCK
// ============================================================

func (a *app) adjust(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	var in struct {
		ItemID int64   `json:"item_id"`
		Delta  float64 `json:"delta"`
		Reason string  `json:"reason"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil || in.ItemID == 0 || in.Delta == 0 || strings.TrimSpace(in.Reason) == "" {
		writeError(w, http.StatusBadRequest, errAdjustInvalid)
		return
	}

	result, err := a.executeAdjustment(in.ItemID, in.Delta, in.Reason)
	if err != nil {
		switch {
		case errors.Is(err, errItemNotFoundSentinel):
			writeError(w, http.StatusNotFound, errItemNotFound)
		case errors.Is(err, errStockNegativeSentinel):
			writeError(w, http.StatusBadRequest, errStockNegative)
		default:
			httpx.InternalError(w, err)
		}
		return
	}

	httpx.JSON(w, http.StatusOK, result)
}

func (a *app) executeAdjustment(itemID int64, delta float64, reason string) (map[string]any, error) {
	tx, err := a.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var current float64
	if err := tx.QueryRow(selectStockForUpdateQuery, itemID).Scan(&current); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errItemNotFoundSentinel
		}
		return nil, err
	}

	newStock := current + delta
	if newStock < 0 {
		return nil, errStockNegativeSentinel
	}

	if _, err := tx.Exec(updateStockQuery, newStock, itemID); err != nil {
		return nil, err
	}

	reason = strings.TrimSpace(reason)
	if _, err := tx.Exec(insertMovementQuery, itemID, delta, reason); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return map[string]any{
		"item_id":     itemID,
		"adjusted_by": delta,
		"stock":       newStock,
		"reason":      reason,
	}, nil
}

// ============================================================
// MOVEMENTS
// ============================================================

func (a *app) movements(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	out, err := a.getMovements()
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (a *app) getMovements() ([]movement, error) {
	rows, err := a.db.Query(selectMovementsQuery, defaultMovementLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []movement{}
	for rows.Next() {
		var x movement
		if err := rows.Scan(&x.ID, &x.ItemID, &x.ItemName, &x.Delta, &x.Reason, &x.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// ============================================================
// SUPPLIERS
// ============================================================

func (a *app) suppliers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.getSuppliers(w)
	case http.MethodPost:
		a.createSupplier(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}

func (a *app) getSuppliers(w http.ResponseWriter) {
	rows, err := a.db.Query(selectSuppliersQuery)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	defer rows.Close()

	out, err := scanSuppliers(rows)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func scanSuppliers(rows *sql.Rows) ([]supplier, error) {
	out := []supplier{}
	for rows.Next() {
		var x supplier
		if err := rows.Scan(&x.ID, &x.Name, &x.Contact, &x.Phone); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (a *app) createSupplier(w http.ResponseWriter, r *http.Request) {
	var x supplier
	if err := httpx.DecodeJSON(r, &x); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidJSON)
		return
	}

	x.Name = strings.TrimSpace(x.Name)
	x.Contact = strings.TrimSpace(x.Contact)
	x.Phone = strings.TrimSpace(x.Phone)

	if x.Name == "" {
		writeError(w, http.StatusBadRequest, errSupplierNameReq)
		return
	}

	res, err := a.db.Exec(insertSupplierQuery, x.Name, x.Contact, x.Phone)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	id, err := res.LastInsertId()
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	x.ID = id

	httpx.JSON(w, http.StatusCreated, x)
}

// ============================================================
// SUMMARY
// ============================================================

func (a *app) summary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	alerts, err := a.countAlerts()
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	total, err := a.countTotalItems()
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	stockValue, err := a.sumStock()
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"inventory_alerts":  alerts,
		"total_items":       total,
		"total_stock_units": stockValue,
	})
}

func (a *app) countAlerts() (int, error) {
	var n int
	err := a.db.QueryRow(summaryAlertsQuery).Scan(&n)
	return n, err
}

func (a *app) countTotalItems() (int, error) {
	var n int
	err := a.db.QueryRow(summaryTotalQuery).Scan(&n)
	return n, err
}

func (a *app) sumStock() (float64, error) {
	var v float64
	err := a.db.QueryRow(summaryStockQuery).Scan(&v)
	return v, err
}
