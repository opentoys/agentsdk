package types

import "context"

type Logger interface {
	Printf(ctx context.Context, msg string, args ...any)
}
