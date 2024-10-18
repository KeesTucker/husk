package drivers

import (
	"context"
	"errors"
	"fmt"

	"go.bug.st/serial/enumerator"
	"husk/logging"
	"husk/protocols"
	"husk/services"
)

type Driver interface {
	// String returns a display name
	String() string
	// Start starts any driver loops and ensure the driver is fully running
	Start(ctx context.Context) Driver
	// Register is used to register the driver service with the service locator and to do any required initialisation
	Register() (Driver, error)
	// SendFrame sends a can frame using the driver
	SendFrame(ctx context.Context, frame *protocols.CanFrame) error
	// ReadFrame reads a can frame using the driver
	ReadFrame(ctx context.Context) (*protocols.CanFrame, error)
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

var errorPortHasBeenClosed = errors.New("Port has been closed")

func ScanForDrivers() {
	l := services.Get(services.ServiceLogger).(*logging.Logger)
	l.WriteToLog("scanning for drivers")

	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		l.WriteToLog(fmt.Sprintf("error: failed to get ports: %v", err))
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
		l.WriteToLog("didn't find any available drivers")
		return
	}
	l.WriteToLog("found available drivers")
}

func Connect(ctx context.Context, name string) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	driver, err := driverNameToDriver[name].Register()
	if err != nil {
		l.WriteToLog("error: failed to connect to driver")
		disconnectEvent()
		ScanForDrivers()
		return
	}
	ctx, disconnectFunc = context.WithCancel(ctx)
	driver.Start(ctx)
	connectEvent()
	l.WriteToLog("connected to driver successfully")
}

func Disconnect() {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	services.Deregister(services.ServiceDriver)
	disconnectFunc()
	disconnectEvent()
	l.WriteToLog("disconnected from driver successfully")
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
