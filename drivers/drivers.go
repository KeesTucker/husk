package drivers

import (
	"context"
	"errors"
	"fmt"
	"husk/canbus"
)

type Driver interface {
	SendCanBusFrame(frame canbus.Frame) error
	ReadCanBusFrame() (*canbus.Frame, error)
	Cleanup() error
}

type ReadFrameCallback func(*canbus.Frame)
type ErrorCallback func(error)

var errDriverClosed = errors.New("error driver has been closed")

func ReadFrames(ctx context.Context, d Driver, frameCallback ReadFrameCallback, errorCallback ErrorCallback) {
	for {
		select {
		case <-ctx.Done():
			fmt.Println("stopping frame reading")
			return
		default:
			frame, err := d.ReadCanBusFrame()
			if err != nil {
				if errors.Is(err, errDriverClosed) {
					fmt.Println("driver closed")
					return
				}
				// Instead of logging, use the error callback
				errorCallback(err)
				continue
			}
			// Call the provided frame callback with the received frame
			frameCallback(frame)
		}
	}
}
