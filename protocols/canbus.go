package protocols

import (
	"fmt"
	"strconv"
	"strings"
)

// CanFrame represents a CAN bus data frame with an 11-bit identifier.
type CanFrame struct {
	ID   uint16   // CAN identifier
	DLC  uint8    // Data Length Code (0-8)
	Data [8]uint8 // Data payload
}

// String method to provide a human-readable representation of the CAN CanFrame.
func (f *CanFrame) String() string {
	formattedData := make([]string, f.DLC)
	for i := 0; i < int(f.DLC); i++ {
		formattedData[i] = fmt.Sprintf("0x%02X", f.Data[i])
	}
	dataString := strings.Join(formattedData, " ")
	return fmt.Sprintf("ID: 0x%X, DLC: %d, Data: %s", f.ID, f.DLC, dataString)
}

func StringToFrame(in string) (*CanFrame, error) {
	// Convert the message to a byte array
	data, err := hexStringToBytes(in)
	if err != nil {
		return nil, fmt.Errorf("error: converting input to bytes: %s", err.Error())
	}

	dlc := uint8(len(data))
	if dlc > 8 {
		return nil, fmt.Errorf("error: can't send more than 8 bytes")
	}

	frame := &CanFrame{
		DLC: dlc,
	}
	copy(frame.Data[:], data)

	return frame, nil
}

// HexStringToBytes converts a hex string to a byte slice
func hexStringToBytes(s string) ([]byte, error) {
	// Ensure the string has an even length
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("error: hex string has an odd length: %v", s)
	}

	// Pre-allocate the byte slice with the exact size
	data := make([]byte, len(s)/2)

	// Loop through the hex string, converting each pair of characters to a byte
	for i := 0; i < len(s); i += 2 {
		byteVal, err := strconv.ParseUint(s[i:i+2], 16, 8)
		if err != nil {
			return nil, fmt.Errorf("error: parsing hex byte at position %d: %v", i, err)
		}
		data[i/2] = byte(byteVal) // Place the parsed byte in the correct position
	}

	return data, nil
}
