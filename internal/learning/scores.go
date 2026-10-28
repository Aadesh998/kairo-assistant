package learning

import "fmt"

func (l *Learner) Auto(action string) (bool, float64) {
	sc := l.store.GetScore(action)
	return sc.Score >= l.cfg.AutoScore && sc.Count >= l.cfg.AutoCount, sc.Score
}

func formatReward(action string, value, score float64) string {
	return fmt.Sprintf("%+.1f %s -> score %.2f", value, action, score)
}
