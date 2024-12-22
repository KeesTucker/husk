package uds

import (
	"fmt"
)

// UDS Subfunction constants for Diagnostic Session Control
const (
	SubfunctionDefaultSession                byte = 0x01
	SubfunctionProgrammingSession            byte = 0x02
	SubfunctionExtendedDiagnosticSession     byte = 0x03
	SubfunctionSafetySystemDiagnosticSession byte = 0x04
)

// UDS Subfunction constants for ECU Reset
const (
	SubfunctionHardReset     byte = 0x01
	SubfunctionKeyOffOnReset byte = 0x02
	SubfunctionSoftReset     byte = 0x03
)

// UDS Subfunction constants for Security Access
const (
	SubfunctionRequestSeed byte = 0x01
	SubfunctionSendKey     byte = 0x02
	// You can extend the security access subfunctions based on levels, for example:
	// byte = 0x03, 0x05, etc. for other levels of access
)

// UDS Subfunction constants for Routine Control
const (
	SubfunctionStartRoutine          byte = 0x01
	SubfunctionStopRoutine           byte = 0x02
	SubfunctionRequestRoutineResults byte = 0x03
)

// UDS Subfunction constants for Communication Control
const (
	SubfunctionEnableRxAndTx        byte = 0x00
	SubfunctionEnableRxAndDisableTx byte = 0x01
	SubfunctionDisableRxAndEnableTx byte = 0x02
	SubfunctionDisableRxAndTx       byte = 0x03
)

// UDS Subfunction constants for Control DTC Setting
const (
	SubfunctionDTCSettingOn  byte = 0x01
	SubfunctionDTCSettingOff byte = 0x02
)

// Map of UDS subfunctions (for specific service IDs) to their names.
var subfunctionNames = map[byte]map[byte]string{
	ServiceDiagnosticSessionControl: {
		SubfunctionDefaultSession:                "Default Session",
		SubfunctionProgrammingSession:            "Programming Session",
		SubfunctionExtendedDiagnosticSession:     "Extended Diagnostic Session",
		SubfunctionSafetySystemDiagnosticSession: "Safety System Diagnostic Session",
	},
	ServiceECUReset: {
		SubfunctionHardReset:     "Hard Reset",
		SubfunctionKeyOffOnReset: "Key Off On Reset",
		SubfunctionSoftReset:     "Soft Reset",
	},
	ServiceSecurityAccess: {
		SubfunctionRequestSeed: "Request Seed",
		SubfunctionSendKey:     "Send Key",
	},
	ServiceRoutineControl: {
		SubfunctionStartRoutine:          "Start Routine",
		SubfunctionStopRoutine:           "Stop Routine",
		SubfunctionRequestRoutineResults: "Request Routine Results",
	},
	ServiceCommunicationControl: {
		SubfunctionEnableRxAndTx:        "Enable Rx and Tx",
		SubfunctionEnableRxAndDisableTx: "Enable Rx and Disable Tx",
		SubfunctionDisableRxAndEnableTx: "Disable Rx and Enable Tx",
		SubfunctionDisableRxAndTx:       "Disable Rx and Tx",
	},
	ServiceControlDTCSetting: {
		SubfunctionDTCSettingOn:  "DTC Setting On",
		SubfunctionDTCSettingOff: "DTC Setting Off",
	},
	// K01
	ServiceReadIdK01: {
		SubfunctionReadVINK01:           "Read VIN",
		SubfunctionReadECUHardwareIdK01: "Read ECU Hardware Id",
		SubfunctionReadECUSoftwareIdK01: "Read ECU Software Id",
		SubfunctionReadCountryK01:       "Read Manufacturer Country Code",
		SubfunctionReadManufacturerK01:  "Read Brand",
		SubfunctionReadModelK01:         "Read Model",
	},
}

func (m *Message) SubfunctionLabel() string {
	if m.Subfunction == nil {
		return "N/A"
	}
	if subMap, exists := subfunctionNames[m.ServiceID]; exists {
		if subName, found := subMap[*m.Subfunction]; found {
			return subName
		}
	}
	return fmt.Sprintf("0x%02X", *m.Subfunction)
}
