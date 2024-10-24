package canbus

import (
	"fmt"
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
	return fmt.Sprintf("ID: 0x%X\nDLC: %d\nData: %s", f.ID, f.DLC, dataString)
}
