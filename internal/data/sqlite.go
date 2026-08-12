package data

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"unicode/utf8"
)

// readOnlySQLiteDSN returns an escaped SQLite file URI that enforces
// read-only access at both the file-open and connection levels.
func readOnlySQLiteDSN(path string) (string, error) {
	return sqliteFileDSN(path, "ro", true)
}

// readWriteSQLiteDSN returns an escaped SQLite file URI that allows writes but
// refuses to create a missing database.
func readWriteSQLiteDSN(path string) (string, error) {
	return sqliteFileDSN(path, "rw", false)
}

func sqliteFileDSN(path, mode string, queryOnly bool) (string, error) {
	if !utf8.ValidString(path) {
		return "", fmt.Errorf("SQLite path is not valid UTF-8")
	}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(absolute) {
		return "", fmt.Errorf("SQLite path could not be made absolute: %q", path)
	}

	volume := filepath.VolumeName(absolute)
	if len(volume) > 0 && volume[len(volume)-1] == ':' &&
		(len(volume) != 2 || !isASCIILetter(volume[0])) {
		return "", fmt.Errorf("SQLite path has unsupported volume %q", volume)
	}

	uriPath := filepath.ToSlash(absolute)
	if volume != "" {
		uriPath = "/" + uriPath
	}
	uri := url.URL{
		Scheme: "file",
		Path:   uriPath,
	}
	query := uri.Query()
	query.Set("mode", mode)
	if queryOnly {
		query.Set("_query_only", "1")
	}
	uri.RawQuery = query.Encode()
	return uri.String(), nil
}

func isASCIILetter(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

// openReadOnlySQLite opens path with the shared read-only SQLite protections.
func openReadOnlySQLite(path string) (*sql.DB, error) {
	dsn, err := readOnlySQLiteDSN(path)
	if err != nil {
		return nil, err
	}
	return sql.Open("sqlite", dsn)
}
