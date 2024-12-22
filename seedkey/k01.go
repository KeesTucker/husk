package seedkey

import (
	"errors"
)

// GenerateK01Key generates a 2 byte K01 key given a 2 byte seed and a level
func GenerateK01Key(seed [2]byte, level SecurityLevel) ([2]byte, error) {
	var magicNumber uint16

	// Select magic number based on security level
	switch level {
	case SecurityLevel1:
		return [2]byte{}, errors.New("missing magic number for Level 1")
	case SecurityLevel2:
		magicNumber = 0x4D4E
	case SecurityLevel3:
		magicNumber = 0x6F31
	default:
		return [2]byte{}, errors.New("invalid level in generateSeedKey")
	}

	// Combine seed bytes into a single 16-bit value
	x := (uint16(seed[0]) << 8) | uint16(seed[1])

	// Calculate the key
	key := (magicNumber * x) & 0xFFFF

	// Split key into two bytes
	keyBytes := [2]byte{
		byte((key >> 8) & 0xFF),
		byte(key & 0xFF),
	}

	return keyBytes, nil
}
