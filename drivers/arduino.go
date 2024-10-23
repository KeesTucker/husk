package drivers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"husk/canbus"
	"husk/logging"
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
	ArduinoRetryDelay               = 200 * time.Millisecond
	ArduinoExponentialBackoffFactor = 2
)

// Error indicating that the serial port has been closed
var errorPortHasBeenClosed = errors.New("serial port has been closed")

// ArduinoDriver handles serial communication with an Arduino device.
type ArduinoDriver struct {
	isRunning        int32 // Use int32 for atomic operations
	portName         string
	port             serial.Port
	readChan         chan []byte
	writeChan        chan []byte
	ackChan          chan bool
	frameBroadcaster *CanFrameBroadcaster
	wg               sync.WaitGroup
	cancelFunc       context.CancelFunc
}

// ScanArduino scans serial ports to find Arduinos and initializes drivers for them.
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

// String returns a string representation of the ArduinoDriver.
func (d *ArduinoDriver) String() string {
	return fmt.Sprintf("Arduino: %s", d.portName)
}

// Register initializes the ArduinoDriver and registers it with the service registry.
func (d *ArduinoDriver) Register() (Driver, error) {
	var err error
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	// Initialize channels and broadcaster
	d.readChan = make(chan []byte, 128)
	d.writeChan = make(chan []byte, 128)
	d.ackChan = make(chan bool, 128)
	d.frameBroadcaster = NewCanFrameBroadcaster()

	// Give the port time to initialize if the Arduino has just been plugged in
	time.Sleep(ArduinoPortOpenDelay)

	// Open serial port
	mode := &serial.Mode{BaudRate: ArduinoBaudRate}
	d.port, err = serial.Open(d.portName, mode)
	if err != nil {
		l.WriteToLog(fmt.Sprintf("Error: opening port: %s", err.Error()), logging.LogTypeLog)
		return nil, err
	}

	// Set read timeout
	err = d.port.SetReadTimeout(ArduinoReadTimeout)
	if err != nil {
		l.WriteToLog(fmt.Sprintf("Error: setting read timeout: %s", err.Error()), logging.LogTypeLog)
		return nil, err
	}

	// Register the driver after successful initialization
	services.Register(services.ServiceDriver, d)

	l.WriteToLog(fmt.Sprintf("Arduino connected on port %s", d.portName), logging.LogTypeLog)
	return d, nil
}

// Start begins the driver's main loops and prepares it for operation.
func (d *ArduinoDriver) Start(ctx context.Context) (Driver, error) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	// Create a cancellable context
	ctx, d.cancelFunc = context.WithCancel(ctx)

	// Mark the driver as running
	atomic.StoreInt32(&d.isRunning, 1)

	// Start the main loops
	d.wg.Add(3)
	go d.assembleFramesFromSerial(ctx)
	go d.processAndBroadcastFrames(ctx)
	go d.writeFramesToSerial(ctx)

	l.WriteToLog("Arduino driver running", logging.LogTypeLog)
	return d, nil
}

// Cleanup stops the driver and releases all resources.
func (d *ArduinoDriver) Cleanup() {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	if !atomic.CompareAndSwapInt32(&d.isRunning, 1, 0) {
		// If isRunning was not 1, Cleanup has already been called
		return
	}

	// Cancel the context to signal goroutines to exit
	if d.cancelFunc != nil {
		d.cancelFunc()
	}

	// Close channels to unblock goroutines
	close(d.readChan)
	close(d.writeChan)
	close(d.ackChan)

	// Cleanup the broadcaster
	if d.frameBroadcaster != nil {
		d.frameBroadcaster.Cleanup()
	}

	// Wait for all goroutines to finish
	d.wg.Wait()

	// Close the serial port
	if d.port != nil {
		err := d.port.Close()
		if err != nil {
			l.WriteToLog(fmt.Sprintf("Error: closing port: %s", err.Error()), logging.LogTypeLog)
		} else {
			l.WriteToLog("Serial port closed successfully", logging.LogTypeLog)
		}
	}
}

// SendFrame sends a CAN bus frame to the Arduino, ensuring safe concurrency.
// Do not use for high-level communications; use the ECU or protocol layer instead.
func (d *ArduinoDriver) SendFrame(ctx context.Context, frame *canbus.CanFrame) error {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	if atomic.LoadInt32(&d.isRunning) == 0 {
		return fmt.Errorf("driver is not running")
	}

	// Don't log tester present. TODO: handle this in a more dynamic way in future. Possibly with filters in the GUI
	if frame.Data[1] != 0x3E {
		l.WriteToLog(fmt.Sprintf("Send: %s", frame.String()), logging.LogTypeCanbusLog)
	}

	frameBytes := d.createFrameBytes(frame)
	ackReceived := false
	retryDelay := ArduinoRetryDelay

	for retries := 0; retries < ArduinoMaxRetries && !ackReceived; retries++ {
		// Send frame to the write channel
		select {
		case d.writeChan <- frameBytes:
		case <-ctx.Done():
			return fmt.Errorf("operation cancelled")
		}

		if retries > 0 {
			l.WriteToLog(fmt.Sprintf("Retrying send frame: %v (attempt %d)", frameBytes, retries+1), logging.LogTypeLog)
		}

		// Wait for ACK or NACK
		select {
		case ackReceived = <-d.ackChan:
			if ackReceived {
				return nil
			}
			l.WriteToLog("NACK received from Arduino", logging.LogTypeLog)
		case <-time.After(ArduinoACKTimeout):
			l.WriteToLog("ACK timeout", logging.LogTypeLog)
		case <-ctx.Done():
			return fmt.Errorf("operation cancelled")
		}

		if !ackReceived {
			// Retry after a delay with exponential backoff
			l.WriteToLog(fmt.Sprintf("ACK not received, retrying in %d milliseconds", retryDelay.Milliseconds()), logging.LogTypeLog)
			time.Sleep(retryDelay)
			retryDelay *= ArduinoExponentialBackoffFactor
		}
	}

	if !ackReceived {
		return fmt.Errorf("failed to receive ACK after %d retries", ArduinoMaxRetries)
	}

	return nil
}

// SubscribeReadFrames allows a subscriber to receive broadcasted CAN frames.
func (d *ArduinoDriver) SubscribeReadFrames() chan *canbus.CanFrame {
	return d.frameBroadcaster.Subscribe()
}

// UnsubscribeReadFrames removes a subscriber from receiving broadcasted CAN frames.
func (d *ArduinoDriver) UnsubscribeReadFrames(ch chan *canbus.CanFrame) {
	d.frameBroadcaster.Unsubscribe(ch)
}

// assembleFramesFromSerial reads raw bytes from the serial port and assembles them into frames.
func (d *ArduinoDriver) assembleFramesFromSerial(ctx context.Context) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)
	defer d.wg.Done()

	var buffer []byte
	inFrame := false
	byteBuffer := make([]byte, 1) // Reuse byte buffer

	for {
		if atomic.LoadInt32(&d.isRunning) == 0 {
			return
		}

		select {
		case <-ctx.Done():
			return
		default:
			byteBuffer[0] = 0x00 // Reset byte buffer so we don't forever read the same byte over and over
			n, err := d.port.Read(byteBuffer)
			if err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, errorPortHasBeenClosed) {
					l.WriteToLog("Serial port has been closed", logging.LogTypeLog)
					d.cancelFunc()
					return
				}
				l.WriteToLog(fmt.Sprintf("Error: reading from port: %s", err.Error()), logging.LogTypeLog)
				atomic.StoreInt32(&d.isRunning, 0)
				d.cancelFunc()
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
					l.WriteToLog("ackChan is full, dropping ACK/NACK", logging.LogTypeLog)
				}
				continue
			}

			switch {
			case b == ArduinoStartMarker:
				// Start of a new frame
				inFrame = true
				buffer = buffer[:0] // Reset the buffer for the new frame

			case b == ArduinoEndMarker && inFrame:
				// End of the current frame
				inFrame = false
				// Send the unstuffed frame to readChan
				select {
				case d.readChan <- buffer:
				case <-ctx.Done():
					return
				}

			case inFrame && b == ArduinoEscapeChar:
				// Handle byte stuffing
				n, err = d.port.Read(byteBuffer)
				if err != nil {
					if err != io.EOF {
						l.WriteToLog(fmt.Sprintf("Rrror: reading from port after escape character: %s", err.Error()), logging.LogTypeLog)
					}
					continue
				}
				if n > 0 {
					unstuffedByte, err := d.unstuffByte(byteBuffer[0])
					if err != nil {
						l.WriteToLog(err.Error(), logging.LogTypeLog)
						// Discard the entire frame if invalid escape sequence
						inFrame = false
						buffer = buffer[:0]
						d.writeErrorResponse()
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

// processAndBroadcastFrames reads complete frames from readChan, processes them, and broadcasts to subscribers.
func (d *ArduinoDriver) processAndBroadcastFrames(ctx context.Context) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)
	defer d.wg.Done()

	for {
		select {
		case <-ctx.Done():
			l.WriteToLog("Stopping CAN bus frame processing due to context cancellation", logging.LogTypeLog)
			return
		default:
			frame, err := d.readFrame(ctx)
			if err != nil {
				if errors.Is(ctx.Err(), context.Canceled) {
					return
				}
				l.WriteToLog(err.Error(), logging.LogTypeLog)
				continue
			}
			if frame != nil {
				if frame.Data[1] != 0x7E {
					// TODO: we seem to miss some messages sometimes. likely something to do with the logging process downstream
					l.WriteToLog(fmt.Sprintf("Read: %s", frame.String()), logging.LogTypeCanbusLog)
				}
				d.frameBroadcaster.Broadcast(frame)
			}
		}
	}
}

// readFrame retrieves a received CAN bus frame from the read channel.
func (d *ArduinoDriver) readFrame(ctx context.Context) (*canbus.CanFrame, error) {
	select {
	case unstuffedBytes, ok := <-d.readChan:
		if !ok {
			return nil, fmt.Errorf("read channel closed")
		}
		if unstuffedBytes == nil {
			return nil, nil
		}

		if len(unstuffedBytes) < 4 {
			d.writeErrorResponse()
			return nil, fmt.Errorf("incomplete frame received (unstuffedBytes < 4)")
		}

		// CAN ID (2 bytes)
		id := (uint16(unstuffedBytes[0]) << 8) | uint16(unstuffedBytes[1])

		// DLC
		dlc := unstuffedBytes[2]
		if dlc > 8 {
			d.writeErrorResponse()
			return nil, fmt.Errorf("invalid DLC value: %d", dlc)
		}

		if len(unstuffedBytes) < 3+int(dlc)+1 {
			d.writeErrorResponse()
			return nil, fmt.Errorf("incomplete frame received, expected %d bytes but got %d", 3+int(dlc)+1, len(unstuffedBytes))
		}

		var dataBuffer [8]byte
		copy(dataBuffer[:], unstuffedBytes[3:3+dlc])

		frame := &canbus.CanFrame{
			ID:  id,
			DLC: dlc,
		}
		copy(frame.Data[:], dataBuffer[:])

		// Checksum
		receivedChecksum := unstuffedBytes[3+dlc]
		calculatedChecksum := calculateCRC8(frame)

		if calculatedChecksum != receivedChecksum {
			d.writeErrorResponse()
			return nil, fmt.Errorf("checksum mismatch: received %d, calculated %d", receivedChecksum, calculatedChecksum)
		}

		// Send ACK
		err := d.sendResponse(ArduinoACK)
		if err != nil {
			return nil, fmt.Errorf("failed to send ACK: %s", err.Error())
		}

		return frame, nil

	case <-ctx.Done():
		return nil, fmt.Errorf("operation cancelled")
	}
}

// writeFramesToSerial reads frames from the write channel and writes them to the serial port.
func (d *ArduinoDriver) writeFramesToSerial(ctx context.Context) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)
	defer d.wg.Done()

	for {
		if atomic.LoadInt32(&d.isRunning) == 0 {
			return
		}

		select {
		case <-ctx.Done():
			return
		case frameBytes, ok := <-d.writeChan:
			if !ok {
				return
			}
			_, err := d.port.Write(frameBytes)
			if err != nil {
				l.WriteToLog(fmt.Sprintf("Error: writing to port: %s", err.Error()), logging.LogTypeLog)
				atomic.StoreInt32(&d.isRunning, 0)
				d.cancelFunc()
				return
			}
		}
	}
}

// createFrameBytes constructs the byte sequence for a CAN bus frame with byte stuffing.
func (d *ArduinoDriver) createFrameBytes(frame *canbus.CanFrame) []byte {
	/*
		Custom Protocol:
		- Start Marker: 0x7E
		- End Marker: 0x7F
		- Escape Character: 0x1B
		- Byte Stuffing:
			- If data byte equals Start Marker, replace with Escape Character followed by 0x01
			- If data byte equals End Marker, replace with Escape Character followed by 0x02
			- If data byte equals Escape Character, replace with Escape Character followed by 0x03
		- Frame Structure:
			- [Start Marker][Frame Data][End Marker]
		- Frame Data:
			- [ID High][ID Low][DLC][Data Bytes][Checksum]
	*/

	frameBytes := []byte{ArduinoStartMarker}

	for _, b := range d.frameToBytes(frame) {
		d.stuffByte(b, &frameBytes)
	}

	// End Marker
	frameBytes = append(frameBytes, ArduinoEndMarker)

	return frameBytes
}

// frameToBytes converts the CAN frame into a byte slice.
func (d *ArduinoDriver) frameToBytes(frame *canbus.CanFrame) []byte {
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

// stuffByte handles byte stuffing for special characters in the frame.
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
		return 0, fmt.Errorf("invalid escape sequence")
	}
}

// writeErrorResponse sends a NACK response to the Arduino.
func (d *ArduinoDriver) writeErrorResponse() {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	err := d.sendResponse(ArduinoNACK)
	if err != nil {
		l.WriteToLog(fmt.Sprintf("Error: while trying to send NACK: %s", err.Error()), logging.LogTypeLog)
	}
}

// sendResponse sends a response (ACK/NACK) to the Arduino via the write channel.
func (d *ArduinoDriver) sendResponse(response byte) error {
	select {
	case d.writeChan <- []byte{response}:
		return nil
	default:
		return fmt.Errorf("write channel is full, cannot send response")
	}
}

// ackByteToBool converts an ACK/NACK byte to a boolean value.
func ackByteToBool(b byte) bool {
	return b == ArduinoACK
}

// calculateCRC8 computes the CRC-8 checksum for the given CAN frame.
func calculateCRC8(frame *canbus.CanFrame) byte {
	// Manually compute the CRC-8 checksum
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
