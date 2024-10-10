package drivers

import (
	"bufio"
	"context"
	"fmt"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"husk/canbus"
	"log"
	"sync"
	"time"
)

const (
	BaudRate    = 115200
	StartMarker = 0x7E
	EndMarker   = 0x7F
	EscapeChar  = 0x1B
)

// ArduinoDriver handles serial communication with an Arduino device.
type ArduinoDriver struct {
	port       serial.Port
	reader     *bufio.Reader
	ctx        context.Context
	cancel     context.CancelFunc
	writeMutex sync.Mutex
	pauseChan  chan struct{}
	resumeChan chan struct{}
	framesChan chan canbus.Frame
	errorChan  chan error
	readDone   sync.WaitGroup
}

// NewArduinoDriver initializes and returns a new ArduinoDriver.
func NewArduinoDriver() (*ArduinoDriver, error) {
	arduinoDriver := &ArduinoDriver{
		pauseChan:  make(chan struct{}, 1),
		resumeChan: make(chan struct{}, 1),
		framesChan: make(chan canbus.Frame, 100), // Buffered channel to hold incoming frames
		errorChan:  make(chan error, 1),
	}

	// Find Arduino port
	portName, err := findArduinoPortName()
	if err != nil {
		return nil, err
	}

	// Open serial port
	mode := &serial.Mode{BaudRate: BaudRate}
	arduinoDriver.port, err = serial.Open(portName, mode)
	if err != nil {
		return nil, err
	}

	// Create reader
	arduinoDriver.reader = bufio.NewReader(arduinoDriver.port)

	// Initialize context
	arduinoDriver.ctx, arduinoDriver.cancel = context.WithCancel(context.Background())

	// Start the read loop
	arduinoDriver.readDone.Add(1)
	go arduinoDriver.readLoop()

	return arduinoDriver, nil
}

// findArduinoPortName scans serial ports to find the Arduino.
func findArduinoPortName() (string, error) {
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return "", err
	}

	// Find the first matching USB port
	for _, port := range ports {
		if port.IsUSB {
			if port.VID == "2341" || port.VID == "1A86" || port.VID == "2A03" {
				return port.Name, nil
			}
		}
	}

	return "", fmt.Errorf("error: no Arduino found on the USB ports")
}

// Cleanup closes the serial port and stops the read loop.
func (d *ArduinoDriver) Cleanup() error {
	// Cancel the context to stop the read loop
	d.cancel()

	// Wait for the read loop to finish
	d.readDone.Wait()

	if d.port != nil {
		return d.port.Close()
	}
	return nil
}

// SendCanBusFrame sends a CAN bus frame to the Arduino, ensuring safe concurrency.
func (d *ArduinoDriver) SendCanBusFrame(frame canbus.Frame) error {
	d.writeMutex.Lock()
	defer d.writeMutex.Unlock()

	// Pause the read loop
	if err := d.pauseReading(); err != nil {
		return err
	}

	// Create frame bytes
	frameBytes := d.createFrameBytes(frame)

	// Write to serial port
	_, err := d.port.Write(frameBytes)
	if err != nil {
		// Resume reading even if write fails
		err := d.resumeReading()
		if err != nil {
			log.Printf("error resuming read on write error: %v", err)
		}
		return err
	}

	// Resume the read loop
	if err := d.resumeReading(); err != nil {
		return err
	}

	return nil
}

// ReadCanBusFrame retrieves a received CAN bus frame from the frames channel.
func (d *ArduinoDriver) ReadCanBusFrame() (*canbus.Frame, error) {
	select {
	case frame := <-d.framesChan:
		return &frame, nil
	case err := <-d.errorChan:
		return nil, err
	case <-d.ctx.Done():
		return nil, fmt.Errorf("driver has been closed")
	}
}

// pauseReading signals the read loop to pause.
func (d *ArduinoDriver) pauseReading() error {
	select {
	case d.pauseChan <- struct{}{}:
		// Signal sent successfully
	case <-time.After(1 * time.Second):
		return fmt.Errorf("timeout while pausing reading")
	}
	return nil
}

// resumeReading signals the read loop to resume.
func (d *ArduinoDriver) resumeReading() error {
	select {
	case d.resumeChan <- struct{}{}:
		// Signal sent successfully
	case <-time.After(1 * time.Second):
		return fmt.Errorf("timeout while resuming reading")
	}
	return nil
}

// readLoop continuously reads from the serial port, handling pause and resume signals.
func (d *ArduinoDriver) readLoop() {
	defer d.readDone.Done()

	for {
		select {
		case <-d.ctx.Done():
			fmt.Println("Read loop exiting due to context cancellation")
			return
		case <-d.pauseChan:
			fmt.Println("Read loop paused for writing")
			// Wait until resume signal is received
			select {
			case <-d.resumeChan:
				fmt.Println("Read loop resumed after writing")
			case <-d.ctx.Done():
				fmt.Println("Read loop exiting while paused due to context cancellation")
				return
			}
		default:
			// Attempt to read a frame
			frame, err := d.ReadCanBusFrameInternal()
			if err != nil {
				// Send error to error channel and exit read loop
				select {
				case d.errorChan <- err:
				default:
					// If error channel is full, discard the error
				}
				return
			}

			// Send the frame to the frames channel
			select {
			case d.framesChan <- *frame:
			case <-d.ctx.Done():
				return
			}
		}
	}
}

// ReadCanBusFrameInternal reads and unpacks a single CAN bus frame from the serial port.
func (d *ArduinoDriver) ReadCanBusFrameInternal() (*canbus.Frame, error) {
	// Read and unstuff the frame
	unstuffed, err := d.readAndUnstuffFrame()
	if err != nil {
		return nil, err
	}

	// Parse unstuffed data
	if len(unstuffed) < 4 {
		return nil, fmt.Errorf("error: incomplete frame received")
	}

	// CAN ID (2 bytes)
	id := (uint16(unstuffed[0]) << 8) | uint16(unstuffed[1])

	// DLC
	dlc := unstuffed[2]
	if dlc > 8 {
		return nil, fmt.Errorf("error: invalid DLC value: %d", dlc)
	}

	// Data (up to DLC bytes)
	if len(unstuffed) < 3+int(dlc) {
		return nil, fmt.Errorf("error: incomplete frame received, expected %d bytes but got %d", 3+int(dlc), len(unstuffed))
	}
	var dataBuffer [8]uint8
	copy(dataBuffer[:], unstuffed[3:3+dlc])

	// Checksum
	receivedChecksum := unstuffed[3+dlc]
	calculatedChecksum := calculateCRC8(dlc, dataBuffer)

	// Verify checksum
	if calculatedChecksum != receivedChecksum {
		return nil, fmt.Errorf("error: checksum mismatch")
	}

	// Create a new canbus.Frame object and populate it
	frame := &canbus.Frame{
		ID:  id,
		DLC: dlc,
	}
	copy(frame.Data[:], dataBuffer[:])

	return frame, nil
}

// readAndUnstuffFrame reads bytes from the serial port and removes byte stuffing.
func (d *ArduinoDriver) readAndUnstuffFrame() ([]byte, error) {
	// Wait for the start marker
	for {
		b, err := d.reader.ReadByte()
		if err != nil {
			return nil, err
		}
		if b == StartMarker {
			break
		}
		// Ignore bytes until start marker
	}

	// Buffer for storing unstuffed bytes
	var unstuffed []byte

	// Read bytes until the end marker is found
	for {
		b, err := d.reader.ReadByte()
		if err != nil {
			return nil, err
		}

		if b == EndMarker {
			// End marker found, break loop
			break
		} else if b == EscapeChar {
			// Handle escape sequences
			tag, err := d.reader.ReadByte()
			if err != nil {
				return nil, err
			}
			switch tag {
			case 0x01:
				unstuffed = append(unstuffed, StartMarker)
			case 0x02:
				unstuffed = append(unstuffed, EndMarker)
			case 0x03:
				unstuffed = append(unstuffed, EscapeChar)
			default:
				return nil, fmt.Errorf("error: invalid escape sequence")
			}
		} else {
			unstuffed = append(unstuffed, b)
		}
	}

	return unstuffed, nil
}

// createFrameBytes constructs the byte sequence for a CAN bus frame with byte stuffing.
func (d *ArduinoDriver) createFrameBytes(frame canbus.Frame) []byte {
	frameBytes := []byte{StartMarker}

	// Byte stuffing helper function
	stuffByte := func(b byte) {
		switch b {
		case StartMarker:
			frameBytes = append(frameBytes, EscapeChar, 0x01)
		case EndMarker:
			frameBytes = append(frameBytes, EscapeChar, 0x02)
		case EscapeChar:
			frameBytes = append(frameBytes, EscapeChar, 0x03)
		default:
			frameBytes = append(frameBytes, b)
		}
	}

	// CAN ID (2 bytes)
	idHigh := byte((frame.ID >> 8) & 0xFF)
	idLow := byte(frame.ID & 0xFF)
	stuffByte(idHigh)
	stuffByte(idLow)

	// DLC
	stuffByte(frame.DLC)

	// Data (only up to DLC)
	for i := 0; i < int(frame.DLC); i++ {
		stuffByte(frame.Data[i])
	}

	// Calculate and add checksum
	checksum := calculateCRC8(frame.DLC, frame.Data)
	stuffByte(checksum)

	// End Marker
	frameBytes = append(frameBytes, EndMarker)

	return frameBytes
}

// calculateCRC8 computes the CRC-8 checksum for the given data.
func calculateCRC8(dlc uint8, dataBuffer [8]uint8) byte {
	crc := byte(0x00)
	const polynomial = byte(0x07) // CRC-8-CCITT

	for i := 0; i < int(dlc); i++ {
		b := dataBuffer[i]
		crc ^= b
		for j := 0; j < 8; j++ {
			if crc&0x80 != 0 {
				crc = (crc << 1) ^ polynomial
			} else {
				crc <<= 1
			}
		}
	}

	return crc
}
