package data

import (
	"database/sql"
	"net/url"
	"path/filepath"
)

// readOnlySQLiteDSN returns an escaped SQLite file URI that enforces
// read-only access at both the file-open and connection levels.
func readOnlySQLiteDSN(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	uriPath := filepath.ToSlash(absolute)
	if filepath.VolumeName(absolute) != "" {
		uriPath = "/" + uriPath
	}
	uri := url.URL{
		Scheme: "file",
		Path:   uriPath,
	}
	query := uri.Query()
	query.Set("mode", "ro")
	query.Set("_query_only", "1")
	uri.RawQuery = query.Encode()
	return uri.String(), nil
}

// openReadOnlySQLite opens path with the shared read-only SQLite protections.
func openReadOnlySQLite(path string) (*sql.DB, error) {
	dsn, err := readOnlySQLiteDSN(path)
	if err != nil {
		return nil, err
	}
	return sql.Open("sqlite", dsn)
}
