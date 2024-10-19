package canbus

import (
	"fmt"
	"strconv"
	"strings"
)

// CanFrame represents a CAN bus data frame with an 11-bit identifier.
type CanFrame struct {
	ID   uint16  // CAN identifier
	DLC  byte    // Data Length Code (0-8)
	Data [8]byte // Data payload
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

func StringToFrameData(in string) ([]byte, error) {
	// Ensure the string has an even length
	if len(in)%2 != 0 {
		return nil, fmt.Errorf("error: hex string has an odd length: %v", in)
	}

	// Pre-allocate the byte slice with the exact size
	data := make([]byte, len(in)/2)

	// Loop through the hex string, converting each pair of characters to a byte
	for i := 0; i < len(in); i += 2 {
		byteVal, err := strconv.ParseUint(in[i:i+2], 16, 8)
		if err != nil {
			return nil, fmt.Errorf("error: parsing hex byte at position %d: %v", i, err)
		}
		data[i/2] = byte(byteVal) // Place the parsed byte in the correct position
	}

	return data, nil
}
