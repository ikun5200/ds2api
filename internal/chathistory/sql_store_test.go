package chathistory

import (
	"strings"
	"testing"
)

func TestNormalizeSQLDialectSupportsExternalTypes(t *testing.T) {
	tests := []struct {
		input  string
		name   string
		driver string
	}{
		{input: "postgresql", name: "postgres", driver: "pgx"},
		{input: "pgx", name: "postgres", driver: "pgx"},
		{input: "mysql", name: "mysql", driver: "mysql"},
		{input: "mariadb", name: "mysql", driver: "mysql"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := normalizeSQLDialect(tt.input)
			if err != nil {
				t.Fatalf("normalizeSQLDialect(%q) failed: %v", tt.input, err)
			}
			if got.name != tt.name || got.driver != tt.driver {
				t.Fatalf("unexpected dialect: %#v", got)
			}
		})
	}
}

func TestSQLTableNameRejectsUnsafePrefix(t *testing.T) {
	if _, err := sqlTableName("tenant_", "chat_history_entries"); err != nil {
		t.Fatalf("expected safe prefix to pass: %v", err)
	}
	if _, err := sqlTableName("tenant;drop_", "chat_history_entries"); err == nil {
		t.Fatal("expected unsafe prefix to be rejected")
	}
	if _, err := sqlTableName("1tenant_", "chat_history_entries"); err == nil {
		t.Fatal("expected digit-leading table name to be rejected")
	}
}

func TestSQLDialectBuildsDriverSpecificStatements(t *testing.T) {
	postgres := sqlDialect{name: "postgres", driver: "pgx"}
	mysql := sqlDialect{name: "mysql", driver: "mysql"}

	if postgres.placeholder(2) != "$2" {
		t.Fatalf("unexpected postgres placeholder: %q", postgres.placeholder(2))
	}
	if mysql.placeholder(2) != "?" {
		t.Fatalf("unexpected mysql placeholder: %q", mysql.placeholder(2))
	}
	if !strings.Contains(postgres.upsertEntrySQL("ds2api_chat_history_entries"), "ON CONFLICT") {
		t.Fatal("expected postgres upsert to use ON CONFLICT")
	}
	if !strings.Contains(mysql.upsertEntrySQL("ds2api_chat_history_entries"), "ON DUPLICATE KEY UPDATE") {
		t.Fatal("expected mysql upsert to use ON DUPLICATE KEY UPDATE")
	}
	if !strings.Contains(mysql.createEntriesTableSQL("ds2api_chat_history_entries"), "LONGTEXT") {
		t.Fatal("expected mysql payload column to use LONGTEXT")
	}
}

func TestMySQLURLToDSN(t *testing.T) {
	got, err := mysqlURLToDSN("mysql://user:pass@example.com:3306/ds2api?parseTime=true")
	if err != nil {
		t.Fatalf("mysqlURLToDSN failed: %v", err)
	}
	want := "user:pass@tcp(example.com:3306)/ds2api?parseTime=true"
	if got != want {
		t.Fatalf("mysqlURLToDSN = %q, want %q", got, want)
	}
}
