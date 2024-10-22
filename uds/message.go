package uds

import (
	"context"
	"fmt"
	"strings"
	"unicode"
)

// Message represents a full UDS message with an optional Subfunction and NRC.
type Message struct {
	SenderID    uint16 // The ID of the sender (CAN ID)
	ServiceID   byte   // The UDS ServiceID (deduced from the byte array)
	Subfunction *byte  // Optional Subfunction (nil if not applicable)
	NRC         *byte  // Optional Negative Response Code (NRC) for negative responses
	Data        []byte // The full byte array of data (excluding ServiceID, Subfunction, and NRC)
	IsSuccess   *bool  // Indicates if the message was successful
}

// RawDataToMessage creates a new UDSMessage instance by deducing the service ID, subfunction, and NRC from a byte array.
func RawDataToMessage(senderID uint16, rawData []byte) *Message {
	if len(rawData) == 0 {
		return nil // Handle cases where the data is empty
	}

	serviceId := rawData[0] - PositiveResponseServiceIdOffset
	isSuccess := rawData[0] != NegativeResponseByte
	var subfunction *byte // Optional subfunction, usually the second byte
	var nrc *byte         // Optional NRC for negative responses
	data := rawData[1:]   // The remaining data, excluding the service ID

	// If it's a negative response, adjust the service ID and extract the NRC
	if !isSuccess && len(data) > 1 {
		serviceId = data[0]
		nrc = &data[1]  // The third byte is the NRC
		data = data[2:] // Remove the service ID and NRC from the data array
	} else if len(data) > 0 {
		// If the message includes a subfunction, extract it
		subfunction = &data[0]
		data = data[1:] // Remove the subfunction from the data array
	}

	return &Message{
		SenderID:    senderID,
		ServiceID:   serviceId,
		Subfunction: subfunction,
		NRC:         nrc,
		Data:        data,
		IsSuccess:   &isSuccess,
	}
}

func (m *Message) ToRawData() []byte {
	var rawData []byte
	if m.IsSuccess == nil {
		// Treat this as an outgoing request since IsSuccess is nil
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

	if *m.IsSuccess {
		// Positive Response Handling
		// The first byte is the original Service ID plus the positive response offset (0x40).
		positiveServiceID := m.ServiceID + PositiveResponseServiceIdOffset
		rawData = append(rawData, positiveServiceID)

		// Include Subfunction if present.
		if m.Subfunction != nil {
			rawData = append(rawData, *m.Subfunction)
		}

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

func (m *Message) String() string {
	dataStr := ""
	for i := 0; i < len(m.Data); i++ {
		dataStr += fmt.Sprintf("0x%02X ", m.Data[i])
	}
	if m.IsSuccess == nil {
		return fmt.Sprintf("Request from: %s Service: %s Subfunction: %s ASCII: %s Data: %s", m.SenderLabel(), m.ServiceLabel(), m.SubfunctionLabel(), m.ASCIIRepresentation(), dataStr)
	}
	if *m.IsSuccess {
		return fmt.Sprintf("Response from: %s (+) Service: %s Subfunction: %s ASCII: %s Data: %s", m.SenderLabel(), m.ServiceLabel(), m.SubfunctionLabel(), m.ASCIIRepresentation(), dataStr)
	}
	return fmt.Sprintf("Response from: %s (-) Service: %s NRC: %s", m.SenderLabel(), m.ServiceLabel(), m.NRCLabel())
}

// ASCIIRepresentation returns the alphanumeric ASCII string representation of the message data.
func (m *Message) ASCIIRepresentation() string {
	var asciiStrings []string
	for _, b := range m.Data {
		char := rune(b)
		if unicode.IsLetter(char) || unicode.IsDigit(char) { // Only keep alphanumeric characters
			asciiStrings = append(asciiStrings, fmt.Sprintf("%c", b))
		}
	}
	return strings.Join(asciiStrings, "")
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
