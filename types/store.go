package types

import "context"

type MessageStorer interface {
	GetMessage(ctx context.Context) (msg []string, e error)
	SaveMessage(ctx context.Context, msg []string) (e error)
}
