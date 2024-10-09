package drivers

import (
	"fmt"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"husk/canbus"
	"husk/utils"
	"log"
	"strconv"
	"strings"
)

const (
	BaudRate         = 115200
	SerialBufferSize = 4100
)

type ArduinoDriver struct {
	port   serial.Port
	buffer []byte
}

func NewArduinoDriver() (d Driver, err error) {
	arduinoDriver := new(ArduinoDriver)

	// find Arduino port
	portName, err := findArduinoPortName()
	if err != nil {
		return nil, err // Return nil and the error, ensuring invalid driver isn't returned
	}

	// open serial port
	mode := &serial.Mode{BaudRate: BaudRate}
	arduinoDriver.port, err = serial.Open(portName, mode)
	if err != nil {
		return nil, err
	}

	// allocate serial buffer
	arduinoDriver.buffer = make([]byte, SerialBufferSize)

	return arduinoDriver, nil
}

func findArduinoPortName() (string, error) {
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return "", err
	}

	// Loop through all ports and find the first match with the given VID
	for _, port := range ports {
		if port.IsUSB {
			if port.VID == "2341" || port.VID == "1A86" || port.VID == "2A03" {
				return port.Name, nil
			}
		}
	}

	return "", fmt.Errorf("error: no Arduino found on the USB ports")
}

func (d *ArduinoDriver) Cleanup() {
	if d.port != nil {
		err := d.port.Close()
		if err != nil {
			log.Fatalf("error: failed to close Arduino serial port: %v", err)
		}
	}
}

func (d *ArduinoDriver) SendCanBusFrame(frame canbus.Frame) error {
	if d.port == nil {
		return fmt.Errorf("error: Arduino port is not available")
	}

	// Convert the CAN ID to a string
	canID := fmt.Sprintf("%03X", frame.ID) // Format as a 3-character hexadecimal string

	// Convert the Data to a hex string, only using the number of bytes specified by DLC
	dataHex := ""
	for i := 0; i < int(frame.DLC); i++ {
		dataHex += fmt.Sprintf("%02X", frame.Data[i]) // Each byte as two hexadecimal characters
	}

	// Create the final string in the format "CANID DATAHEX"
	frameString := canID + " " + dataHex

	// Send the frame string over the serial port
	_, err := d.port.Write([]byte(frameString))
	if err != nil {
		return err
	}

	return nil
}

func (d *ArduinoDriver) ReadCanBusFrame() (*canbus.Frame, error) {
	if d.port == nil {
		return nil, fmt.Errorf("error: Arduino port is not available")
	}

	n, err := d.port.Read(d.buffer)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}

	input := string(d.buffer[:n])
	// Don't read if message is not complete
	if !strings.Contains(input, "\n") {
		return nil, nil
	}

	// Clean all newline etc
	cleaned := strings.ReplaceAll(input, "\n", "")
	cleaned = strings.ReplaceAll(cleaned, "\r", "")

	// Split into two parts (CAN ID and data)
	parts := strings.SplitN(cleaned, " ", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("error: incorrect serial dataframe format")
	}

	// Parse CAN ID from the first part
	id, err := strconv.ParseUint(parts[0], 16, 16)
	if err != nil {
		return nil, fmt.Errorf("error: failed to parse CAN ID: %v", err)
	}

	// Parse the data part
	dataHex := parts[1]
	data, err := utils.HexStringToBytes(dataHex)
	if err != nil {
		return nil, fmt.Errorf("error: failed to parse data bytes: %v", err)
	}

	// Create a new canbus.Frame object and populate it
	frame := &canbus.Frame{
		ID:  uint16(id),
		DLC: uint16(len(data)),
	}

	// Copy the parsed data into the frame's Data array
	copy(frame.Data[:], data)

	return frame, nil
}
