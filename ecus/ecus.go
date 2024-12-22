package ecus

import (
	"context"

	"husk/logging"
	"husk/services"
)

type (
	ECUProcessor interface {
		Register() (ECUProcessor, error)
		Start(ctx context.Context) (ECUProcessor, error)
		Cleanup()
		String() string
		GetTesterId() uint16
		GetECUId() uint16
		ReadErrors(ctx context.Context) []string
		ClearErrors(ctx context.Context)
	}
	ECUType int
	ECUId   struct {
		hardwareId   string
		softwareId   string
		manufacturer string
		model        string
		vin          string
	}
)

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
	l.WriteLog("Scanning for ecus", logging.LogLevelInfo)
	availableECUs = []ECUProcessor{}
	// Add more ecu types here
	availableECUs = ScanK01(ctx, availableECUs)
	availableECUIds = make([]string, len(availableECUs))
	ecuIdToECU = make(map[string]ECUProcessor)
	for i, ecu := range availableECUs {
		availableECUIds[i] = ecu.String()
		ecuIdToECU[ecu.String()] = ecu
	}
	scanEvent(availableECUIds)
	if len(availableECUIds) == 0 {
		l.WriteLog("Didn't find any available ecus", logging.LogLevelWarning)
		return
	}
	l.WriteLog("Found available ecus", logging.LogLevelSuccess)
}

func Connect(ctx context.Context, name string) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)
	driver, err := ecuIdToECU[name].Register()
	if err != nil {
		l.WriteLog("Error failed to connect to ECU", logging.LogLevelError)
		disconnectEvent()
		ScanForECUs(ctx)
		return
	}
	ctx, disconnectFunc = context.WithCancel(ctx)
	_, err = driver.Start(ctx)
	if err != nil {
		l.WriteLog("Error failed to start ECU processor", logging.LogLevelError)
		disconnectEvent()
		ScanForECUs(ctx)
		return
	}
	connectEvent()
	l.WriteLog("Connected to ECU successfully", logging.LogLevelSuccess)
}

func Disconnect() {
	l := services.Get(services.ServiceLogger).(*logging.Logger)
	if disconnectFunc != nil {
		disconnectFunc()
	}
	disconnectEvent()
	services.Deregister(services.ServiceECU)
	l.WriteLog("Disconnected from ECU successfully", logging.LogLevelSuccess)
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
