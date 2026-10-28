package learning

import (
	"kairo/internal/config"
	"kairo/internal/store"
)

type Learner struct {
	store *store.Store
	cfg   config.Learning
}

func New(s *store.Store, cfg config.Learning) *Learner {
	return &Learner{store: s, cfg: cfg}
}

func (l *Learner) Reward(action string, value float64, note string) {
	l.store.AddReward(action, value, note)
	sc := l.store.GetScore(action)
	sc.Score = sc.Score + l.cfg.Rate*(value-sc.Score)
	sc.Count++
	l.store.SaveScore(sc)
	l.store.Log("learn", formatReward(action, value, sc.Score))
}
