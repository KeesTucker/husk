package ecus

import (
	"context"
)

type ECUType int

type ECUProcessor interface {
	Start(ctx context.Context) ECUProcessor
	SendData(ctx context.Context, data []byte) error
	GetTesterId() uint16
	GetECUId() uint16
}

func RegisterProcessor(ecuType ECUType) ECUProcessor {
	switch ecuType {
	case ECUTypeHusqvarnaKTM:
		return RegisterHusqvarnaKtmEuro4Processor()
	}

	return nil
}

// TODO: gracefully handle connection and disconnection of ECUs
