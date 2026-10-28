package store

import (
	"database/sql"
	_ "embed"
	"os"
	"time"

	_ "modernc.org/sqlite"

	"kairo/internal/config"
)

//go:embed schema.sql
var schema string

const timeFormat = time.RFC3339

type Store struct {
	db *sql.DB
}

func Open() (*Store, error) {
	if err := os.MkdirAll(config.Home(), 0o700); err != nil {
		return nil, err
	}
	dsn := "file:" + config.Path("kairo.db") + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func now() string {
	return time.Now().Format(timeFormat)
}

func parseTime(v string) time.Time {
	t, _ := time.Parse(timeFormat, v)
	return t
}
