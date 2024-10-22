package uds

// Service IDs

const (
	ServiceReadIdKTM16To20 byte = 0x1A
)

// ReadId Subfunctions

const (
	SubfunctionReadVINKTM16To20          byte = 0x01
	SubfunctionReadVersionKTM16To20      byte = 0x02
	SubfunctionReadUnknownID1KTM16To20   byte = 0x03
	SubfunctionReadUnknownID2KTM16To20   byte = 0x04
	SubfunctionReadEngineNumberKTM16To20 byte = 0x05
	SubfunctionReadCountryKTM16To20      byte = 0x06
	SubfunctionReadBrandKTM16To20        byte = 0x07
	SubfunctionReadModelKTM16To20        byte = 0x08
)
