package frames

import (
	"fmt"
	"strings"
)

// Frame represents a CAN bus data frame with an 11-bit identifier.
type Frame struct {
	ID   uint16   // CAN identifier
	DLC  uint8    // Data Length Code (0-8)
	Data [8]uint8 // Data payload
}

// String method to provide a human-readable representation of the CAN Frame.
func (f Frame) String() string {
	formattedData := make([]string, f.DLC)
	for i := 0; i < int(f.DLC); i++ {
		formattedData[i] = fmt.Sprintf("0x%02X", f.Data[i])
	}
	dataString := strings.Join(formattedData, " ")
	return fmt.Sprintf("ID: 0x%X, DLC: %d, Data: %s", f.ID, f.DLC, dataString)
}