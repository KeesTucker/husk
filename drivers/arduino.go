package drivers

import (
	"bufio"
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
	BaudRate = 115200
)

type ArduinoDriver struct {
	port   serial.Port
	reader *bufio.Reader
}

func NewArduinoDriver() (d Driver, err error) {
	arduinoDriver := new(ArduinoDriver)

	// Find Arduino port
	portName, err := findArduinoPortName()
	if err != nil {
		return nil, err
	}

	// Open serial port
	mode := &serial.Mode{BaudRate: BaudRate}
	arduinoDriver.port, err = serial.Open(portName, mode)
	if err != nil {
		return nil, err
	}

	// Create reader
	arduinoDriver.reader = bufio.NewReader(arduinoDriver.port)

	return arduinoDriver, nil
}

func findArduinoPortName() (string, error) {
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return "", err
	}

	// Find the first matching USB port
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
	canID := fmt.Sprintf("%03X", frame.ID)

	// Convert the Data to a hex string, only using the number of bytes specified by DLC
	dataHex := ""
	for i := 0; i < int(frame.DLC); i++ {
		dataHex += fmt.Sprintf("%02X", frame.Data[i])
	}

	// Create the final string in the format "CANID DATAHEX"
	frameString := fmt.Sprintf("%s:%s\n", canID, dataHex)

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

	// Read the entire message until '\n'
	input, err := d.reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	// Clean newline characters and trim the input
	cleaned := strings.TrimSpace(input)

	// Split into two parts (CAN ID and data)
	parts := strings.SplitN(cleaned, ":", 2)
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
		DLC: uint8(len(data)),
	}

	// Copy the parsed data into the frame's Data array
	copy(frame.Data[:], data)

	return frame, nil
}
