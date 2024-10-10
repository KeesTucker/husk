// arduino_driver_test.go
package drivers

import (
	"io"
	"sync"
	"testing"
	"time"

	"go.bug.st/serial"
	"husk/canbus"
)

// MockSerialPort is a mock implementation of serial.Port
type MockSerialPort struct {
	readBuf     []byte
	writeBuf    []byte
	readMutex   sync.Mutex
	writeMutex  sync.Mutex
	readIndex   int
	readTimeout time.Duration
	closed      bool
}

func (m *MockSerialPort) Read(p []byte) (n int, err error) {
	m.readMutex.Lock()
	defer m.readMutex.Unlock()

	if m.closed {
		return 0, io.EOF
	}

	if m.readIndex >= len(m.readBuf) {
		// Simulate read timeout
		if m.readTimeout > 0 {
			time.Sleep(m.readTimeout)
		}
		return 0, nil
	}

	n = copy(p, m.readBuf[m.readIndex:])
	m.readIndex += n
	return n, nil
}

func (m *MockSerialPort) Write(p []byte) (n int, err error) {
	m.writeMutex.Lock()
	defer m.writeMutex.Unlock()

	if m.closed {
		return 0, io.EOF
	}

	m.writeBuf = append(m.writeBuf, p...)
	return len(p), nil
}

func (m *MockSerialPort) ResetInputBuffer() error {
	m.readMutex.Lock()
	defer m.readMutex.Unlock()

	m.readBuf = nil
	m.readIndex = 0
	return nil
}

func (m *MockSerialPort) ResetOutputBuffer() error {
	m.writeMutex.Lock()
	defer m.writeMutex.Unlock()

	m.writeBuf = nil
	return nil
}

func (m *MockSerialPort) GetWrittenData() []byte {
	m.writeMutex.Lock()
	defer m.writeMutex.Unlock()

	return m.writeBuf
}

func (m *MockSerialPort) FeedReadData(data []byte) {
	m.readMutex.Lock()
	defer m.readMutex.Unlock()

	m.readBuf = append(m.readBuf, data...)
}

func (m *MockSerialPort) GetReadPosition() int {
	m.readMutex.Lock()
	defer m.readMutex.Unlock()

	return m.readIndex
}

func (m *MockSerialPort) IsClosed() bool {
	return m.closed
}

func (m *MockSerialPort) Close() error {
	m.closed = true
	return nil
}

func (m *MockSerialPort) SetMode(_ *serial.Mode) error {
	return nil
}

func (m *MockSerialPort) SetReadTimeout(t time.Duration) error {
	m.readTimeout = t
	return nil
}

func (m *MockSerialPort) Drain() error {
	return nil
}

func (m *MockSerialPort) SetDTR(_ bool) error {
	return nil
}

func (m *MockSerialPort) SetRTS(_ bool) error {
	return nil
}

func (m *MockSerialPort) GetModemStatusBits() (*serial.ModemStatusBits, error) {
	return nil, nil
}

func (m *MockSerialPort) Break(_ time.Duration) error {
	return nil
}

// Test for SendCanBusFrame with normal data
func TestSendCanBusFrame(t *testing.T) {
	// Create a mock serial port
	mockPort := &MockSerialPort{}

	// Create an ArduinoDriver with the mock port
	driver := &ArduinoDriver{
		port: mockPort,
	}

	// Prepare a frame to send
	frame := canbus.Frame{
		ID:   0x123,
		DLC:  3,
		Data: [8]byte{0x01, 0x02, 0x03},
	}

	// Prepare the mock port to respond with ACK
	mockPort.FeedReadData([]byte{ACK})

	// Call SendCanBusFrame
	err := driver.SendCanBusFrame(frame)
	if err != nil {
		t.Errorf("SendCanBusFrame returned error: %v", err)
	}

	// Verify that the correct bytes were written to the port
	writtenData := mockPort.GetWrittenData()

	// Since createFrameBytes is private, we can call it directly
	expectedData := driver.createFrameBytes(frame)

	if len(writtenData) != len(expectedData) {
		t.Errorf("Written data length mismatch. Expected %d, got %d", len(expectedData), len(writtenData))
	}

	for i := range expectedData {
		if writtenData[i] != expectedData[i] {
			t.Errorf("Written data mismatch at index %d. Expected 0x%X, got 0x%X", i, expectedData[i], writtenData[i])
		}
	}
}

// Test for ReadCanBusFrame with normal data
func TestReadCanBusFrame(t *testing.T) {
	// Create a mock serial port
	mockPort := &MockSerialPort{}

	// Create an ArduinoDriver with the mock port
	driver := &ArduinoDriver{
		port: mockPort,
	}

	// Prepare a frame to read
	frame := canbus.Frame{
		ID:   0x123,
		DLC:  3,
		Data: [8]byte{0x01, 0x02, 0x03},
	}

	// Create the frame bytes as they would come over the serial port
	frameBytes := driver.createFrameBytes(frame)

	// Prepare the mock port with the frame bytes
	mockPort.FeedReadData(frameBytes)

	// Call ReadCanBusFrame
	receivedFrame, err := driver.ReadCanBusFrame()
	if err != nil {
		t.Errorf("ReadCanBusFrame returned error: %v", err)
	}

	if receivedFrame == nil {
		t.Errorf("ReadCanBusFrame returned nil frame")
	}

	// Verify the received frame matches the original frame
	if receivedFrame.ID != frame.ID {
		t.Errorf("Frame ID mismatch. Expected 0x%X, got 0x%X", frame.ID, receivedFrame.ID)
	}

	if receivedFrame.DLC != frame.DLC {
		t.Errorf("Frame DLC mismatch. Expected %d, got %d", frame.DLC, receivedFrame.DLC)
	}

	for i := 0; i < int(frame.DLC); i++ {
		if receivedFrame.Data[i] != frame.Data[i] {
			t.Errorf("Frame data mismatch at index %d. Expected 0x%X, got 0x%X", i, frame.Data[i], receivedFrame.Data[i])
		}
	}

	// Verify that an ACK was written to the port
	writtenData := mockPort.GetWrittenData()
	if len(writtenData) == 0 {
		t.Errorf("No data was written to the port")
	} else {
		if writtenData[0] != ACK {
			t.Errorf("Expected ACK to be written, got 0x%X", writtenData[0])
		}
	}
}

// Test for checksum mismatch
func TestReadCanBusFrame_ChecksumMismatch(t *testing.T) {
	// Create a mock serial port
	mockPort := &MockSerialPort{}

	// Create an ArduinoDriver with the mock port
	driver := &ArduinoDriver{
		port: mockPort,
	}

	// Prepare a frame to read
	frame := canbus.Frame{
		ID:   0x123,
		DLC:  3,
		Data: [8]byte{0x01, 0x02, 0x03},
	}

	// Create the frame bytes as they would come over the serial port
	frameBytes := driver.createFrameBytes(frame)

	// Corrupt the checksum
	frameBytes[len(frameBytes)-2] ^= 0xFF // Flip some bits in checksum

	// Prepare the mock port with the corrupted frame bytes
	mockPort.FeedReadData(frameBytes)

	// Call ReadCanBusFrame
	receivedFrame, err := driver.ReadCanBusFrame()

	// We expect an error due to checksum mismatch
	if err == nil {
		t.Errorf("Expected error due to checksum mismatch, but got nil")
	}

	if err != nil && err.Error() != "error checksum mismatch" {
		t.Errorf("Expected checksum mismatch error, got: %v", err)
	}

	if receivedFrame != nil {
		t.Errorf("Expected received frame to be nil due to error, but got a frame")
	}

	// Verify that a NACK was written to the port
	writtenData := mockPort.GetWrittenData()
	if len(writtenData) == 0 {
		t.Errorf("No data was written to the port")
	} else {
		if writtenData[0] != NACK {
			t.Errorf("Expected NACK to be written, got 0x%X", writtenData[0])
		}
	}
}

// Test for SendCanBusFrame with data requiring byte stuffing
func TestSendCanBusFrame_WithByteStuffing(t *testing.T) {
	// Create a mock serial port
	mockPort := &MockSerialPort{}

	// Create an ArduinoDriver with the mock port
	driver := &ArduinoDriver{
		port: mockPort,
	}

	// Prepare a frame with data that includes special bytes
	frame := canbus.Frame{
		ID:   0x7E7F, // Include StartMarker and EndMarker in ID bytes
		DLC:  3,
		Data: [8]byte{StartMarker, EndMarker, EscapeChar},
	}

	// Prepare the mock port to respond with ACK
	mockPort.FeedReadData([]byte{ACK})

	// Call SendCanBusFrame
	err := driver.SendCanBusFrame(frame)
	if err != nil {
		t.Errorf("SendCanBusFrame returned error: %v", err)
	}

	// Verify that the correct bytes were written to the port
	writtenData := mockPort.GetWrittenData()

	// Manually construct expected data with proper byte stuffing
	expectedData := []byte{StartMarker}
	// ID bytes
	expectedData = append(expectedData, EscapeChar, 0x01) // StartMarker in ID high byte
	expectedData = append(expectedData, EscapeChar, 0x02) // EndMarker in ID low byte
	// DLC
	expectedData = append(expectedData, frame.DLC)
	// Data bytes with byte stuffing
	expectedData = append(expectedData, EscapeChar, 0x01) // StartMarker
	expectedData = append(expectedData, EscapeChar, 0x02) // EndMarker
	expectedData = append(expectedData, EscapeChar, 0x03) // EscapeChar
	// Checksum
	checksum := calculateCRC8(&frame)
	// Check if checksum needs to be escaped
	switch checksum {
	case StartMarker:
		expectedData = append(expectedData, EscapeChar, 0x01)
	case EndMarker:
		expectedData = append(expectedData, EscapeChar, 0x02)
	case EscapeChar:
		expectedData = append(expectedData, EscapeChar, 0x03)
	default:
		expectedData = append(expectedData, checksum)
	}
	// EndMarker
	expectedData = append(expectedData, EndMarker)

	if len(writtenData) != len(expectedData) {
		t.Errorf("Written data length mismatch. Expected %d, got %d", len(expectedData), len(writtenData))
	}

	for i := range expectedData {
		if writtenData[i] != expectedData[i] {
			t.Errorf("Written data mismatch at index %d. Expected 0x%X, got 0x%X", i, expectedData[i], writtenData[i])
		}
	}
}

// Test for ReadCanBusFrame with data requiring byte unstuffing
func TestReadCanBusFrame_WithByteUnstuffing(t *testing.T) {
	// Create a mock serial port
	mockPort := &MockSerialPort{}

	// Create an ArduinoDriver with the mock port
	driver := &ArduinoDriver{
		port: mockPort,
	}

	// Prepare a frame with data that includes special bytes
	frame := canbus.Frame{
		ID:   0x7E7F, // Include StartMarker and EndMarker in ID bytes
		DLC:  3,
		Data: [8]byte{StartMarker, EndMarker, EscapeChar},
	}

	// Manually construct frame bytes with proper byte stuffing
	frameBytes := []byte{StartMarker}
	// ID bytes
	frameBytes = append(frameBytes, EscapeChar, 0x01) // StartMarker in ID high byte
	frameBytes = append(frameBytes, EscapeChar, 0x02) // EndMarker in ID low byte
	// DLC
	frameBytes = append(frameBytes, frame.DLC)
	// Data bytes with byte stuffing
	frameBytes = append(frameBytes, EscapeChar, 0x01) // StartMarker
	frameBytes = append(frameBytes, EscapeChar, 0x02) // EndMarker
	frameBytes = append(frameBytes, EscapeChar, 0x03) // EscapeChar
	// Checksum
	checksum := calculateCRC8(&frame)
	// Check if checksum needs to be escaped
	switch checksum {
	case StartMarker:
		frameBytes = append(frameBytes, EscapeChar, 0x01)
	case EndMarker:
		frameBytes = append(frameBytes, EscapeChar, 0x02)
	case EscapeChar:
		frameBytes = append(frameBytes, EscapeChar, 0x03)
	default:
		frameBytes = append(frameBytes, checksum)
	}
	// EndMarker
	frameBytes = append(frameBytes, EndMarker)

	// Prepare the mock port with the frame bytes
	mockPort.FeedReadData(frameBytes)

	// Call ReadCanBusFrame
	receivedFrame, err := driver.ReadCanBusFrame()
	if err != nil {
		t.Errorf("ReadCanBusFrame returned error: %v", err)
	}

	if receivedFrame == nil {
		t.Errorf("ReadCanBusFrame returned nil frame")
	}

	// Verify the received frame matches the original frame
	if receivedFrame.ID != frame.ID {
		t.Errorf("Frame ID mismatch. Expected 0x%X, got 0x%X", frame.ID, receivedFrame.ID)
	}

	if receivedFrame.DLC != frame.DLC {
		t.Errorf("Frame DLC mismatch. Expected %d, got %d", frame.DLC, receivedFrame.DLC)
	}

	for i := 0; i < int(frame.DLC); i++ {
		if receivedFrame.Data[i] != frame.Data[i] {
			t.Errorf("Frame data mismatch at index %d. Expected 0x%X, got 0x%X", i, frame.Data[i], receivedFrame.Data[i])
		}
	}

	// Verify that an ACK was written to the port
	writtenData := mockPort.GetWrittenData()
	if len(writtenData) == 0 {
		t.Errorf("No data was written to the port")
	} else {
		if writtenData[0] != ACK {
			t.Errorf("Expected ACK to be written, got 0x%X", writtenData[0])
		}
	}
}

// Test for Round-trip of frames requiring byte stuffing
func TestSendAndReceiveFrame_WithByteStuffing(t *testing.T) {
	// Create a mock serial port
	mockPort := &MockSerialPort{}

	// Create an ArduinoDriver with the mock port
	driver := &ArduinoDriver{
		port: mockPort,
	}

	// Prepare a frame with data that includes special bytes
	frame := canbus.Frame{
		ID:   0x1B7E, // Include EscapeChar and StartMarker in ID bytes
		DLC:  4,
		Data: [8]byte{EscapeChar, StartMarker, EndMarker, 0x00},
	}

	// Prepare the mock port to respond with ACK after sending
	mockPort.FeedReadData([]byte{ACK})

	// Simulate that the device sends back the same frame
	frameBytes := driver.createFrameBytes(frame)
	mockPort.FeedReadData(frameBytes)

	// Call SendCanBusFrame
	err := driver.SendCanBusFrame(frame)
	if err != nil {
		t.Errorf("SendCanBusFrame returned error: %v", err)
	}

	// Call ReadCanBusFrame
	receivedFrame, err := driver.ReadCanBusFrame()
	if err != nil {
		t.Errorf("ReadCanBusFrame returned error: %v", err)
	}

	// Verify the received frame matches the sent frame
	if receivedFrame.ID != frame.ID {
		t.Errorf("Frame ID mismatch. Expected 0x%X, got 0x%X", frame.ID, receivedFrame.ID)
	}

	if receivedFrame.DLC != frame.DLC {
		t.Errorf("Frame DLC mismatch. Expected %d, got %d", frame.DLC, receivedFrame.DLC)
	}

	for i := 0; i < int(frame.DLC); i++ {
		if receivedFrame.Data[i] != frame.Data[i] {
			t.Errorf("Frame data mismatch at index %d. Expected 0x%X, got 0x%X", i, frame.Data[i], receivedFrame.Data[i])
		}
	}
}
