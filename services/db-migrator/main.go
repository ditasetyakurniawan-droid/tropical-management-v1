package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/dbx"
	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/httpx"
	migrator "github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/migrate"
)

type targetConfig struct {
	name      string
	env       string
	verifySQL []string
}

var targets = []targetConfig{
	{
		name: "auth",
		env:  "AUTH_DB_DSN",
		verifySQL: []string{
			`SELECT id,name,email,password_hash,role,active,created_at FROM users LIMIT 0`,
		},
	},
	{
		name: "audit",
		env:  "AUDIT_DB_DSN",
		verifySQL: []string{
			`SELECT id,restaurant,auditor,cleanliness,sop,food_quality,score,notes,created_at FROM audits LIMIT 0`,
			`SELECT id,audit_id,title,severity,status,assigned_to,due_date,corrective_action,created_at,updated_at FROM issues LIMIT 0`,
		},
	},
	{
		name: "inventory",
		env:  "INVENTORY_DB_DSN",
		verifySQL: []string{
			`SELECT id,name,contact,phone,created_at FROM suppliers LIMIT 0`,
			`SELECT id,sku,name,unit,stock,reorder_level,supplier_id,updated_at FROM inventory_items LIMIT 0`,
			`SELECT id,item_id,delta,reason,created_at FROM stock_movements LIMIT 0`,
		},
	},
	{
		name: "sales",
		env:  "SALES_DB_DSN",
		verifySQL: []string{
			`SELECT id,business_date,orders,revenue,channel,created_at FROM sales_entries LIMIT 0`,
		},
	},
	{
		name: "chat",
		env:  "CHAT_DB_DSN",
		verifySQL: []string{
			`SELECT id,user_id,user_name,role,body,created_at FROM chat_messages LIMIT 0`,
		},
	},
	{
		name: "workforce",
		env:  "WORKFORCE_DB_DSN",
		verifySQL: []string{
			`SELECT id,employee_id,employee_name,shift_date,start_time,end_time,station,status,notes,created_by_id,created_by_name,created_at FROM shifts LIMIT 0`,
			`SELECT id,shift_id,employee_id,employee_name,work_date,clock_in,clock_out,status,notes,created_at FROM attendance LIMIT 0`,
			`SELECT id,employee_id,employee_name,start_date,end_date,type,reason,status,reviewed_by_id,reviewed_by_name,review_note,created_at,updated_at FROM time_off_requests LIMIT 0`,
			`SELECT id,shift_date,title,station,assigned_to_id,assigned_to_name,priority,status,created_by_id,created_by_name,completed_by_id,completed_by_name,completed_at,created_at FROM shift_tasks LIMIT 0`,
		},
	},
}

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds|log.LUTC)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	for _, cfg := range targets {
		dsn, err := requiredConfig(cfg.env)
		if err != nil {
			logger.Fatal(err)
		}
		db, err := dbx.Open(dsn)
		if err != nil {
			logger.Fatalf("event=migration_db_connect_failed target=%q error=%q", cfg.name, err)
		}

		logger.Printf("event=migration_target_started target=%q", cfg.name)
		err = migrator.Up(ctx, migrator.Target{Name: cfg.name, DB: db, VerifySQL: cfg.verifySQL})
		closeErr := db.Close()
		if err != nil {
			logger.Fatalf("event=migration_target_failed target=%q error=%q", cfg.name, err)
		}
		if closeErr != nil {
			logger.Fatalf("event=migration_db_close_failed target=%q error=%q", cfg.name, closeErr)
		}
		logger.Printf("event=migration_target_completed target=%q", cfg.name)
	}

	logger.Printf("event=migration_run_completed targets=%d", len(targets))
}

func requiredConfig(key string) (string, error) {
	value := strings.TrimSpace(httpx.Env(key, ""))
	if value == "" {
		return "", fmt.Errorf("required database configuration %s or %s_FILE is missing", key, key)
	}
	return value, nil
}
