package tools

import (
	"context"
	"strings"
	"time"
)

func keyboardTools() []Tool {
	return []Tool{
		{
			Name:        "type_text",
			Description: "Type text into the focused window like a keyboard.",
			Params:      map[string]any{"text": prop("string", "Text to type")},
			Required:    []string{"text"},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				_, err := run(ctx, 30*time.Second, "wtype", str(in, "text"))
				return Result{Text: "typed"}, err
			},
		},
		{
			Name:        "press_keys",
			Description: "Press a key combination in the focused window, for example ctrl+l, ctrl+shift+t, Return, Escape.",
			Params:      map[string]any{"keys": prop("string", "Keys joined with +")},
			Required:    []string{"keys"},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				_, err := run(ctx, 10*time.Second, "wtype", keyArgs(str(in, "keys"))...)
				return Result{Text: "pressed " + str(in, "keys")}, err
			},
		},
	}
}

func keyArgs(combo string) []string {
	modifiers := map[string]string{"ctrl": "ctrl", "control": "ctrl", "shift": "shift", "alt": "alt", "super": "logo", "win": "logo"}
	parts := strings.Split(combo, "+")
	var args, release []string
	for _, p := range parts[:len(parts)-1] {
		if m, ok := modifiers[strings.ToLower(strings.TrimSpace(p))]; ok {
			args = append(args, "-M", m)
			release = append(release, "-m", m)
		}
	}
	key := strings.TrimSpace(parts[len(parts)-1])
	if len(key) == 1 {
		key = strings.ToLower(key)
	}
	args = append(args, "-k", key)
	return append(args, release...)
}
