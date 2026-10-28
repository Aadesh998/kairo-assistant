package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"kairo/internal/store"
)

func taskTools(s *store.Store) []Tool {
	return []Tool{
		{
			Name:        "add_task",
			Description: "Save a task or reminder. The owner gets reminded at the due time.",
			Params: map[string]any{
				"title": prop("string", "What needs to be done"),
				"due":   prop("string", "Due time as 'YYYY-MM-DD HH:MM' in local time, empty if none"),
			},
			Required: []string{"title"},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				due, err := ParseDue(str(in, "due"))
				if err != nil {
					return Result{}, err
				}
				id, err := s.AddTask(str(in, "title"), due)
				return Result{Text: fmt.Sprintf("task %d saved", id)}, err
			},
		},
		{
			Name:        "list_tasks",
			Description: "List the owner's open tasks.",
			Params:      map[string]any{},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				tasks, err := s.OpenTasks()
				if err != nil {
					return Result{}, err
				}
				var lines []string
				for _, t := range tasks {
					lines = append(lines, fmt.Sprintf("#%d %s %s", t.ID, t.Title, DueText(t.DueAt)))
				}
				if len(lines) == 0 {
					return Result{Text: "no open tasks"}, nil
				}
				return Result{Text: strings.Join(lines, "\n")}, nil
			},
		},
		{
			Name:        "complete_task",
			Description: "Mark a task as done.",
			Params:      map[string]any{"id": prop("integer", "Task id")},
			Required:    []string{"id"},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				return Result{Text: "done"}, s.CompleteTask(num(in, "id"))
			},
		},
	}
}

func ParseDue(text string) (time.Time, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return time.Time{}, nil
	}
	if d, err := time.ParseDuration(strings.TrimPrefix(text, "in ")); err == nil {
		return time.Now().Add(d), nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04", text, time.Local); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("15:04", text, time.Local); err == nil {
		n := time.Now()
		due := time.Date(n.Year(), n.Month(), n.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
		if due.Before(n) {
			due = due.Add(24 * time.Hour)
		}
		return due, nil
	}
	return time.Time{}, fmt.Errorf("cannot read due time %q, use 'YYYY-MM-DD HH:MM', 'HH:MM' or '30m'", text)
}

func DueText(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return "(due " + t.Local().Format("Mon 02 Jan 15:04") + ")"
}
