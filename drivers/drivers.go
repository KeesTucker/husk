package drivers

import (
	"husk/canbus"
)

type Driver interface {
	SendCanBusFrame(frame canbus.Frame) error
	ReadCanBusFrame() (*canbus.Frame, error)
	Cleanup() error
}

type ReadFrameCallback func(*canbus.Frame)
type ErrorCallback func(error)
