package llm

import (
	"context"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

type Client struct {
	api    anthropic.Client
	model  string
	effort string
}

func New(model, effort string) *Client {
	return &Client{api: anthropic.NewClient(), model: model, effort: effort}
}

func (c *Client) Send(ctx context.Context, stable, volatile string, messages []anthropic.BetaMessageParam, tools []anthropic.BetaToolUnionParam) (*anthropic.BetaMessage, error) {
	params := anthropic.BetaMessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: 16000,
		System: []anthropic.BetaTextBlockParam{
			{Text: stable, CacheControl: anthropic.NewBetaCacheControlEphemeralParam()},
			{Text: volatile},
		},
		Messages:     messages,
		Tools:        tools,
		OutputConfig: anthropic.BetaOutputConfigParam{Effort: anthropic.BetaOutputConfigEffort(c.effort)},
		Fallbacks:    anthropic.BetaFallbacksParamOfDefault(),
		Betas:        []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01},
	}
	return c.api.Beta.Messages.New(ctx, params)
}

func (c *Client) Ask(ctx context.Context, system, prompt string) (string, error) {
	msg, err := c.Send(ctx, system, "", []anthropic.BetaMessageParam{
		anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(prompt)),
	}, nil)
	if err != nil {
		return "", err
	}
	return TextOf(msg), nil
}

func TextOf(msg *anthropic.BetaMessage) string {
	if msg.StopReason == anthropic.BetaStopReasonRefusal {
		return "I can't help with that request."
	}
	var parts []string
	for _, block := range msg.Content {
		if text, ok := block.AsAny().(anthropic.BetaTextBlock); ok {
			parts = append(parts, text.Text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}
