package types

import "context"

type Processer = func(ctx context.Context, typ string, msg string)
