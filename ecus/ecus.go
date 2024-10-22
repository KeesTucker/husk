package ecus

import (
	"context"

	"husk/logging"
	"husk/services"
	"husk/uds"
)

type ECUType int
type ECUProcessor interface {
	String() string
	GetTesterId() uint16
	GetECUId() uint16
	Start(ctx context.Context) (ECUProcessor, error)
	Register() (ECUProcessor, error)
	SendMessage(ctx context.Context, data []byte) error
	ReadMessage(ctx context.Context, serviceId *byte, subfunction *byte) (*uds.Message, error)
	SubscribeReadMessages() (chan *uds.Message, error)
	UnsubscribeReadMessages(ch chan *uds.Message)
	Cleanup()
}

var (
	availableECUs            []ECUProcessor
	availableECUIds          []string
	ecuIdToECU               map[string]ECUProcessor
	ecuScanCallbacks         []func(availableECUIds []string)
	ecuConnectedCallbacks    []func()
	ecuDisconnectedCallbacks []func()
	disconnectFunc           func()
)

func ScanForECUs(ctx context.Context) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)
	l.WriteToLog("Scanning for ecus")
	availableECUs = []ECUProcessor{}
	availableECUs = ScanKTM16To20(ctx, availableECUs)
	availableECUIds = make([]string, len(availableECUs))
	ecuIdToECU = make(map[string]ECUProcessor)
	for i, driver := range availableECUs {
		availableECUIds[i] = driver.String()
		ecuIdToECU[driver.String()] = driver
	}
	scanEvent(availableECUIds)
	if len(availableECUIds) == 0 {
		l.WriteToLog("Didn't find any available ecus")
		return
	}
	l.WriteToLog("Found available ecus")
}
func Connect(ctx context.Context, name string) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)
	driver, err := ecuIdToECU[name].Register()
	if err != nil {
		l.WriteToLog("Error: failed to connect to ECU")
		disconnectEvent()
		ScanForECUs(ctx)
		return
	}
	ctx, disconnectFunc = context.WithCancel(ctx)
	_, err = driver.Start(ctx)
	if err != nil {
		l.WriteToLog("Error: failed to start ECU processor")
		disconnectEvent()
		ScanForECUs(ctx)
		return
	}
	connectEvent()
	l.WriteToLog("Connected to ECU successfully")
}
func Disconnect() {
	l := services.Get(services.ServiceLogger).(*logging.Logger)
	if disconnectFunc != nil {
		disconnectFunc()
	}
	disconnectEvent()
	services.Deregister(services.ServiceECU)
	l.WriteToLog("Disconnected from ECU successfully")
}
func SubscribeToScanEvent(callback func(availableECUIds []string)) {
	ecuScanCallbacks = append(ecuScanCallbacks, callback)
}
func SubscribeToConnectedEvent(callback func()) {
	ecuConnectedCallbacks = append(ecuConnectedCallbacks, callback)
}
func SubscribeToDisconnectedEvent(callback func()) {
	ecuDisconnectedCallbacks = append(ecuDisconnectedCallbacks, callback)
}
func scanEvent(availableECUIds []string) {
	for _, callback := range ecuScanCallbacks {
		callback(availableECUIds)
	}
}
func connectEvent() {
	for _, callback := range ecuConnectedCallbacks {
		callback()
	}
}
func disconnectEvent() {
	for _, callback := range ecuDisconnectedCallbacks {
		callback()
	}
}
