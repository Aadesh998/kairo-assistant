package tools

import (
	"context"
	"os/exec"
	"time"
)

func screenTools() []Tool {
	return []Tool{
		{
			Name:        "screenshot",
			Description: "Take a screenshot of the screen so you can see what the owner sees.",
			Params:      map[string]any{},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()
				png, err := exec.CommandContext(ctx, "grim", "-s", "0.5", "-").Output()
				if err != nil {
					return Result{}, err
				}
				return Result{Text: "screenshot taken", Image: png}, nil
			},
		},
	}
}
