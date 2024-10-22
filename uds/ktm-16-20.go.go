package uds

// Service IDs

const (
	ServiceReadIdKTM16To20 byte = 0x1A
)

// ReadId Subfunctions

const (
	SubfunctionReadVINKTM16To20           byte = 0x01
	SubfunctionReadECUHardwareIdKTM16To20 byte = 0x02
	SubfunctionReadECUSoftwareIdKTM16To20 byte = 0x05
	SubfunctionReadCountryKTM16To20       byte = 0x06
	SubfunctionReadManufacturerKTM16To20  byte = 0x07
	SubfunctionReadModelKTM16To20         byte = 0x08
)
