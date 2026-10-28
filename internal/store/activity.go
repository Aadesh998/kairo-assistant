package store

import "time"

type Activity struct {
	ID   int64
	At   time.Time
	Kind string
	Text string
}

func (s *Store) Log(kind, text string) {
	s.db.Exec(`INSERT INTO activity (at, kind, text) VALUES (?, ?, ?)`, now(), kind, text)
}

func (s *Store) Activity(since time.Time, afterID int64, limit int) ([]Activity, error) {
	rows, err := s.db.Query(
		`SELECT id, at, kind, text FROM activity WHERE at >= ? AND id > ? ORDER BY id DESC LIMIT ?`,
		since.Format(timeFormat), afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Activity
	for rows.Next() {
		var a Activity
		var at string
		if err := rows.Scan(&a.ID, &at, &a.Kind, &a.Text); err != nil {
			return nil, err
		}
		a.At = parseTime(at)
		list = append(list, a)
	}
	return list, nil
}
