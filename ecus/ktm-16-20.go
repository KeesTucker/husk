package ecus

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
	identification     *ECUId
	messageBroadcaster *uds.MessageBroadcaster
	wg                 sync.WaitGroup
	cancelFunc         context.CancelFunc
}

const (
	TesterPresentDelayKTM16To20 = 2 * time.Second
	ReadTimeoutKTM16To20        = 5 * time.Second
)

var CompatibleECUHardwareIdsKTM16To20 = []string{
	"613.41.031.300",
}

var CompatibleECUSoftwareIdsKTM16To20 = []string{
	"KM2A0EU17H0631",
}

var CompatibleModelsKTM16To20 = []string{
	"FE/FS 701",
}

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
		l.WriteToLog(fmt.Sprintf("Failed to register temp KTM16To20 ECU Processor: %v", err), logging.LogTypeLog)
		return nil
	}
	// We want to deregister our temporary ecu service so that the actual ecu service can be registered
	defer services.Deregister(services.ServiceECU)
	_, err = tempProcessor.Start(ctx)
	if err != nil {
		l.WriteToLog(fmt.Sprintf("Failed to start temp KTM16To20 ECU Processor: %v", err), logging.LogTypeLog)
		return nil
	}
	defer tempProcessor.Cleanup()
	// Attempt to communicate with the ECU
	l.WriteToLog("Scanning for 2016 to 2020 KTM/Husqvarna", logging.LogTypeLog)

	err = uds.SendTesterPresent(ctx)
	if err != nil {
		l.WriteToLog(fmt.Sprintf("Failed to send tester present: %v", err), logging.LogTypeLog)
		return nil
	}

	// Make sure we get a valid response after sending tester preset.
	service := uds.ServiceTesterPresent
	_, err = tempProcessor.readMessage(ctx, &service, nil)
	if err != nil {
		l.WriteToLog(fmt.Sprintf("Failed to get tester present response: %v", err), logging.LogTypeLog)
		return nil
	}

	l.WriteToLog("Communication established\n", logging.LogTypeLog)
	// Send ECU identification request
	identification, err := tempProcessor.scanEcu(ctx)
	if err != nil {
		l.WriteToLog(fmt.Sprintf("No compatible ECU detected: %v", err), logging.LogTypeLog)
		return nil
	}
	// Create a fresh ecu processor instance and return that so it can be registered at the user's leisure
	e := &ProcessorKTM16To20{}
	e.identification = &identification
	ecus = append(ecus, e)
	return ecus
}

// String returns the id of the ECU
func (e *ProcessorKTM16To20) String() string {
	return fmt.Sprintf("%s %s ECU: %s", e.identification.manufacturer, e.identification.model, e.identification.hardwareId)
}

func (e *ProcessorKTM16To20) Register() (ECUProcessor, error) {
	services.Register(services.ServiceECU, e)
	e.messageBroadcaster = uds.NewUDSMessageBroadcaster()
	return e, nil
}

func (e *ProcessorKTM16To20) Start(ctx context.Context) (ECUProcessor, error) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)
	// Create a cancellable context
	ecuCtx, cancelFunc := context.WithCancel(ctx)
	e.cancelFunc = cancelFunc
	// Mark the driver as running
	atomic.StoreInt32(&e.isRunning, 1)
	// Start the main loops
	e.wg.Add(2)
	go e.testerPresentLoop(ecuCtx)
	go e.processAndBroadcastUDSMessages(ecuCtx)
	l.WriteToLog("ECU processor running", logging.LogTypeLog)
	return e, nil
}

func (e *ProcessorKTM16To20) subscribeReadMessages() (chan *uds.Message, error) {
	if atomic.LoadInt32(&e.isRunning) == 0 {
		return nil, fmt.Errorf("can't subscribe to messages, ecu is not connected")
	}
	return e.messageBroadcaster.Subscribe(), nil
}

func (e *ProcessorKTM16To20) unsubscribeReadMessages(ch chan *uds.Message) {
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

func (e *ProcessorKTM16To20) ReadErrors(ctx context.Context) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	serviceId := uds.ServiceReadErrorsKTM16To20
	req := &uds.Message{
		SenderID:  uds.TesterID,
		ServiceID: serviceId,
	}
	err := req.Send(ctx)
	if err != nil {
		l.WriteToLog(fmt.Sprintf("Error: failed to send read error message: %v", err), logging.LogTypeLog)
		return
	}
	resp, err := e.readMessage(ctx, &serviceId, nil)
	if err != nil {
		l.WriteToLog(fmt.Sprintf("Error: failed to read read error response: %v", err), logging.LogTypeLog)
		return
	}
	var dtcs []string
	for i := 1; i < len(resp.Data); i += 2 {
		// Convert each dtc to a string representing the dtc code
		dtc := fmt.Sprintf("%02X%02X", resp.Data[i], resp.Data[i+1])
		dtcs = append(dtcs, dtc)

	}
	if len(dtcs) > 0 {
		result := "ERRORS:\n"
		for _, dtc := range dtcs {
			result += fmt.Sprintf("DTC: %s\n", uds.GetDTCLabel(dtc))
		}
		l.WriteToLog(result, logging.LogTypeLog)

		return
	}
	l.WriteToLog("NO ERRORS FOUND", logging.LogTypeLog)
}

func (e *ProcessorKTM16To20) ClearErrors(ctx context.Context) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	serviceId := uds.ServiceClearErrorsKTM16To20
	req := &uds.Message{
		SenderID:  uds.TesterID,
		ServiceID: serviceId,
	}
	err := req.Send(ctx)
	if err != nil {
		l.WriteToLog(fmt.Sprintf("Error: failed to send clear error message: %v", err), logging.LogTypeLog)
		return
	}
	_, err = e.readMessage(ctx, &serviceId, nil)
	if err != nil {
		l.WriteToLog(fmt.Sprintf("Error: failed to read clear error response: %v", err), logging.LogTypeLog)
		return
	}

	l.WriteToLog("CLEARED ERRORS SUCCESSFULLY", logging.LogTypeLog)
}

// readMessage will read the next UDS message received. It will block by the specified read timeout and will filter based on serviceId and subfunction
func (e *ProcessorKTM16To20) readMessage(ctx context.Context, serviceId *byte, subfunction *byte) (*uds.Message, error) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	if atomic.LoadInt32(&e.isRunning) == 0 {
		return nil, fmt.Errorf("can't read message ecu is not connected")
	}

	messageChan, _ := e.subscribeReadMessages()
	defer e.unsubscribeReadMessages(messageChan)

	readCtx, cancel := context.WithTimeout(ctx, ReadTimeoutKTM16To20)
	defer cancel()
	for {
		select {
		case message := <-messageChan:
			// if service id is provided match service ids
			if serviceId != nil && message.ServiceID != *serviceId {
				continue
			}
			// IsPositive should never be nil on a response
			if message.IsPositive == nil {
				continue
			}
			// negative responses don't have subfunctions so we can return early
			if !*message.IsPositive {
				return message, nil
			}
			// if the subfunction filter was provided ensure message subfunction is present and then match subfunctions
			if subfunction != nil && (message.Subfunction == nil || *message.Subfunction != *subfunction) {
				continue
			}
			return message, nil
		case <-readCtx.Done():
			l.WriteToLog("Timeout waiting for response", logging.LogTypeLog)
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
			l.WriteToLog("Stopping UDS message processing due to context cancellation", logging.LogTypeLog)
			return
		default:
			message, err := uds.Read(ctx)
			if err != nil {
				if errors.Is(ctx.Err(), context.Canceled) {
					return
				}
				l.WriteToLog(err.Error(), logging.LogTypeLog)
				continue
			}
			if message != nil {
				if message.ServiceID != uds.ServiceTesterPresent {
					l.WriteToLog(message.String(), logging.LogTypeProtocolLog)
				}
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
				l.WriteToLog(fmt.Sprintf("Error: couldn't send UDS tester present"), logging.LogTypeLog)
			}
		}
		time.Sleep(TesterPresentDelayKTM16To20)
	}
}

func (e *ProcessorKTM16To20) scanEcu(ctx context.Context) (identification ECUId, err error) {
	// Check hardware ID
	serviceId := uds.ServiceReadIdKTM16To20
	subfunction := uds.SubfunctionReadECUHardwareIdKTM16To20
	req := &uds.Message{
		SenderID:    uds.TesterID,
		ServiceID:   serviceId,
		Subfunction: &subfunction,
	}
	err = req.Send(ctx)
	if err != nil {
		return
	}
	resp, err := e.readMessage(ctx, &serviceId, &subfunction)
	if err != nil {
		return
	}
	if !slices.Contains(CompatibleECUHardwareIdsKTM16To20, resp.ASCIIRepresentation()) {
		return identification, fmt.Errorf("incompatible hardware ID: %s", resp.ASCIIRepresentation())
	}
	identification.hardwareId = resp.ASCIIRepresentation()

	// Check software ID
	subfunction = uds.SubfunctionReadECUSoftwareIdKTM16To20
	req.Subfunction = &subfunction
	err = req.Send(ctx)
	if err != nil {
		return
	}
	resp, err = e.readMessage(ctx, &serviceId, &subfunction)
	if err != nil {
		return
	}
	if !slices.Contains(CompatibleECUSoftwareIdsKTM16To20, resp.ASCIIRepresentation()) {
		return identification, fmt.Errorf("incompatible software ID: %s", resp.ASCIIRepresentation())
	}
	identification.softwareId = resp.ASCIIRepresentation()

	// Check model
	subfunction = uds.SubfunctionReadModelKTM16To20
	req.Subfunction = &subfunction
	err = req.Send(ctx)
	if err != nil {
		return
	}
	resp, err = e.readMessage(ctx, &serviceId, &subfunction)
	if err != nil {
		return
	}
	if !slices.Contains(CompatibleModelsKTM16To20, resp.ASCIIRepresentation()) {
		return identification, fmt.Errorf("incompatible model: %s", resp.ASCIIRepresentation())
	}
	identification.model = resp.ASCIIRepresentation()

	// Get VIN
	subfunction = uds.SubfunctionReadVINKTM16To20
	req.Subfunction = &subfunction
	err = req.Send(ctx)
	if err != nil {
		return
	}
	resp, err = e.readMessage(ctx, &serviceId, &subfunction)
	if err != nil {
		return
	}
	identification.vin = resp.ASCIIRepresentation()

	// Get manufacturer
	subfunction = uds.SubfunctionReadManufacturerKTM16To20
	req.Subfunction = &subfunction
	err = req.Send(ctx)
	if err != nil {
		return
	}
	resp, err = e.readMessage(ctx, &serviceId, &subfunction)
	if err != nil {
		return
	}
	identification.manufacturer = resp.ASCIIRepresentation()
	return
}
