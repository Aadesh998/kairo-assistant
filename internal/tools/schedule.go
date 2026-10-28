package tools

import (
	"context"
	"fmt"
	"strings"

	"kairo/internal/store"
)

func scheduleTools(s *store.Store, stopJobs func() int) []Tool {
	return []Tool{
		{
			Name:        "stop_running",
			Description: "Immediately stop a meeting join or scheduled job that is running right now, for example when the owner says 'don't join'.",
			Params:      map[string]any{},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				if stopJobs == nil {
					return Result{Text: "nothing running"}, nil
				}
				return Result{Text: fmt.Sprintf("stopped %d running job(s)", stopJobs())}, nil
			},
		},
		{
			Name:        "schedule_join",
			Description: "Join a meeting automatically at a later time, with mic and camera off, and wait there for the owner. The owner approves now, so at that time Kairo joins without asking again.",
			Params: map[string]any{
				"url":   prop("string", "Meeting URL"),
				"at":    prop("string", "When to join: 'HH:MM', 'YYYY-MM-DD HH:MM' or a duration like '30m'"),
				"title": prop("string", "Meeting name"),
			},
			Required: []string{"url", "at"},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				at, err := ParseDue(str(in, "at"))
				if err != nil {
					return Result{}, err
				}
				id, err := s.AddScheduled(at, "join", str(in, "title"), str(in, "url"))
				return Result{Text: fmt.Sprintf("job #%d: will join %s at %s", id, str(in, "url"), at.Format("Mon 15:04"))}, err
			},
		},
		{
			Name:        "schedule_instruction",
			Description: "Do something at a later time: the instruction is given to Kairo at that time and run like a normal request. Use this instead of shell timers or sleep.",
			Params: map[string]any{
				"at":          prop("string", "When: 'HH:MM', 'YYYY-MM-DD HH:MM' or a duration like '30m'"),
				"instruction": prop("string", "What Kairo should do at that time"),
			},
			Required: []string{"at", "instruction"},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				at, err := ParseDue(str(in, "at"))
				if err != nil {
					return Result{}, err
				}
				id, err := s.AddScheduled(at, "instruction", firstLine(str(in, "instruction")), str(in, "instruction"))
				return Result{Text: fmt.Sprintf("job #%d scheduled for %s", id, at.Format("Mon 15:04"))}, err
			},
		},
		{
			Name:        "list_scheduled",
			Description: "List pending scheduled jobs (meeting joins and later instructions).",
			Params:      map[string]any{},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				list, err := s.PendingScheduled()
				if err != nil {
					return Result{}, err
				}
				return Result{Text: FormatScheduled(list)}, nil
			},
		},
		{
			Name:        "cancel_scheduled",
			Description: "Cancel a pending scheduled job by id.",
			Params:      map[string]any{"id": prop("integer", "Job id")},
			Required:    []string{"id"},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				return Result{Text: "cancelled"}, s.SetScheduledStatus(num(in, "id"), "cancelled")
			},
		},
	}
}

func FormatScheduled(list []store.Scheduled) string {
	if len(list) == 0 {
		return "nothing scheduled"
	}
	var lines []string
	for _, j := range list {
		lines = append(lines, fmt.Sprintf("#%d %s %s %s", j.ID, j.RunAt.Format("Mon 02 Jan 15:04"), j.Kind, j.Title))
	}
	return strings.Join(lines, "\n")
}
