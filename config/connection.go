package config

import (
	"database/sql"
	"fmt"
)

func NewConnection(cfg *Config) (*sql.DB, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.PgHost, cfg.PgPort, cfg.PgUser, cfg.PgPass, cfg.PgDBName, cfg.PgSSLMode,
	)
	return sql.Open("postgres", connStr)
}
