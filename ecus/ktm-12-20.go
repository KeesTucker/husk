package ecus

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"husk/logging"
	"husk/protocols"
	"husk/services"
)

// KTM16To20 Covers all KTM & Husqvarna 690 models from 2012 to 2020.
// Support for additional models/years will come soon

type KTM16To20Processor struct {
	isRunning  int32 // Use int32 for atomic operations
	id         string
	cancelFunc context.CancelFunc
}

const (
	KTM16To20TesterPresentDelay = 2000 * time.Millisecond
	KTM16To20ReadIdTimeout      = 1000 * time.Millisecond
)

const (
	KTM16To20ReadIdentificationServiceId byte = 0x1A
)

func (e *KTM16To20Processor) GetTesterId() uint16 {
	return protocols.UDSTesterID
}

func (e *KTM16To20Processor) GetECUId() uint16 {
	return protocols.UDSECUID
}

func ScanKTM16To20(ecus []ECUProcessor) []ECUProcessor {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	// Create an instance of the processor
	e := &KTM16To20Processor{}

	// Attempt to communicate with the ECU
	l.WriteToLog("Scanning for 2016 to 2020 KTM/Husqvarna")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Send ECU identification request
	id, err := e.readECUIdentification(ctx)
	if err != nil {
		l.WriteToLog(fmt.Sprintf("Failed to read identification from ECU: %v", err))
		return nil
	}

	e.id = id

	ecus = append(ecus, e)

	return ecus
}

func (e *KTM16To20Processor) readECUIdentification(ctx context.Context) (result string, err error) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	for identifier := byte(0x01); identifier <= byte(0xFF); identifier++ {
		requestData := []byte{KTM16To20ReadIdentificationServiceId, identifier}
		err = e.SendData(ctx, requestData)
		if err != nil {
			return "", err
		}
		readIdCtx, cancel := context.WithTimeout(ctx, KTM16To20ReadIdTimeout)

	readIdLoop:
		for {
			select {
			case <-readIdCtx.Done():
				l.WriteToLog(fmt.Sprintf("Timeout waiting for response for identifier %d\n", identifier))
				break readIdLoop
			default:
				data, err := e.ReadData(readIdCtx)
				if err != nil {
					cancel()
					return "", err
				}

				if len(data) < 2 {
					continue
				}

				isNegative, nrc := protocols.IsUDSNegativeResponse(KTM16To20ReadIdentificationServiceId, data)
				if isNegative {
					cancel()
					return "", fmt.Errorf(
						"received negative response from ECU when reading identification: service ID %d, NRC %d, identifier %d",
						KTM16To20ReadIdentificationServiceId, nrc, identifier)
				}

				if protocols.IsUDSPositiveResponse(KTM16To20ReadIdentificationServiceId, data) &&
					data[1] == identifier {
					identificationData := data[2:]
					result += string(identificationData) + "\n"
					break readIdLoop
				}
			}
		}
		cancel()
	}
	return result, nil
}

// String returns the id of the ECU
func (e *KTM16To20Processor) String() string {
	return fmt.Sprintf("ECU: %s", e.id)
}

func (e *KTM16To20Processor) Register() (ECUProcessor, error) {
	services.Register(services.ServiceECU, e)
	return e, nil
}

func (e *KTM16To20Processor) Start(ctx context.Context) (ECUProcessor, error) {
	go e.testerPresentLoop(ctx)
	return e, nil
}

// Cleanup stops the driver and releases all resources.
func (e *KTM16To20Processor) Cleanup() {
	if !atomic.CompareAndSwapInt32(&e.isRunning, 1, 0) {
		// If isRunning was not 1, Cleanup has already been called
		return
	}

	// Cancel the context to signal goroutines to exit
	if e.cancelFunc != nil {
		e.cancelFunc()
	}

	// Close channels to unblock goroutines
}

func (e *KTM16To20Processor) SendData(ctx context.Context, data []byte) error {
	err := protocols.SendUDS(ctx, protocols.UDSTesterID, protocols.UDSECUID, data)
	if err != nil {
		return err
	}

	return nil
}

func (e *KTM16To20Processor) ReadData(ctx context.Context) ([]byte, error) {
	data, err := protocols.ReadUDS(ctx, protocols.UDSECUID)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (e *KTM16To20Processor) testerPresentLoop(ctx context.Context) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			err := protocols.SendUDSTesterPresent(ctx, protocols.UDSTesterID, protocols.UDSECUID)
			if err != nil {
				l.WriteToLog(fmt.Sprintf("Error: couldn't send uds tester present"))
			}
		}
		time.Sleep(KTM16To20TesterPresentDelay)
	}
}
