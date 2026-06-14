package config

import "testing"

func TestDatabaseFromEnvDefaultsToBuiltin(t *testing.T) {
	t.Setenv("DS2API_DATABASE_MODE", "")
	t.Setenv("DS2API_DATABASE_TYPE", "")
	t.Setenv("DS2API_DATABASE_DSN", "")
	t.Setenv("DS2API_DATABASE_URL", "")
	t.Setenv("DATABASE_URL", "")

	cfg := DatabaseFromEnv()
	if cfg.Mode != DatabaseModeBuiltin {
		t.Fatalf("expected builtin mode, got %q", cfg.Mode)
	}
	if cfg.ExternalEnabled() {
		t.Fatal("expected external database to be disabled")
	}
}

func TestDatabaseFromEnvEnablesExternalFromDSN(t *testing.T) {
	t.Setenv("DS2API_DATABASE_MODE", "")
	t.Setenv("DS2API_DATABASE_TYPE", "postgresql")
	t.Setenv("DS2API_DATABASE_DSN", "postgres://user:pass@example.com:5432/ds2api")
	t.Setenv("DS2API_DATABASE_TABLE_PREFIX", "tenant_")
	t.Setenv("DS2API_DATABASE_MAX_OPEN_CONNS", "7")
	t.Setenv("DS2API_DATABASE_MAX_IDLE_CONNS", "3")
	t.Setenv("DS2API_DATABASE_CONN_MAX_LIFETIME_SECONDS", "60")

	cfg := DatabaseFromEnv()
	if cfg.Mode != DatabaseModeExternal {
		t.Fatalf("expected external mode, got %q", cfg.Mode)
	}
	if cfg.Type != "postgres" {
		t.Fatalf("expected normalized postgres type, got %q", cfg.Type)
	}
	if cfg.TablePrefix != "tenant_" {
		t.Fatalf("expected table prefix to be preserved, got %q", cfg.TablePrefix)
	}
	if cfg.MaxOpenConns != 7 || cfg.MaxIdleConns != 3 || cfg.ConnMaxLifetimeSeconds != 60 {
		t.Fatalf("unexpected pool settings: %#v", cfg)
	}
}

func TestDatabaseFromEnvInfersTypeFromDatabaseURL(t *testing.T) {
	t.Setenv("DS2API_DATABASE_MODE", "external")
	t.Setenv("DS2API_DATABASE_TYPE", "")
	t.Setenv("DS2API_DATABASE_DSN", "")
	t.Setenv("DS2API_DATABASE_URL", "")
	t.Setenv("DATABASE_URL", "mysql://user:pass@example.com:3306/ds2api")

	cfg := DatabaseFromEnv()
	if cfg.Type != "mysql" {
		t.Fatalf("expected mysql type inferred from DATABASE_URL, got %q", cfg.Type)
	}
	if cfg.DSN == "" {
		t.Fatal("expected database url to be used as dsn")
	}
}

func TestDatabaseFromEnvDoesNotImplicitlyUseGenericDatabaseURL(t *testing.T) {
	t.Setenv("DS2API_DATABASE_MODE", "")
	t.Setenv("DS2API_DATABASE_TYPE", "")
	t.Setenv("DS2API_DATABASE_DSN", "")
	t.Setenv("DS2API_DATABASE_URL", "")
	t.Setenv("DATABASE_URL", "postgres://user:pass@example.com:5432/other")

	cfg := DatabaseFromEnv()
	if cfg.Mode != DatabaseModeBuiltin {
		t.Fatalf("expected generic DATABASE_URL alone to keep builtin mode, got %q", cfg.Mode)
	}
}
