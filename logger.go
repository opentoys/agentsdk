package agentsdk

import (
	"context"
	"fmt"
	"os"
)

type Logger interface {
	Printf(ctx context.Context, msg string, args ...any)
}

type DefaultLog struct{}

func (s *DefaultLog) Printf(ctx context.Context, msg string, args ...any) {
	fmt.Fprintf(os.Stderr, msg, args...)
}
