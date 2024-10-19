package protocols

import (
	"context"
	"errors"
	"time"

	"husk/canbus"
	"husk/drivers"
	"husk/logging"
	"husk/services"
)

const frameWaitTimeout = 1000 * time.Millisecond

const (
	UDSPCIFrameTypeSF byte = 0x0
	UDSPCIFrameTypeFF byte = 0x1
	UDSPCIFrameTypeCF byte = 0x2
	UDSPCIFrameTypeFC byte = 0x3
)

var (
	errorFCFrameTimeout        = errors.New("timeout while waiting for flow control frame from ecu for multi frame send")
	errorMultiFrameReadTimeout = errors.New("timeout while waiting for consecutive frames from ecu")
	errorUnexpectedFrameIndex  = errors.New("unexpected frame index")
)

// TODO: potentially add a UDSFrame which has id, dlc, data length, pci, frame type, payload (further broken down into: service id, subfunction, params, data, is negative response)?

func SendUDSTesterPresent(ctx context.Context, id uint16, ecuId uint16) error {
	err := SendUDS(ctx, id, ecuId, []byte{0x3E})
	if err != nil {
		return err
	}
	return nil
}

// SendUDS is what you should use when writing high level comms with the ECU. SendUDS Sends a byte array using the UDS protocol to the ECU
func SendUDS(ctx context.Context, testerId uint16, ecuId uint16, data []byte) error {
	dataLength := uint16(len(data))

	// Single frame message
	if dataLength <= 7 {
		err := sendSingleFrame(ctx, testerId, dataLength, data)
		if err != nil {
			return err
		}
		return nil
	}
	// Multi frame message
	// Send First Frame (FF)
	err := sendFirstFrame(ctx, testerId, dataLength, data)
	if err != nil {
		return err
	}
	// Wait for Flow Control Frame from ECU (FC)
	separationTime, err := waitForFlowControlFrame(ctx, ecuId)
	if err != nil {
		return err
	}
	// Wait for separation time from FC frame
	sleepForSeparationTime(separationTime)
	// Send the consecutive frames
	err = sendConsecutiveFrames(ctx, testerId, data, separationTime)
	if err != nil {
		return err
	}
	return nil
}

func sendSingleFrame(ctx context.Context, id uint16, dataLength uint16, data []byte) error {
	d := services.Get(services.ServiceDriver).(drivers.Driver)
	frame := &canbus.CanFrame{ID: id}
	frame.DLC = byte(dataLength) + 1

	// Set PCI. Upper nibble is 0x0 (Single Frame) and lower nibble is length
	frame.Data[0] = UDSPCIFrameTypeSF | byte(dataLength&0x0F)
	// Set the actual data bytes
	copy(frame.Data[1:], data)

	err := d.SendFrame(ctx, frame)
	if err != nil {
		return err
	}
	return nil
}

func sendFirstFrame(ctx context.Context, id uint16, dataLength uint16, data []byte) error {
	d := services.Get(services.ServiceDriver).(drivers.Driver)

	frame := &canbus.CanFrame{ID: id, DLC: 8}
	// Set PCI. Upper nibble is 0x1 (First Frame) and lower nibble is the upper 4 bits of the data length
	frame.Data[0] = UDSPCIFrameTypeFF | byte((dataLength>>8)&0x0F)
	// Send second byte holds the remaining 8 bits of the 12 bit data length
	frame.Data[1] = byte(dataLength & 0xFF)
	// Copy in the first 6 data bytes
	copy(frame.Data[2:], data[:6])
	return d.SendFrame(ctx, frame)
}

func waitForFlowControlFrame(ctx context.Context, ecuId uint16) (separationTime byte, err error) {
	d := services.Get(services.ServiceDriver).(drivers.Driver)
	readCtx, cancel := context.WithTimeout(ctx, frameWaitTimeout)
	defer cancel()
	frameChan := d.SubscribeToReadFrames()
	defer d.UnsubscribeToReadFrames(frameChan)

	for {
		select {
		case frame := <-frameChan:
			if frame.ID != ecuId {
				continue
			}
			pciFrameType := (frame.Data[0] & 0xF0) >> 4
			if pciFrameType != UDSPCIFrameTypeFC {
				continue
			}
			// flowStatus := frame.Data[0] & 0x0F
			// blockSize := frame.Data[1]
			separationTime = frame.Data[2]
			return separationTime, nil
		case <-readCtx.Done():
			return 0, errorFCFrameTimeout
		}
	}
}

func sleepForSeparationTime(separationTime byte) {
	l := services.Get(services.ServiceLogger).(*logging.Logger)

	if separationTime <= 0x7F {
		// Separation time is in milliseconds (values 0x00 to 0x7F)
		time.Sleep(time.Duration(separationTime) * time.Millisecond)
	} else if separationTime >= 0xF1 && separationTime <= 0xF9 {
		// Separation time is in microseconds (values 0xF1 to 0xF9)
		microseconds := 100 * (int(separationTime) - 0xF0)
		time.Sleep(time.Duration(microseconds) * time.Microsecond)
	} else {
		l.WriteToLog("invalid separation time received, sleeping for 10ms")
		time.Sleep(10 * time.Millisecond)
	}
}

func sendConsecutiveFrames(ctx context.Context, id uint16, data []byte, separationTime byte) error {
	d := services.Get(services.ServiceDriver).(drivers.Driver)

	frameIndex := byte(1) // Consecutive Frame index starts at 1
	chunkSize := 7        // Consecutive frames carry 7 bytes of data
	bytesSent := 6        // Start from the 7th byte (as first 6 bytes were sent in the first frame)
	totalBytes := len(data)

	for bytesSent < totalBytes {
		frame := &canbus.CanFrame{ID: id}

		// Set PCI. Upper nibble is 0x2 (Consecutive Frame) and lower nibble is the frame index (mod 16)
		frame.Data[0] = (UDSPCIFrameTypeCF << 4) | (frameIndex & 0x0F)

		// Determine the number of bytes to send in this frame
		bytesToSend := totalBytes - bytesSent
		if bytesToSend > chunkSize {
			bytesToSend = chunkSize
		}

		// Copy the data chunk into the frame
		copy(frame.Data[1:], data[bytesSent:bytesSent+bytesToSend])

		// Set the correct DLC (PCI byte + actual data bytes)
		frame.DLC = byte(1 + bytesToSend)

		// Send the frame
		err := d.SendFrame(ctx, frame)
		if err != nil {
			return err
		}

		// Update bytesSent and frameIndex
		bytesSent += bytesToSend
		frameIndex = (frameIndex + 1) % 16 // Sequence number cycles from 0 to 15

		// Sleep for the separation time between consecutive frames
		sleepForSeparationTime(separationTime)
	}

	return nil
}

// ReadUDS is what you should use when writing high level comms with the ECU. ReadUDS blocks until a frame is received and then returns the complete byte array from the frame/s
func ReadUDS(ctx context.Context, ecuId uint16) ([]byte, error) {
	d := services.Get(services.ServiceDriver).(drivers.Driver)
	frameChan := d.SubscribeToReadFrames()
	defer d.UnsubscribeToReadFrames(frameChan)

	for {
		select {
		case frame := <-frameChan:
			if frame.ID != ecuId {
				continue
			}
			pciFrameType := (frame.Data[0] & 0xF0) >> 4
			switch pciFrameType {
			case UDSPCIFrameTypeSF:
				// Handle single frame reception
				return receiveSingleFrame(frame)
			case UDSPCIFrameTypeFF:
				// Handle multi-frame reception
				return receiveMultiFrame(ctx, frameChan, frame)
			default:
				// Ignore frames that don't match expected types
				continue
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func receiveSingleFrame(frame *canbus.CanFrame) ([]byte, error) {
	// Extract data length from the lower nibble of the PCI byte
	dataLength := frame.Data[0] & 0x0F
	// Copy the data into a byte slice
	data := make([]byte, dataLength)
	copy(data, frame.Data[1:dataLength+1])
	return data, nil
}

func receiveMultiFrame(ctx context.Context, frameChan <-chan *canbus.CanFrame, firstFrame *canbus.CanFrame) ([]byte, error) {
	readCtx, cancel := context.WithTimeout(ctx, frameWaitTimeout)
	defer cancel()

	// Extract data length from the first two bytes of the first frame
	dataLength := (uint16(firstFrame.Data[0]&0x0F) << 8) | uint16(firstFrame.Data[1])

	// Allocate a buffer to hold the entire message
	data := make([]byte, dataLength)

	// Copy the first 6 bytes of data from the first frame
	copy(data, firstFrame.Data[2:8])

	bytesReceived := 6
	frameIndex := byte(1)

	for bytesReceived < int(dataLength) {
		select {
		case frame := <-frameChan:
			if frame.ID != firstFrame.ID {
				continue
			}
			pciFrameType := (frame.Data[0] & 0xF0) >> 4
			if pciFrameType != UDSPCIFrameTypeCF {
				// We are expecting consecutive frames; ignore any other frames
				continue
			}

			// Check the sequence number
			seqNum := frame.Data[0] & 0x0F
			if seqNum != frameIndex {
				return nil, errorUnexpectedFrameIndex
			}

			// Determine how many bytes to copy
			bytesToCopy := int(dataLength) - bytesReceived
			if bytesToCopy > 7 {
				bytesToCopy = 7
			}

			// Copy the data from the frame
			copy(data[bytesReceived:], frame.Data[1:bytesToCopy+1])
			bytesReceived += bytesToCopy
			frameIndex = (frameIndex + 1) % 16
		case <-readCtx.Done():
			return nil, errorMultiFrameReadTimeout
		}
	}
	return data, nil
}
