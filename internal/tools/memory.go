package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"kairo/internal/store"
)

func memoryTools(s *store.Store, notify func(string)) []Tool {
	return []Tool{
		{
			Name:        "remember",
			Description: "Save a fact or preference about the owner for the future.",
			Params:      map[string]any{"text": prop("string", "The fact to remember")},
			Required:    []string{"text"},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				id, err := s.AddMemory("fact", str(in, "text"))
				return Result{Text: fmt.Sprintf("remembered #%d", id)}, err
			},
		},
		{
			Name:        "recall",
			Description: "Search saved memories.",
			Params:      map[string]any{"query": prop("string", "Word or phrase to search")},
			Required:    []string{"query"},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				list, err := s.SearchMemories(str(in, "query"))
				if err != nil {
					return Result{}, err
				}
				var lines []string
				for _, m := range list {
					lines = append(lines, fmt.Sprintf("#%d [%s] %s", m.ID, m.Kind, m.Text))
				}
				if len(lines) == 0 {
					return Result{Text: "nothing found"}, nil
				}
				return Result{Text: strings.Join(lines, "\n")}, nil
			},
		},
		{
			Name:        "recent_activity",
			Description: "Read what Kairo did today: actions, meetings, reminders, messages.",
			Params:      map[string]any{},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				n := time.Now()
				list, err := s.Activity(time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.Local), 0, 80)
				if err != nil {
					return Result{}, err
				}
				var lines []string
				for i := len(list) - 1; i >= 0; i-- {
					a := list[i]
					lines = append(lines, fmt.Sprintf("%s %s %s", a.At.Local().Format("15:04"), a.Kind, a.Text))
				}
				return Result{Text: strings.Join(lines, "\n") + " "}, nil
			},
		},
		{
			Name:        "send_update",
			Description: "Send a message to the owner on their connected channels (Telegram, Slack, WhatsApp).",
			Params:      map[string]any{"text": prop("string", "Message to send")},
			Required:    []string{"text"},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				if notify != nil {
					notify(str(in, "text"))
				}
				return Result{Text: "sent"}, nil
			},
		},
	}
}
