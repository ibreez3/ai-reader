package novel

import (
    "context"
    "time"
)

type ChatClient interface {
    Chat(ctx context.Context, model string, system string, user string) (string, error)
    ChatWithRetry(ctx context.Context, model string, system string, user string, retries int, backoff time.Duration) (string, error)
}
