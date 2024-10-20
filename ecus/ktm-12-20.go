package ecus

import (
	"context"
	"fmt"
	"sync"
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
	wg         sync.WaitGroup
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

		// TODO: probably worth extracting this to a function. this is kinda feral to have to copy paste everywhere
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
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	// Create a cancellable context
	ctx, e.cancelFunc = context.WithCancel(ctx)

	// Mark the driver as running
	atomic.StoreInt32(&e.isRunning, 1)

	// Start the main loops
	e.wg.Add(1)
	go e.testerPresentLoop(ctx)

	l.WriteToLog("ECU processor running")
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

	// Wait for all goroutines to finish
	e.wg.Wait()
}

func (e *KTM16To20Processor) SendData(ctx context.Context, data []byte) error {
	if atomic.LoadInt32(&e.isRunning) == 0 {
		return fmt.Errorf("ecu is not connected")
	}

	err := protocols.SendUDS(ctx, protocols.UDSTesterID, protocols.UDSECUID, data)
	if err != nil {
		return err
	}

	return nil
}

func (e *KTM16To20Processor) ReadData(ctx context.Context) ([]byte, error) {
	if atomic.LoadInt32(&e.isRunning) == 0 {
		return nil, fmt.Errorf("ecu is not connected")
	}

	data, err := protocols.ReadUDS(ctx, protocols.UDSECUID)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (e *KTM16To20Processor) testerPresentLoop(ctx context.Context) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)
	defer e.wg.Done()

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
