package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func req(method, target, body string, user identity) *http.Request {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	if user.ID > 0 {
		r.Header.Set("X-User-ID", strconv.FormatInt(user.ID, 10))
		r.Header.Set("X-User-Name", user.Name)
		r.Header.Set("X-User-Role", user.Role)
	}
	return r
}

func owner() identity { return identity{ID: 1, Name: "Owner Tropical", Role: roleAdmin} }
func pic() identity   { return identity{ID: 2, Name: "PIC Malam", Role: roleAuditor} }
func staff() identity { return identity{ID: 3, Name: "Ayu Service", Role: roleStaff} }

func TestIdentityAndValidationHelpers(t *testing.T) {
	t.Run("valid identity", func(t *testing.T) {
		got, err := requestIdentity(req(http.MethodGet, "/", "", staff()))
		if err != nil || got.ID != 3 || got.Role != roleStaff {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	})

	for name, mutate := range map[string]func(*http.Request){
		"missing id": func(r *http.Request) { r.Header.Del("X-User-ID") },
		"zero id":    func(r *http.Request) { r.Header.Set("X-User-ID", "0") },
		"blank name": func(r *http.Request) { r.Header.Set("X-User-Name", " ") },
		"bad role":   func(r *http.Request) { r.Header.Set("X-User-Role", "guest") },
	} {
		t.Run(name, func(t *testing.T) {
			r := req(http.MethodGet, "/", "", staff())
			mutate(r)
			if _, err := requestIdentity(r); err == nil {
				t.Fatal("expected identity error")
			}
		})
	}

	if !isManager(roleAdmin) || !isManager(roleAuditor) || isManager(roleStaff) {
		t.Fatal("manager role mapping invalid")
	}

	now := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	r := httptest.NewRequest(http.MethodGet, "/?from=2026-08-30&to=2026-09-03", nil)
	from, to, ok := parseRange(r, now)
	if !ok || from != "2026-08-30" || to != "2026-09-03" {
		t.Fatalf("range=%s..%s ok=%v", from, to, ok)
	}
	if _, _, ok := parseRange(httptest.NewRequest(http.MethodGet, "/?from=bad", nil), now); ok {
		t.Fatal("invalid range accepted")
	}
	if _, _, ok := parseRange(httptest.NewRequest(http.MethodGet, "/?from=2026-08-30&to=2026-10-15", nil), now); ok {
		t.Fatal("oversized range accepted")
	}

	if !validDate("2026-08-30") || validDate("30/08/2026") || !validClock("11:30") || validClock("25:00") {
		t.Fatal("date/time validation invalid")
	}
	if !validStation("Kitchen") || validStation("office") || !validPriority("high") || validPriority("urgent") || !validTimeOffType("sick") || validTimeOffType("vacation") {
		t.Fatal("enum validation invalid")
	}
}

func TestInputNormalizationAndValidation(t *testing.T) {
	shiftIn := shiftInput{EmployeeID: 3, EmployeeName: " Ayu ", ShiftDate: " 2026-08-30 ", StartTime: " 11:00 ", EndTime: " 19:00 ", Station: " Service ", Notes: " front "}
	shiftIn.normalize()
	if !shiftIn.valid() || shiftIn.EmployeeName != "Ayu" || shiftIn.Station != "service" || shiftIn.Notes != "front" {
		t.Fatalf("shift=%+v", shiftIn)
	}
	shiftIn.EndTime = "10:00"
	if shiftIn.valid() {
		t.Fatal("reverse shift accepted")
	}

	leave := timeOffInput{StartDate: " 2026-09-01 ", EndDate: " 2026-09-02 ", Type: " Permission ", Reason: " Family "}
	leave.normalize()
	if !leave.valid() || leave.Type != "permission" || leave.Reason != "Family" {
		t.Fatalf("leave=%+v", leave)
	}
	leave.EndDate = "2026-09-30"
	if leave.valid() {
		t.Fatal("long leave accepted")
	}

	taskIn := taskInput{ShiftDate: " 2026-08-30 ", Title: " Opening station ", Station: " Kitchen ", AssignedToID: 3, AssignedToName: " Ayu ", Priority: " "}
	taskIn.normalize()
	if !taskIn.valid() || taskIn.Priority != "normal" || taskIn.Station != "kitchen" {
		t.Fatalf("task=%+v", taskIn)
	}
	taskIn.AssignedToName = ""
	if taskIn.valid() {
		t.Fatal("named assignment without name accepted")
	}
}

func TestSummaryForManagerAndStaff(t *testing.T) {
	t.Run("manager", func(t *testing.T) {
		db, script := openTestDB(t,
			queryStep("COUNT(*) FROM shifts", []string{"count"}, row(int64(8))),
			queryStep("COUNT(*) FROM attendance", []string{"count"}, row(int64(5))),
			queryStep("COUNT(*) FROM time_off_requests", []string{"count"}, row(int64(2))),
			queryStep("COUNT(*) FROM shift_tasks", []string{"count"}, row(int64(4))),
		)
		a := &app{db: db, queryTimeout: time.Second}
		w := httptest.NewRecorder()
		a.handleSummary(w, req(http.MethodGet, "/api/workforce/summary", "", owner()))
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"shifts_today":8`) || !strings.Contains(w.Body.String(), `"on_duty":5`) {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		script.assertDone(t)
	})

	t.Run("staff", func(t *testing.T) {
		db, script := openTestDB(t,
			queryStep("shifts WHERE shift_date=CURDATE()", []string{"count"}, row(int64(1))),
			queryStep("attendance WHERE work_date=CURDATE()", []string{"count"}, row(int64(1))),
			queryStep("time_off_requests WHERE status='pending'", []string{"count"}, row(int64(0))),
			queryStep("shift_tasks WHERE shift_date=CURDATE()", []string{"count"}, row(int64(2))),
		)
		a := &app{db: db, queryTimeout: time.Second}
		w := httptest.NewRecorder()
		a.handleSummary(w, req(http.MethodGet, "/api/workforce/summary", "", staff()))
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"shifts_today":1`) || !strings.Contains(w.Body.String(), `"open_tasks":2`) {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		script.assertDone(t)
	})

	w := httptest.NewRecorder()
	(&app{}).handleSummary(w, req(http.MethodPost, "/api/workforce/summary", "", staff()))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", w.Code)
	}

	w = httptest.NewRecorder()
	(&app{}).handleSummary(w, httptest.NewRequest(http.MethodGet, "/api/workforce/summary", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestShiftHandlers(t *testing.T) {
	now := time.Date(2026, 8, 30, 11, 0, 0, 0, time.UTC)
	db, script := openTestDB(t,
		queryStep("FROM shifts WHERE shift_date BETWEEN", []string{"id", "employee_id", "employee_name", "shift_date", "start_time", "end_time", "station", "status", "notes", "created_by_name", "created_at"},
			row(int64(10), int64(3), "Ayu Service", "2026-08-30", "11:00", "19:00", "service", "scheduled", "", "PIC Malam", now)),
		execStep("INSERT INTO shifts", 11, 1),
	)
	a := &app{db: db, queryTimeout: time.Second}

	w := httptest.NewRecorder()
	a.handleShifts(w, req(http.MethodGet, "/api/workforce/shifts?from=2026-08-30&to=2026-08-30", "", staff()))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"employee_name":"Ayu Service"`) {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	body := `{"employee_id":3,"employee_name":" Ayu Service ","shift_date":"2026-08-31","start_time":"11:00","end_time":"19:00","station":"service","notes":"closing"}`
	w = httptest.NewRecorder()
	a.handleShifts(w, req(http.MethodPost, "/api/workforce/shifts", body, pic()))
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"id":11`) {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	script.assertDone(t)

	w = httptest.NewRecorder()
	(&app{}).handleShifts(w, req(http.MethodPost, "/api/workforce/shifts", body, staff()))
	if w.Code != http.StatusForbidden {
		t.Fatalf("staff create status=%d", w.Code)
	}

	w = httptest.NewRecorder()
	(&app{}).handleShifts(w, req(http.MethodPost, "/api/workforce/shifts", `{}`, owner()))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid create status=%d", w.Code)
	}

	w = httptest.NewRecorder()
	(&app{}).handleShifts(w, req(http.MethodDelete, "/api/workforce/shifts", "", owner()))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("delete status=%d", w.Code)
	}
}

func TestAttendanceHandlers(t *testing.T) {
	now := time.Date(2026, 8, 30, 11, 2, 0, 0, time.UTC)
	out := now.Add(8 * time.Hour)
	db, script := openTestDB(t,
		queryStep("FROM attendance a WHERE", []string{"id", "shift_id", "employee_id", "employee_name", "work_date", "clock_in", "clock_out", "status", "notes"},
			row(int64(1), int64(10), int64(3), "Ayu Service", "2026-08-30", now, out, "completed", "")),
		queryStep("FROM shifts WHERE employee_id", []string{"id", "employee_name", "station", "start_time", "end_time"}, row(int64(10), "Ayu Service", "service", "11:00", "19:00")),
		execStep("INSERT INTO attendance", 2, 1),
		execStep("UPDATE attendance SET clock_out", 0, 1),
	)
	a := &app{db: db, queryTimeout: time.Second}

	w := httptest.NewRecorder()
	a.handleAttendance(w, req(http.MethodGet, "/api/workforce/attendance?from=2026-08-30&to=2026-08-30", "", staff()))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"status":"completed"`) {
		t.Fatalf("attendance status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.handleClockIn(w, req(http.MethodPost, "/api/workforce/attendance/clock-in", "", staff()))
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"station":"service"`) {
		t.Fatalf("clock-in status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.handleClockOut(w, req(http.MethodPost, "/api/workforce/attendance/clock-out", "", staff()))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"completed"`) {
		t.Fatalf("clock-out status=%d body=%s", w.Code, w.Body.String())
	}
	script.assertDone(t)

	w = httptest.NewRecorder()
	(&app{}).handleAttendance(w, req(http.MethodPost, "/api/workforce/attendance", "", staff()))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("attendance method status=%d", w.Code)
	}

	w = httptest.NewRecorder()
	(&app{}).handleClockIn(w, req(http.MethodGet, "/api/workforce/attendance/clock-in", "", staff()))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("clock-in method status=%d", w.Code)
	}

	w = httptest.NewRecorder()
	(&app{}).handleClockOut(w, req(http.MethodGet, "/api/workforce/attendance/clock-out", "", staff()))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("clock-out method status=%d", w.Code)
	}
}

func TestAttendanceConflictBranches(t *testing.T) {
	t.Run("no shift", func(t *testing.T) {
		db, script := openTestDB(t, queryStep("FROM shifts WHERE employee_id", []string{"id", "employee_name", "station", "start_time", "end_time"}))
		a := &app{db: db, queryTimeout: time.Second}
		w := httptest.NewRecorder()
		a.handleClockIn(w, req(http.MethodPost, "/api/workforce/attendance/clock-in", "", staff()))
		if w.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		script.assertDone(t)
	})
	t.Run("no active attendance", func(t *testing.T) {
		db, script := openTestDB(t, execStep("UPDATE attendance SET clock_out", 0, 0))
		a := &app{db: db, queryTimeout: time.Second}
		w := httptest.NewRecorder()
		a.handleClockOut(w, req(http.MethodPost, "/api/workforce/attendance/clock-out", "", staff()))
		if w.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		script.assertDone(t)
	})
}

func TestTimeOffHandlers(t *testing.T) {
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	db, script := openTestDB(t,
		queryStep("FROM time_off_requests WHERE employee_id", []string{"id", "employee_id", "employee_name", "start_date", "end_date", "type", "reason", "status", "reviewed_by_name", "review_note", "created_at", "updated_at"},
			row(int64(5), int64(3), "Ayu Service", "2026-09-01", "2026-09-01", "permission", "family", "pending", "", "", now, now)),
		execStep("INSERT INTO time_off_requests", 6, 1),
		execStep("UPDATE time_off_requests SET status", 0, 1),
	)
	a := &app{db: db, queryTimeout: time.Second}

	w := httptest.NewRecorder()
	a.handleTimeOff(w, req(http.MethodGet, "/api/workforce/time-off", "", staff()))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"permission"`) {
		t.Fatalf("list status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.handleTimeOff(w, req(http.MethodPost, "/api/workforce/time-off", `{"start_date":"2026-09-02","end_date":"2026-09-02","type":"sick","reason":"doctor"}`, staff()))
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"id":6`) {
		t.Fatalf("create status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.handleTimeOff(w, req(http.MethodPatch, "/api/workforce/time-off", `{"id":5,"status":"approved","review_note":"covered"}`, pic()))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"approved"`) {
		t.Fatalf("review status=%d body=%s", w.Code, w.Body.String())
	}
	script.assertDone(t)

	w = httptest.NewRecorder()
	(&app{}).handleTimeOff(w, req(http.MethodPatch, "/api/workforce/time-off", `{"id":5,"status":"approved"}`, staff()))
	if w.Code != http.StatusForbidden {
		t.Fatalf("staff review status=%d", w.Code)
	}

	w = httptest.NewRecorder()
	(&app{}).handleTimeOff(w, req(http.MethodPost, "/api/workforce/time-off", `{}`, staff()))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid create status=%d", w.Code)
	}

	w = httptest.NewRecorder()
	(&app{}).handleTimeOff(w, req(http.MethodPatch, "/api/workforce/time-off", `{"id":0,"status":"maybe"}`, pic()))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid review status=%d", w.Code)
	}
}

func TestTaskHandlers(t *testing.T) {
	now := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)
	db, script := openTestDB(t,
		queryStep("FROM shift_tasks WHERE shift_date BETWEEN", []string{"id", "shift_date", "title", "station", "assigned_to_id", "assigned_to_name", "priority", "status", "created_by_name", "completed_by_name", "completed_at", "created_at"},
			row(int64(7), "2026-08-30", "Cek chiller", "kitchen", int64(0), "Semua Tim", "high", "open", "PIC Malam", "", nil, now)),
		execStep("INSERT INTO shift_tasks", 8, 1),
		execStep("UPDATE shift_tasks SET status", 0, 1),
		execStep("UPDATE shift_tasks SET status", 0, 1),
	)
	a := &app{db: db, queryTimeout: time.Second}

	w := httptest.NewRecorder()
	a.handleTasks(w, req(http.MethodGet, "/api/workforce/tasks?from=2026-08-30&to=2026-08-30", "", staff()))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"Cek chiller"`) {
		t.Fatalf("list status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.handleTasks(w, req(http.MethodPost, "/api/workforce/tasks", `{"shift_date":"2026-08-30","title":"Sanitasi grill","station":"kitchen","assigned_to_id":0,"priority":"high"}`, pic()))
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"id":8`) {
		t.Fatalf("create status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.handleTasks(w, req(http.MethodPatch, "/api/workforce/tasks", `{"id":7,"status":"done"}`, staff()))
	if w.Code != http.StatusOK {
		t.Fatalf("staff update status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.handleTasks(w, req(http.MethodPatch, "/api/workforce/tasks", `{"id":7,"status":"open"}`, owner()))
	if w.Code != http.StatusOK {
		t.Fatalf("owner update status=%d body=%s", w.Code, w.Body.String())
	}
	script.assertDone(t)

	w = httptest.NewRecorder()
	(&app{}).handleTasks(w, req(http.MethodPost, "/api/workforce/tasks", `{}`, staff()))
	if w.Code != http.StatusForbidden {
		t.Fatalf("staff create status=%d", w.Code)
	}

	w = httptest.NewRecorder()
	(&app{}).handleTasks(w, req(http.MethodPatch, "/api/workforce/tasks", `{}`, staff()))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid patch status=%d", w.Code)
	}

	w = httptest.NewRecorder()
	(&app{}).handleTasks(w, req(http.MethodDelete, "/api/workforce/tasks", "", pic()))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("delete status=%d", w.Code)
	}
}

func TestTaskAndTimeOffConflictBranches(t *testing.T) {
	t.Run("task not assigned", func(t *testing.T) {
		db, script := openTestDB(t, execStep("UPDATE shift_tasks SET status", 0, 0))
		a := &app{db: db, queryTimeout: time.Second}
		w := httptest.NewRecorder()
		a.handleTasks(w, req(http.MethodPatch, "/api/workforce/tasks", `{"id":7,"status":"done"}`, staff()))
		if w.Code != http.StatusForbidden {
			t.Fatalf("status=%d", w.Code)
		}
		script.assertDone(t)
	})
	t.Run("timeoff already reviewed", func(t *testing.T) {
		db, script := openTestDB(t, execStep("UPDATE time_off_requests SET status", 0, 0))
		a := &app{db: db, queryTimeout: time.Second}
		w := httptest.NewRecorder()
		a.handleTimeOff(w, req(http.MethodPatch, "/api/workforce/time-off", `{"id":4,"status":"rejected","review_note":"coverage"}`, owner()))
		if w.Code != http.StatusConflict {
			t.Fatalf("status=%d", w.Code)
		}
		script.assertDone(t)
	})
}

func TestDatabaseErrorsReturnServerError(t *testing.T) {
	boom := errors.New("db unavailable")
	cases := []struct {
		name string
		step testDBStep
		call func(*app, *httptest.ResponseRecorder)
	}{
		{"summary", queryErrorStep("COUNT(*) FROM shifts", boom), func(a *app, w *httptest.ResponseRecorder) {
			a.handleSummary(w, req(http.MethodGet, "/api/workforce/summary", "", owner()))
		}},
		{"shifts", queryErrorStep("FROM shifts WHERE shift_date BETWEEN", boom), func(a *app, w *httptest.ResponseRecorder) {
			a.handleShifts(w, req(http.MethodGet, "/api/workforce/shifts?from=2026-08-30&to=2026-08-30", "", owner()))
		}},
		{"attendance", queryErrorStep("FROM attendance a WHERE", boom), func(a *app, w *httptest.ResponseRecorder) {
			a.handleAttendance(w, req(http.MethodGet, "/api/workforce/attendance?from=2026-08-30&to=2026-08-30", "", owner()))
		}},
		{"timeoff", queryErrorStep("FROM time_off_requests", boom), func(a *app, w *httptest.ResponseRecorder) {
			a.handleTimeOff(w, req(http.MethodGet, "/api/workforce/time-off", "", owner()))
		}},
		{"tasks", queryErrorStep("FROM shift_tasks", boom), func(a *app, w *httptest.ResponseRecorder) {
			a.handleTasks(w, req(http.MethodGet, "/api/workforce/tasks?from=2026-08-30&to=2026-08-30", "", owner()))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, script := openTestDB(t, tc.step)
			a := &app{db: db, queryTimeout: time.Second}
			w := httptest.NewRecorder()
			tc.call(a, w)
			if w.Code != http.StatusInternalServerError {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			script.assertDone(t)
		})
	}
}

func TestMutationDatabaseErrorsReturnServerError(t *testing.T) {
	boom := errors.New("write failed")
	cases := []struct {
		name string
		step testDBStep
		call func(*app, *httptest.ResponseRecorder)
	}{
		{"create shift", execErrorStep("INSERT INTO shifts", boom), func(a *app, w *httptest.ResponseRecorder) {
			a.handleShifts(w, req(http.MethodPost, "/api/workforce/shifts", `{"employee_id":3,"employee_name":"Ayu","shift_date":"2026-08-31","start_time":"11:00","end_time":"19:00","station":"service"}`, pic()))
		}},
		{"create time off", execErrorStep("INSERT INTO time_off_requests", boom), func(a *app, w *httptest.ResponseRecorder) {
			a.handleTimeOff(w, req(http.MethodPost, "/api/workforce/time-off", `{"start_date":"2026-09-01","end_date":"2026-09-01","type":"permission","reason":"family"}`, staff()))
		}},
		{"review time off", execErrorStep("UPDATE time_off_requests SET status", boom), func(a *app, w *httptest.ResponseRecorder) {
			a.handleTimeOff(w, req(http.MethodPatch, "/api/workforce/time-off", `{"id":5,"status":"approved"}`, pic()))
		}},
		{"create task", execErrorStep("INSERT INTO shift_tasks", boom), func(a *app, w *httptest.ResponseRecorder) {
			a.handleTasks(w, req(http.MethodPost, "/api/workforce/tasks", `{"shift_date":"2026-08-30","title":"Opening","station":"service","assigned_to_id":0,"priority":"normal"}`, pic()))
		}},
		{"update task", execErrorStep("UPDATE shift_tasks SET status", boom), func(a *app, w *httptest.ResponseRecorder) {
			a.handleTasks(w, req(http.MethodPatch, "/api/workforce/tasks", `{"id":7,"status":"done"}`, staff()))
		}},
		{"clock out", execErrorStep("UPDATE attendance SET clock_out", boom), func(a *app, w *httptest.ResponseRecorder) {
			a.handleClockOut(w, req(http.MethodPost, "/api/workforce/attendance/clock-out", "", staff()))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, script := openTestDB(t, tc.step)
			a := &app{db: db, queryTimeout: time.Second}
			w := httptest.NewRecorder()
			tc.call(a, w)
			if w.Code != http.StatusInternalServerError {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			script.assertDone(t)
		})
	}
}

func TestClockInDatabaseErrors(t *testing.T) {
	boom := errors.New("clock in db failure")
	t.Run("shift lookup", func(t *testing.T) {
		db, script := openTestDB(t, queryErrorStep("FROM shifts WHERE employee_id", boom))
		a := &app{db: db, queryTimeout: time.Second}
		w := httptest.NewRecorder()
		a.handleClockIn(w, req(http.MethodPost, "/api/workforce/attendance/clock-in", "", staff()))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		script.assertDone(t)
	})

	t.Run("attendance insert", func(t *testing.T) {
		db, script := openTestDB(t,
			queryStep("FROM shifts WHERE employee_id", []string{"id", "employee_name", "station", "start_time", "end_time"}, row(int64(10), "Ayu Service", "service", "11:00", "19:00")),
			execErrorStep("INSERT INTO attendance", boom),
		)
		a := &app{db: db, queryTimeout: time.Second}
		w := httptest.NewRecorder()
		a.handleClockIn(w, req(http.MethodPost, "/api/workforce/attendance/clock-in", "", staff()))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		script.assertDone(t)
	})
}

func TestRangeAndAuthorizationFailures(t *testing.T) {
	cases := []struct {
		name string
		call func(*app, *httptest.ResponseRecorder)
		want int
	}{
		{"invalid shift range", func(a *app, w *httptest.ResponseRecorder) {
			a.handleShifts(w, req(http.MethodGet, "/api/workforce/shifts?from=bad", "", staff()))
		}, http.StatusBadRequest},
		{"invalid attendance range", func(a *app, w *httptest.ResponseRecorder) {
			a.handleAttendance(w, req(http.MethodGet, "/api/workforce/attendance?from=bad", "", staff()))
		}, http.StatusBadRequest},
		{"invalid task range", func(a *app, w *httptest.ResponseRecorder) {
			a.handleTasks(w, req(http.MethodGet, "/api/workforce/tasks?from=bad", "", staff()))
		}, http.StatusBadRequest},
		{"timeoff unsupported method", func(a *app, w *httptest.ResponseRecorder) {
			a.handleTimeOff(w, req(http.MethodDelete, "/api/workforce/time-off", "", owner()))
		}, http.StatusMethodNotAllowed},
		{"missing task identity", func(a *app, w *httptest.ResponseRecorder) {
			a.handleTasks(w, httptest.NewRequest(http.MethodGet, "/api/workforce/tasks", nil))
		}, http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tc.call(&app{}, w)
			if w.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}
