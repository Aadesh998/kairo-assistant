package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type window struct {
	Class     string `json:"class"`
	Title     string `json:"title"`
	Workspace struct {
		Name string `json:"name"`
	} `json:"workspace"`
}

func windowTools() []Tool {
	return []Tool{
		{
			Name:        "list_windows",
			Description: "List open windows with their app class, title and workspace.",
			Params:      map[string]any{},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				out, err := run(ctx, 5*time.Second, "hyprctl", "clients", "-j")
				if err != nil {
					return Result{}, err
				}
				var list []window
				if err := json.Unmarshal([]byte(out), &list); err != nil {
					return Result{}, err
				}
				var lines []string
				for _, w := range list {
					lines = append(lines, fmt.Sprintf("%s | %s | workspace %s", w.Class, w.Title, w.Workspace.Name))
				}
				return Result{Text: strings.Join(lines, "\n")}, nil
			},
		},
		{
			Name:        "focus_window",
			Description: "Focus a window by its app class (as shown in list_windows).",
			Params:      map[string]any{"class": prop("string", "Window class")},
			Required:    []string{"class"},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				out, err := hyprDispatch(ctx, "hl.dsp.focus({ window = "+luaString("class:"+str(in, "class"))+" })")
				return Result{Text: out}, err
			},
		},
		{
			Name:        "switch_workspace",
			Description: "Switch to a workspace number.",
			Params:      map[string]any{"number": prop("integer", "Workspace number")},
			Required:    []string{"number"},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				out, err := hyprDispatch(ctx, fmt.Sprintf("hl.dsp.focus({ workspace = %q })", fmt.Sprint(num(in, "number"))))
				return Result{Text: out}, err
			},
		},
	}
}
