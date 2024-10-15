package main

import (
	"context"
	"errors"
	"fmt"
	"husk/canbus"
	"husk/drivers"
	"husk/gui"
	"husk/logging"
	"husk/utils"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Create a context that can be canceled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure the context is canceled to free resources when main function exits

	// Set up a channel to listen for OS signals to gracefully handle shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	// Initialize the GUI
	g := new(gui.GUI)

	// Initialise the logger
	l := logging.NewLogger(g)

	// Initialize the Arduino driver with the context
	// TODO: Do this after clicking a "connect" button etc.
	d, err := drivers.NewArduinoDriver(ctx, l)
	if err != nil {
		l.WriteToLog(fmt.Sprintf("error initializing Arduino driver: %s", err))
		return
	}

	// Set up callback to send frames from GUI to Arduino driver
	g.SetSendManualCanBusFrameCallback(func(message string) {
		// Convert the message to a byte array
		data, err := utils.HexStringToBytes(message)
		if err != nil {
			l.WriteToLog(fmt.Sprintf("error converting input to bytes: %s", err.Error()))
			return
		}

		dlc := uint8(len(data))
		if dlc > 8 {
			l.WriteToLog("error: can't send more than 8 bytes")
			return
		}

		frame := canbus.Frame{
			ID:  canbus.CanIDTester,
			DLC: dlc,
		}
		copy(frame.Data[:], data)

		// Send the frame to the Arduino driver
		if err = d.SendCanBusFrame(frame); err != nil {
			l.WriteToLog(fmt.Sprintf("error sending CAN bus frame: %s", err.Error()))
		}
	})

	// Start reading frames from Arduino and updating GUI in a separate goroutine
	go func() {
		for {
			select {
			case <-ctx.Done(): // Exit the loop if the context is canceled
				l.WriteToLog("Stopping CAN bus frame reading due to context cancellation")
				return
			default:
				frame, err := d.ReadCanBusFrame()
				if err != nil {
					// Handle context cancellation separately to avoid logging unnecessary errors during shutdown
					if errors.Is(ctx.Err(), context.Canceled) {
						l.WriteToLog("Frame reading stopped as the context is canceled")
						return
					}
					// Instead of logging, use the error callback
					l.WriteToLog(err.Error())
				}
				if frame != nil {
					// Call the provided frame callback with the received frame
					g.OnCanBusFrameReceive(frame)
				}
			}
		}
	}()

	// Start a separate goroutine to listen for OS signals to handle shutdown gracefully
	go func() {
		<-signalChan
		l.WriteToLog("Received shutdown signal, canceling context and cleaning up...")
		cancel()
	}()

	// Run the GUI application (this will block, but frame reading runs in parallel)
	g.RunApp()

	// Ensure cleanup of resources
	if err = d.Cleanup(); err != nil {
		l.WriteToLog(fmt.Sprintf("error during driver cleanup: %s", err.Error()))
	}
}
