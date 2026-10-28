package store

type Memory struct {
	ID   int64
	Kind string
	Text string
}

type Message struct {
	Role string
	Text string
}

func (s *Store) AddMemory(kind, text string) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO memories (kind, text, created_at) VALUES (?, ?, ?)`, kind, text, now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) Memories(limit int) ([]Memory, error) {
	return s.queryMemories(`SELECT id, kind, text FROM memories ORDER BY id DESC LIMIT ?`, limit)
}

func (s *Store) SearchMemories(query string) ([]Memory, error) {
	return s.queryMemories(`SELECT id, kind, text FROM memories WHERE text LIKE ? ORDER BY id DESC LIMIT 20`, "%"+query+"%")
}

func (s *Store) DeleteMemory(id int64) error {
	_, err := s.db.Exec(`DELETE FROM memories WHERE id = ?`, id)
	return err
}

func (s *Store) queryMemories(query string, args ...any) ([]Memory, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.Kind, &m.Text); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, nil
}

func (s *Store) AddMessage(thread, role, text string) {
	s.db.Exec(`INSERT INTO messages (thread, role, text, at) VALUES (?, ?, ?, ?)`, thread, role, text, now())
}

func (s *Store) RecentMessages(thread string, limit int) ([]Message, error) {
	rows, err := s.db.Query(
		`SELECT role, text FROM (SELECT id, role, text FROM messages WHERE thread = ? ORDER BY id DESC LIMIT ?) ORDER BY id`,
		thread, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.Role, &m.Text); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, nil
}

func (s *Store) MemoriesByKind(kind string) ([]Memory, error) {
	return s.queryMemories(`SELECT id, kind, text FROM memories WHERE kind = ? ORDER BY id`, kind)
}

func (s *Store) DeleteMemoriesByKind(kind string) error {
	_, err := s.db.Exec(`DELETE FROM memories WHERE kind = ?`, kind)
	return err
}

func (s *Store) LongThreads(max int) ([]string, error) {
	rows, err := s.db.Query(`SELECT thread FROM messages GROUP BY thread HAVING COUNT(*) > ?`, max)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, nil
}

func (s *Store) OldestMessages(thread string, keep int) ([]int64, []Message, error) {
	rows, err := s.db.Query(
		`SELECT id, role, text FROM messages WHERE thread = ? AND id NOT IN (SELECT id FROM messages WHERE thread = ? ORDER BY id DESC LIMIT ?) ORDER BY id`,
		thread, thread, keep)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var ids []int64
	var list []Message
	for rows.Next() {
		var id int64
		var m Message
		if err := rows.Scan(&id, &m.Role, &m.Text); err != nil {
			return nil, nil, err
		}
		ids = append(ids, id)
		list = append(list, m)
	}
	return ids, list, nil
}

func (s *Store) DeleteMessages(ids []int64) {
	for _, id := range ids {
		s.db.Exec(`DELETE FROM messages WHERE id = ?`, id)
	}
}
