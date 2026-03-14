package store

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/chairswithlegs/monstera/internal/config"
)

// DatabaseConnectionString returns a PostgreSQL connection string.
// The withPool flags controls whether or not to include the extra connection parameters
// supported by pgxpool.
func DatabaseConnectionString(cfg *config.Config, withPool bool) string {
	query := databaseConnectionStringQueryParams(cfg, withPool)
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?%s", cfg.DatabaseUsername, cfg.DatabasePassword, cfg.DatabaseHost, cfg.DatabasePort, cfg.DatabaseName, query.Encode())
}

func databaseConnectionStringQueryParams(cfg *config.Config, withPool bool) url.Values {
	query := url.Values{}
	if withPool && cfg.DatabaseMaxOpenConns > 0 {
		query.Add("pool_max_conns", strconv.Itoa(cfg.DatabaseMaxOpenConns))
	}
	if cfg.DatabaseSSLMode != "" {
		query.Add("sslmode", cfg.DatabaseSSLMode)
	}
	return query
}
