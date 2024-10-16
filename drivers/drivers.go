package drivers

import (
	"husk/frames"
)

type Driver interface {
	SendCanBusFrame(frame *frames.Frame) error
	ReadCanBusFrame() (*frames.Frame, error)
	Cleanup() error
}
