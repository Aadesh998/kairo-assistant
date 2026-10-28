package store

import "time"

type Task struct {
	ID     int64
	Title  string
	DueAt  time.Time
	Status string
}

func (s *Store) AddTask(title string, due time.Time) (int64, error) {
	dueText := ""
	if !due.IsZero() {
		dueText = due.Format(timeFormat)
	}
	res, err := s.db.Exec(`INSERT INTO tasks (title, due_at, created_at) VALUES (?, ?, ?)`, title, dueText, now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) OpenTasks() ([]Task, error) {
	return s.queryTasks(`SELECT id, title, due_at, status FROM tasks WHERE status = 'open' ORDER BY due_at`)
}

func (s *Store) DueTasks(at time.Time) ([]Task, error) {
	return s.queryTasks(
		`SELECT id, title, due_at, status FROM tasks WHERE status = 'open' AND reminded = 0 AND due_at != '' AND due_at <= ?`,
		at.Format(timeFormat))
}

func (s *Store) MarkReminded(id int64) error {
	_, err := s.db.Exec(`UPDATE tasks SET reminded = 1 WHERE id = ?`, id)
	return err
}

func (s *Store) CompleteTask(id int64) error {
	_, err := s.db.Exec(`UPDATE tasks SET status = 'done' WHERE id = ?`, id)
	return err
}

func (s *Store) SnoozeTask(id int64, until time.Time) error {
	_, err := s.db.Exec(`UPDATE tasks SET due_at = ?, reminded = 0 WHERE id = ?`, until.Format(timeFormat), id)
	return err
}

func (s *Store) queryTasks(query string, args ...any) ([]Task, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Task
	for rows.Next() {
		var t Task
		var due string
		if err := rows.Scan(&t.ID, &t.Title, &due, &t.Status); err != nil {
			return nil, err
		}
		t.DueAt = parseTime(due)
		list = append(list, t)
	}
	return list, nil
}
