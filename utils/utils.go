package utils

import (
	"fmt"
	"strconv"
)

func HexStringToByteArray(in string) ([]byte, error) {
	// Ensure the string has an even length
	if len(in)%2 != 0 {
		return nil, fmt.Errorf("hex string has an odd length: %v", in)
	}

	// Pre-allocate the byte slice with the exact size
	data := make([]byte, len(in)/2)

	// Loop through the hex string, converting each pair of characters to a byte
	for i := 0; i < len(in); i += 2 {
		byteVal, err := strconv.ParseUint(in[i:i+2], 16, 8)
		if err != nil {
			return nil, fmt.Errorf("parsing hex byte at position %d: %v", i, err)
		}
		data[i/2] = byte(byteVal) // Place the parsed byte in the correct position
	}

	return data, nil
}
