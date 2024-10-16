package drivers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"husk/logging"
	"husk/protocols"
	"husk/services"
)

const (
	ArduinoBaudRate                 = 921600
	ArduinoStartMarker              = 0x7E
	ArduinoEndMarker                = 0x7F
	ArduinoEscapeChar               = 0x1B
	ArduinoACK                      = 0x06
	ArduinoNACK                     = 0x15
	ArduinoMaxRetries               = 3
	ArduinoPortOpenDelay            = 500 * time.Millisecond
	ArduinoReadTimeout              = 5 * time.Millisecond
	ArduinoACKTimeout               = 100 * time.Millisecond
	ArduinoRetryDelay               = 100 * time.Millisecond
	ArduinoExponentialBackoffFactor = 2
)

// ArduinoDriver handles serial communication with an Arduino device.
type ArduinoDriver struct {
	isRunning bool
	portName  string
	port      serial.Port
	readChan  chan []byte
	writeChan chan []byte
	ackChan   chan bool
}

// ScanArduino scans serial ports to find Arduinos
func ScanArduino(ports []*enumerator.PortDetails, drivers []Driver) []Driver {
	for _, port := range ports {
		if port.IsUSB {
			// VID 2341 for Arduino, 1A86 for CH340, 2A03 for Arduino clone
			if port.VID == "2341" || port.VID == "1A86" || port.VID == "2A03" {
				drivers = append(drivers, &ArduinoDriver{
					portName: port.Name,
				})
			}
		}
	}

	return drivers
}

func (d *ArduinoDriver) String() string {
	return fmt.Sprintf("Arduino: %s", d.portName)
}

func (d *ArduinoDriver) Register() (Driver, error) {
	var err error
	l := services.Get(services.ServiceLogger).(*logging.Logger)
	defer services.Register(services.ServiceDriver, d)

	d.readChan = make(chan []byte, 32)
	d.writeChan = make(chan []byte, 32)
	d.ackChan = make(chan bool, 32)

	// Give port time to init if arduino has just been plugged in.
	time.Sleep(ArduinoPortOpenDelay)
	// Open serial port
	mode := &serial.Mode{BaudRate: ArduinoBaudRate}
	d.port, err = serial.Open(d.portName, mode)
	if err != nil {
		l.WriteToLog(fmt.Sprintf("error: opening port: %s", err.Error()))
		return nil, err
	}
	// Set read timeout
	err = d.port.SetReadTimeout(ArduinoReadTimeout)
	if err != nil {
		l.WriteToLog(fmt.Sprintf("error: setting read timeout: %s", err.Error()))
		return nil, err
	}

	l.WriteToLog(fmt.Sprintf("arduino connected on port %s", d.portName))
	return d, nil
}

func (d *ArduinoDriver) Start(ctx context.Context) Driver {
	go func() {
		l := services.Get(services.ServiceLogger).(*logging.Logger)
		// Start read and write loops
		go d.readLoop(ctx)
		go d.writeLoop(ctx)
		go func() {
			<-ctx.Done()
			d.Cleanup()
		}()

		d.isRunning = true

		l.WriteToLog("arduino driver running")
	}()

	return d
}

func (d *ArduinoDriver) Cleanup() {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	d.isRunning = false
	if d.port != nil {
		err := d.port.Close()
		if err != nil {
			l.WriteToLog(fmt.Sprintf("error: closing port: %s", err.Error()))
		}
		l.WriteToLog("serial port closed successfully")
	}
}

// SendFrame sends a CAN bus frame to the Arduino, ensuring safe concurrency.
func (d *ArduinoDriver) SendFrame(ctx context.Context, frame *protocols.CanFrame) error {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	if !d.isRunning {
		return nil
	}

	frameBytes := d.createFrameBytes(frame)
	ackReceived := false
	retryDelay := ArduinoRetryDelay

	for retries := 0; retries < ArduinoMaxRetries && !ackReceived; retries++ {
		// Send frame to the write channel without context.Done() check
		d.writeChan <- frameBytes

		if retries > 0 {
			l.WriteToLog(fmt.Sprintf("sent frame: %v (attempt %d)", frameBytes, retries+1))
		}

		// Wait for ACK or NACK
		select {
		case ackReceived = <-d.ackChan:
			if ackReceived {
				return nil
			}
			l.WriteToLog("NACK received from Arduino")
		case <-time.After(ArduinoACKTimeout):
			l.WriteToLog("ACK timeout")
		case <-ctx.Done():
			return fmt.Errorf("operation cancelled")
		}

		if !ackReceived {
			// Retry after a delay with exponential backoff
			l.WriteToLog(fmt.Sprintf("ACK not received, retrying in %d milliseconds", retryDelay.Milliseconds()))
			time.Sleep(retryDelay)
			retryDelay *= ArduinoExponentialBackoffFactor
		}
	}

	if !ackReceived {
		return fmt.Errorf("error: failed to receive ACK after %d retries", ArduinoMaxRetries)
	}

	return nil
}

// ReadFrame retrieves a received CAN bus frame from the read channel.
func (d *ArduinoDriver) ReadFrame(ctx context.Context) (*protocols.CanFrame, error) {
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

		frame := &protocols.CanFrame{
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
		err := d.sendResponse(ArduinoACK)
		if err != nil {
			return nil, fmt.Errorf("error: failed to send ACK: %s", err.Error())
		}

		return frame, nil

	case <-ctx.Done():
		return nil, fmt.Errorf("operation cancelled")
	}
}

// readLoop continuously reads from the serial port and sends complete protocols to the read channel.
func (d *ArduinoDriver) readLoop(ctx context.Context) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	var buffer []byte
	inFrame := false

	for {
		select {
		case <-ctx.Done():
			return
		default:
			byteBuffer := make([]byte, 1)
			n, err := d.port.Read(byteBuffer)
			if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, errorPortHasBeenClosed) {
				l.WriteToLog(fmt.Sprintf("error: reading from port, killing arduino driver: %s", err))
				d.isRunning = false
				return
			}

			if n <= 0 {
				continue
			}

			b := byteBuffer[0]

			// Handle ACK or NACK bytes immediately
			if !inFrame && (b == ArduinoACK || b == ArduinoNACK) {
				select {
				case d.ackChan <- ackByteToBool(b):
					// Successfully sent ACK/NACK
				default:
					l.WriteToLog("ackChan is full, dropping ACK/NACK")
				}
				continue
			}

			switch {
			case b == ArduinoStartMarker:
				// Start of a new frame
				inFrame = true
				buffer = []byte{} // Reset the buffer for the new frame

			case b == ArduinoEndMarker && inFrame:
				// End of the current frame
				inFrame = false
				d.readChan <- buffer

			case inFrame && b == ArduinoEscapeChar:
				// Handle byte stuffing
				n, err := d.port.Read(byteBuffer)
				if err != nil && err != io.EOF {
					l.WriteToLog(fmt.Sprintf("error: reading from port after escape character: %s", err))
					continue
				}
				if n > 0 {
					unstuffedByte, err := d.unstuffByte(byteBuffer[0])
					if err != nil {
						l.WriteToLog(err.Error())
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

// writeLoop continuously writes data from the write channel to the serial port.
func (d *ArduinoDriver) writeLoop(ctx context.Context) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	for {
		select {
		case <-ctx.Done():
			return
		case frameBytes := <-d.writeChan:
			_, err := d.port.Write(frameBytes)
			if err != nil {
				l.WriteToLog(fmt.Sprintf("error: writing to port, killing arduino driver: %s", err))
				d.isRunning = false
				return
			}
		}
	}
}

func ackByteToBool(b byte) bool {
	if b == ArduinoACK {
		return true
	}

	return false
}

// writeErrorResponse sends a NACK
func (d *ArduinoDriver) writeErrorResponse() {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	err := d.sendResponse(ArduinoNACK)
	if err != nil {
		l.WriteToLog(fmt.Sprintf("error: while trying to send NACK: %s", err.Error()))
	}
}

// sendResponse sends a response (ACK/NACK) to the Arduino.
func (d *ArduinoDriver) sendResponse(response byte) error {
	d.writeChan <- []byte{response}
	return nil
}

// createFrameBytes constructs the byte sequence for a CAN bus frame with byte stuffing.
func (d *ArduinoDriver) createFrameBytes(frame *protocols.CanFrame) []byte {
	frameBytes := []byte{ArduinoStartMarker}

	for _, b := range d.frameToBytes(frame) {
		d.stuffByte(b, &frameBytes)
	}

	// End Marker
	frameBytes = append(frameBytes, ArduinoEndMarker)

	return frameBytes
}

// frameToBytes converts the frame to a sequence of bytes.
func (d *ArduinoDriver) frameToBytes(frame *protocols.CanFrame) []byte {
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
	case ArduinoStartMarker:
		*output = append(*output, ArduinoEscapeChar, 0x01)
	case ArduinoEndMarker:
		*output = append(*output, ArduinoEscapeChar, 0x02)
	case ArduinoEscapeChar:
		*output = append(*output, ArduinoEscapeChar, 0x03)
	default:
		*output = append(*output, b)
	}
}

// unstuffByte handles byte unstuffing and returns the original byte.
func (d *ArduinoDriver) unstuffByte(b byte) (byte, error) {
	switch b {
	case 0x01:
		return ArduinoStartMarker, nil
	case 0x02:
		return ArduinoEndMarker, nil
	case 0x03:
		return ArduinoEscapeChar, nil
	default:
		return 0, fmt.Errorf("error: invalid escape sequence")
	}
}

// calculateCRC8 computes the CRC-8 checksum for the given data.
func calculateCRC8(frame *protocols.CanFrame) byte {
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
