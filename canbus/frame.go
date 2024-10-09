package canbus

import "fmt"

const CanIDTransmit = 0x7E0

// Frame represents a CAN bus data frame with an 11-bit identifier
type Frame struct {
	ID   uint16      // CAN identifier
	DLC  uint16      // Data Length Code (0-4095)
	Data [4095]uint8 // Data payload
}

// NewFrame creates a new CAN Frame
func NewFrame(id uint16, dlc uint16, data [4095]uint8) Frame {
	return Frame{
		ID:   id,
		DLC:  dlc,
		Data: data,
	}
}

// String method to provide a human-readable representation of the CAN Frame
func (f Frame) String() string {
	// Ensure DLC is valid before accessing the data slice
	if f.DLC > 4095 {
		f.DLC = 4095
	}
	return fmt.Sprintf("ID: 0x%X, DLC: %d, Data: % X", f.ID, f.DLC, f.Data[:f.DLC])
}
