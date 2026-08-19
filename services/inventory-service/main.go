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
	db, err := dbx.Open(httpx.Env("INVENTORY_DB_DSN", "tropical:tropical@tcp(mysql:3306)/tropical_inventory?parseTime=true&charset=utf8mb4"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	a := &app{db: db}
	if err := a.migrate(); err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, 200, map[string]string{"status": "ok", "service": "inventory-service"})
	})
	mux.HandleFunc("/api/inventory", a.inventory)
	mux.HandleFunc("/api/inventory/adjust", a.adjust)
	mux.HandleFunc("/api/inventory/movements", a.movements)
	mux.HandleFunc("/api/suppliers", a.suppliers)
	mux.HandleFunc("/internal/summary", a.summary)
	log.Println("inventory-service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func (a *app) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS suppliers(id BIGINT PRIMARY KEY AUTO_INCREMENT,name VARCHAR(160) NOT NULL,contact VARCHAR(160),phone VARCHAR(60),created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS inventory_items(id BIGINT PRIMARY KEY AUTO_INCREMENT,sku VARCHAR(80) NOT NULL UNIQUE,name VARCHAR(180) NOT NULL,unit VARCHAR(40) NOT NULL,stock DECIMAL(12,2) NOT NULL DEFAULT 0,reorder_level DECIMAL(12,2) NOT NULL DEFAULT 0,supplier_id BIGINT NULL,updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,INDEX idx_stock(stock)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS stock_movements(id BIGINT PRIMARY KEY AUTO_INCREMENT,item_id BIGINT NOT NULL,delta DECIMAL(12,2) NOT NULL,reason VARCHAR(220) NOT NULL,created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,INDEX idx_movement_item(item_id),INDEX idx_movement_created(created_at)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, q := range queries {
		if _, err := a.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (a *app) inventory(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query("SELECT id,sku,name,unit,stock,reorder_level,COALESCE(supplier_id,0),updated_at FROM inventory_items ORDER BY name")
		if err != nil {
			httpx.JSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		defer rows.Close()
		out := []item{}
		for rows.Next() {
			var x item
			if rows.Scan(&x.ID, &x.SKU, &x.Name, &x.Unit, &x.Stock, &x.ReorderLevel, &x.SupplierID, &x.UpdatedAt) == nil {
				out = append(out, x)
			}
		}
		httpx.JSON(w, 200, out)

	case http.MethodPost:
		var x item
		if httpx.DecodeJSON(r, &x) != nil {
			httpx.JSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		x.SKU = strings.TrimSpace(x.SKU)
		x.Name = strings.TrimSpace(x.Name)
		x.Unit = strings.TrimSpace(x.Unit)
		if x.SKU == "" || x.Name == "" || x.Unit == "" || x.Stock < 0 || x.ReorderLevel < 0 {
			httpx.JSON(w, 400, map[string]string{"error": "sku/name/unit required and stock values must be non-negative"})
			return
		}
		res, err := a.db.Exec("INSERT INTO inventory_items(sku,name,unit,stock,reorder_level,supplier_id) VALUES(?,?,?,?,?,NULLIF(?,0))", x.SKU, x.Name, x.Unit, x.Stock, x.ReorderLevel, x.SupplierID)
		if err != nil {
			httpx.JSON(w, 409, map[string]string{"error": "sku already exists or invalid data"})
			return
		}
		x.ID, _ = res.LastInsertId()
		x.UpdatedAt = time.Now()
		if x.Stock > 0 {
			_, _ = a.db.Exec("INSERT INTO stock_movements(item_id,delta,reason) VALUES(?,?,?)", x.ID, x.Stock, "Initial stock")
		}
		httpx.JSON(w, 201, x)

	default:
		httpx.JSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}

func (a *app) adjust(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.JSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var in struct {
		ItemID int64   `json:"item_id"`
		Delta  float64 `json:"delta"`
		Reason string  `json:"reason"`
	}
	if httpx.DecodeJSON(r, &in) != nil || in.ItemID == 0 || in.Delta == 0 || strings.TrimSpace(in.Reason) == "" {
		httpx.JSON(w, 400, map[string]string{"error": "item_id, non-zero delta, and reason are required"})
		return
	}

	tx, err := a.db.Begin()
	if err != nil {
		httpx.JSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	defer tx.Rollback()

	var current float64
	if err := tx.QueryRow("SELECT stock FROM inventory_items WHERE id=? FOR UPDATE", in.ItemID).Scan(&current); err != nil {
		if err == sql.ErrNoRows {
			httpx.JSON(w, 404, map[string]string{"error": "item not found"})
		} else {
			httpx.JSON(w, 500, map[string]string{"error": err.Error()})
		}
		return
	}
	newStock := current + in.Delta
	if newStock < 0 {
		httpx.JSON(w, 400, map[string]string{"error": "stock adjustment would make stock negative"})
		return
	}
	if _, err := tx.Exec("UPDATE inventory_items SET stock=? WHERE id=?", newStock, in.ItemID); err != nil {
		httpx.JSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if _, err := tx.Exec("INSERT INTO stock_movements(item_id,delta,reason) VALUES(?,?,?)", in.ItemID, in.Delta, strings.TrimSpace(in.Reason)); err != nil {
		httpx.JSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if err := tx.Commit(); err != nil {
		httpx.JSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	httpx.JSON(w, 200, map[string]any{"item_id": in.ItemID, "adjusted_by": in.Delta, "stock": newStock, "reason": strings.TrimSpace(in.Reason)})
}

func (a *app) movements(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.JSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	rows, err := a.db.Query(`SELECT m.id,m.item_id,i.name,m.delta,m.reason,m.created_at FROM stock_movements m JOIN inventory_items i ON i.id=m.item_id ORDER BY m.id DESC LIMIT 100`)
	if err != nil {
		httpx.JSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()
	out := []movement{}
	for rows.Next() {
		var x movement
		if rows.Scan(&x.ID, &x.ItemID, &x.ItemName, &x.Delta, &x.Reason, &x.CreatedAt) == nil {
			out = append(out, x)
		}
	}
	httpx.JSON(w, 200, out)
}

func (a *app) suppliers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query("SELECT id,name,COALESCE(contact,''),COALESCE(phone,'') FROM suppliers ORDER BY name")
		if err != nil {
			httpx.JSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		defer rows.Close()
		out := []supplier{}
		for rows.Next() {
			var x supplier
			if rows.Scan(&x.ID, &x.Name, &x.Contact, &x.Phone) == nil {
				out = append(out, x)
			}
		}
		httpx.JSON(w, 200, out)

	case http.MethodPost:
		var x supplier
		if httpx.DecodeJSON(r, &x) != nil {
			httpx.JSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		x.Name = strings.TrimSpace(x.Name)
		x.Contact = strings.TrimSpace(x.Contact)
		x.Phone = strings.TrimSpace(x.Phone)
		if x.Name == "" {
			httpx.JSON(w, 400, map[string]string{"error": "supplier name is required"})
			return
		}
		res, err := a.db.Exec("INSERT INTO suppliers(name,contact,phone) VALUES(?,?,?)", x.Name, x.Contact, x.Phone)
		if err != nil {
			httpx.JSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		x.ID, _ = res.LastInsertId()
		httpx.JSON(w, 201, x)

	default:
		httpx.JSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}

func (a *app) summary(w http.ResponseWriter, _ *http.Request) {
	var alerts, total int
	var stockValue float64
	_ = a.db.QueryRow("SELECT COUNT(*) FROM inventory_items WHERE stock<=reorder_level").Scan(&alerts)
	_ = a.db.QueryRow("SELECT COUNT(*) FROM inventory_items").Scan(&total)
	_ = a.db.QueryRow("SELECT COALESCE(SUM(stock),0) FROM inventory_items").Scan(&stockValue)
	httpx.JSON(w, 200, map[string]any{"inventory_alerts": alerts, "total_items": total, "total_stock_units": stockValue})
}
