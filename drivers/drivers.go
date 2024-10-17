package drivers

import (
	"context"

	"husk/protocols"
)

type Driver interface {
	Start(ctx context.Context) Driver
	SendFrame(ctx context.Context, frame *protocols.CanFrame) error
	ReadFrame(ctx context.Context) (*protocols.CanFrame, error)
	Cleanup() error
}
