package openai

import (
	"context"

	openai "github.com/openai/openai-go/v3" // imported as openai
	"github.com/openai/openai-go/v3/option"
)

type Client struct {
	cli openai.Client
}

func NewClient(apiKey string, baseURL string) *Client {
	openAICli := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	)
	return &Client{
		cli: openAICli,
	}
}

func (c *Client) Chat(ctx context.Context, model string, system string, user string) (string, error) {
	res, err := c.cli.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(system),
			openai.UserMessage(user),
		},
	})
	if err != nil {
		return "", err
	}
	return res.Choices[0].Message.Content, nil
}
