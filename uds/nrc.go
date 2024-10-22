package uds

import (
	"fmt"
)

// UDS Negative Response Code (NRC) constants
const (
	NRCGeneralReject                             byte = 0x10
	NRCServiceNotSupported                       byte = 0x11
	NRCSubFunctionNotSupported                   byte = 0x12
	NRCIncorrectMessageLengthOrInvalidFormat     byte = 0x13
	NRCResponseTooLong                           byte = 0x14
	NRCBusyRepeatRequest                         byte = 0x21
	NRCConditionsNotCorrect                      byte = 0x22
	NRCRequestSequenceError                      byte = 0x24
	NRCNoResponseFromSubnetComponent             byte = 0x25
	NRCFailurePreventsExecutionOfRequestedAction byte = 0x26
	NRCRequestOutOfRange                         byte = 0x31
	NRCSecurityAccessDenied                      byte = 0x33
	NRCInvalidKey                                byte = 0x35
	NRCExceededNumberOfAttempts                  byte = 0x36
	NRCRequiredTimeDelayNotExpired               byte = 0x37
	NRCUploadDownloadNotAccepted                 byte = 0x70
	NRCTransferDataSuspended                     byte = 0x71
	NRCGeneralProgrammingFailure                 byte = 0x72
	NRCWrongBlockSequenceCounter                 byte = 0x73
	NRCRequestCorrectlyReceivedResponsePending   byte = 0x78
	NRCSubFunctionNotSupportedInActiveSession    byte = 0x7E
	NRCServiceNotSupportedInActiveSession        byte = 0x7F
	NRCVehicleSpeedTooHigh                       byte = 0x81
	NRCRPMTooHigh                                byte = 0x82
	NRCRPMTooLow                                 byte = 0x83
	NRCEngineIsRunning                           byte = 0x84
	NRCEngineIsNotRunning                        byte = 0x85
	NRCEngineRunTimeTooLow                       byte = 0x86
	NRCTemperatureTooHigh                        byte = 0x87
	NRCTemperatureTooLow                         byte = 0x88
	NRCThrottlePedalTooHigh                      byte = 0x89
	NRCThrottlePedalTooLow                       byte = 0x8A
	NRCTransmissionRangeNotInNeutral             byte = 0x8B
	NRCTransmissionRangeNotInGear                byte = 0x8C
	NRCBrakeSwitchNotClosed                      byte = 0x8D
	NRCShifterLeverNotInPark                     byte = 0x8F
	NRCTorqueConverterClutchLocked               byte = 0x90
	NRCVoltageTooHigh                            byte = 0x91
	NRCVoltageTooLow                             byte = 0x92
)

// Map of NRC codes to their names.
var nrcNames = map[byte]string{
	NRCGeneralReject:                             "General Reject",
	NRCServiceNotSupported:                       "Service Not Supported",
	NRCSubFunctionNotSupported:                   "SubFunction Not Supported",
	NRCIncorrectMessageLengthOrInvalidFormat:     "Incorrect Message Length or Invalid Format",
	NRCResponseTooLong:                           "Response Too Long",
	NRCBusyRepeatRequest:                         "Busy Repeat Request",
	NRCConditionsNotCorrect:                      "Conditions Not Correct",
	NRCRequestSequenceError:                      "Request Sequence Error",
	NRCNoResponseFromSubnetComponent:             "No Response From Subnet Component",
	NRCFailurePreventsExecutionOfRequestedAction: "Failure Prevents Execution of Requested Action",
	NRCRequestOutOfRange:                         "Request Out of Range",
	NRCSecurityAccessDenied:                      "Security Access Denied",
	NRCInvalidKey:                                "Invalid Key",
	NRCExceededNumberOfAttempts:                  "Exceeded Number of Attempts",
	NRCRequiredTimeDelayNotExpired:               "Required Time Delay Not Expired",
	NRCUploadDownloadNotAccepted:                 "Upload/Download Not Accepted",
	NRCTransferDataSuspended:                     "Transfer Data Suspended",
	NRCGeneralProgrammingFailure:                 "General Programming Failure",
	NRCWrongBlockSequenceCounter:                 "Wrong Block Sequence Counter",
	NRCRequestCorrectlyReceivedResponsePending:   "Request Correctly Received - Response Pending",
	NRCSubFunctionNotSupportedInActiveSession:    "SubFunction Not Supported in Active Session",
	NRCServiceNotSupportedInActiveSession:        "Service Not Supported in Active Session",
	NRCVehicleSpeedTooHigh:                       "Vehicle Speed Too High",
	NRCRPMTooHigh:                                "RPM Too High",
	NRCRPMTooLow:                                 "RPM Too Low",
	NRCEngineIsRunning:                           "Engine is Running",
	NRCEngineIsNotRunning:                        "Engine is Not Running",
	NRCEngineRunTimeTooLow:                       "Engine Run Time Too Low",
	NRCTemperatureTooHigh:                        "Temperature Too High",
	NRCTemperatureTooLow:                         "Temperature Too Low",
	NRCThrottlePedalTooHigh:                      "Throttle Pedal Too High",
	NRCThrottlePedalTooLow:                       "Throttle Pedal Too Low",
	NRCTransmissionRangeNotInNeutral:             "Transmission Range Not In Neutral",
	NRCTransmissionRangeNotInGear:                "Transmission Range Not In Gear",
	NRCBrakeSwitchNotClosed:                      "Brake Switch Not Closed",
	NRCShifterLeverNotInPark:                     "Shifter Lever Not In Park",
	NRCTorqueConverterClutchLocked:               "Torque Converter Clutch Locked",
	NRCVoltageTooHigh:                            "Voltage Too High",
	NRCVoltageTooLow:                             "Voltage Too Low",
}

func (m *Message) NRCLabel() string {
	if m.NRC == nil {
		return "N/A"
	}
	// Lookup the NRC name
	if nrcName, ok := nrcNames[*m.NRC]; ok {
		return nrcName
	}
	return fmt.Sprintf("0x%02X", *m.NRC)
}
