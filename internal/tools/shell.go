package tools

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"kairo/internal/config"
)

func shellTools() []Tool {
	return []Tool{
		{
			Name:        "run_command",
			Description: "Run a bash command on the owner's Linux machine and return its output.",
			Params: map[string]any{
				"command": prop("string", "The bash command to run"),
				"dir":     prop("string", "Working directory, default is home"),
			},
			Required: []string{"command"},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
				defer cancel()
				cmd := exec.CommandContext(ctx, "bash", "-lc", str(in, "command"))
				cmd.Dir = config.Expand("~/")
				if dir := str(in, "dir"); dir != "" {
					cmd.Dir = config.Expand(dir)
				}
				out, err := cmd.CombinedOutput()
				text := strings.TrimSpace(string(out))
				if len(text) > 8000 {
					text = text[:8000] + "\n...(truncated)"
				}
				if err != nil {
					text = text + "\nexit: " + err.Error()
				}
				return Result{Text: text}, nil
			},
		},
	}
}
