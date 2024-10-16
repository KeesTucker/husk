package drivers

import (
	"context"
	"fmt"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"husk/canbus"
	"husk/logging"
	"io"
	"time"
)

const (
	BaudRate                 = 921600
	StartMarker              = 0x7E
	EndMarker                = 0x7F
	EscapeChar               = 0x1B
	ACK                      = 0x06
	NACK                     = 0x15
	MaxRetries               = 3
	PortScanDelay            = 100 * time.Millisecond
	PortOpenDelay            = 1000 * time.Millisecond
	ReadTimeout              = 5 * time.Millisecond
	ACKTimeout               = 100 * time.Millisecond
	RetryDelay               = 100 * time.Millisecond
	ExponentialBackoffFactor = 2
)

// ArduinoDriver handles serial communication with an Arduino device.
type ArduinoDriver struct {
	ctx       context.Context
	l         *logging.Logger
	port      serial.Port
	readChan  chan []byte
	writeChan chan []byte
	ackChan   chan bool
	cancel    context.CancelFunc
}

// NewArduinoDriver initializes and returns a new ArduinoDriver.
func NewArduinoDriver(ctx context.Context, logger *logging.Logger) (*ArduinoDriver, error) {
	ctx, cancel := context.WithCancel(ctx)

	arduinoDriver := &ArduinoDriver{
		ctx:       ctx,
		l:         logger,
		readChan:  make(chan []byte, 32),
		writeChan: make(chan []byte, 32),
		ackChan:   make(chan bool, 32),
		cancel:    cancel,
	}

	// Find Arduino port
	portName, err := findArduinoPort(ctx)
	if err != nil {
		return nil, err
	}

	// Give port time to init if arduino has just been plugged in.
	time.Sleep(PortOpenDelay)

	// Open serial port
	mode := &serial.Mode{BaudRate: BaudRate}
	arduinoDriver.port, err = serial.Open(portName, mode)
	if err != nil {
		return nil, err
	}
	// Set read timeout
	err = arduinoDriver.port.SetReadTimeout(ReadTimeout)
	if err != nil {
		return nil, err
	}

	arduinoDriver.l.WriteToLog(fmt.Sprintf("arduino connected on port %s", portName))

	// Start read and write loops
	go arduinoDriver.readLoop()
	go arduinoDriver.writeLoop()

	return arduinoDriver, nil
}

// findArduinoPort scans serial ports to find the Arduino, blocks until arduino is found or context is cancelled.
func findArduinoPort(ctx context.Context) (string, error) {
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("error: no Arduino found on the USB ports")
		default:
			ports, err := enumerator.GetDetailedPortsList()
			if err != nil {
				return "", err
			}

			// Find the first matching USB port
			for _, port := range ports {
				if port.IsUSB {
					// VID 2341 for Arduino, 1A86 for CH340, 2A03 for Arduino clone
					if port.VID == "2341" || port.VID == "1A86" || port.VID == "2A03" {
						return port.Name, nil
					}
				}
			}

			time.Sleep(PortScanDelay)
		}
	}
}

// Cleanup closes the serial port and stops the read and write loops.
func (d *ArduinoDriver) Cleanup() error {
	d.cancel()
	if d.port != nil {
		err := d.port.Close()
		if err != nil {
			d.l.WriteToLog(fmt.Sprintf("error: closing port: %s", err.Error()))
			return err
		}
		d.l.WriteToLog("serial port closed successfully")
	}
	return nil
}

// SendCanBusFrame sends a CAN bus frame to the Arduino, ensuring safe concurrency.
func (d *ArduinoDriver) SendCanBusFrame(frame *canbus.Frame) error {
	frameBytes := d.createFrameBytes(frame)
	ackReceived := false
	retryDelay := RetryDelay

	for retries := 0; retries < MaxRetries && !ackReceived; retries++ {
		// Send frame to the write channel without context.Done() check
		d.writeChan <- frameBytes

		if retries > 0 {
			d.l.WriteToLog(fmt.Sprintf("sent frame: %v (attempt %d)", frameBytes, retries+1))
		}

		// Wait for ACK or NACK
		select {
		case ackReceived = <-d.ackChan:
			if ackReceived {
				return nil
			}
			d.l.WriteToLog("NACK received from Arduino")
		case <-time.After(ACKTimeout):
			d.l.WriteToLog("ACK timeout")
		case <-d.ctx.Done():
			return fmt.Errorf("operation cancelled")
		}

		if !ackReceived {
			// Retry after a delay with exponential backoff
			d.l.WriteToLog(fmt.Sprintf("ACK not received, retrying in %d milliseconds", retryDelay.Milliseconds()))
			time.Sleep(retryDelay)
			retryDelay *= ExponentialBackoffFactor
		}
	}

	if !ackReceived {
		return fmt.Errorf("error: failed to receive ACK after %d retries", MaxRetries)
	}

	return nil
}

// ReadCanBusFrame retrieves a received CAN bus frame from the read channel.
func (d *ArduinoDriver) ReadCanBusFrame() (*canbus.Frame, error) {
	select {
	case unstuffedBytes := <-d.readChan:
		if unstuffedBytes == nil {
			return nil, nil
		}

		if len(unstuffedBytes) < 4 {
			d.writeErrorResponse()
			return nil, fmt.Errorf("error: incomplete frame received (unstuffedBytes < 4)")
		}

		// CAN ID (2 bytes)
		id := (uint16(unstuffedBytes[0]) << 8) | uint16(unstuffedBytes[1])

		// DLC
		dlc := unstuffedBytes[2]
		if dlc > 8 {
			d.writeErrorResponse()
			return nil, fmt.Errorf("error: invalid DLC value: %d", dlc)
		}

		if len(unstuffedBytes) < 3+int(dlc)+1 {
			d.writeErrorResponse()
			return nil, fmt.Errorf("error: incomplete frame received, expected %d bytes but got %d", 3+int(dlc)+1, len(unstuffedBytes))
		}

		var dataBuffer [8]uint8
		copy(dataBuffer[:], unstuffedBytes[3:3+dlc])

		frame := &canbus.Frame{
			ID:  id,
			DLC: dlc,
		}
		copy(frame.Data[:], dataBuffer[:])

		// Checksum
		receivedChecksum := unstuffedBytes[3+dlc]
		calculatedChecksum := calculateCRC8(frame)

		if calculatedChecksum != receivedChecksum {
			d.writeErrorResponse()
			return nil, fmt.Errorf("error: checksum mismatch: received %d, calculated %d", receivedChecksum, calculatedChecksum)
		}

		// Send ACK
		err := d.sendResponse(ACK)
		if err != nil {
			return nil, fmt.Errorf("error: failed to send ACK: %s", err.Error())
		}

		return frame, nil

	case <-d.ctx.Done():
		return nil, fmt.Errorf("operation cancelled")
	}
}

// readLoop continuously reads from the serial port and sends complete frames to the read channel.
func (d *ArduinoDriver) readLoop() {
	var buffer []byte
	inFrame := false

	for {
		select {
		case <-d.ctx.Done():
			return
		default:
			byteBuffer := make([]byte, 1)
			n, err := d.port.Read(byteBuffer)
			if err != nil && err != io.EOF {
				d.l.WriteToLog(fmt.Sprintf("error: reading from port: %s", err))
				continue
			}

			if n <= 0 {
				continue
			}

			b := byteBuffer[0]

			// Handle ACK or NACK bytes immediately
			if !inFrame && (b == ACK || b == NACK) {
				select {
				case d.ackChan <- ackByteToBool(b):
					// Successfully sent ACK/NACK
				default:
					d.l.WriteToLog("ackChan is full, dropping ACK/NACK")
				}
				continue
			}

			switch {
			case b == StartMarker:
				// Start of a new frame
				inFrame = true
				buffer = []byte{} // Reset the buffer for the new frame

			case b == EndMarker && inFrame:
				// End of the current frame
				inFrame = false
				d.readChan <- buffer

			case inFrame && b == EscapeChar:
				// Handle byte stuffing
				n, err := d.port.Read(byteBuffer)
				if err != nil && err != io.EOF {
					d.l.WriteToLog(fmt.Sprintf("error: reading from port after escape character: %s", err))
					continue
				}
				if n > 0 {
					unstuffedByte, err := d.unstuffByte(byteBuffer[0])
					if err != nil {
						d.l.WriteToLog(err.Error())
						continue
					}
					buffer = append(buffer, unstuffedByte)
				}

			case inFrame:
				// Add the byte to the current frame buffer
				buffer = append(buffer, b)
			}
		}
	}
}

func ackByteToBool(b byte) bool {
	if b == ACK {
		return true
	}

	return false
}

// writeLoop continuously writes data from the write channel to the serial port.
func (d *ArduinoDriver) writeLoop() {
	for {
		select {
		case <-d.ctx.Done():
			return
		case frameBytes := <-d.writeChan:
			_, err := d.port.Write(frameBytes)
			if err != nil {
				d.l.WriteToLog(fmt.Sprintf("error: writing to port: %s", err))
			}
		}
	}
}

// writeErrorResponse sends a NACK
func (d *ArduinoDriver) writeErrorResponse() {
	err := d.sendResponse(NACK)
	if err != nil {
		d.l.WriteToLog(fmt.Sprintf("error: while trying to send NACK: %s", err.Error()))
	}
}

// sendResponse sends a response (ACK/NACK) to the Arduino.
func (d *ArduinoDriver) sendResponse(response byte) error {
	d.writeChan <- []byte{response}
	return nil
}

// createFrameBytes constructs the byte sequence for a CAN bus frame with byte stuffing.
func (d *ArduinoDriver) createFrameBytes(frame *canbus.Frame) []byte {
	frameBytes := []byte{StartMarker}

	for _, b := range d.frameToBytes(frame) {
		d.stuffByte(b, &frameBytes)
	}

	// End Marker
	frameBytes = append(frameBytes, EndMarker)

	return frameBytes
}

// frameToBytes converts the frame to a sequence of bytes.
func (d *ArduinoDriver) frameToBytes(frame *canbus.Frame) []byte {
	var frameBytes []byte

	// CAN ID (2 bytes)
	idHigh := byte((frame.ID >> 8) & 0xFF)
	idLow := byte(frame.ID & 0xFF)
	frameBytes = append(frameBytes, idHigh, idLow)

	// DLC
	frameBytes = append(frameBytes, frame.DLC)

	// Data (only up to DLC)
	for i := 0; i < int(frame.DLC); i++ {
		frameBytes = append(frameBytes, frame.Data[i])
	}

	// Calculate and add checksum
	checksum := calculateCRC8(frame)
	frameBytes = append(frameBytes, checksum)

	return frameBytes
}

// stuffByte handles byte stuffing and appends to the output.
func (d *ArduinoDriver) stuffByte(b byte, output *[]byte) {
	switch b {
	case StartMarker:
		*output = append(*output, EscapeChar, 0x01)
	case EndMarker:
		*output = append(*output, EscapeChar, 0x02)
	case EscapeChar:
		*output = append(*output, EscapeChar, 0x03)
	default:
		*output = append(*output, b)
	}
}

// unstuffByte handles byte unstuffing and returns the original byte.
func (d *ArduinoDriver) unstuffByte(b byte) (byte, error) {
	switch b {
	case 0x01:
		return StartMarker, nil
	case 0x02:
		return EndMarker, nil
	case 0x03:
		return EscapeChar, nil
	default:
		return 0, fmt.Errorf("error: invalid escape sequence")
	}
}

// calculateCRC8 computes the CRC-8 checksum for the given data.
func calculateCRC8(frame *canbus.Frame) byte {
	crc := byte(0x00)
	const polynomial = byte(0x07) // CRC-8-CCITT

	xorShift := func(crc, b byte) byte {
		crc ^= b
		for i := 0; i < 8; i++ {
			if crc&0x80 != 0 {
				crc = (crc << 1) ^ polynomial
			} else {
				crc <<= 1
			}
		}
		return crc
	}

	// Include ID (2 bytes)
	idBytes := []byte{byte(frame.ID >> 8), byte(frame.ID & 0xFF)}
	for _, b := range idBytes {
		crc = xorShift(crc, b)
	}

	// Include DLC
	crc = xorShift(crc, frame.DLC)

	// Include Data bytes
	for i := 0; i < int(frame.DLC); i++ {
		crc = xorShift(crc, frame.Data[i])
	}

	return crc
}
