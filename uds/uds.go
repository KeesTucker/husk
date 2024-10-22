package uds

import (
	"context"
	"errors"
	"fmt"
	"time"

	"husk/canbus"
	"husk/drivers"
	"husk/logging"
	"husk/services"
)

const frameWaitTimeout = 10 * time.Second

const (
	PCIFrameTypeSF byte = 0x0
	PCIFrameTypeFF byte = 0x1
	PCIFrameTypeCF byte = 0x2
	PCIFrameTypeFC byte = 0x3
)

const (
	TesterID uint16 = 0x7E0
	ECUID    uint16 = 0x7E8
)

const (
	NegativeResponseByte            byte = 0x7F
	PositiveResponseServiceIdOffset byte = 0x40
)

var (
	errorFCFrameTimeout        = errors.New("timeout while waiting for flow control frame from ecu for multi frame send")
	errorMultiFrameReadTimeout = errors.New("timeout while waiting for consecutive frames from ecu")
	errorUnexpectedFrameIndex  = errors.New("unexpected frame index")
)

func SendTesterPresent(ctx context.Context) error {
	message := &Message{
		SenderID:  TesterID,
		ServiceID: ServiceTesterPresent,
	}

	err := message.Send(ctx)
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
	frame.Data[0] = PCIFrameTypeSF | byte(dataLength&0x0F)
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
	frame.Data[0] = PCIFrameTypeFF | byte((dataLength>>8)&0x0F)
	// Send second byte holds the remaining 8 bits of the 12 bit data length
	frame.Data[1] = byte(dataLength & 0xFF)
	// Copy in the first 6 data bytes
	copy(frame.Data[2:], data[:6])
	return d.SendFrame(ctx, frame)
}

func waitForFlowControlFrame(ctx context.Context) (separationTime byte, err error) {
	d := services.Get(services.ServiceDriver).(drivers.Driver)
	readCtx, cancel := context.WithTimeout(ctx, frameWaitTimeout)
	defer cancel()
	frameChan := d.SubscribeReadFrames()
	defer d.UnsubscribeReadFrames(frameChan)

	for {
		select {
		case frame := <-frameChan:
			pciFrameType := (frame.Data[0] & 0xF0) >> 4
			if pciFrameType != PCIFrameTypeFC {
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
		l.WriteToLog("Invalid separation time received, setting separation time to 10 milliseconds")
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
		frame.Data[0] = (PCIFrameTypeCF << 4) | (frameIndex & 0x0F)

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

func Read(ctx context.Context) (*Message, error) {
	d := services.Get(services.ServiceDriver).(drivers.Driver)
	frameChan := d.SubscribeReadFrames()
	defer d.UnsubscribeReadFrames(frameChan)

	for {
		select {
		case frame := <-frameChan:
			pciFrameType := (frame.Data[0] & 0xF0) >> 4
			switch pciFrameType {
			case PCIFrameTypeSF:
				// Handle single frame reception
				rawData, err := receiveSingleFrame(frame)
				if err != nil {
					return nil, err
				}
				return RawDataToMessage(frame.ID, rawData), nil
			case PCIFrameTypeFF:
				rawData, err := receiveMultiFrame(ctx, frame)
				if err != nil {
					return nil, err
				}
				return RawDataToMessage(frame.ID, rawData), nil
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

func receiveMultiFrame(ctx context.Context, firstFrame *canbus.CanFrame) ([]byte, error) {
	d := services.Get(services.ServiceDriver).(drivers.Driver)

	frameChan := d.SubscribeReadFrames()
	defer d.UnsubscribeReadFrames(frameChan)

	// Extract data length from the first two bytes of the first frame
	dataLength := (uint16(firstFrame.Data[0]&0x0F) << 8) | uint16(firstFrame.Data[1])

	// Allocate a buffer to hold the entire message
	data := make([]byte, dataLength)

	// Copy the first 6 bytes of data from the first frame
	copy(data, firstFrame.Data[2:8])

	bytesReceived := 6
	frameIndex := byte(1)

	// Send Flow Control Frame before proceeding
	err := sendFlowControlFrame()
	if err != nil {
		return nil, fmt.Errorf("failed to send flow control frame: %v", err)
	}
	fmt.Println("sending flow control frame")

	var cancel context.CancelFunc
	for bytesReceived < int(dataLength) {
		var readCtx context.Context
		readCtx, cancel = context.WithTimeout(ctx, frameWaitTimeout)
		select {
		case frame := <-frameChan:
			if frame.ID != firstFrame.ID {
				continue
			}
			pciFrameType := (frame.Data[0] & 0xF0) >> 4
			if pciFrameType != PCIFrameTypeCF {
				// We are expecting consecutive frames; ignore any other frames
				continue
			}

			// Check the sequence number
			seqNum := frame.Data[0] & 0x0F
			if seqNum != frameIndex {
				cancel()
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
			cancel()
		case <-readCtx.Done():
			cancel()
			return nil, errorMultiFrameReadTimeout
		}
	}
	cancel()
	return data, nil
}

func sendFlowControlFrame() error {
	d := services.Get(services.ServiceDriver).(drivers.Driver)

	// Construct the FC frame data
	fcFrameData := [8]byte{}
	fcFrameData[0] = (PCIFrameTypeFC << 4) | 0x00 // Flow Status: Continue to send (CTS)
	fcFrameData[1] = 0x00                         // Block Size (BS): 0 means sender can send all CFs without waiting for further FCs
	fcFrameData[2] = 0x00                         // Separation Time (STmin): 0 means minimum separation time

	// Create the CAN frame
	fcFrame := &canbus.CanFrame{
		ID:   TesterID,
		DLC:  3,
		Data: fcFrameData,
	}

	// Send the FC frame using your CAN bus interface
	err := d.SendFrame(context.Background(), fcFrame)
	if err != nil {
		return err
	}
	return nil
}
