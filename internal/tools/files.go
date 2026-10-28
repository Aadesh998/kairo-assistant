package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"kairo/internal/config"
)

func fileTools() []Tool {
	return []Tool{
		{
			Name:        "read_file",
			Description: "Read a text file.",
			Params:      map[string]any{"path": prop("string", "File path, ~ allowed")},
			Required:    []string{"path"},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				data, err := os.ReadFile(config.Expand(str(in, "path")))
				if err != nil {
					return Result{}, err
				}
				text := string(data)
				if len(text) > 20000 {
					text = text[:20000] + "\n...(truncated)"
				}
				return Result{Text: text}, nil
			},
		},
		{
			Name:        "list_dir",
			Description: "List files in a directory.",
			Params:      map[string]any{"path": prop("string", "Directory path, ~ allowed")},
			Required:    []string{"path"},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				entries, err := os.ReadDir(config.Expand(str(in, "path")))
				if err != nil {
					return Result{}, err
				}
				var lines []string
				for _, e := range entries {
					name := e.Name()
					if e.IsDir() {
						name += "/"
					}
					lines = append(lines, name)
				}
				return Result{Text: fmt.Sprintf("%d entries\n%s", len(lines), strings.Join(lines, "\n"))}, nil
			},
		},
	}
}
