package uds

import (
	"fmt"
)

var dtcMap = map[string]string{
	"0105": "Manifold Absolute Pressure/Barometric Pressure Circuit Malfunction",
	"0110": "Intake Air Temperature Circuit Malfunction",
	"0115": "Engine Coolant Temperature Circuit Malfunction",
	"0120": "Throttle Pedal Position Sensor/Switch A Circuit Malfunction",
	"0220": "Throttle/Pedal Position Sensor/Switch B Circuit Malfunction",
	"0708": "Transmission Range Sensor Circuit High Input",
	"1590": "SideStand Sensor Error",
	"1632": "Module Supply Voltage Out Of Range",
	"1685": "Metering Oil Pump Malfunction",
	"2120": "Throttle/Pedal Pos Sensor/Switch D Circuit",
	"2125": "Throttle/Pedal Pos Sensor/Switch E Circuit",
	"2226": "Barometric Pressure Circuit",
	"2803": "Transmission Range Sensor B Circuit High",

	// Common DTC Codes
	"0001": "Fuel Volume Regulator Control Circuit/Open",
	"0002": "Fuel Volume Regulator Control Circuit Range/Performance",
	"0003": "Fuel Volume Regulator Control Circuit Low",
	"0004": "Fuel Volume Regulator Control Circuit High",
	"0100": "Mass or Volume Air Flow Circuit Malfunction",
	"0101": "Mass or Volume Air Flow Circuit Range/Performance Problem",
	"0102": "Mass or Volume Air Flow Circuit Low Input",
	"0103": "Mass or Volume Air Flow Circuit High Input",
	"0112": "Intake Air Temperature Sensor 1 Circuit Low Input",
	"0113": "Intake Air Temperature Sensor 1 Circuit High Input",
	"0201": "Injector Circuit Malfunction - Cylinder 1",
	"0202": "Injector Circuit Malfunction - Cylinder 2",
	"0300": "Random/Multiple Cylinder Misfire Detected",
	"0301": "Cylinder 1 Misfire Detected",
	"0302": "Cylinder 2 Misfire Detected",
	"0303": "Cylinder 3 Misfire Detected",
	"0304": "Cylinder 4 Misfire Detected",
	"0401": "Exhaust Gas Recirculation (EGR) Flow Insufficient Detected",
	"0402": "Exhaust Gas Recirculation (EGR) Flow Excessive Detected",
	"0420": "Catalyst System Efficiency Below Threshold (Bank 1)",
	"0430": "Catalyst System Efficiency Below Threshold (Bank 2)",
	"0440": "Evaporative Emission Control System Malfunction",
	"0441": "Evaporative Emission Control System Incorrect Purge Flow",
	"0442": "Evaporative Emission Control System Leak Detected (small leak)",
	"0446": "Evaporative Emission Control System Vent Control Circuit Malfunction",
	"0500": "Vehicle Speed Sensor Malfunction",
	"0562": "System Voltage Low",
	"0563": "System Voltage High",
	"0600": "Serial Communication Link Malfunction",
	"0705": "Transmission Range Sensor Circuit Malfunction (PRNDL Input)",
	"0715": "Input/Turbine Speed Sensor Circuit Malfunction",
	"0720": "Output Speed Sensor Circuit Malfunction",
	"0730": "Incorrect Gear Ratio",
	"0740": "Torque Converter Clutch Circuit Malfunction",
	"0750": "Shift Solenoid A Malfunction",
	"0755": "Shift Solenoid B Malfunction",
	"0760": "Shift Solenoid C Malfunction",
	"0765": "Shift Solenoid D Malfunction",
	"0850": "Park/Neutral Position (PNP) Switch Circuit Malfunction",
	"1100": "Engine Coolant Temperature Sensor 1 Circuit Range/Performance",
	"1120": "Throttle Position Sensor/Switch Circuit Malfunction",
	"1130": "Throttle Position Sensor Circuit Malfunction",
	"1237": "Fuel Pump Secondary Circuit Malfunction",
	"1402": "EGR System - Insufficient Flow Detected",
	"1500": "Vehicle Speed Sensor A Malfunction",
}

func GetDTCLabel(dtcCode string) string {
	if label, exists := dtcMap[dtcCode]; exists {
		return fmt.Sprintf("%s: %s", dtcCode, label)
	}
	return dtcCode
}
