package store

import "time"

type Event struct {
	ID       string
	Title    string
	Start    time.Time
	End      time.Time
	AllDay   bool
	Link     string
	Location string
}

func (s *Store) ReplaceEvents(from, to time.Time, list []Event) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM events WHERE start_at >= ? AND start_at < ?`, from.Format(timeFormat), to.Format(timeFormat)); err != nil {
		return err
	}
	for _, e := range list {
		_, err := tx.Exec(
			`INSERT OR REPLACE INTO events (id, title, start_at, end_at, all_day, link, location) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			e.ID, e.Title, e.Start.Format(timeFormat), e.End.Format(timeFormat), e.AllDay, e.Link, e.Location)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) EventsBetween(from, to time.Time) ([]Event, error) {
	rows, err := s.db.Query(
		`SELECT id, title, start_at, end_at, all_day, link, location FROM events WHERE end_at > ? AND start_at < ? ORDER BY start_at`,
		from.Format(timeFormat), to.Format(timeFormat))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Event
	for rows.Next() {
		var e Event
		var start, end string
		if err := rows.Scan(&e.ID, &e.Title, &start, &end, &e.AllDay, &e.Link, &e.Location); err != nil {
			return nil, err
		}
		e.Start, e.End = parseTime(start), parseTime(end)
		list = append(list, e)
	}
	return list, nil
}
