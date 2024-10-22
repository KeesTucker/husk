package ecus

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"husk/logging"
	"husk/services"
	"husk/uds"
)

// KTM16To20 Covers all KTM & Husqvarna 690 models from 2016 to 2020.
// Support for additional models/years will come soon

type ProcessorKTM16To20 struct {
	isRunning          int32 // Use int32 for atomic operations
	id                 string
	messageBroadcaster *uds.MessageBroadcaster
	wg                 sync.WaitGroup
	cancelFunc         context.CancelFunc
}

const (
	TesterPresentDelayKTM16To20 = 2 * time.Second
	ReadTimeoutKTM16To20        = 10 * time.Second
)

func (e *ProcessorKTM16To20) GetTesterId() uint16 {
	return uds.TesterID
}

func (e *ProcessorKTM16To20) GetECUId() uint16 {
	return uds.ECUID
}

func ScanKTM16To20(ctx context.Context, ecus []ECUProcessor) []ECUProcessor {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	// Create a temporary instance of the processor
	tempProcessor := &ProcessorKTM16To20{}
	// Register said instance so we can send ID requests
	_, err := tempProcessor.Register()
	if err != nil {
		l.WriteToLog(fmt.Sprintf("Failed to register temp KTM16To20 ECU Processor: %v", err))
		return nil
	}
	// We want to deregister our temporary ecu service so that the actual ecu service can be registered
	defer services.Deregister(services.ServiceECU)

	_, err = tempProcessor.Start(ctx)
	if err != nil {
		l.WriteToLog(fmt.Sprintf("Failed to start temp KTM16To20 ECU Processor: %v", err))
		return nil
	}
	defer tempProcessor.Cleanup()

	// Attempt to communicate with the ECU
	l.WriteToLog("Scanning for 2016 to 2020 KTM/Husqvarna")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Send ECU identification request
	id, err := tempProcessor.readECUIdentification(ctx)
	if err != nil {
		l.WriteToLog(fmt.Sprintf("Failed to read identification from ECU: %v", err))
		return nil
	}

	// Create a fresh ecu processor instance and return that so it can be registered at the user's leisure
	e := &ProcessorKTM16To20{}

	e.id = id

	ecus = append(ecus, e)

	return ecus
}

// String returns the id of the ECU
func (e *ProcessorKTM16To20) String() string {
	return fmt.Sprintf("ECU: %s", e.id)
}

func (e *ProcessorKTM16To20) Register() (ECUProcessor, error) {
	services.Register(services.ServiceECU, e)

	e.messageBroadcaster = uds.NewUDSMessageBroadcaster()

	return e, nil
}

func (e *ProcessorKTM16To20) Start(ctx context.Context) (ECUProcessor, error) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	// Create a cancellable context
	ctx, e.cancelFunc = context.WithCancel(ctx)

	// Mark the driver as running
	atomic.StoreInt32(&e.isRunning, 1)

	// Start the main loops
	e.wg.Add(2)
	go e.testerPresentLoop(ctx)
	go e.processAndBroadcastUDSMessages(ctx)

	l.WriteToLog("ECU processor running")
	return e, nil
}

func (e *ProcessorKTM16To20) SubscribeReadMessages() (chan *uds.Message, error) {
	if atomic.LoadInt32(&e.isRunning) == 0 {
		return nil, fmt.Errorf("can't subscribe to messages, ecu is not connected")
	}
	return e.messageBroadcaster.Subscribe(), nil
}

func (e *ProcessorKTM16To20) UnsubscribeReadMessages(ch chan *uds.Message) {
	if e.messageBroadcaster != nil {
		e.messageBroadcaster.Unsubscribe(ch)
	}

	return
}

// Cleanup stops the driver and releases all resources.
func (e *ProcessorKTM16To20) Cleanup() {
	if !atomic.CompareAndSwapInt32(&e.isRunning, 1, 0) {
		// If isRunning was not 1, Cleanup has already been called
		return
	}

	// Cancel the context to signal goroutines to exit
	if e.cancelFunc != nil {
		e.cancelFunc()
	}

	// Cleanup the broadcaster
	if e.messageBroadcaster != nil {
		e.messageBroadcaster.Cleanup()
	}

	// Wait for all goroutines to finish
	e.wg.Wait()
}

func (e *ProcessorKTM16To20) SendMessage(ctx context.Context, data []byte) error {
	if atomic.LoadInt32(&e.isRunning) == 0 {
		return fmt.Errorf("can't send message ecu is not connected")
	}

	err := uds.RawDataToMessage(uds.TesterID, data).Send(ctx)
	if err != nil {
		return err
	}

	return nil
}

// ReadMessage will read the next UDS message received. It will block by the specified read timeout and will filter based on serviceId and subfunction
func (e *ProcessorKTM16To20) ReadMessage(ctx context.Context, serviceId *byte, subfunction *byte) (*uds.Message, error) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	if atomic.LoadInt32(&e.isRunning) == 0 {
		return nil, fmt.Errorf("can't read message ecu is not connected")
	}

	messageChan, _ := e.SubscribeReadMessages()
	defer e.UnsubscribeReadMessages(messageChan)

	readCtx, cancel := context.WithTimeout(ctx, ReadTimeoutKTM16To20)
	defer cancel()

	for {
		select {
		case message := <-messageChan:
			// if service id is provided match service ids
			if serviceId != nil && message.ServiceID != *serviceId {
				continue
			}

			// IsSuccess should never be nil on a response
			if message.IsSuccess == nil {
				continue
			}

			// negative responses don't have subfunctions so we can return early
			if !*message.IsSuccess {
				return message, nil
			}

			// if the subfunction filter was provided ensure message subfunction is present and then match subfunctions
			if subfunction != nil && (message.Subfunction == nil || *message.Subfunction != *subfunction) {
				continue
			}

			return message, nil
		case <-readCtx.Done():
			l.WriteToLog("Timeout waiting for response")
			return nil, readCtx.Err()
		}
	}
}

// processAndBroadcastUDSMessages reads complete UDS messages from readChan, processes them, and broadcasts to subscribers.
func (e *ProcessorKTM16To20) processAndBroadcastUDSMessages(ctx context.Context) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)
	defer e.wg.Done()

	for {
		select {
		case <-ctx.Done():
			l.WriteToLog("Stopping UDS message processing due to context cancellation")
			return
		default:
			message, err := uds.Read(ctx)
			if err != nil {
				if errors.Is(ctx.Err(), context.Canceled) {
					return
				}
				l.WriteToLog(err.Error())
				continue
			}
			if message != nil {
				l.WriteToLog(message.String())
				e.messageBroadcaster.Broadcast(message)
			}
		}
	}
}

func (e *ProcessorKTM16To20) testerPresentLoop(ctx context.Context) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)
	defer e.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			err := uds.SendTesterPresent(ctx)
			if err != nil {
				l.WriteToLog(fmt.Sprintf("Error: couldn't send UDS tester present"))
			}
		}
		time.Sleep(TesterPresentDelayKTM16To20)
	}
}

func (e *ProcessorKTM16To20) readECUIdentification(ctx context.Context) (result string, err error) {
	for subfunction := byte(0x01); subfunction <= byte(0x08); subfunction++ {
		message := &uds.Message{
			SenderID:    uds.TesterID,
			ServiceID:   uds.ServiceReadIdKTM16To20,
			Subfunction: &subfunction,
		}
		err = message.Send(ctx)
		if err != nil {
			return "", err
		}

		serviceId := uds.ServiceReadIdKTM16To20
		message, err = e.ReadMessage(ctx, &serviceId, &subfunction)
		if err != nil {
			return "", err
		}

		fmt.Println(message.ASCIIRepresentation())
		result += message.ASCIIRepresentation() + "\n"
	}
	return result, nil
}
