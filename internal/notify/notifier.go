package notify

import "context"

type Notifier interface {
	Success(ctx context.Context, msg string) error
	Error(ctx context.Context, errMsg string) error
}
