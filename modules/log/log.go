package log

import (
	"context"
	"fmt"
	"os"
)

type DefaultLog struct{}

func (s *DefaultLog) Debugf(ctx context.Context, msg string, args ...any) {
	fmt.Fprintf(os.Stderr, msg, args...)
}
