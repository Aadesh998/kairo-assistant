package store

type Score struct {
	Action string
	Score  float64
	Count  int
}

func (s *Store) AddReward(action string, value float64, note string) {
	s.db.Exec(`INSERT INTO rewards (action, value, note, at) VALUES (?, ?, ?, ?)`, action, value, note, now())
}

func (s *Store) GetScore(action string) Score {
	sc := Score{Action: action}
	s.db.QueryRow(`SELECT score, count FROM scores WHERE action = ?`, action).Scan(&sc.Score, &sc.Count)
	return sc
}

func (s *Store) SaveScore(sc Score) error {
	_, err := s.db.Exec(
		`INSERT INTO scores (action, score, count) VALUES (?, ?, ?)
		 ON CONFLICT(action) DO UPDATE SET score = excluded.score, count = excluded.count`,
		sc.Action, sc.Score, sc.Count)
	return err
}

func (s *Store) Scores() ([]Score, error) {
	rows, err := s.db.Query(`SELECT action, score, count FROM scores ORDER BY score DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Score
	for rows.Next() {
		var sc Score
		if err := rows.Scan(&sc.Action, &sc.Score, &sc.Count); err != nil {
			return nil, err
		}
		list = append(list, sc)
	}
	return list, nil
}

func (s *Store) ResetScore(action string) error {
	_, err := s.db.Exec(`DELETE FROM scores WHERE action = ?`, action)
	return err
}

func (s *Store) RoutineRan(key string) bool {
	res, err := s.db.Exec(`INSERT OR IGNORE INTO routine_runs (key, at) VALUES (?, ?)`, key, now())
	if err != nil {
		return true
	}
	n, _ := res.RowsAffected()
	return n == 0
}
