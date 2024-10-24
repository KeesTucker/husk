package uds

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"husk/logging"
	"husk/services"
)

// Message represents a full UDS message with an optional Subfunction and NRC.
type Message struct {
	// SenderID is the CAN ID of the sender
	SenderID uint16
	// ServiceID is the UDS ServiceID
	ServiceID byte
	// Subfunction is optional UDS Subfunction
	Subfunction *byte
	// NRC is the Negative Response Code (NRC) for negative responses
	NRC *byte
	// Data is the full byte array of data (excluding ServiceID, and NRC)
	Data []byte
	// IsResponse indicates if the message was a response or a request
	IsResponse bool
	// IsPositive indicates if the message was successful
	IsPositive *bool
}

// RawDataToMessage creates a new UDSMessage instance by deducing the service ID, subfunction, and NRC from a byte array.
func RawDataToMessage(senderID uint16, rawData []byte, isResponse bool) *Message {
	if len(rawData) == 0 {
		return nil // Handle cases where the data is empty
	}

	var isPositive bool   // Was response positive?
	var serviceId byte    // Service id of uds message
	var subfunction *byte // Optional subfunction, usually the second byte
	var nrc *byte         // Optional NRC for negative responses

	var data []byte
	if isResponse {
		// This is a response
		// Was response positive or negative?
		isPositive = rawData[0] != NegativeResponseByte

		if isPositive {
			serviceId = rawData[0] - PositiveResponseServiceIdOffset // Subtract positive response offset from first byte to get service id
			if len(rawData) > 1 {
				subfunction = &rawData[1] // The subfunction is the second byte of response
			}
			data = rawData[1:] // Strip service id and store in data slice
		} else {
			serviceId = rawData[1] // The service id is the second byte if response is negative
			nrc = &rawData[2]      // The third byte is the NRC
			data = rawData[3:]     // Strip negative response byte, service id and nrc and store in data slice
		}
	} else {
		// This is a request
		serviceId = rawData[0] // Service id is the first byte of request
		if len(rawData) > 1 {
			subfunction = &rawData[1] // Subfunction is the second byte of request
		}
		data = rawData[1:] // Strip service id and store in data slice
	}
	return &Message{
		SenderID:    senderID,
		ServiceID:   serviceId,
		Subfunction: subfunction,
		NRC:         nrc,
		Data:        data,
		IsPositive:  &isPositive,
		IsResponse:  isResponse,
	}
}

func (m *Message) ToRawData() []byte {
	var rawData []byte
	if !m.IsResponse {
		// Outgoing Request Handling
		rawData = append(rawData, m.ServiceID)
		// Include Subfunction if present.
		if m.Subfunction != nil {
			rawData = append(rawData, *m.Subfunction)
		}
		// Append the remaining data.
		rawData = append(rawData, m.Data...)
		return rawData
	}
	if *m.IsPositive {
		// Positive Response Handling
		// The first byte is the original Service ID plus the positive response offset (0x40).
		positiveServiceID := m.ServiceID + PositiveResponseServiceIdOffset
		rawData = append(rawData, positiveServiceID)
		// Append the remaining data.
		rawData = append(rawData, m.Data...)
		return rawData
	}
	// Negative Response Handling
	// The first byte is the Negative Response Service ID (0x7F).
	rawData = append(rawData, NegativeResponseByte)
	// The second byte is the original Service ID that caused the negative response.
	rawData = append(rawData, m.ServiceID)
	// The third byte is the Negative Response Code (NRC). This should always be present. Could be worth throwing an error here in future if it is nil
	if m.NRC != nil {
		rawData = append(rawData, *m.NRC)
	}
	// Append the remaining data, if any.
	rawData = append(rawData, m.Data...)
	return rawData
}

func (m *Message) SenderLabel() string {
	switch m.SenderID {
	case ECUID:
		return "ECU"
	case TesterID:
		return "Tester"
	default:
		return fmt.Sprintf("0x%03X", m.SenderID)
	}
}

func (m *Message) Send(ctx context.Context) error {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	if m.ServiceID != ServiceTesterPresent {
		l.WriteMessage("UDS: "+m.String(), logging.MessageTypeUDSWrite)
	}

	rawData := m.ToRawData()
	dataLength := uint16(len(rawData))
	// Single frame message
	if dataLength <= 7 {
		err := sendSingleFrame(ctx, m.SenderID, dataLength, rawData)
		if err != nil {
			return err
		}
		return nil
	}
	// Multi frame message
	// Send First Frame (FF)
	err := sendFirstFrame(ctx, m.SenderID, dataLength, rawData)
	if err != nil {
		return err
	}
	// Wait for Flow Control Frame from ECU (FC)
	separationTime, err := waitForFlowControlFrame(ctx)
	if err != nil {
		return err
	}
	// Wait for separation time from FC frame
	sleepForSeparationTime(separationTime)
	// Send the consecutive frames
	err = sendConsecutiveFrames(ctx, m.SenderID, rawData, separationTime)
	if err != nil {
		return err
	}
	return nil
}

func (m *Message) String() string {
	dataStr := ""
	for i := 0; i < len(m.Data); i++ {
		dataStr += fmt.Sprintf("0x%02X ", m.Data[i])
	}
	if dataStr == "" {
		dataStr = "N/A"
	}
	if !m.IsResponse {
		return fmt.Sprintf("Request:\nId: %s\nService: %s\nSubfunction: %s\nASCII: %s\nData: %s", m.SenderLabel(), m.ServiceLabel(), m.SubfunctionLabel(), m.ASCIIRepresentation(), dataStr)
	}
	if *m.IsPositive {
		return fmt.Sprintf("Response:\nId: %s\nService: %s\nSubfunction: %s\nASCII: %s\nData: %s", m.SenderLabel(), m.ServiceLabel(), m.SubfunctionLabel(), m.ASCIIRepresentation(), dataStr)
	}
	return fmt.Sprintf("NEGATIVE Response:\nId: %s\nService: %s\nNRC: %s", m.SenderLabel(), m.ServiceLabel(), m.NRCLabel())
}

// ASCIIRepresentation returns the alphanumeric ASCII string representation of the message data.
func (m *Message) ASCIIRepresentation() string {
	var asciiStrings []string
	for _, b := range m.Data {
		char := rune(b)
		if unicode.IsPrint(char) { // Only keep alphanumeric characters
			asciiStrings = append(asciiStrings, fmt.Sprintf("%c", b))
		}
	}
	result := strings.Join(asciiStrings, "")
	if result == "" {
		return "N/A"
	}
	return result
}
