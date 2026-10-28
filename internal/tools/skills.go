package tools

import (
	"context"

	"kairo/internal/skills"
)

func skillTools(lib *skills.Library) []Tool {
	return []Tool{
		{
			Name:        "use_skill",
			Description: "Load the full instructions of one skill from the skill list in your prompt, then follow them within your guardrails.",
			Params:      map[string]any{"name": prop("string", "Skill name exactly as listed")},
			Required:    []string{"name"},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				body, err := lib.Body(str(in, "name"))
				return Result{Text: body}, err
			},
		},
	}
}
