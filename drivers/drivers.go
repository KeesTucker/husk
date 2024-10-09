package drivers

import (
	"husk/canbus"
)

type Driver interface {
	Cleanup()
	SendCanBusFrame(frame canbus.Frame) error
	ReadCanBusFrame() (frame *canbus.Frame, err error)
}
