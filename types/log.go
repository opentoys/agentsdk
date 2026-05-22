package types

import "context"

type Logger interface {
	Debugf(ctx context.Context, msg string, args ...any)
}
