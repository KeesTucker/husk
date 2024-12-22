package uds

// Service IDs

const (
	ServiceReadIdK01      byte = 0x1A
	ServiceReadErrorsK01  byte = 0x03
	ServiceClearErrorsK01 byte = 0x04
)

// ReadId Subfunctions

const (
	SubfunctionReadVINK01           byte = 0x01
	SubfunctionReadECUHardwareIdK01 byte = 0x02
	SubfunctionReadECUSoftwareIdK01 byte = 0x05
	SubfunctionReadCountryK01       byte = 0x06
	SubfunctionReadManufacturerK01  byte = 0x07
	SubfunctionReadModelK01         byte = 0x08
)
