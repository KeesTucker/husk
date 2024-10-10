package main

import (
	"husk/canbus"
	"husk/drivers"
	"husk/gui"
	"husk/utils"
	"log"
	"sync"
)

func main() {
	// Initialize the Arduino driver
	d, err := drivers.NewArduinoDriver()
	if err != nil {
		log.Fatalf("error initializing Arduino driver: %v", err)
	}
	defer func() {
		if err := d.Cleanup(); err != nil {
			log.Printf("error during driver cleanup: %v", err)
		}
	}()

	// Initialize the GUI
	g := new(gui.GUI)

	// Create a WaitGroup for frame reading
	var wg sync.WaitGroup

	// Create a stop signal channel to notify goroutines to stop
	stopChan := make(chan struct{})

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
			log.Printf("error: can't send more than 8 bytes")
			return
		}

		frame := canbus.Frame{
			ID:  canbus.CanIDTransmit,
			DLC: dlc,
		}
		copy(frame.Data[:], data)

		// Send the frame to the Arduino driver (pauses reading, sends frame, resumes reading)
		if err := d.SendCanBusFrame(frame); err != nil {
			log.Printf("error sending CAN bus frame: %v", err)
		}
	})

	// Start reading frames from Arduino and updating GUI in a separate goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		readFrames(d, g, stopChan)
	}()

	// Run the GUI application (this will block, but frame reading runs in parallel)
	g.RunApp()

	// Ensure we wait for reading frames to finish before exit
	wg.Wait()
}

// readFrames continuously reads frames from the Arduino driver and updates the GUI.
func readFrames(d *drivers.ArduinoDriver, g *gui.GUI, stopChan chan struct{}) {
	for {
		select {
		case <-stopChan:
			log.Println("Stopping frame reading...")
			return
		default:
			frame, err := d.ReadCanBusFrame()
			if err != nil {
				log.Printf("error reading CAN bus frame: %v", err)
				continue
			}
			g.OnCanBusFrameReceive(frame)
		}
	}
}
