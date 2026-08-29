package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/configx"
	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/dbx"
	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/httpx"
	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/logx"
)

const (
	serviceName = "workforce-service"
	listenAddr  = ":8080"
	defaultDSN  = "tropical:tropical@tcp(mysql:3306)/tropical_workforce?parseTime=true&charset=utf8mb4"

	roleAdmin   = "admin"
	roleAuditor = "auditor"
	roleStaff   = "staff"

	statusScheduled = "scheduled"
	statusCompleted = "completed"
	statusOpen      = "open"
	statusDone      = "done"
	statusPending   = "pending"
	statusApproved  = "approved"
	statusRejected  = "rejected"
)

const (
	selectShiftsBase = `SELECT id,employee_id,employee_name,shift_date,TIME_FORMAT(start_time,'%H:%i'),TIME_FORMAT(end_time,'%H:%i'),station,status,notes,created_by_name,created_at FROM shifts WHERE shift_date BETWEEN ? AND ?`
	insertShift      = `INSERT INTO shifts(employee_id,employee_name,shift_date,start_time,end_time,station,status,notes,created_by_id,created_by_name) VALUES(?,?,?,?,?,?,?,?,?,?)`

	selectAttendanceBase = `SELECT a.id,a.shift_id,a.employee_id,a.employee_name,a.work_date,a.clock_in,a.clock_out,a.status,a.notes FROM attendance a WHERE a.work_date BETWEEN ? AND ?`
	selectTodayShift     = `SELECT id,employee_name,station,TIME_FORMAT(start_time,'%H:%i'),TIME_FORMAT(end_time,'%H:%i') FROM shifts WHERE employee_id=? AND shift_date=CURDATE() AND status='scheduled' ORDER BY start_time LIMIT 1`
	insertAttendance     = `INSERT INTO attendance(shift_id,employee_id,employee_name,work_date,clock_in,status) VALUES(?,?,?,CURDATE(),NOW(),'present')`
	clockOutAttendance   = `UPDATE attendance SET clock_out=NOW(),status='completed' WHERE employee_id=? AND work_date=CURDATE() AND clock_out IS NULL ORDER BY id DESC LIMIT 1`

	selectTimeOffBase = `SELECT id,employee_id,employee_name,start_date,end_date,type,reason,status,reviewed_by_name,review_note,created_at,updated_at FROM time_off_requests`
	insertTimeOff     = `INSERT INTO time_off_requests(employee_id,employee_name,start_date,end_date,type,reason,status) VALUES(?,?,?,?,?,?,'pending')`
	reviewTimeOff     = `UPDATE time_off_requests SET status=?,reviewed_by_id=?,reviewed_by_name=?,review_note=?,updated_at=CURRENT_TIMESTAMP WHERE id=? AND status='pending'`

	selectTasksBase   = `SELECT id,shift_date,title,station,assigned_to_id,assigned_to_name,priority,status,created_by_name,completed_by_name,completed_at,created_at FROM shift_tasks WHERE shift_date BETWEEN ? AND ?`
	insertTask        = `INSERT INTO shift_tasks(shift_date,title,station,assigned_to_id,assigned_to_name,priority,status,created_by_id,created_by_name) VALUES(?,?,?,?,?,?,'open',?,?)`
	completeTaskStaff = `UPDATE shift_tasks SET status=?,completed_by_id=?,completed_by_name=?,completed_at=IF(?='done',NOW(),NULL) WHERE id=? AND (assigned_to_id=0 OR assigned_to_id=?)`
	completeTaskMgr   = `UPDATE shift_tasks SET status=?,completed_by_id=?,completed_by_name=?,completed_at=IF(?='done',NOW(),NULL) WHERE id=?`
)

type app struct {
	db           *sql.DB
	queryTimeout time.Duration
}

type identity struct {
	ID   int64
	Name string
	Role string
}

type shift struct {
	ID            int64     `json:"id"`
	EmployeeID    int64     `json:"employee_id"`
	EmployeeName  string    `json:"employee_name"`
	ShiftDate     string    `json:"shift_date"`
	StartTime     string    `json:"start_time"`
	EndTime       string    `json:"end_time"`
	Station       string    `json:"station"`
	Status        string    `json:"status"`
	Notes         string    `json:"notes"`
	CreatedByName string    `json:"created_by_name"`
	CreatedAt     time.Time `json:"created_at"`
}

type attendance struct {
	ID           int64      `json:"id"`
	ShiftID      int64      `json:"shift_id"`
	EmployeeID   int64      `json:"employee_id"`
	EmployeeName string     `json:"employee_name"`
	WorkDate     string     `json:"work_date"`
	ClockIn      time.Time  `json:"clock_in"`
	ClockOut     *time.Time `json:"clock_out,omitempty"`
	Status       string     `json:"status"`
	Notes        string     `json:"notes"`
}

type timeOffRequest struct {
	ID             int64     `json:"id"`
	EmployeeID     int64     `json:"employee_id"`
	EmployeeName   string    `json:"employee_name"`
	StartDate      string    `json:"start_date"`
	EndDate        string    `json:"end_date"`
	Type           string    `json:"type"`
	Reason         string    `json:"reason"`
	Status         string    `json:"status"`
	ReviewedByName string    `json:"reviewed_by_name"`
	ReviewNote     string    `json:"review_note"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type task struct {
	ID              int64      `json:"id"`
	ShiftDate       string     `json:"shift_date"`
	Title           string     `json:"title"`
	Station         string     `json:"station"`
	AssignedToID    int64      `json:"assigned_to_id"`
	AssignedToName  string     `json:"assigned_to_name"`
	Priority        string     `json:"priority"`
	Status          string     `json:"status"`
	CreatedByName   string     `json:"created_by_name"`
	CompletedByName string     `json:"completed_by_name"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type summary struct {
	ShiftsToday    int `json:"shifts_today"`
	OnDuty         int `json:"on_duty"`
	PendingTimeOff int `json:"pending_time_off"`
	OpenTasks      int `json:"open_tasks"`
}

func main() {
	defer logx.ConfigureBestEffort(serviceName)()
	cfg := dbx.RuntimeConfig()
	db, err := dbx.OpenWithConfig(configx.Sensitive("WORKFORCE_DB_DSN", defaultDSN), cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	a := &app{db: db, queryTimeout: cfg.QueryTimeout}
	mux := http.NewServeMux()
	httpx.RegisterHealthRoutes(mux, serviceName, httpx.DBReadinessCheck("mysql", db))
	mux.HandleFunc("/api/workforce/summary", a.handleSummary)
	mux.HandleFunc("/api/workforce/shifts", a.handleShifts)
	mux.HandleFunc("/api/workforce/attendance", a.handleAttendance)
	mux.HandleFunc("/api/workforce/attendance/clock-in", a.handleClockIn)
	mux.HandleFunc("/api/workforce/attendance/clock-out", a.handleClockOut)
	mux.HandleFunc("/api/workforce/time-off", a.handleTimeOff)
	mux.HandleFunc("/api/workforce/tasks", a.handleTasks)

	log.Println(serviceName + " listening on " + listenAddr)
	server := httpx.NewServer(listenAddr, httpx.RequestLogger(serviceName, mux))
	if err := httpx.RunServer(server, serviceName, 0); err != nil {
		log.Fatal(err)
	}
}

func requestIdentity(r *http.Request) (identity, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(r.Header.Get("X-User-ID")), 10, 64)
	if err != nil || id <= 0 {
		return identity{}, errors.New("invalid user identity")
	}
	name := strings.TrimSpace(r.Header.Get("X-User-Name"))
	role := strings.ToLower(strings.TrimSpace(r.Header.Get("X-User-Role")))
	if name == "" || (role != roleAdmin && role != roleAuditor && role != roleStaff) {
		return identity{}, errors.New("invalid user identity")
	}
	return identity{ID: id, Name: name, Role: role}, nil
}

func isManager(role string) bool { return role == roleAdmin || role == roleAuditor }

func requireIdentity(w http.ResponseWriter, r *http.Request) (identity, bool) {
	user, err := requestIdentity(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid identity")
		return identity{}, false
	}
	return user, true
}

func requireManager(w http.ResponseWriter, user identity) bool {
	if !isManager(user.Role) {
		httpx.WriteError(w, http.StatusForbidden, "PIC or owner role required")
		return false
	}
	return true
}

func parseRange(r *http.Request, now time.Time) (string, string, bool) {
	from := strings.TrimSpace(r.URL.Query().Get("from"))
	to := strings.TrimSpace(r.URL.Query().Get("to"))
	if from == "" {
		from = now.Format("2006-01-02")
	}
	if to == "" {
		to = now.AddDate(0, 0, 7).Format("2006-01-02")
	}
	start, errStart := time.Parse("2006-01-02", from)
	end, errEnd := time.Parse("2006-01-02", to)
	if errStart != nil || errEnd != nil || end.Before(start) || end.Sub(start) > 31*24*time.Hour {
		return "", "", false
	}
	return from, to, true
}

func validDate(value string) bool {
	_, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	return err == nil
}

func validClock(value string) bool {
	_, err := time.Parse("15:04", strings.TrimSpace(value))
	return err == nil
}

func validStation(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "kitchen", "prep", "service", "cashier", "beverage", "steward":
		return true
	default:
		return false
	}
}

func validPriority(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low", "normal", "high":
		return true
	default:
		return false
	}
}

func validTimeOffType(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "leave", "sick", "permission":
		return true
	default:
		return false
	}
}

func collectRows[T any](rows *sql.Rows, scan func(*sql.Rows) (T, error)) ([]T, error) {
	defer rows.Close()
	result := []T{}
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func scanShift(rows *sql.Rows) (shift, error) {
	var item shift
	err := rows.Scan(&item.ID, &item.EmployeeID, &item.EmployeeName, &item.ShiftDate, &item.StartTime, &item.EndTime, &item.Station, &item.Status, &item.Notes, &item.CreatedByName, &item.CreatedAt)
	return item, err
}

func scanAttendance(rows *sql.Rows) (attendance, error) {
	var item attendance
	var clockOut sql.NullTime
	if err := rows.Scan(&item.ID, &item.ShiftID, &item.EmployeeID, &item.EmployeeName, &item.WorkDate, &item.ClockIn, &clockOut, &item.Status, &item.Notes); err != nil {
		return attendance{}, err
	}
	if clockOut.Valid {
		item.ClockOut = &clockOut.Time
	}
	return item, nil
}

func scanTimeOff(rows *sql.Rows) (timeOffRequest, error) {
	var item timeOffRequest
	err := rows.Scan(&item.ID, &item.EmployeeID, &item.EmployeeName, &item.StartDate, &item.EndDate, &item.Type, &item.Reason, &item.Status, &item.ReviewedByName, &item.ReviewNote, &item.CreatedAt, &item.UpdatedAt)
	return item, err
}

func scanTask(rows *sql.Rows) (task, error) {
	var item task
	var completed sql.NullTime
	if err := rows.Scan(&item.ID, &item.ShiftDate, &item.Title, &item.Station, &item.AssignedToID, &item.AssignedToName, &item.Priority, &item.Status, &item.CreatedByName, &item.CompletedByName, &completed, &item.CreatedAt); err != nil {
		return task{}, err
	}
	if completed.Valid {
		item.CompletedAt = &completed.Time
	}
	return item, nil
}

func (a *app) withTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return dbx.WithQueryTimeout(parent, a.queryTimeout)
}

func (a *app) handleSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := requireIdentity(w, r)
	if !ok {
		return
	}
	result, err := a.loadSummary(r.Context(), user)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (a *app) loadSummary(parent context.Context, user identity) (summary, error) {
	ctx, cancel := a.withTimeout(parent)
	defer cancel()

	queries := []struct {
		sql  string
		args []any
		dest *int
	}{}
	var out summary
	if isManager(user.Role) {
		queries = []struct {
			sql  string
			args []any
			dest *int
		}{
			{`SELECT COUNT(*) FROM shifts WHERE shift_date=CURDATE() AND status='scheduled'`, nil, &out.ShiftsToday},
			{`SELECT COUNT(*) FROM attendance WHERE work_date=CURDATE() AND clock_out IS NULL`, nil, &out.OnDuty},
			{`SELECT COUNT(*) FROM time_off_requests WHERE status='pending'`, nil, &out.PendingTimeOff},
			{`SELECT COUNT(*) FROM shift_tasks WHERE shift_date=CURDATE() AND status='open'`, nil, &out.OpenTasks},
		}
	} else {
		queries = []struct {
			sql  string
			args []any
			dest *int
		}{
			{`SELECT COUNT(*) FROM shifts WHERE shift_date=CURDATE() AND status='scheduled' AND employee_id=?`, []any{user.ID}, &out.ShiftsToday},
			{`SELECT COUNT(*) FROM attendance WHERE work_date=CURDATE() AND clock_out IS NULL AND employee_id=?`, []any{user.ID}, &out.OnDuty},
			{`SELECT COUNT(*) FROM time_off_requests WHERE status='pending' AND employee_id=?`, []any{user.ID}, &out.PendingTimeOff},
			{`SELECT COUNT(*) FROM shift_tasks WHERE shift_date=CURDATE() AND status='open' AND (assigned_to_id=0 OR assigned_to_id=?)`, []any{user.ID}, &out.OpenTasks},
		}
	}
	for _, query := range queries {
		if err := a.db.QueryRowContext(ctx, query.sql, query.args...).Scan(query.dest); err != nil {
			return summary{}, err
		}
	}
	return out, nil
}

func (a *app) handleShifts(w http.ResponseWriter, r *http.Request) {
	user, ok := requireIdentity(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		a.listShifts(w, r, user)
	case http.MethodPost:
		if requireManager(w, user) {
			a.createShift(w, r, user)
		}
	default:
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *app) listShifts(w http.ResponseWriter, r *http.Request, user identity) {
	from, to, ok := parseRange(r, time.Now())
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid date range")
		return
	}
	query := selectShiftsBase
	args := []any{from, to}
	if !isManager(user.Role) {
		query += " AND employee_id=?"
		args = append(args, user.ID)
	}
	query += " ORDER BY shift_date,start_time,employee_name"

	ctx, cancel := a.withTimeout(r.Context())
	defer cancel()
	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	result, err := collectRows(rows, scanShift)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

type shiftInput struct {
	EmployeeID   int64  `json:"employee_id"`
	EmployeeName string `json:"employee_name"`
	ShiftDate    string `json:"shift_date"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	Station      string `json:"station"`
	Notes        string `json:"notes"`
}

func (in *shiftInput) normalize() {
	in.EmployeeName = strings.TrimSpace(in.EmployeeName)
	in.ShiftDate = strings.TrimSpace(in.ShiftDate)
	in.StartTime = strings.TrimSpace(in.StartTime)
	in.EndTime = strings.TrimSpace(in.EndTime)
	in.Station = strings.ToLower(strings.TrimSpace(in.Station))
	in.Notes = strings.TrimSpace(in.Notes)
}

func (in shiftInput) valid() bool {
	if in.EmployeeID <= 0 || in.EmployeeName == "" || !validDate(in.ShiftDate) || !validClock(in.StartTime) || !validClock(in.EndTime) || !validStation(in.Station) {
		return false
	}
	start, _ := time.Parse("15:04", in.StartTime)
	end, _ := time.Parse("15:04", in.EndTime)
	return end.After(start)
}

func (a *app) createShift(w http.ResponseWriter, r *http.Request, user identity) {
	var input shiftInput
	if err := httpx.DecodeJSON(r, &input); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}
	input.normalize()
	if !input.valid() {
		httpx.WriteError(w, http.StatusBadRequest, "invalid shift")
		return
	}
	ctx, cancel := a.withTimeout(r.Context())
	defer cancel()
	result, err := a.db.ExecContext(ctx, insertShift, input.EmployeeID, input.EmployeeName, input.ShiftDate, input.StartTime, input.EndTime, input.Station, statusScheduled, input.Notes, user.ID, user.Name)
	if err != nil {
		if dbx.IsDuplicateKey(err) {
			httpx.WriteError(w, http.StatusConflict, "employee already has a shift at that start time")
		} else {
			httpx.InternalError(w, err)
		}
		return
	}
	id, _ := result.LastInsertId()
	httpx.JSON(w, http.StatusCreated, map[string]any{"id": id, "status": statusScheduled})
}

func (a *app) handleAttendance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := requireIdentity(w, r)
	if !ok {
		return
	}
	from, to, rangeOK := parseRange(r, time.Now())
	if !rangeOK {
		httpx.WriteError(w, http.StatusBadRequest, "invalid date range")
		return
	}
	query := selectAttendanceBase
	args := []any{from, to}
	if !isManager(user.Role) {
		query += " AND a.employee_id=?"
		args = append(args, user.ID)
	}
	query += " ORDER BY a.work_date DESC,a.clock_in DESC"

	ctx, cancel := a.withTimeout(r.Context())
	defer cancel()
	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	result, err := collectRows(rows, scanAttendance)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (a *app) handleClockIn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := requireIdentity(w, r)
	if !ok {
		return
	}
	ctx, cancel := a.withTimeout(r.Context())
	defer cancel()
	var shiftID int64
	var employeeName, station, startTime, endTime string
	if err := a.db.QueryRowContext(ctx, selectTodayShift, user.ID).Scan(&shiftID, &employeeName, &station, &startTime, &endTime); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(w, http.StatusConflict, "no scheduled shift today")
		} else {
			httpx.InternalError(w, err)
		}
		return
	}
	result, err := a.db.ExecContext(ctx, insertAttendance, shiftID, user.ID, user.Name)
	if err != nil {
		if dbx.IsDuplicateKey(err) {
			httpx.WriteError(w, http.StatusConflict, "already clocked in for this shift")
		} else {
			httpx.InternalError(w, err)
		}
		return
	}
	id, _ := result.LastInsertId()
	httpx.JSON(w, http.StatusCreated, map[string]any{"id": id, "shift_id": shiftID, "station": station, "start_time": startTime, "end_time": endTime})
}

func (a *app) handleClockOut(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := requireIdentity(w, r)
	if !ok {
		return
	}
	ctx, cancel := a.withTimeout(r.Context())
	defer cancel()
	result, err := a.db.ExecContext(ctx, clockOutAttendance, user.ID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httpx.WriteError(w, http.StatusConflict, "no active attendance to clock out")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": statusCompleted})
}

func (a *app) handleTimeOff(w http.ResponseWriter, r *http.Request) {
	user, ok := requireIdentity(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		a.listTimeOff(w, r, user)
	case http.MethodPost:
		a.createTimeOff(w, r, user)
	case http.MethodPatch:
		if requireManager(w, user) {
			a.reviewTimeOff(w, r, user)
		}
	default:
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *app) listTimeOff(w http.ResponseWriter, r *http.Request, user identity) {
	query := selectTimeOffBase
	args := []any{}
	if !isManager(user.Role) {
		query += " WHERE employee_id=?"
		args = append(args, user.ID)
	}
	query += " ORDER BY created_at DESC LIMIT 100"
	ctx, cancel := a.withTimeout(r.Context())
	defer cancel()
	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	result, err := collectRows(rows, scanTimeOff)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

type timeOffInput struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Type      string `json:"type"`
	Reason    string `json:"reason"`
}

func (in *timeOffInput) normalize() {
	in.StartDate = strings.TrimSpace(in.StartDate)
	in.EndDate = strings.TrimSpace(in.EndDate)
	in.Type = strings.ToLower(strings.TrimSpace(in.Type))
	in.Reason = strings.TrimSpace(in.Reason)
}

func (in timeOffInput) valid() bool {
	if !validDate(in.StartDate) || !validDate(in.EndDate) || !validTimeOffType(in.Type) || in.Reason == "" || len(in.Reason) > 500 {
		return false
	}
	start, _ := time.Parse("2006-01-02", in.StartDate)
	end, _ := time.Parse("2006-01-02", in.EndDate)
	return !end.Before(start) && end.Sub(start) <= 14*24*time.Hour
}

func (a *app) createTimeOff(w http.ResponseWriter, r *http.Request, user identity) {
	var input timeOffInput
	if err := httpx.DecodeJSON(r, &input); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}
	input.normalize()
	if !input.valid() {
		httpx.WriteError(w, http.StatusBadRequest, "invalid time-off request")
		return
	}
	ctx, cancel := a.withTimeout(r.Context())
	defer cancel()
	result, err := a.db.ExecContext(ctx, insertTimeOff, user.ID, user.Name, input.StartDate, input.EndDate, input.Type, input.Reason)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	httpx.JSON(w, http.StatusCreated, map[string]any{"id": id, "status": statusPending})
}

type reviewInput struct {
	ID         int64  `json:"id"`
	Status     string `json:"status"`
	ReviewNote string `json:"review_note"`
}

func (a *app) reviewTimeOff(w http.ResponseWriter, r *http.Request, user identity) {
	var input reviewInput
	if err := httpx.DecodeJSON(r, &input); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.ReviewNote = strings.TrimSpace(input.ReviewNote)
	if input.ID <= 0 || (input.Status != statusApproved && input.Status != statusRejected) || len(input.ReviewNote) > 500 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid review")
		return
	}
	ctx, cancel := a.withTimeout(r.Context())
	defer cancel()
	result, err := a.db.ExecContext(ctx, reviewTimeOff, input.Status, user.ID, user.Name, input.ReviewNote, input.ID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httpx.WriteError(w, http.StatusConflict, "request already reviewed or not found")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"id": input.ID, "status": input.Status})
}

func (a *app) handleTasks(w http.ResponseWriter, r *http.Request) {
	user, ok := requireIdentity(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		a.listTasks(w, r, user)
	case http.MethodPost:
		if requireManager(w, user) {
			a.createTask(w, r, user)
		}
	case http.MethodPatch:
		a.updateTask(w, r, user)
	default:
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *app) listTasks(w http.ResponseWriter, r *http.Request, user identity) {
	from, to, ok := parseRange(r, time.Now())
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid date range")
		return
	}
	query := selectTasksBase
	args := []any{from, to}
	if !isManager(user.Role) {
		query += " AND (assigned_to_id=0 OR assigned_to_id=?)"
		args = append(args, user.ID)
	}
	query += " ORDER BY shift_date,FIELD(priority,'high','normal','low'),id DESC"
	ctx, cancel := a.withTimeout(r.Context())
	defer cancel()
	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	result, err := collectRows(rows, scanTask)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

type taskInput struct {
	ShiftDate      string `json:"shift_date"`
	Title          string `json:"title"`
	Station        string `json:"station"`
	AssignedToID   int64  `json:"assigned_to_id"`
	AssignedToName string `json:"assigned_to_name"`
	Priority       string `json:"priority"`
}

func (in *taskInput) normalize() {
	in.ShiftDate = strings.TrimSpace(in.ShiftDate)
	in.Title = strings.TrimSpace(in.Title)
	in.Station = strings.ToLower(strings.TrimSpace(in.Station))
	in.AssignedToName = strings.TrimSpace(in.AssignedToName)
	in.Priority = strings.ToLower(strings.TrimSpace(in.Priority))
	if in.Priority == "" {
		in.Priority = "normal"
	}
}

func (in taskInput) valid() bool {
	return validDate(in.ShiftDate) && in.Title != "" && len(in.Title) <= 180 && validStation(in.Station) && in.AssignedToID >= 0 && validPriority(in.Priority) && (in.AssignedToID == 0 || in.AssignedToName != "")
}

func (a *app) createTask(w http.ResponseWriter, r *http.Request, user identity) {
	var input taskInput
	if err := httpx.DecodeJSON(r, &input); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}
	input.normalize()
	if !input.valid() {
		httpx.WriteError(w, http.StatusBadRequest, "invalid task")
		return
	}
	if input.AssignedToID == 0 && input.AssignedToName == "" {
		input.AssignedToName = "Semua Tim"
	}
	ctx, cancel := a.withTimeout(r.Context())
	defer cancel()
	result, err := a.db.ExecContext(ctx, insertTask, input.ShiftDate, input.Title, input.Station, input.AssignedToID, input.AssignedToName, input.Priority, user.ID, user.Name)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	httpx.JSON(w, http.StatusCreated, map[string]any{"id": id, "status": statusOpen})
}

type taskUpdateInput struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
}

func (a *app) updateTask(w http.ResponseWriter, r *http.Request, user identity) {
	var input taskUpdateInput
	if err := httpx.DecodeJSON(r, &input); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	if input.ID <= 0 || (input.Status != statusOpen && input.Status != statusDone) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid task update")
		return
	}
	ctx, cancel := a.withTimeout(r.Context())
	defer cancel()
	var (
		result sql.Result
		err    error
	)
	if isManager(user.Role) {
		result, err = a.db.ExecContext(ctx, completeTaskMgr, input.Status, user.ID, user.Name, input.Status, input.ID)
	} else {
		result, err = a.db.ExecContext(ctx, completeTaskStaff, input.Status, user.ID, user.Name, input.Status, input.ID, user.ID)
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httpx.WriteError(w, http.StatusForbidden, "task not assigned to current user")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"id": input.ID, "status": input.Status})
}
