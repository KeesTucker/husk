package uds

import (
	"fmt"
)

// UDS Service ID constants
const (
	ServiceDiagnosticSessionControl       byte = 0x10
	ServiceECUReset                       byte = 0x11
	ServiceClearDiagnosticInformation     byte = 0x14
	ServiceReadDTCInformation             byte = 0x19
	ServiceReadDataByIdentifier           byte = 0x22
	ServiceReadMemoryByAddress            byte = 0x23
	ServiceReadScalingDataByIdentifier    byte = 0x24
	ServiceSecurityAccess                 byte = 0x27
	ServiceCommunicationControl           byte = 0x28
	ServiceWriteDataByIdentifier          byte = 0x2E
	ServiceInputOutputControlByIdentifier byte = 0x2F
	ServiceRoutineControl                 byte = 0x31
	ServiceRequestDownload                byte = 0x34
	ServiceRequestUpload                  byte = 0x35
	ServiceTransferData                   byte = 0x36
	ServiceRequestTransferExit            byte = 0x37
	ServiceTesterPresent                  byte = 0x3E
	ServiceControlDTCSetting              byte = 0x85
)

// Map of UDS service IDs to their names.
var serviceIDNames = map[byte]string{
	ServiceDiagnosticSessionControl:       "Diagnostic Session Control",
	ServiceECUReset:                       "ECU Reset",
	ServiceClearDiagnosticInformation:     "Clear Diagnostic Information",
	ServiceReadDTCInformation:             "Read DTC Information",
	ServiceReadDataByIdentifier:           "Read Data By Identifier",
	ServiceReadMemoryByAddress:            "Read Memory By Address",
	ServiceReadScalingDataByIdentifier:    "Read Scaling Data By Identifier",
	ServiceSecurityAccess:                 "Security Access",
	ServiceCommunicationControl:           "Communication Control",
	ServiceWriteDataByIdentifier:          "Write Data By Identifier",
	ServiceInputOutputControlByIdentifier: "Input Output Control By Identifier",
	ServiceRoutineControl:                 "Routine Control",
	ServiceRequestDownload:                "Request Download",
	ServiceRequestUpload:                  "Request Upload",
	ServiceTransferData:                   "Transfer Data",
	ServiceRequestTransferExit:            "Request Transfer Exit",
	ServiceTesterPresent:                  "Tester Present",
	ServiceControlDTCSetting:              "Control DTC Setting",
	ServiceReadIdKTM16To20:                "Read ECU ID",
	ServiceReadErrorsKTM16To20:            "Read Errors",
	ServiceClearErrorsKTM16To20:           "Clear Errors",
}

func (m *Message) ServiceLabel() string {
	// Lookup the service ID name
	if serviceName, ok := serviceIDNames[m.ServiceID]; ok {
		return serviceName
	}
	return fmt.Sprintf("0x%02X", m.ServiceID)
}
