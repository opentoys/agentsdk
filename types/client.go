package types

import "context"

type Processer = func(ctx context.Context, typ string, msg string)

type Runner = func(ctx context.Context, in string) (out string, e error)
