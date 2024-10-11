package drivers

import (
	"fmt"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"husk/canbus"
	"io"
	"sync"
	"time"
)

const (
	BaudRate    = 115200
	StartMarker = 0x7E
	EndMarker   = 0x7F
	EscapeChar  = 0x1B
	ACK         = 0x06
	NACK        = 0x15
	MaxRetries  = 3
	ReadTimeout = 5 * time.Millisecond
	ACKTimeout  = 100 * time.Millisecond
	RetryDelay  = 100 * time.Millisecond
)

// ArduinoDriver handles serial communication with an Arduino device.
type ArduinoDriver struct {
	port        serial.Port
	serialMutex sync.Mutex
}

// NewArduinoDriver initializes and returns a new ArduinoDriver.
func NewArduinoDriver() (*ArduinoDriver, error) {
	arduinoDriver := &ArduinoDriver{}

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
	// Set read timeout
	err = arduinoDriver.port.SetReadTimeout(ReadTimeout)
	if err != nil {
		return nil, err
	}

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

	return "", fmt.Errorf("error no Arduino found on the USB ports")
}

// Cleanup closes the serial port and stops the read loop.
func (d *ArduinoDriver) Cleanup() error {
	if d.port != nil {
		return d.port.Close()
	}

	return nil
}

// SendCanBusFrame sends a CAN bus frame to the Arduino, ensuring safe concurrency.
func (d *ArduinoDriver) SendCanBusFrame(frame canbus.Frame) error {
	d.serialMutex.Lock()
	defer d.serialMutex.Unlock()

	frameBytes := d.createFrameBytes(frame)

	ackReceived := false

	for retries := 0; retries < MaxRetries && !ackReceived; retries++ {
		// Write to serial port
		_, err := d.port.Write(frameBytes)
		if err != nil {
			return err
		}

		// Wait for ACK or NACK
		ackReceived, err = d.waitForAck()
		if err != nil {
			return err
		}

		if !ackReceived {
			// Retry after a delay
			time.Sleep(RetryDelay)
		}
	}

	if !ackReceived {
		return fmt.Errorf("error failed to receive ACK after %d retries", MaxRetries)
	}

	return nil
}

func (d *ArduinoDriver) waitForAck() (bool, error) {
	timeout := time.After(ACKTimeout)
	ackBuffer := make([]byte, 1)

	for {
		select {
		case <-timeout:
			return false, nil
		default:
			n, err := d.port.Read(ackBuffer)
			if err != nil {
				if err == io.EOF {
					continue
				}
				return false, err
			}

			if n > 0 {
				if ackBuffer[0] == ACK {
					return true, nil
				} else if ackBuffer[0] == NACK {
					return false, nil
				}
			}
		}
	}
}

// ReadCanBusFrame retrieves a received CAN bus frame from the frames channel.
func (d *ArduinoDriver) ReadCanBusFrame() (*canbus.Frame, error) {
	d.serialMutex.Lock()
	defer d.serialMutex.Unlock()

	// Read and unstuff the frame
	unstuffed, err := d.readAndUnstuffFrame()
	if err != nil {
		_, err = d.port.Write([]byte{NACK})
		if err != nil {
			return nil, fmt.Errorf("error failed to send nack on read and unstuff error: %s", err)
		}
		return nil, err
	}

	if unstuffed == nil {
		return nil, nil
	}

	// Parse unstuffed data
	if len(unstuffed) < 4 {
		_, err = d.port.Write([]byte{NACK})
		if err != nil {
			return nil, fmt.Errorf("error failed to send nack on incomplete frame: %s", err)
		}
		return nil, fmt.Errorf("error incomplete frame received")
	}

	// CAN ID (2 bytes)
	id := (uint16(unstuffed[0]) << 8) | uint16(unstuffed[1])

	// DLC
	dlc := unstuffed[2]
	if dlc > 8 {
		_, err = d.port.Write([]byte{NACK})
		if err != nil {
			return nil, fmt.Errorf("error failed to send nack on invalid DLC: %s", err)
		}
		return nil, fmt.Errorf("error invalid DLC value: %d", dlc)
	}

	// Data
	if len(unstuffed) < 3+int(dlc)+1 {
		_, err = d.port.Write([]byte{NACK})
		if err != nil {
			return nil, fmt.Errorf("error failed to send nack on incomplete frame: %s", err)
		}
		return nil, fmt.Errorf("error incomplete frame received, expected %d bytes but got %d", 3+int(dlc), len(unstuffed))
	}
	var dataBuffer [8]uint8
	copy(dataBuffer[:], unstuffed[3:3+dlc])

	// Create a new canbus.Frame object and populate it
	frame := &canbus.Frame{
		ID:  id,
		DLC: dlc,
	}
	copy(frame.Data[:], dataBuffer[:])

	// Checksum
	receivedChecksum := unstuffed[3+dlc]
	calculatedChecksum := calculateCRC8(frame)

	// Verify checksum
	if calculatedChecksum != receivedChecksum {
		_, err = d.port.Write([]byte{NACK})
		if err != nil {
			return nil, fmt.Errorf("error failed to send nack on bad checksum: %s", err)
		}
		return nil, fmt.Errorf("error checksum mismatch")
	}

	// Send ACK
	_, err = d.port.Write([]byte{ACK})
	if err != nil {
		return nil, fmt.Errorf("error failed to send ack: %s", err)
	}

	return frame, nil
}

// readAndUnstuffFrame reads bytes from the serial port and removes byte stuffing.
func (d *ArduinoDriver) readAndUnstuffFrame() ([]byte, error) {
	byteBuffer := make([]byte, 1)

	_, err := d.port.Read(byteBuffer)
	if err != nil {
		return nil, err
	}
	b := byteBuffer[0]
	if b != StartMarker {
		return nil, nil
	}

	// Buffer for storing unstuffed bytes
	var unstuffed []byte

	// Read bytes until the end marker is found
	for {
		_, err = d.port.Read(byteBuffer)
		if err != nil {
			return nil, err
		}
		b = byteBuffer[0]

		if b == EndMarker {
			// End marker found, break loop
			break
		} else if b == EscapeChar {
			// Handle escape sequences
			byteBuffer = make([]byte, 1)
			_, err = d.port.Read(byteBuffer)
			if err != nil {
				return nil, err
			}
			tag := byteBuffer[0]
			switch tag {
			case 0x01:
				unstuffed = append(unstuffed, StartMarker)
			case 0x02:
				unstuffed = append(unstuffed, EndMarker)
			case 0x03:
				unstuffed = append(unstuffed, EscapeChar)
			default:
				return nil, fmt.Errorf("error invalid escape sequence")
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
	checksum := calculateCRC8(&frame)
	stuffByte(checksum)

	// End Marker
	frameBytes = append(frameBytes, EndMarker)

	return frameBytes
}

// calculateCRC8 computes the CRC-8 checksum for the given data.
func calculateCRC8(frame *canbus.Frame) byte {
	crc := byte(0x00)
	const polynomial = byte(0x07) // CRC-8-CCITT

	// Include ID (2 bytes)
	idBytes := []byte{byte(frame.ID >> 8), byte(frame.ID & 0xFF)}
	for _, b := range idBytes {
		crc ^= b
		for i := 0; i < 8; i++ {
			if crc&0x80 != 0 {
				crc = (crc << 1) ^ polynomial
			} else {
				crc <<= 1
			}
		}
	}

	// Include DLC
	crc ^= frame.DLC
	for i := 0; i < 8; i++ {
		if crc&0x80 != 0 {
			crc = (crc << 1) ^ polynomial
		} else {
			crc <<= 1
		}
	}

	// Include Data bytes
	for i := 0; i < int(frame.DLC); i++ {
		b := frame.Data[i]
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
