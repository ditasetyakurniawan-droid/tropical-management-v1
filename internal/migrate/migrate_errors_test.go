package migrate

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
)

func TestLoadUnknownService(t *testing.T) {
	_, err := Load("does-not-exist")
	if err == nil || !strings.Contains(err.Error(), "no migrations found") {
		t.Fatalf("unexpected err=%v", err)
	}
}

func TestLoadFileRejectsMalformedNames(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{path: "sql/auth/bad.sql", want: "invalid migration filename"},
		{path: "sql/auth/000000_zero.sql", want: "invalid migration version"},
		{path: "sql/auth/nope_name.sql", want: "invalid migration version"},
		{path: "sql/auth/999999_missing.sql", want: "read migration"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			_, err := loadFile(tc.path)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("loadFile(%q) err=%v want substring %q", tc.path, err, tc.want)
			}
		})
	}
}

func TestUpRejectsNilDatabase(t *testing.T) {
	err := Up(context.Background(), Target{Name: "auth"})
	if err == nil || !strings.Contains(err.Error(), "database is nil") {
		t.Fatalf("unexpected err=%v", err)
	}
}

func TestUpRejectsTargetWithoutMigrations(t *testing.T) {
	db, _ := testDB(t)
	err := Up(context.Background(), Target{Name: "unknown", DB: db})
	if err == nil || !strings.Contains(err.Error(), "no migrations found") {
		t.Fatalf("unexpected err=%v", err)
	}
}

type openErrorDriver struct{ err error }

func (d openErrorDriver) Open(string) (driver.Conn, error) { return nil, d.err }

func TestUpFailsWhenConnectionCannotBeAcquired(t *testing.T) {
	const driverName = "migrate-open-error"
	sql.Register(driverName, openErrorDriver{err: errors.New("open failed")})
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	err = Up(context.Background(), Target{Name: "auth", DB: db})
	if err == nil || !strings.Contains(err.Error(), "acquire migration connection") {
		t.Fatalf("unexpected err=%v", err)
	}
}

func TestUpFailsWhenLockQueryFails(t *testing.T) {
	db, script := testDB(t,
		dbStep{op: "query", match: "SELECT GET_LOCK", err: errors.New("lock query failed")},
	)
	err := Up(context.Background(), Target{Name: "auth", DB: db})
	if err == nil || !strings.Contains(err.Error(), "acquire migration lock") {
		t.Fatalf("unexpected err=%v", err)
	}
	script.done(t)
}

func TestUpFailsWhenLockIsNotAcquired(t *testing.T) {
	db, script := testDB(t,
		q("SELECT GET_LOCK", []string{"lock"}, values(int64(0))),
	)
	err := Up(context.Background(), Target{Name: "auth", DB: db})
	if err == nil || !strings.Contains(err.Error(), "not acquired within 30s") {
		t.Fatalf("unexpected err=%v", err)
	}
	script.done(t)
}

func TestUpFailsCreatingMigrationTable(t *testing.T) {
	db, script := testDB(t,
		q("SELECT GET_LOCK", []string{"lock"}, values(int64(1))),
		dbStep{op: "exec", match: "CREATE TABLE IF NOT EXISTS schema_migrations", err: errors.New("ddl denied")},
		q("SELECT RELEASE_LOCK", []string{"released"}, values(int64(1))),
	)
	err := Up(context.Background(), Target{Name: "auth", DB: db})
	if err == nil || !strings.Contains(err.Error(), "create schema_migrations") {
		t.Fatalf("unexpected err=%v", err)
	}
	script.done(t)
}

func TestUpFailsReadingMigrationState(t *testing.T) {
	db, script := testDB(t,
		q("SELECT GET_LOCK", []string{"lock"}, values(int64(1))),
		e("CREATE TABLE IF NOT EXISTS schema_migrations"),
		dbStep{op: "query", match: "SELECT checksum, dirty FROM schema_migrations", err: errors.New("read failed")},
		q("SELECT RELEASE_LOCK", []string{"released"}, values(int64(1))),
	)
	err := Up(context.Background(), Target{Name: "auth", DB: db})
	if err == nil || !strings.Contains(err.Error(), "read auth migration") {
		t.Fatalf("unexpected err=%v", err)
	}
	script.done(t)
}

func TestUpFailsMarkingMigrationDirty(t *testing.T) {
	db, script := testDB(t,
		q("SELECT GET_LOCK", []string{"lock"}, values(int64(1))),
		e("CREATE TABLE IF NOT EXISTS schema_migrations"),
		q("SELECT checksum, dirty FROM schema_migrations", []string{"checksum", "dirty"}),
		dbStep{op: "exec", match: "INSERT INTO schema_migrations", err: errors.New("insert failed")},
		q("SELECT RELEASE_LOCK", []string{"released"}, values(int64(1))),
	)
	err := Up(context.Background(), Target{Name: "auth", DB: db})
	if err == nil || !strings.Contains(err.Error(), "mark auth migration") {
		t.Fatalf("unexpected err=%v", err)
	}
	script.done(t)
}

func TestUpFailsApplyingMigrationSQL(t *testing.T) {
	db, script := testDB(t,
		q("SELECT GET_LOCK", []string{"lock"}, values(int64(1))),
		e("CREATE TABLE IF NOT EXISTS schema_migrations"),
		q("SELECT checksum, dirty FROM schema_migrations", []string{"checksum", "dirty"}),
		e("INSERT INTO schema_migrations"),
		dbStep{op: "exec", match: "CREATE TABLE IF NOT EXISTS users", err: errors.New("migration ddl failed")},
		q("SELECT RELEASE_LOCK", []string{"released"}, values(int64(1))),
	)
	err := Up(context.Background(), Target{Name: "auth", DB: db})
	if err == nil || !strings.Contains(err.Error(), "apply auth migration") {
		t.Fatalf("unexpected err=%v", err)
	}
	script.done(t)
}

func TestUpFailsMarkingMigrationClean(t *testing.T) {
	db, script := testDB(t,
		q("SELECT GET_LOCK", []string{"lock"}, values(int64(1))),
		e("CREATE TABLE IF NOT EXISTS schema_migrations"),
		q("SELECT checksum, dirty FROM schema_migrations", []string{"checksum", "dirty"}),
		e("INSERT INTO schema_migrations"),
		e("CREATE TABLE IF NOT EXISTS users"),
		dbStep{op: "exec", match: "UPDATE schema_migrations SET dirty=FALSE", err: errors.New("update failed")},
		q("SELECT RELEASE_LOCK", []string{"released"}, values(int64(1))),
	)
	err := Up(context.Background(), Target{Name: "auth", DB: db})
	if err == nil || !strings.Contains(err.Error(), "mark auth migration") {
		t.Fatalf("unexpected err=%v", err)
	}
	script.done(t)
}

func TestUpFailsSchemaVerificationQuery(t *testing.T) {
	migrations, err := Load("auth")
	if err != nil {
		t.Fatal(err)
	}
	db, script := testDB(t,
		q("SELECT GET_LOCK", []string{"lock"}, values(int64(1))),
		e("CREATE TABLE IF NOT EXISTS schema_migrations"),
		q("SELECT checksum, dirty FROM schema_migrations", []string{"checksum", "dirty"}, values(migrations[0].Checksum, false)),
		dbStep{op: "query", match: "SELECT id FROM users LIMIT 0", err: errors.New("verify failed")},
		q("SELECT RELEASE_LOCK", []string{"released"}, values(int64(1))),
	)
	err = Up(context.Background(), Target{Name: "auth", DB: db, VerifySQL: []string{"SELECT id FROM users LIMIT 0"}})
	if err == nil || !strings.Contains(err.Error(), "verify schema auth") {
		t.Fatalf("unexpected err=%v", err)
	}
	script.done(t)
}
