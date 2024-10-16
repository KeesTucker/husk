package utils

import (
	"fmt"
	"strconv"
)

// HexStringToBytes converts a hex string to a byte slice
func HexStringToBytes(s string) ([]byte, error) {
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
