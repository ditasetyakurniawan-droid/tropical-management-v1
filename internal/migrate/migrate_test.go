package migrate

import (
	"context"
	"strings"
	"testing"
)

func TestLoadAllServices(t *testing.T) {
	expected := map[string]int{
		"auth":      1,
		"audit":     2,
		"inventory": 3,
		"sales":     1,
		"chat":      1,
		"workforce": 4,
	}
	for service, count := range expected {
		migrations, err := Load(service)
		if err != nil {
			t.Fatalf("Load(%q): %v", service, err)
		}
		if len(migrations) != count {
			t.Fatalf("Load(%q) count=%d want=%d", service, len(migrations), count)
		}
		var previous int64
		for _, migration := range migrations {
			if migration.Version <= previous {
				t.Fatalf("%s versions are not increasing", service)
			}
			if migration.Name == "" || migration.SQL == "" || len(migration.Checksum) != 64 {
				t.Fatalf("invalid migration loaded for %s: %+v", service, migration)
			}
			previous = migration.Version
		}
	}
}

func TestLoadRejectsInvalidService(t *testing.T) {
	for _, service := range []string{"", "../auth", "auth/x", `auth\x`} {
		if _, err := Load(service); err == nil {
			t.Fatalf("Load(%q) should fail", service)
		}
	}
}

func TestBaselineMigrationsUseIdempotentCreate(t *testing.T) {
	for _, service := range []string{"auth", "audit", "inventory", "sales", "chat", "workforce"} {
		migrations, err := Load(service)
		if err != nil {
			t.Fatal(err)
		}
		for _, migration := range migrations {
			upper := strings.ToUpper(migration.SQL)
			if !strings.HasPrefix(upper, "CREATE TABLE IF NOT EXISTS") {
				t.Fatalf("baseline %s/%06d is not idempotent CREATE TABLE IF NOT EXISTS", service, migration.Version)
			}
		}
	}
}

func TestUpAppliesPendingMigrationAndVerifiesSchema(t *testing.T) {
	migrations, err := Load("auth")
	if err != nil {
		t.Fatal(err)
	}
	m := migrations[0]
	db, script := testDB(t,
		q("SELECT GET_LOCK", []string{"lock"}, values(int64(1))),
		e("CREATE TABLE IF NOT EXISTS schema_migrations"),
		q("SELECT checksum, dirty FROM schema_migrations", []string{"checksum", "dirty"}),
		e("INSERT INTO schema_migrations"),
		e("CREATE TABLE IF NOT EXISTS users"),
		e("UPDATE schema_migrations SET dirty=FALSE"),
		q("SELECT id,name,email,password_hash,role,active,created_at FROM users LIMIT 0", []string{"id"}),
		q("SELECT RELEASE_LOCK", []string{"released"}, values(int64(1))),
	)
	err = Up(context.Background(), Target{
		Name: "auth",
		DB:   db,
		VerifySQL: []string{
			"SELECT id,name,email,password_hash,role,active,created_at FROM users LIMIT 0",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if m.Checksum == "" {
		t.Fatal("checksum must not be empty")
	}
	script.done(t)
}

func TestUpSkipsAppliedMigrationWithMatchingChecksum(t *testing.T) {
	migrations, err := Load("auth")
	if err != nil {
		t.Fatal(err)
	}
	m := migrations[0]
	db, script := testDB(t,
		q("SELECT GET_LOCK", []string{"lock"}, values(int64(1))),
		e("CREATE TABLE IF NOT EXISTS schema_migrations"),
		q("SELECT checksum, dirty FROM schema_migrations", []string{"checksum", "dirty"}, values(m.Checksum, false)),
		q("SELECT RELEASE_LOCK", []string{"released"}, values(int64(1))),
	)
	if err := Up(context.Background(), Target{Name: "auth", DB: db}); err != nil {
		t.Fatal(err)
	}
	script.done(t)
}

func TestUpFailsClosedOnDirtyMigration(t *testing.T) {
	migrations, err := Load("auth")
	if err != nil {
		t.Fatal(err)
	}
	m := migrations[0]
	db, _ := testDB(t,
		q("SELECT GET_LOCK", []string{"lock"}, values(int64(1))),
		e("CREATE TABLE IF NOT EXISTS schema_migrations"),
		q("SELECT checksum, dirty FROM schema_migrations", []string{"checksum", "dirty"}, values(m.Checksum, true)),
		q("SELECT RELEASE_LOCK", []string{"released"}, values(int64(1))),
	)
	err = Up(context.Background(), Target{Name: "auth", DB: db})
	if err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("unexpected err=%v", err)
	}
}

func TestUpFailsWhenMigrationChecksumChanged(t *testing.T) {
	db, _ := testDB(t,
		q("SELECT GET_LOCK", []string{"lock"}, values(int64(1))),
		e("CREATE TABLE IF NOT EXISTS schema_migrations"),
		q("SELECT checksum, dirty FROM schema_migrations", []string{"checksum", "dirty"}, values("different", false)),
		q("SELECT RELEASE_LOCK", []string{"released"}, values(int64(1))),
	)
	err := Up(context.Background(), Target{Name: "auth", DB: db})
	if err == nil || !strings.Contains(err.Error(), "checksum changed") {
		t.Fatalf("unexpected err=%v", err)
	}
}
