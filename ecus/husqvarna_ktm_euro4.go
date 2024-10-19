package ecus

import (
	"context"
	"fmt"
	"time"

	"husk/logging"
	"husk/protocols"
	"husk/services"
)

type HusqvarnaKTMProcessor struct{}

const (
	ECUTypeHusqvarnaKTM                 ECUType = 1
	HusqvarnaKTMEuro4TesterPresentDelay         = 2000 * time.Millisecond
	husqvarnaKTMEuro4CanIDTester        uint16  = 0x7E0
	husqvarnaKTMEuro4CanIDECU           uint16  = 0x7E8
)

func RegisterHusqvarnaKtmEuro4Processor() ECUProcessor {
	e := &HusqvarnaKTMProcessor{}
	defer services.Register(services.ServiceECU, e)
	return e
}

func (e *HusqvarnaKTMProcessor) Start(ctx context.Context) ECUProcessor {
	go e.testerPresentLoop(ctx)
	return e
}

func (e *HusqvarnaKTMProcessor) testerPresentLoop(ctx context.Context) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			err := protocols.SendUDSTesterPresent(ctx, husqvarnaKTMEuro4CanIDTester, husqvarnaKTMEuro4CanIDECU)
			if err != nil {
				l.WriteToLog(fmt.Sprintf("error: couldn't send uds tester present"))
			}
		}
		time.Sleep(HusqvarnaKTMEuro4TesterPresentDelay)
	}
}

func (e *HusqvarnaKTMProcessor) SendData(ctx context.Context, data []byte) error {
	err := protocols.SendUDS(ctx, husqvarnaKTMEuro4CanIDTester, husqvarnaKTMEuro4CanIDECU, data)
	if err != nil {
		return err
	}

	return nil
}

func (e *HusqvarnaKTMProcessor) GetTesterId() uint16 {
	return husqvarnaKTMEuro4CanIDTester
}

func (e *HusqvarnaKTMProcessor) GetECUId() uint16 {
	return husqvarnaKTMEuro4CanIDECU
}
