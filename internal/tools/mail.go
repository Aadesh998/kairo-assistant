package tools

import (
	"context"
	"fmt"
	"strings"

	"kairo/internal/store"
)

func mailTools(s *store.Store) []Tool {
	return []Tool{
		{
			Name:        "recent_mail",
			Description: "List the owner's recent non-spam emails with Kairo's importance rating and summary.",
			Params:      map[string]any{},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				list, err := s.RecentMails(20)
				if err != nil {
					return Result{}, err
				}
				var lines []string
				for _, m := range list {
					action := ""
					if m.NeedsAction {
						action = " [needs action]"
					}
					lines = append(lines, fmt.Sprintf("%s %s%s | %s | %s | %s", m.At.Local().Format("02 Jan 15:04"), m.Importance, action, m.From, m.Subject, m.Summary))
				}
				if len(lines) == 0 {
					return Result{Text: "no mail seen yet"}, nil
				}
				return Result{Text: strings.Join(lines, "\n")}, nil
			},
		},
	}
}
