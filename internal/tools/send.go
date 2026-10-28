package tools

import (
	"context"
	"errors"

	"kairo/internal/config"
)

func fileSendTools(send func(ctx context.Context, path, caption string) (string, error)) []Tool {
	return []Tool{
		{
			Name:        "send_file",
			Description: "Send a file from this computer to the owner's phone (Telegram/Slack). Files over 50 MB are uploaded to the owner's own Google Drive and the link is sent instead.",
			Params: map[string]any{
				"path":    prop("string", "File path, ~ allowed"),
				"caption": prop("string", "Short message to go with the file"),
			},
			Required: []string{"path"},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				if send == nil {
					return Result{}, errors.New("no channel can send files")
				}
				how, err := send(ctx, config.Expand(str(in, "path")), str(in, "caption"))
				return Result{Text: how}, err
			},
		},
	}
}
