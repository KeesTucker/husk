package drivers

import (
	"context"
	"fmt"

	"go.bug.st/serial/enumerator"
	"husk/canbus"
	"husk/logging"
	"husk/services"
)

type Driver interface {
	// String returns a display name
	String() string
	// Start starts any driver loops and ensure the driver is fully running
	Start(ctx context.Context) (Driver, error)
	// Register is used to register the driver service with the service locator and to do any required initialisation
	Register() (Driver, error)
	// SendFrame sends a can frame using the driver
	SendFrame(ctx context.Context, frame *canbus.CanFrame) error
	SubscribeReadFrames() chan *canbus.CanFrame
	UnsubscribeReadFrames(ch chan *canbus.CanFrame)
	// Cleanup cleans up any memory, channels, loops etc
	Cleanup()
}

var (
	availableDrivers     []Driver
	availableDriverNames []string
	driverNameToDriver   map[string]Driver

	driverScanCallbacks         []func(availableDriverNames []string)
	driverConnectedCallbacks    []func()
	driverDisconnectedCallbacks []func()

	disconnectFunc func()
)

func ScanForDrivers() {
	l := services.Get(services.ServiceLogger).(*logging.Logger)
	l.WriteToLog("Scanning for drivers")

	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		l.WriteToLog(fmt.Sprintf("Error: failed to get ports: %v", err))
	}

	availableDrivers = []Driver{}
	availableDrivers = ScanArduino(ports, availableDrivers)

	availableDriverNames = make([]string, len(availableDrivers))
	driverNameToDriver = make(map[string]Driver)
	for i, driver := range availableDrivers {
		availableDriverNames[i] = driver.String()
		driverNameToDriver[driver.String()] = driver
	}

	scanEvent(availableDriverNames)

	if len(availableDriverNames) == 0 {
		l.WriteToLog("Didn't find any available drivers")
		return
	}
	l.WriteToLog("Found available drivers")
}

func Connect(ctx context.Context, name string) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	driver, err := driverNameToDriver[name].Register()
	if err != nil {
		l.WriteToLog("Error: failed to connect to driver")
		disconnectEvent()
		ScanForDrivers()
		return
	}
	ctx, disconnectFunc = context.WithCancel(ctx)
	_, err = driver.Start(ctx)
	if err != nil {
		l.WriteToLog("Error: failed to start driver")
		disconnectEvent()
		ScanForDrivers()
		return
	}
	connectEvent()
	l.WriteToLog("Connected to driver successfully")
}

func Disconnect() {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	if disconnectFunc != nil {
		disconnectFunc()
	}
	disconnectEvent()
	services.Deregister(services.ServiceDriver)
	l.WriteToLog("Disconnected from driver successfully")
}

func SubscribeToScanEvent(callback func(availableDriverNames []string)) {
	driverScanCallbacks = append(driverScanCallbacks, callback)
}

func SubscribeToConnectedEvent(callback func()) {
	driverConnectedCallbacks = append(driverConnectedCallbacks, callback)
}

func SubscribeToDisconnectedEvent(callback func()) {
	driverDisconnectedCallbacks = append(driverDisconnectedCallbacks, callback)
}

func scanEvent(availableDriverNames []string) {
	for _, callback := range driverScanCallbacks {
		callback(availableDriverNames)
	}
}

func connectEvent() {
	for _, callback := range driverConnectedCallbacks {
		callback()
	}
}

func disconnectEvent() {
	for _, callback := range driverDisconnectedCallbacks {
		callback()
	}
}
