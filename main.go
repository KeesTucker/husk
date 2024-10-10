package main

import (
	"husk/canbus"
	"husk/drivers"
	"husk/gui"
	"husk/utils"
	"log"
)

func main() {
	// Initialize the Arduino driver
	d, err := drivers.NewArduinoDriver()
	if err != nil {
		log.Fatalf("error initializing Arduino driver: %v", err)
	}

	// Initialize the GUI
	g := new(gui.GUI)

	// Set up callback to send frames from GUI to Arduino driver
	g.SetSendManualCanBusFrameCallback(func(message string) {
		// Convert the message to a byte array
		data, err := utils.HexStringToBytes(message)
		if err != nil {
			log.Printf("error converting input to bytes: %v", err)
			return
		}

		dlc := uint8(len(data))
		if dlc > 8 {
			log.Printf("error can't send more than 8 bytes")
			return
		}

		frame := canbus.Frame{
			ID:  canbus.CanIDTransmit,
			DLC: dlc,
		}
		copy(frame.Data[:], data)

		// Send the frame to the Arduino driver (pauses reading, sends frame, resumes reading)
		if err = d.SendCanBusFrame(frame); err != nil {
			log.Printf("error sending CAN bus frame: %v", err)
		}
	})

	// Start reading frames from Arduino and updating GUI in a separate goroutine
	go func() {
		for {
			frame, err := d.ReadCanBusFrame()
			if err != nil {
				// Instead of logging, use the error callback
				g.DisplayError(err)
			}
			if frame != nil {
				// Call the provided frame callback with the received frame
				g.OnCanBusFrameReceive(frame)
			}
		}
	}()

	// Run the GUI application (this will block, but frame reading runs in parallel)
	g.RunApp()

	if err = d.Cleanup(); err != nil {
		log.Printf("error during driver cleanup: %v", err)
	}

}
