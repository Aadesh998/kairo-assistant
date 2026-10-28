package memory

import (
	"context"
	"regexp"
	"strings"

	"kairo/internal/config"
	"kairo/internal/jev"
	"kairo/internal/learning"
	"kairo/internal/llm"
	"kairo/internal/store"
)

type Manager struct {
	Config  *config.Config
	Store   *store.Store
	LLM     *llm.Client
	Jev     *jev.Client
	Learner *learning.Learner
}

type signals struct {
	praise     bool
	correction bool
	fact       bool
}

var (
	praiseWords     = regexp.MustCompile(`(?i)\b(good job|well done|perfect|great|thanks|thank you|nice)\b`)
	correctionWords = regexp.MustCompile(`(?i)^(no\b|don't|dont|stop doing|wrong|not like that|never do)`)
	factWords       = regexp.MustCompile(`(?i)\b(i am|i'm|my |i prefer|i like|i usually|i always|i never|remember)\b`)
)

func (m *Manager) Capture(ctx context.Context, text string, lastActions []string) {
	if len(strings.Fields(text)) < 2 {
		return
	}
	sig := m.signals(ctx, text)
	if sig.praise {
		for _, a := range lastActions {
			m.Learner.Reward(a, 1, "praise")
		}
	}
	if sig.correction && len(lastActions) > 0 {
		for _, a := range lastActions {
			m.Learner.Reward(a, -1, "correction")
		}
		m.Store.AddMemory("rule", "Owner correction: "+text)
	}
	if sig.fact {
		if fact := m.writeFact(ctx, text); fact != "" {
			m.Store.AddMemory("fact", fact)
			m.Store.Log("memory", "learned: "+fact)
		}
	}
}

func (m *Manager) signals(ctx context.Context, text string) signals {
	if m.Jev.Ready() {
		answers, err := m.Jev.Decide(ctx, text, map[string]jev.Question{
			"praise":     jev.Noul("The owner praises or thanks the assistant for what it just did"),
			"correction": jev.Noul("The owner says the assistant did something wrong or should not do it again"),
			"fact":       jev.Noul("The owner states a lasting fact, preference, habit, person or plan about themselves that is worth remembering for months"),
		})
		if err == nil {
			t := m.Config.Jev.Threshold
			return signals{
				praise:     answers["praise"].Noul >= t,
				correction: answers["correction"].Noul >= t,
				fact:       answers["fact"].Noul >= t,
			}
		}
		m.Store.Log("error", "jev: "+err.Error())
	}
	return signals{
		praise:     praiseWords.MatchString(text),
		correction: correctionWords.MatchString(text),
		fact:       factWords.MatchString(text),
	}
}

func (m *Manager) writeFact(ctx context.Context, text string) string {
	fact, err := m.LLM.Ask(ctx,
		"Extract one short lasting fact about the owner from their message. Write it in third person, at most 20 words. Reply with only the fact, or NONE if there is no lasting fact.",
		text)
	fact = strings.TrimSpace(fact)
	if err != nil || fact == "" || strings.EqualFold(fact, "NONE") {
		return ""
	}
	return fact
}
