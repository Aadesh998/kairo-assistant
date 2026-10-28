package memory

import (
	"context"
	"fmt"
	"strings"

	"kairo/internal/jev"
	"kairo/internal/store"
)

func (m *Manager) Compact(ctx context.Context) error {
	if err := m.compactThreads(ctx); err != nil {
		return err
	}
	return m.compactFacts(ctx)
}

func (m *Manager) compactThreads(ctx context.Context) error {
	max := m.Config.Memory.MaxThreadLength
	threads, err := m.Store.LongThreads(max)
	if err != nil {
		return err
	}
	for _, thread := range threads {
		ids, old, err := m.Store.OldestMessages(thread, max/2)
		if err != nil || len(old) == 0 {
			continue
		}
		var b strings.Builder
		for _, msg := range old {
			b.WriteString(msg.Role + ": " + msg.Text + "\n")
		}
		summary, err := m.LLM.Ask(ctx,
			"Summarize this older conversation into at most 5 short lines: lasting facts about the owner, decisions made and unfinished work. Plain text, one per line.",
			b.String())
		if err != nil {
			return err
		}
		for _, line := range lines(summary) {
			m.Store.AddMemory("summary", line)
		}
		m.Store.DeleteMessages(ids)
		m.Store.Log("memory", fmt.Sprintf("compacted %d %s messages", len(ids), thread))
	}
	return nil
}

func (m *Manager) compactFacts(ctx context.Context) error {
	facts, _ := m.Store.MemoriesByKind("fact")
	summaries, _ := m.Store.MemoriesByKind("summary")
	all := append(facts, summaries...)
	if len(all) <= m.Config.Memory.MaxFacts {
		return nil
	}
	keep := m.useful(ctx, all)
	var b strings.Builder
	for _, f := range keep {
		b.WriteString("- " + f.Text + "\n")
	}
	merged, err := m.LLM.Ask(ctx,
		fmt.Sprintf("Merge these notes about the owner into at most %d short facts. Remove duplicates and outdated items, keep the newest when they conflict. One fact per line, no numbering.", m.Config.Memory.MaxFacts/2),
		b.String())
	if err != nil {
		return err
	}
	result := lines(merged)
	if len(result) == 0 {
		return nil
	}
	m.Store.DeleteMemoriesByKind("fact")
	m.Store.DeleteMemoriesByKind("summary")
	for _, line := range result {
		m.Store.AddMemory("fact", line)
	}
	m.Store.Log("memory", fmt.Sprintf("long-term memory compacted %d -> %d facts", len(all), len(result)))
	return nil
}

func (m *Manager) useful(ctx context.Context, list []store.Memory) []store.Memory {
	if !m.Jev.Ready() {
		return list
	}
	var state strings.Builder
	questions := map[string]jev.Question{}
	for _, f := range list {
		id := fmt.Sprintf("f%d", f.ID)
		state.WriteString(id + ": " + f.Text + "\n")
		questions[id] = jev.Noul("Note " + id + " is still useful for a personal assistant and is not trivial")
	}
	answers, err := m.Jev.Decide(ctx, state.String(), questions)
	if err != nil {
		return list
	}
	var keep []store.Memory
	for _, f := range list {
		if answers[fmt.Sprintf("f%d", f.ID)].Noul >= 0.3 {
			keep = append(keep, f)
		}
	}
	return keep
}

func lines(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(line, "-•* "))
		if line != "" && !strings.EqualFold(line, "none") {
			out = append(out, line)
		}
	}
	return out
}
