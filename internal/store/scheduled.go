package store

import "time"

type Scheduled struct {
	ID      int64
	RunAt   time.Time
	Kind    string
	Title   string
	Payload string
	Status  string
}

func (s *Store) AddScheduled(runAt time.Time, kind, title, payload string) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO scheduled (run_at, kind, title, payload, created_at) VALUES (?, ?, ?, ?, ?)`,
		runAt.Format(timeFormat), kind, title, payload, now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) DueScheduled(at time.Time) ([]Scheduled, error) {
	return s.queryScheduled(`SELECT id, run_at, kind, title, payload, status FROM scheduled WHERE status = 'pending' AND run_at <= ? ORDER BY run_at`, at.Format(timeFormat))
}

func (s *Store) PendingScheduled() ([]Scheduled, error) {
	return s.queryScheduled(`SELECT id, run_at, kind, title, payload, status FROM scheduled WHERE status = 'pending' ORDER BY run_at`)
}

func (s *Store) SetScheduledStatus(id int64, status string) error {
	_, err := s.db.Exec(`UPDATE scheduled SET status = ? WHERE id = ?`, status, id)
	return err
}

func (s *Store) HasJoinFor(link string) bool {
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM scheduled WHERE kind = 'join' AND payload = ? AND status != 'cancelled'`, link).Scan(&n)
	return n > 0
}

func (s *Store) queryScheduled(query string, args ...any) ([]Scheduled, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Scheduled
	for rows.Next() {
		var j Scheduled
		var at string
		if err := rows.Scan(&j.ID, &at, &j.Kind, &j.Title, &j.Payload, &j.Status); err != nil {
			return nil, err
		}
		j.RunAt = parseTime(at)
		list = append(list, j)
	}
	return list, nil
}
