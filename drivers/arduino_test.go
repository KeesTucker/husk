// arduino_test.go

package drivers

import (
	"bufio"
	"bytes"
	"fmt"
	"husk/canbus"
	"testing"
)

// Helper function to simulate reading from the serial port
func (d *ArduinoDriver) simulateRead(data []byte) {
	d.reader = bufio.NewReader(bytes.NewReader(data))
}

func TestByteStuffing(t *testing.T) {
	driver := &ArduinoDriver{}

	// Test frames with various data, including special characters
	testFrames := []canbus.Frame{
		{
			ID:  0x123,
			DLC: 3,
			Data: [8]byte{
				0x7E, // StartMarker
				0x7F, // EndMarker
				0x1B, // EscapeChar
			},
		},
		{
			ID:  0x456,
			DLC: 5,
			Data: [8]byte{
				0x11, 0x22, 0x33, 0x44, 0x55,
			},
		},
		{
			ID:  0x7FF,
			DLC: 8,
			Data: [8]byte{
				0x00, 0xFF, 0x7E, 0x7F, 0x1B, 0xAA, 0xBB, 0xCC,
			},
		},
	}

	for _, frame := range testFrames {
		// Create frame bytes with byte-stuffing
		stuffedBytes := driver.createFrameBytes(frame)

		// Simulate reading the stuffed bytes
		driver.simulateRead(stuffedBytes)

		// Read and unstuff the frame
		unstuffed, err := driver.readAndUnstuffFrame()
		if err != nil {
			t.Fatalf("Error unstuffing frame: %v", err)
		}

		// Parse the unstuffed data
		parsedFrame, err := parseUnstuffedData(unstuffed)
		if err != nil {
			t.Fatalf("Error parsing frame: %v", err)
		}

		// Compare the original and parsed frames
		if parsedFrame.ID != frame.ID || parsedFrame.DLC != frame.DLC || parsedFrame.Data != frame.Data {
			t.Errorf("Frames do not match.\nOriginal: %+v\nParsed:   %+v", frame, parsedFrame)
		}
	}
}

// Helper function to parse unstuffed data into a canbus.Frame
func parseUnstuffedData(data []byte) (*canbus.Frame, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("incomplete frame")
	}

	id := (uint16(data[0]) << 8) | uint16(data[1])
	dlc := data[2]
	if dlc > 8 {
		return nil, fmt.Errorf("invalid DLC")
	}

	if len(data) < int(3+dlc+1) { // +1 for checksum
		return nil, fmt.Errorf("data length mismatch")
	}

	var dataBuffer [8]byte
	copy(dataBuffer[:], data[3:3+dlc])

	// Recalculate checksum
	frame := canbus.Frame{
		ID:  id,
		DLC: dlc,
	}
	copy(frame.Data[:], dataBuffer[:])

	calculatedChecksum := calculateCRC8(&frame)
	receivedChecksum := data[3+dlc]

	if calculatedChecksum != receivedChecksum {
		return nil, fmt.Errorf("checksum mismatch")
	}

	return &frame, nil
}
