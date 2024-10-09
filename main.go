package main

import (
	"husk/canbus"
	"husk/drivers"
	"husk/gui"
	"husk/utils"
	"log"
	"time"
)

func main() {
	// Initialize driver
	d, err := drivers.NewArduinoDriver()
	if err != nil {
		log.Fatalf("error initializing Arduino driver: %v", err)
	}
	defer d.Cleanup() // Ensure cleanup when the program exits

	// Initialize GUI
	g := new(gui.GUI)
	g.SetSendManualCanBusFrameCallback(func(message string) {
		sendManualCanBusFrame(d, message)
	})

	// Start reading CAN frames in a background goroutine
	go startCanFrameReader(g, d)

	g.RunApp()
}

// sendManualCanBusFrame processes the manual CAN bus frame input from the GUI and sends it
func sendManualCanBusFrame(d drivers.Driver, message string) {
	// Convert the input string to a CAN frame
	data, err := utils.HexStringToBytes(message)
	if err != nil {
		log.Printf("error converting input to bytes: %v", err)
		return
	}

	// Construct the CAN frame
	frame := canbus.Frame{
		ID:  canbus.CanIDTransmit,
		DLC: uint16(len(data)),
	}
	copy(frame.Data[:], data)

	// Send the CAN frame
	if err := d.SendCanBusFrame(frame); err != nil {
		log.Printf("error sending CAN bus frame: %v", err)
	}
}

// startCanFrameReader continuously reads CAN frames and passes them to the GUI
func startCanFrameReader(g *gui.GUI, d drivers.Driver) {
	for {
		// Read a CAN frame from the driver
		err := readCanFrame(g, d)
		if err != nil {
			log.Printf("error reading CAN bus frame: %v", err)
		}

		// Wait briefly before reading the next frame
		time.Sleep(5 * time.Millisecond)
	}
}

// readCanFrame reads a CAN frame from the driver and sends it to the GUI
func readCanFrame(g *gui.GUI, d drivers.Driver) error {
	// Read the CAN bus frame
	frame, err := d.ReadCanBusFrame()
	if err != nil {
		return err
	}

	// Ignore frames with no data
	if frame == nil || frame.DLC == 0 {
		return nil
	}

	// Pass the frame to the GUI for display
	g.OnCanBusFrameReceive(frame)

	return nil
}
