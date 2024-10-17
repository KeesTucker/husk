package ecus

import (
	"context"

	"husk/protocols"
)

type ECUType int

type ECUProcessor interface {
	Start(ctx context.Context) ECUProcessor
	ProcessFrame(frame *protocols.CanFrame)
	SendFrame(ctx context.Context, frame *protocols.CanFrame) error
}

func RegisterProcessor(ecuType ECUType) ECUProcessor {
	switch ecuType {
	case ECUTypeHusqvarnaKTM:
		return RegisterHusqvarnaKtmEuro4Processor()
	}

	return nil
}
