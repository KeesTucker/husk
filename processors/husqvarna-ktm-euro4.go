package processors

import (
	"context"
	"fmt"
	"time"

	"husk/drivers"
	"husk/frames"
	"husk/logging"
)

type HusqvarnaKTMProcessor struct {
	ctx context.Context
	d   drivers.Driver
	l   *logging.Logger
}

const (
	CanIDTester        = 0x7E0
	TesterPresentDelay = 2000 * time.Millisecond
)

func NewHusqvarnaKTMProcessor(ctx context.Context, d drivers.Driver, l *logging.Logger) ECUProcessor {
	ecuProcessor := &HusqvarnaKTMProcessor{
		ctx: ctx,
		d:   d,
		l:   l,
	}
	go ecuProcessor.TesterPresentLoop()
	return ecuProcessor
}

func (e *HusqvarnaKTMProcessor) TesterPresentLoop() {
	for {
		select {
		case <-e.ctx.Done():
			return
		default:
			e.sendCanBusFrame(frames.UDSTesterPresentFrame)
			e.l.WriteToLog("sent tester present frame")
		}
		time.Sleep(TesterPresentDelay)
	}
}

func (e *HusqvarnaKTMProcessor) ProcessFrame(frame *frames.Frame) {
	return
}

func (e *HusqvarnaKTMProcessor) sendCanBusFrame(frame *frames.Frame) {
	frame.ID = CanIDTester
	err := e.d.SendCanBusFrame(frame)
	if err != nil {
		e.l.WriteToLog(fmt.Sprintf("error: sending frame: %v", err))
	}
}
