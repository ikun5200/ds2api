package config

import (
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	DatabaseModeBuiltin  = "builtin"
	DatabaseModeExternal = "external"
)

type DatabaseConfig struct {
	Mode                   string
	Type                   string
	DSN                    string
	TablePrefix            string
	MaxOpenConns           int
	MaxIdleConns           int
	ConnMaxLifetimeSeconds int
}

func DatabaseFromEnv() DatabaseConfig {
	dsn := firstEnv("DS2API_DATABASE_DSN", "DS2API_DATABASE_URL")
	genericDSN := firstEnv("DATABASE_URL")
	dbType := firstEnv("DS2API_DATABASE_TYPE", "DS2API_DB_TYPE")
	mode := normalizeDatabaseMode(firstEnv("DS2API_DATABASE_MODE", "DS2API_DB_MODE"))
	if mode == "" {
		if strings.TrimSpace(dsn) != "" || strings.TrimSpace(dbType) != "" {
			mode = DatabaseModeExternal
		} else {
			mode = DatabaseModeBuiltin
		}
	}
	if mode == DatabaseModeExternal && strings.TrimSpace(dsn) == "" {
		dsn = genericDSN
	}
	if dbType == "" {
		dbType = inferDatabaseType(dsn)
	}
	return DatabaseConfig{
		Mode:                   mode,
		Type:                   normalizeDatabaseType(dbType),
		DSN:                    strings.TrimSpace(dsn),
		TablePrefix:            firstEnv("DS2API_DATABASE_TABLE_PREFIX", "DS2API_DB_TABLE_PREFIX"),
		MaxOpenConns:           envInt("DS2API_DATABASE_MAX_OPEN_CONNS", 0),
		MaxIdleConns:           envInt("DS2API_DATABASE_MAX_IDLE_CONNS", 0),
		ConnMaxLifetimeSeconds: envInt("DS2API_DATABASE_CONN_MAX_LIFETIME_SECONDS", 0),
	}
}

func (c DatabaseConfig) ExternalEnabled() bool {
	return c.Mode == DatabaseModeExternal
}

func normalizeDatabaseMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", DatabaseModeBuiltin, "internal", "embedded", "file", "local":
		if strings.TrimSpace(mode) == "" {
			return ""
		}
		return DatabaseModeBuiltin
	case DatabaseModeExternal, "sql", "database", "db":
		return DatabaseModeExternal
	default:
		return strings.ToLower(strings.TrimSpace(mode))
	}
}

func normalizeDatabaseType(dbType string) string {
	switch strings.ToLower(strings.TrimSpace(dbType)) {
	case "postgres", "postgresql", "pg", "pgx":
		return "postgres"
	case "mysql", "mariadb":
		return "mysql"
	default:
		return strings.ToLower(strings.TrimSpace(dbType))
	}
}

func inferDatabaseType(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return ""
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		return ""
	}
	return normalizeDatabaseType(parsed.Scheme)
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if val := strings.TrimSpace(os.Getenv(key)); val != "" {
			return val
		}
	}
	return ""
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}
