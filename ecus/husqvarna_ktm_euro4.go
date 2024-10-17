package ecus

import (
	"context"
	"errors"
	"fmt"
	"time"

	"husk/drivers"
	"husk/logging"
	"husk/protocols"
	"husk/services"
)

type HusqvarnaKTMProcessor struct{}

const (
	ECUTypeHusqvarnaKTM                 ECUType = 1
	HusqvarnaKTMEuro4CanIDTester                = 0x7E0
	HusqvarnaKTMEuro4TesterPresentDelay         = 2000 * time.Millisecond
)

func RegisterHusqvarnaKtmEuro4Processor() ECUProcessor {
	e := &HusqvarnaKTMProcessor{}
	defer services.Register(services.ServiceECU, e)
	return e
}

func (e *HusqvarnaKTMProcessor) Start(ctx context.Context) ECUProcessor {
	go e.testerPresentLoop(ctx)
	go e.processECUFrames(ctx)
	return e
}

func (e *HusqvarnaKTMProcessor) testerPresentLoop(ctx context.Context) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// todo: this should call uds send frame which should call the driver send can bus frame function
			err := e.SendFrame(ctx, protocols.GetUDSTesterPresentFrame())
			if err != nil {
				l.WriteToLog(fmt.Sprintf("error: sending tester present frame: %v", err))
				continue
			}
			l.WriteToLog("sent tester present frame")
		}
		time.Sleep(HusqvarnaKTMEuro4TesterPresentDelay)
	}
}
func (e *HusqvarnaKTMProcessor) processECUFrames(ctx context.Context) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)
	d := services.Get(services.ServiceDriver).(drivers.Driver)

	for {
		select {
		case <-ctx.Done(): // Exit the loop if the context is canceled
			l.WriteToLog("stopping CAN bus frame reading due to context cancellation")
			return
		default:
			// todo: this should call uds read frame which should call the driver read can bus frame function
			frame, err := d.ReadFrame(ctx)
			if err != nil {
				// Handle context cancellation separately to avoid logging unnecessary errors during shutdown
				if errors.Is(ctx.Err(), context.Canceled) {
					l.WriteToLog("frame reading stopped as the context is canceled")
					return
				}
				l.WriteToLog(err.Error())
			}
			if frame != nil {
				l.WriteToLog(fmt.Sprintf("RECIEVED: %s", frame.String()))
				e.ProcessFrame(frame)
			}
		}
	}
}

func (e *HusqvarnaKTMProcessor) ProcessFrame(frame *protocols.CanFrame) {
	return
}

func (e *HusqvarnaKTMProcessor) SendFrame(ctx context.Context, frame *protocols.CanFrame) error {
	d := services.Get(services.ServiceDriver).(drivers.Driver)

	frame.ID = HusqvarnaKTMEuro4CanIDTester
	// todo: this should call uds send frame which should call the driver send can bus frame function
	err := d.SendFrame(ctx, frame)
	if err != nil {
		return err
	}

	return nil
}
