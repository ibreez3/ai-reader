package openai

import (
    "context"
    "time"

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

func (c *Client) ChatWithRetry(ctx context.Context, model string, system string, user string, retries int, backoff time.Duration) (string, error) {
    var lastErr error
    attempts := retries
    if attempts <= 0 { attempts = 1 }
    for i := 0; i < attempts; i++ {
        res, err := c.Chat(ctx, model, system, user)
        if err == nil { return res, nil }
        lastErr = err
        if i < attempts-1 {
            if backoff > 0 { select { case <-time.After(backoff): case <-ctx.Done(): return "", ctx.Err() } }
        }
    }
    return "", lastErr
}
