package store

import "time"

type Mail struct {
	UID         uint32
	From        string
	Subject     string
	Summary     string
	Importance  string
	NeedsAction bool
	At          time.Time
}

func (s *Store) SaveMail(mailbox string, m Mail) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO mails (mailbox, uid, sender, subject, summary, importance, needs_action, at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		mailbox, m.UID, m.From, m.Subject, m.Summary, m.Importance, m.NeedsAction, m.At.Format(timeFormat))
	return err
}

func (s *Store) RecentMails(limit int) ([]Mail, error) {
	rows, err := s.db.Query(
		`SELECT uid, sender, subject, summary, importance, needs_action, at FROM mails WHERE importance != 'ignore' ORDER BY at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Mail
	for rows.Next() {
		var m Mail
		var at string
		if err := rows.Scan(&m.UID, &m.From, &m.Subject, &m.Summary, &m.Importance, &m.NeedsAction, &at); err != nil {
			return nil, err
		}
		m.At = parseTime(at)
		list = append(list, m)
	}
	return list, nil
}

func (s *Store) Get(key string) string {
	var value string
	s.db.QueryRow(`SELECT value FROM kv WHERE key = ?`, key).Scan(&value)
	return value
}

func (s *Store) Set(key, value string) {
	s.db.Exec(`INSERT OR REPLACE INTO kv (key, value) VALUES (?, ?)`, key, value)
}
